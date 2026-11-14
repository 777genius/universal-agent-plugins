package github

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/plugininstall/domain"
)

func requireBindTests(t *testing.T) {
	t.Helper()
	if os.Getenv("PLUGIN_KIT_AI_BIND_TESTS") == "1" {
		return
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("requires loopback bind support or PLUGIN_KIT_AI_BIND_TESTS=1: %v", err)
	}
	_ = ln.Close()
}

func TestClient_GetReleaseByTag_notFound(t *testing.T) {
	t.Parallel()
	requireBindTests(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)

	c := NewClient("")
	c.BaseURL = srv.URL
	c.APIClient = srv.Client()

	_, err := c.GetReleaseByTag(context.Background(), "o", "r", "v1")
	if err == nil {
		t.Fatal("expected error")
	}
	de, ok := err.(*domain.Error)
	if !ok || de.Code != domain.ExitRelease {
		t.Fatalf("got %v", err)
	}
}

func TestClient_GetReleaseByTag_forbidden(t *testing.T) {
	t.Parallel()
	requireBindTests(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	t.Cleanup(srv.Close)

	c := NewClient("")
	c.BaseURL = srv.URL
	c.APIClient = srv.Client()

	_, err := c.GetReleaseByTag(context.Background(), "o", "r", "v1")
	if err == nil {
		t.Fatal("expected error")
	}
	de, ok := err.(*domain.Error)
	if !ok || de.Code != domain.ExitNetwork {
		t.Fatalf("got %v", err)
	}
}

func TestClient_GetReleaseByTag_ok(t *testing.T) {
	t.Parallel()
	requireBindTests(t)
	payload := map[string]any{
		"tag_name":   "v1",
		"draft":      false,
		"prerelease": false,
		"assets":     []any{},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(payload)
	}))
	t.Cleanup(srv.Close)

	c := NewClient("")
	c.BaseURL = srv.URL
	c.APIClient = srv.Client()

	rel, err := c.GetReleaseByTag(context.Background(), "o", "r", "v1")
	if err != nil {
		t.Fatal(err)
	}
	if rel.TagName != "v1" {
		t.Fatalf("tag %q", rel.TagName)
	}
}

func TestClient_GetLatestRelease_ok(t *testing.T) {
	t.Parallel()
	requireBindTests(t)
	payload := map[string]any{
		"tag_name":   "v9.0.0",
		"draft":      false,
		"prerelease": false,
		"assets":     []any{},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/o/r/releases/latest" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(payload)
	}))
	t.Cleanup(srv.Close)

	c := NewClient("")
	c.BaseURL = srv.URL
	c.APIClient = srv.Client()

	rel, err := c.GetLatestRelease(context.Background(), "o", "r")
	if err != nil {
		t.Fatal(err)
	}
	if rel.TagName != "v9.0.0" {
		t.Fatalf("tag %q", rel.TagName)
	}
}

func TestClient_GetLatestRelease_notFound(t *testing.T) {
	t.Parallel()
	requireBindTests(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)

	c := NewClient("")
	c.BaseURL = srv.URL
	c.APIClient = srv.Client()

	_, err := c.GetLatestRelease(context.Background(), "o", "r")
	if err == nil {
		t.Fatal("expected error")
	}
	de, ok := err.(*domain.Error)
	if !ok || de.Code != domain.ExitRelease {
		t.Fatalf("got %v", err)
	}
}

func TestClient_GetReleaseByTag_responseTooLarge(t *testing.T) {
	t.Parallel()
	requireBindTests(t)
	const maxBytes int64 = 500
	huge := strings.Repeat("a", int(maxBytes)+1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(huge))
	}))
	t.Cleanup(srv.Close)

	c := NewClient("")
	c.BaseURL = srv.URL
	c.APIClient = srv.Client()
	c.ReleaseJSONMaxBytes = maxBytes

	_, err := c.GetReleaseByTag(context.Background(), "o", "r", "v1")
	if err == nil {
		t.Fatal("expected error")
	}
	de, ok := err.(*domain.Error)
	if !ok || de.Code != domain.ExitNetwork {
		t.Fatalf("got %v", err)
	}
	if !strings.Contains(de.Message, "exceeds") {
		t.Fatalf("message %q", de.Message)
	}
}

func TestClient_DownloadAsset_followsHTTPRedirect(t *testing.T) {
	t.Parallel()
	requireBindTests(t)
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/a":
			http.Redirect(w, r, srv.URL+"/b", http.StatusFound)
		case "/b":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("final-body"))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	c := NewClient("")
	c.DLClient = srv.Client()
	body, _, err := c.DownloadAsset(context.Background(), srv.URL+"/a")
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "final-body" {
		t.Fatalf("got %q", body)
	}
}

func TestClient_FindReleaseByTag_allowsDraft(t *testing.T) {
	t.Parallel()
	requireBindTests(t)
	payload := map[string]any{
		"id":         12,
		"tag_name":   "v1",
		"draft":      true,
		"prerelease": false,
		"upload_url": "https://uploads.example/releases/12/assets{?name,label}",
		"assets": []any{
			map[string]any{"id": 7, "name": "demo_bundle.tar.gz", "browser_download_url": "https://example/a", "size": 10},
		},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(payload)
	}))
	t.Cleanup(srv.Close)

	c := NewClient("")
	c.BaseURL = srv.URL
	c.APIClient = srv.Client()

	rel, err := c.FindReleaseByTag(context.Background(), "o", "r", "v1")
	if err != nil {
		t.Fatal(err)
	}
	if !rel.Draft || rel.ID != 12 || rel.UploadURL == "" {
		t.Fatalf("release = %#v", rel)
	}
	if len(rel.Assets) != 1 || rel.Assets[0].ID != 7 {
		t.Fatalf("assets = %#v", rel.Assets)
	}
}

func TestClient_CreateRelease_ok(t *testing.T) {
	t.Parallel()
	requireBindTests(t)
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/repos/o/r/releases" {
			http.NotFound(w, r)
			return
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if payload["tag_name"] != "v1" || payload["draft"] != false {
			t.Fatalf("payload = %#v", payload)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":         44,
			"tag_name":   "v1",
			"draft":      false,
			"prerelease": false,
			"upload_url": srv.URL + "/upload/assets{?name,label}",
			"assets":     []any{},
		})
	}))
	t.Cleanup(srv.Close)

	c := NewClient("")
	c.BaseURL = srv.URL
	c.APIClient = srv.Client()

	rel, err := c.CreateRelease(context.Background(), "o", "r", "v1", false)
	if err != nil {
		t.Fatal(err)
	}
	if rel.ID != 44 || rel.Draft {
		t.Fatalf("release = %#v", rel)
	}
}

func TestClient_CreateRelease_draftOk(t *testing.T) {
	t.Parallel()
	requireBindTests(t)
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/repos/o/r/releases" {
			http.NotFound(w, r)
			return
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if payload["tag_name"] != "v1" || payload["draft"] != true {
			t.Fatalf("payload = %#v", payload)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":         45,
			"tag_name":   "v1",
			"draft":      true,
			"prerelease": false,
			"upload_url": srv.URL + "/upload/assets{?name,label}",
			"assets":     []any{},
		})
	}))
	t.Cleanup(srv.Close)

	c := NewClient("")
	c.BaseURL = srv.URL
	c.APIClient = srv.Client()

	rel, err := c.CreateRelease(context.Background(), "o", "r", "v1", true)
	if err != nil {
		t.Fatal(err)
	}
	if rel.ID != 45 || !rel.Draft {
		t.Fatalf("release = %#v", rel)
	}
}

func TestClient_UpdateReleaseDraftState_ok(t *testing.T) {
	t.Parallel()
	requireBindTests(t)
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch || r.URL.Path != "/repos/o/r/releases/44" {
			http.NotFound(w, r)
			return
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if payload["draft"] != false {
			t.Fatalf("payload = %#v", payload)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":         44,
			"tag_name":   "v1",
			"draft":      false,
			"prerelease": false,
			"upload_url": srv.URL + "/upload/assets{?name,label}",
			"assets":     []any{},
		})
	}))
	t.Cleanup(srv.Close)

	c := NewClient("")
	c.BaseURL = srv.URL
	c.APIClient = srv.Client()

	rel, err := c.UpdateReleaseDraftState(context.Background(), "o", "r", 44, false)
	if err != nil {
		t.Fatal(err)
	}
	if rel.ID != 44 || rel.Draft {
		t.Fatalf("release = %#v", rel)
	}
}

func TestClient_UploadAndDeleteReleaseAsset_ok(t *testing.T) {
	t.Parallel()
	requireBindTests(t)
	var uploadedName string
	var uploadedBody string
	var deletedID int64
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/upload/assets":
			uploadedName = r.URL.Query().Get("name")
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatal(err)
			}
			uploadedBody = string(body)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":                   91,
				"name":                 uploadedName,
				"browser_download_url": srv.URL + "/download/" + uploadedName,
				"size":                 len(body),
			})
		case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/repos/o/r/releases/assets/"):
			id, err := strconv.ParseInt(strings.TrimPrefix(r.URL.Path, "/repos/o/r/releases/assets/"), 10, 64)
			if err != nil {
				t.Fatal(err)
			}
			deletedID = id
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	c := NewClient("")
	c.BaseURL = srv.URL
	c.APIClient = srv.Client()

	asset, err := c.UploadReleaseAsset(context.Background(), srv.URL+"/upload/assets{?name,label}", "demo_bundle.tar.gz", []byte("payload"), "application/gzip")
	if err != nil {
		t.Fatal(err)
	}
	if uploadedName != "demo_bundle.tar.gz" || uploadedBody != "payload" {
		t.Fatalf("upload name=%q body=%q", uploadedName, uploadedBody)
	}
	if asset.ID != 91 || asset.Name != "demo_bundle.tar.gz" {
		t.Fatalf("asset = %#v", asset)
	}
	if err := c.DeleteReleaseAsset(context.Background(), "o", "r", 91); err != nil {
		t.Fatal(err)
	}
	if deletedID != 91 {
		t.Fatalf("deletedID = %d", deletedID)
	}
}
