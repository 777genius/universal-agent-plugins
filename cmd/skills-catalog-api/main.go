// skills-catalog-api serves a bounded, cached anonymous skills catalog.
package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/777genius/plugin-kit-ai/internal/skillscatalog"
)

func env(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
func main() {
	listen := flag.String("listen", env("SKILLS_CATALOG_LISTEN", "127.0.0.1:13120"), "HTTP listen address")
	cacheDir := flag.String("cache-dir", env("SKILLS_CATALOG_CACHE_DIR", ".cache/skills-catalog"), "Private persistent cache directory")
	origins := flag.String("origins", env("SKILLS_CATALOG_ORIGINS", "https://universal-agent-plugins.com,https://www.universal-agent-plugins.com"), "Comma-separated exact CORS origins; configure localhost explicitly for development")
	flag.Parse()
	service, err := skillscatalog.NewService(skillscatalog.NewSkillsSH(), *cacheDir)
	if err != nil {
		log.Fatal(err)
	}
	configured := []string{}
	for _, origin := range strings.Split(*origins, ",") {
		if origin = strings.TrimSpace(origin); origin != "" {
			configured = append(configured, origin)
		}
	}
	handler, err := skillscatalog.NewHandler(service, configured)
	if err != nil {
		log.Fatal(err)
	}
	server := &http.Server{Addr: *listen, Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 16 << 10}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdown)
	}()
	log.Printf("Skills catalog listening on %s", *listen)
	if err = server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
