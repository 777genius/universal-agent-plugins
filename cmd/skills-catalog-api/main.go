// skills-catalog-api serves a bounded, cached anonymous skills catalog.
package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
	"unicode"

	"github.com/777genius/plugin-kit-ai/internal/skillscatalog"
	"github.com/777genius/plugin-kit-ai/internal/skillsresolve"
	"github.com/777genius/plugin-kit-ai/internal/skillsresolve/gitsource"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/sourceacquisition"
)

func env(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}
func run() error {
	listen := flag.String("listen", env("SKILLS_CATALOG_LISTEN", "127.0.0.1:13120"), "HTTP listen address")
	cacheDir := flag.String("cache-dir", env("SKILLS_CATALOG_CACHE_DIR", ".cache/skills-catalog"), "Private persistent cache directory")
	origins := flag.String("origins", env("SKILLS_CATALOG_ORIGINS", "https://universal-agent-plugins.com,https://www.universal-agent-plugins.com"), "Comma-separated exact CORS origins; configure localhost explicitly for development")
	flag.Parse()
	service, err := skillscatalog.NewService(skillscatalog.NewSkillsSH(), *cacheDir)
	if err != nil {
		return err
	}
	configured := []string{}
	for _, origin := range strings.Split(*origins, ",") {
		if origin = strings.TrimSpace(origin); origin != "" {
			configured = append(configured, origin)
		}
	}
	handler, err := skillscatalog.NewHandler(service, configured)
	if err != nil {
		return err
	}
	serviceResolver, protected := resolveHandlers()
	documents, err := skillscatalog.NewDocumentHandler(serviceResolver,
		sourceacquisition.Acquirer{HTTPClient: skillscatalog.SafePublisherClient()}, configured)
	if err != nil {
		return err
	}
	mux := http.NewServeMux()
	mux.Handle("/v1/skills/resolve", protected)
	mux.Handle("/v1/skills/document", documents)
	mux.Handle("/", handler)
	server := &http.Server{Addr: *listen, Handler: mux, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 35 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 16 << 10}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdown)
	}()
	log.Printf("Skills catalog listening on %s", *listen)
	if err = server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func resolveHandlers() (skillscatalog.SourceResolver, http.Handler) {
	token := os.Getenv("SKILLS_RESOLVER_SERVICE_TOKEN")
	_ = os.Unsetenv("SKILLS_RESOLVER_SERVICE_TOKEN") // Git children never need the service capability.
	if len(token) < 32 || len(token) > 512 ||
		strings.TrimSpace(token) != token || strings.ContainsFunc(token, unicode.IsControl) {
		// The hosted Git reader and public Git previews share the explicitly
		// provisioned, validated resolver capability. Invalid credentials disable both.
		return nil, skillscatalog.NewResolveHandler(nil, "")
	}
	source, err := gitsource.NewHosted(os.Getenv("SKILLS_RESOLVER_TEMP_DIR"))
	if err != nil {
		log.Print("Skill Git preview unavailable: bounded runtime is not configured")
		return nil, skillscatalog.NewResolveHandler(nil, "")
	}
	resolver := skillsresolve.New(source)
	return resolver, skillscatalog.NewResolveHandler(resolver, token)
}
