package packageview

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
)

func (l *Lease) legacy() (State, error) {
	p, e := l.source.pin("plugin/plugin.yaml", false)
	if e != nil {
		return stateOf(e), nil
	}
	defer p.file.Close()
	if l.source.legacyInfo == nil {
		l.source.legacyInfo = p.info
	}
	if p.info.Mode().IsRegular() {
		return Present, nil
	}
	return WrongKind, nil
}

// legacyGuard checks both initial and current exact sentinel identity without
// opening data. Hardlinks are withheld, even when the sentinel is absent, so a
// canonical hardlink alias cannot be hidden by moving its directory entry.
func (l *Lease) legacyGuard(info os.FileInfo) bool {
	if multipleLinks(info) {
		return false
	}
	if l.source.legacyInfo != nil && os.SameFile(info, l.source.legacyInfo) {
		return false
	}
	p, e := l.source.pin("plugin/plugin.yaml", false)
	if e != nil {
		return stateOf(e) == Absent || stateOf(e) == WrongKind
	}
	defer p.file.Close()
	return !os.SameFile(info, p.info)
}
func (l *Lease) metadata(rel string, nofollow bool) (*pinned, error) {
	if l.hooks != nil && l.hooks.metadata != nil {
		if e := l.hooks.metadata(rel); e != nil {
			return nil, e
		}
	}
	return l.source.pin(rel, nofollow)
}
func (l *Lease) document(ctx context.Context, rel string, limit int64) (Document, error) {
	d := Document{Path: rel, State: Blocked}
	if e := contextError(ctx); e != nil {
		return d, e
	}
	p, e := l.metadata(rel, false)
	if e != nil {
		d.State = stateOf(e)
		if d.State != Absent {
			l.find("host", "document_"+string(d.State), rel)
		}
		return d, nil
	}
	defer p.file.Close()
	if !p.info.Mode().IsRegular() {
		d.State = WrongKind
		l.find("host", "document_wrong_kind", rel)
		return d, nil
	}
	if !l.legacyGuard(p.info) {
		l.find("host", "excluded_alias", rel)
		return d, nil
	}
	b, e := l.read(ctx, rel, p, min(limit, l.limits.DocumentBytes-l.documents))
	if e != nil {
		var safe *Error
		if errors.As(e, &safe) {
			return d, e
		}
		d.State = Unreadable
		l.find("host", "document_unreadable", rel)
		return d, nil
	}
	l.documents += int64(len(b))
	d.State = Present
	d.Bytes = append([]byte(nil), b...)
	return d, nil
}
func (l *Lease) read(ctx context.Context, rel string, p *pinned, limit int64) ([]byte, error) {
	if !p.info.Mode().IsRegular() {
		return nil, fail("wrong_kind")
	}
	if b, ok := l.contents[rel]; ok {
		if !same(l.observations[rel], p.info) {
			return nil, fail("source_changed")
		}
		if int64(len(b)) > limit {
			return nil, fail("byte_limit")
		}
		return b, nil
	}
	size := p.info.Size()
	if size < 0 || size > limit || size > l.limits.FileBytes || size > l.limits.TotalBytes-l.total {
		return nil, fail("byte_limit")
	}
	if l.hooks != nil && l.hooks.beforeDataOpen != nil {
		l.hooks.beforeDataOpen(rel)
	}
	// Recheck the name before data access; a replacement cannot be opened because
	// reopen uses the already-verified inode. Changes also invalidate the capture.
	current, e := l.source.pin(rel, false)
	if e != nil {
		return nil, fail("source_changed")
	}
	ok := same(p.info, current.info)
	ce := current.file.Close()
	if !ok {
		return nil, fail("source_changed")
	}
	if ce != nil {
		return nil, fail("close_failed")
	}
	if !l.legacyGuard(p.info) {
		return nil, fail("source_changed")
	}
	if e := contextError(ctx); e != nil {
		return nil, e
	}
	if l.hooks != nil && l.hooks.afterNameCheck != nil {
		l.hooks.afterNameCheck(rel)
	}
	if l.hooks != nil && l.hooks.dataOpenError != nil {
		if e := l.hooks.dataOpenError(rel); e != nil {
			return nil, e
		}
	}
	f, e := p.reopen(false)
	if e != nil {
		return nil, e
	}
	closed := false
	defer func() {
		if !closed {
			_ = f.Close()
		}
	}()
	// Allocate only after all length/aggregate checks. Read size+1 incrementally;
	// the extra byte detects growth without an unbounded ReadAll allocation.
	b := make([]byte, int(size)+1)
	n := 0
	for n < len(b) {
		if e := contextError(ctx); e != nil {
			return nil, e
		}
		k, re := f.Read(b[n:min(len(b), n+32*1024)])
		n += k
		if l.hooks != nil && l.hooks.afterChunk != nil {
			l.hooks.afterChunk(rel)
		}
		if re == io.EOF {
			break
		}
		if re != nil {
			return nil, re
		}
		if k == 0 {
			return nil, fail("read_stalled")
		}
	}
	if e := contextError(ctx); e != nil {
		return nil, e
	}
	after, e := f.Stat()
	if e != nil || int64(n) != size || !same(p.info, after) {
		return nil, fail("source_changed")
	}
	if e = verifyRead(ctx, f, b[:n]); e != nil {
		return nil, e
	}
	after, e = f.Stat()
	if e != nil || !same(p.info, after) {
		return nil, fail("source_changed")
	}
	ce = f.Close()
	closed = true
	if ce != nil {
		return nil, fail("close_failed")
	}
	current, e = l.source.pin(rel, false)
	if e != nil {
		return nil, fail("source_changed")
	}
	ok = same(p.info, current.info)
	ce = current.file.Close()
	if !ok || !l.legacyGuard(p.info) {
		return nil, fail("source_changed")
	}
	if ce != nil {
		return nil, fail("close_failed")
	}
	b = b[:n]
	// Numeric physical names support opaque/case-colliding logical paths without
	// creating them in private storage or treating portability as conformance.
	name := filepath.Join(l.private, strconv.Itoa(len(l.contents)))
	out, e := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if e != nil {
		return nil, fail("storage_failed")
	}
	_, we := out.Write(b)
	ce = out.Close()
	if we != nil || ce != nil {
		return nil, fail("storage_failed")
	}
	if e = os.Chmod(name, 0400); e != nil {
		return nil, fail("storage_failed")
	}
	l.total += size
	l.contents[rel] = b
	l.observations[rel] = p.info
	return b, nil
}

// verifyRead bounds a second pass to the captured length plus one. Metadata can
// be unchanged within one filesystem clock tick: require a second matching byte
// observation as well. This still does not certify an atomic revision against a
// writer coordinating changes between observations. Total source read work is
// at most three times the capture byte budget including final verification,
// plus EOF probes, with no second file-sized
// allocation. Only an already verified regular descriptor reaches Seek/Read.
func verifyRead(ctx context.Context, f *os.File, want []byte) error {
	if _, e := f.Seek(0, io.SeekStart); e != nil {
		return e
	}
	buf := make([]byte, min(32*1024, len(want)+1))
	n := 0
	for {
		if e := contextError(ctx); e != nil {
			return e
		}
		k, e := f.Read(buf[:min(len(buf), len(want)-n+1)])
		if k > len(want)-n || !bytes.Equal(buf[:k], want[n:n+k]) {
			return fail("source_changed")
		}
		n += k
		if e == io.EOF {
			if n != len(want) {
				return fail("source_changed")
			}
			return nil
		}
		if e != nil {
			return e
		}
		if k == 0 {
			return fail("read_stalled")
		}
	}
}

// Final verification also covers the interval between core capture and the
// caller-authorized component stage, even if metadata has the same clock tick.
func (l *Lease) verifyCaptured(ctx context.Context, rel string, p *pinned) error {
	want, ok := l.contents[rel]
	if !ok || !l.legacyGuard(p.info) {
		return fail("source_changed")
	}
	f, e := p.reopen(false)
	if e != nil {
		return fail("verification_failed")
	}
	e = verifyRead(ctx, f, want)
	after, se := f.Stat()
	ce := f.Close()
	if e != nil {
		var safe *Error
		if errors.As(e, &safe) {
			return e
		}
		return fail("verification_failed")
	}
	if se != nil || !same(p.info, after) {
		return fail("source_changed")
	}
	if ce != nil {
		return fail("close_failed")
	}
	current, e := l.source.pin(rel, false)
	if e != nil {
		return fail("source_changed")
	}
	ok = same(p.info, current.info)
	ce = current.file.Close()
	if !ok || !l.legacyGuard(p.info) {
		return fail("source_changed")
	}
	if ce != nil {
		return fail("close_failed")
	}
	return nil
}
