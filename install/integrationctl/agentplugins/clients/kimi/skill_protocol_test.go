package kimi

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

type kimiHTTPBoundary func(*http.Request) (*http.Response, error)

func (f kimiHTTPBoundary) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestKimiFiniteProtocolRejectsAggregateOverflowAndRepeatedIdentity(t *testing.T) {
	for _, condition := range []string{"aggregate-body-budget", "repeated-readiness", "repeated-header", "redirect", "duplicate-json", "refused-shutdown"} {
		t.Run(condition, func(t *testing.T) {
			f := newKimiCompletionFixture(t, "2.3.2")
			if condition == "aggregate-body-budget" {
				f.rows = []any{f.workspace()}
				f.change = func(request *http.Request, data map[string]any) {
					if strings.HasSuffix(request.URL.Path, "/meta") {
						data["padding"] = strings.Repeat("x", (1<<20)-700)
					}
				}
			}
			if condition == "repeated-readiness" {
				f.banner = "  native  Kimi server ready  2.3.2\n  Local:    http://127.0.0.1:12345/#token=" + strings.Repeat("A", 43) + "\n  Token:    " + strings.Repeat("A", 43) + "\n  Local:    http://127.0.0.1:12345/#token=" + strings.Repeat("A", 43) + "\n"
			}
			if condition == "repeated-header" {
				f.banner = "  native  Kimi server ready  2.3.2\n  Local:    http://127.0.0.1:12345/#token=" + strings.Repeat("A", 43) + "\n  Token:    " + strings.Repeat("A", 43) + "\n  native  Kimi server ready  2.3.2\n"
			}
			if condition == "refused-shutdown" {
				f.change = func(request *http.Request, data map[string]any) {
					if strings.HasSuffix(request.URL.Path, "/shutdown") {
						data["ok"] = false
					}
				}
			}
			f.env.NativeHTTP = kimiHTTPBoundary(func(request *http.Request) (*http.Response, error) {
				response, err := f.RoundTrip(request)
				if strings.HasSuffix(request.URL.Path, "/meta") {
					if condition == "redirect" {
						response.StatusCode = http.StatusFound
						response.Header.Set("Location", "https://foreign.invalid/steal")
					}
					if condition == "duplicate-json" {
						raw, readErr := io.ReadAll(response.Body)
						if readErr != nil || response.Body.Close() != nil {
							t.Fatal("valid metadata fixture could not be read")
						}
						// Preserve every valid server identity field; only duplicate code.
						body := strings.Replace(string(raw), `"code":0`, `"code":0,"code":0`, 1)
						response.Body = io.NopCloser(strings.NewReader(body))
					}
				}
				return response, err
			})
			outcome, err := New().Activate(t.Context(), f.env, f.request)
			if err != nil || outcome.Activation != domain.ActivationPrepared || f.queries != 1 {
				t.Fatal("unqualified finite exchange promoted or lost closed failure", outcome, err, f.calls)
			}
			if condition == "duplicate-json" {
				for _, call := range f.calls {
					if strings.HasSuffix(call, "/skills") {
						t.Fatal("duplicate-key metadata reached the Skills endpoint", f.calls)
					}
				}
			}
			if len(f.calls) == 0 || f.calls[len(f.calls)-1] != "POST /api/v1/shutdown" {
				t.Fatal("known authenticated origin did not receive ordinary shutdown", f.calls)
			}
		})
	}
}
