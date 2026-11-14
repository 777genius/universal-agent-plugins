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
	"unicode"

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
	body, length, err := acquirer.archiveOpen(ctx, endpoint, explicit)
	if err != nil {
		return archiveUnknown, "", err
	}
	defer func() { _ = body.Close() }()
	reader := bufio.NewReader(&archiveContextReader{ctx: ctx, reader: body})
	prefix, peekErr := reader.Peek(512)
	format := archiveMagic(prefix)
	if format == archiveUnknown {
		if ctx.Err() != nil {
			return format, "", ctx.Err()
		}
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
	if declared := archiveFilenameFormat(endpoint); declared != archiveUnknown && declared != format {
		return format, "", errors.New("archive format is incompatible with the initial filename suffix")
	}
	checksum, err := archiveSave(ctx, reader, destination)
	return format, checksum, err
}

func (acquirer Acquirer) archiveOpen(ctx context.Context, endpoint string, explicit bool) (io.ReadCloser, int64, error) {
	if strings.HasPrefix(endpoint, "https://") {
		return acquirer.archiveHTTP(ctx, endpoint, explicit)
	}
	file, err := os.Open(endpoint)
	if err != nil {
		return nil, 0, errors.New("open local archive")
	}
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		_ = file.Close()
		return nil, 0, errors.New("local archive must be a regular file")
	}
	return file, info.Size(), nil
}

func (acquirer Acquirer) archiveHTTP(ctx context.Context, endpoint string, explicit bool) (io.ReadCloser, int64, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, http.NoBody)
	if err != nil {
		return nil, 0, errors.New("create archive download request")
	}
	request.Header.Set("Accept-Encoding", "identity")
	client := acquirer.archiveHTTPClient()
	response, err := client.Do(request)
	if err != nil {
		client.CloseIdleConnections()
		if ctx.Err() != nil {
			return nil, 0, ctx.Err()
		}
		return nil, 0, errors.New("archive HTTPS request failed (check TLS, redirects and connectivity)")
	}
	if response.StatusCode != http.StatusOK {
		_ = response.Body.Close()
		client.CloseIdleConnections()
		if !explicit {
			return nil, 0, ErrNotArchive
		}
		return nil, 0, fmt.Errorf("archive download returned HTTP %d", response.StatusCode)
	}
	if response.Header.Get("Content-Encoding") != "" && response.Header.Get("Content-Encoding") != "identity" {
		_ = response.Body.Close()
		client.CloseIdleConnections()
		return nil, 0, errors.New("archive download refuses HTTP content encoding")
	}
	return &archiveHTTPBody{ReadCloser: response.Body, client: client}, response.ContentLength, nil
}

type archiveHTTPBody struct {
	io.ReadCloser
	client *http.Client
}

func (body *archiveHTTPBody) Close() error {
	err := body.ReadCloser.Close()
	body.client.CloseIdleConnections()
	return err
}

func archiveSave(ctx context.Context, reader io.Reader, destination string) (string, error) {
	file, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return "", errors.New("create private archive download")
	}
	hash := sha256.New()
	n, copyErr := io.Copy(io.MultiWriter(file, hash), io.LimitReader(reader, archiveMaxBytes+1))
	closeErr := file.Close()
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if n > archiveMaxBytes {
		return "", errors.New("archive download exceeds 256 MiB")
	}
	if copyErr != nil || closeErr != nil {
		return "", errors.New("archive download was incomplete")
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
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
	if client.Transport == nil {
		client.Transport = http.DefaultTransport
	}
	if transport, ok := client.Transport.(*http.Transport); ok {
		transport = transport.Clone()
		if transport.TLSClientConfig != nil {
			transport.TLSClientConfig = transport.TLSClientConfig.Clone()
			transport.TLSClientConfig.InsecureSkipVerify = false
		}
		client.Transport = transport
	}
	client.CheckRedirect = func(request *http.Request, via []*http.Request) error {
		if len(via) > 5 {
			return errors.New("archive redirect limit exceeded")
		}
		if err := archiveRedirectURL(request.URL); err != nil {
			return err
		}
		clear(request.Header)
		request.Header.Set("Accept-Encoding", "identity")
		return nil
	}
	return client
}

// Signed CDN coordinates exist only during this request. Initial intent stays
// query-free, and this validator deliberately preserves signed query encoding.
func archiveRedirectURL(target *url.URL) error {
	value := target.String()
	if len(value) > 16<<10 || target.Scheme != "https" || target.Hostname() == "" || target.User != nil || target.Fragment != "" || target.Opaque != "" || strings.ContainsRune(value, '\\') || strings.ContainsFunc(value, unicode.IsControl) || strings.ContainsFunc(target.Path, unicode.IsControl) {
		return errors.New("archive redirects require bounded HTTPS URLs without userinfo, fragments or controls")
	}
	return nil
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
	return err == nil && (sourcespec.HasArchiveSuffix(u.Path) || sourcespec.IsProviderArchiveURL(u))
}

func archiveFilenameFormat(endpoint string) archiveFormat {
	filename := endpoint
	if strings.HasPrefix(endpoint, "https://") {
		parsed, err := url.Parse(endpoint)
		if err != nil {
			return archiveUnknown
		}
		filename = parsed.Path
	}
	filename = strings.ToLower(filename)
	switch {
	case strings.HasSuffix(filename, ".tar.gz"), strings.HasSuffix(filename, ".tgz"):
		return archiveGzip
	case strings.HasSuffix(filename, ".zip"):
		return archiveZIP
	case strings.HasSuffix(filename, ".tar"):
		return archiveTAR
	default:
		return archiveUnknown
	}
}
