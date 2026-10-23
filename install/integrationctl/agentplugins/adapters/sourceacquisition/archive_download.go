package sourceacquisition

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/sourcespec"
)

const (
	archiveMaxBytes = int64(256 << 20)
	archiveTimeout  = 2 * time.Minute
)

type archiveFormat uint8

const (
	archiveUnknown archiveFormat = iota
	archiveZIP
	archiveTAR
	archiveGzip
)

// archiveMagic commits the caller to archive semantics even if the rest of the
// stream is corrupt. Content-Type and a server's filename are not authoritative.
func archiveMagic(prefix []byte) archiveFormat {
	if len(prefix) >= 4 && string(prefix[:2]) == "PK" {
		switch string(prefix[2:4]) {
		case "\x03\x04", "\x05\x06", "\x06\x06", "\x07\x08":
			return archiveZIP
		}
	}
	if len(prefix) >= 2 && prefix[0] == 0x1f && prefix[1] == 0x8b {
		return archiveGzip
	}
	if len(prefix) >= 512 {
		if string(prefix[257:262]) == "ustar" || archiveZero(prefix[:512]) {
			return archiveTAR
		}
		// V7 TAR has no magic; its checksum is a positive recognition signal.
		want, err := archiveTarNumber(prefix[148:156])
		var sum int64
		for i, value := range prefix[:512] {
			if i >= 148 && i < 156 {
				value = ' '
			}
			sum += int64(value)
		}
		if err == nil && want == sum {
			return archiveTAR
		}
	}
	return archiveUnknown
}

// archiveDownload writes at most one retained byte stream. Nonarchive HTTP
// responses can signal native Git fallback; every positively recognized archive
// failure remains an archive error, including oversized/truncated bodies.
func (acquirer Acquirer) archiveDownload(ctx context.Context, endpoint, destination string, explicit bool) (archiveFormat, string, error) {
	var body io.ReadCloser
	var length int64 = -1
	if strings.HasPrefix(endpoint, "https://") {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return archiveUnknown, "", errors.New("create archive download request")
		}
		request.Header.Set("Accept-Encoding", "identity")
		response, err := acquirer.archiveHTTPClient().Do(request)
		if err != nil {
			if ctx.Err() != nil {
				return archiveUnknown, "", ctx.Err()
			}
			return archiveUnknown, "", errors.New("archive HTTPS request failed (check TLS, redirects and connectivity)")
		}
		body, length = response.Body, response.ContentLength
		defer body.Close()
		if response.StatusCode != http.StatusOK {
			if !explicit {
				return archiveUnknown, "", ErrNotArchive
			}
			return archiveUnknown, "", fmt.Errorf("archive download returned HTTP %d", response.StatusCode)
		}
		if response.Header.Get("Content-Encoding") != "" && response.Header.Get("Content-Encoding") != "identity" {
			return archiveUnknown, "", errors.New("archive download refuses HTTP content encoding")
		}
	} else {
		file, err := os.Open(endpoint)
		if err != nil {
			return archiveUnknown, "", errors.New("open local archive")
		}
		body = file
		defer body.Close()
		info, err := file.Stat()
		if err != nil || !info.Mode().IsRegular() {
			return archiveUnknown, "", errors.New("local archive must be a regular file")
		}
		length = info.Size()
	}
	reader := bufio.NewReader(&archiveContextReader{ctx: ctx, reader: body})
	prefix, peekErr := reader.Peek(512)
	format := archiveMagic(prefix)
	if format == archiveUnknown {
		if peekErr != nil && !errors.Is(peekErr, io.EOF) && !errors.Is(peekErr, bufio.ErrBufferFull) {
			return format, "", errors.New("read archive signature")
		}
		if !explicit {
			return format, "", ErrNotArchive
		}
		return format, "", errors.New("source is not a supported ZIP, TAR or gzip TAR archive")
	}
	if length > archiveMaxBytes {
		return format, "", errors.New("archive download exceeds 256 MiB")
	}
	file, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return format, "", errors.New("create private archive download")
	}
	hash := sha256.New()
	n, copyErr := io.Copy(io.MultiWriter(file, hash), io.LimitReader(reader, archiveMaxBytes+1))
	closeErr := file.Close()
	if n > archiveMaxBytes {
		return format, "", errors.New("archive download exceeds 256 MiB")
	}
	if copyErr != nil || closeErr != nil {
		if ctx.Err() != nil {
			return format, "", ctx.Err()
		}
		return format, "", errors.New("archive download was incomplete")
	}
	return format, hex.EncodeToString(hash.Sum(nil)), nil
}

func (acquirer Acquirer) archiveHTTPClient() *http.Client {
	// Retain a host-supplied CA/proxy transport, without inheriting cookies,
	// authentication callbacks, redirect exceptions or unbounded timeouts.
	client := &http.Client{Timeout: archiveTimeout}
	if acquirer.HTTPClient != nil {
		client.Transport = acquirer.HTTPClient.Transport
		if timeout := acquirer.HTTPClient.Timeout; timeout > 0 && timeout < client.Timeout {
			client.Timeout = timeout
		}
	}
	client.CheckRedirect = func(request *http.Request, via []*http.Request) error {
		if len(via) > 5 {
			return errors.New("archive redirect limit exceeded")
		}
		if _, err := sourcespec.NormalizeArchiveURL(request.URL.String()); err != nil {
			return errors.New("archive redirects require credential-free HTTPS URLs")
		}
		request.Header = make(http.Header)
		request.Header.Set("Accept-Encoding", "identity")
		return nil
	}
	return client
}

type archiveContextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (reader *archiveContextReader) Read(body []byte) (int, error) {
	if err := reader.ctx.Err(); err != nil {
		return 0, err
	}
	return reader.reader.Read(body)
}

func archiveExplicit(endpoint string) bool {
	u, err := url.Parse(endpoint)
	return err == nil && sourcespec.HasArchiveSuffix(u.Path)
}
