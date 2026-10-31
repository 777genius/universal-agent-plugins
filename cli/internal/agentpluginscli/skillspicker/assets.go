package skillspicker

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

//go:embed webdist
var bundled embed.FS

// Bundled reports whether the production UI build replaced the source placeholder.
func Bundled() bool { _, err := bundled.ReadFile("webdist/assets-manifest.json"); return err == nil }

func serveAssets(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	assets, _ := fs.Sub(bundled, "webdist")
	clean := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
	if clean == "" {
		clean = "index.html"
		if !Bundled() {
			clean = "placeholder.html"
		}
	}
	if !Bundled() && (clean == "index.html" || !strings.Contains(path.Base(clean), ".")) {
		clean = "placeholder.html"
	}
	if _, err := fs.Stat(assets, clean); err != nil {
		if strings.HasPrefix(clean, "assets/") || strings.Contains(path.Base(clean), ".") {
			http.NotFound(w, r)
			return
		}
		clean = "index.html"
		if !Bundled() {
			clean = "placeholder.html"
		}
	}
	request := r.Clone(r.Context())
	request.URL.Path = "/" + clean
	// FileServer canonicalizes /index.html to ./; serve the directory index
	// directly so root and SPA routes do not redirect back into this handler.
	if clean == "index.html" {
		request.URL.Path = "/"
	}
	http.FileServer(http.FS(assets)).ServeHTTP(w, request)
}
