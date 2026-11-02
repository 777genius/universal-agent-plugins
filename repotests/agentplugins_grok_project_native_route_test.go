//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"context"
	"encoding/json"
	"mime"
	"net"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

type grokProjectRouteObservation struct {
	Method           string `json:"method"`
	URI              string `json:"uri"`
	PeerLoopback     bool   `json:"peer_loopback"`
	HostMatches      bool   `json:"host_matches"`
	RelativeURL      bool   `json:"relative_url"`
	EmptyURLHost     bool   `json:"empty_url_host"`
	EmptyRawPath     bool   `json:"empty_raw_path"`
	ContentTypeExact bool   `json:"content_type_exact"`
	ContentTypeCount string `json:"content_type_count"`
	ContentTypeKind  string `json:"content_type_kind"`
}

func (p *grokProjectProvider) failWithRequest(reason string, request *http.Request) {
	p.mu.Lock()
	if p.receipt.Failure == "none" {
		p.receipt.Failure = reason
		if reason == "route" && request != nil {
			observation := grokProjectObserveRoute(request, p.listener.Addr().String())
			p.receipt.RouteObservation = &observation
		}
	}
	p.mu.Unlock()
	p.cancel()
}

func grokProjectObserveRoute(request *http.Request, host string) grokProjectRouteObservation {
	observation := grokProjectRouteObservation{Method: "other", URI: "other"}
	switch request.Method {
	case http.MethodPost, http.MethodGet:
		observation.Method = request.Method
	}
	switch request.RequestURI {
	case "/", "/v1/chat/completions", "/chat/completions", "/v1/models", "/models", "/v1/responses", "/responses":
		observation.URI = request.RequestURI
	}
	peer, _, err := net.SplitHostPort(request.RemoteAddr)
	observation.PeerLoopback = err == nil && peer == "127.0.0.1"
	observation.HostMatches = request.Host == host
	if request.URL != nil {
		observation.RelativeURL = !request.URL.IsAbs()
		observation.EmptyURLHost = request.URL.Host == ""
		observation.EmptyRawPath = request.URL.RawPath == ""
	}
	values := request.Header.Values("Content-Type")
	observation.ContentTypeExact = len(values) == 1 && values[0] == "application/json"
	observation.ContentTypeCount, observation.ContentTypeKind = grokProjectObserveContentType(values)
	return observation
}

func grokProjectObserveContentType(values []string) (string, string) {
	if len(values) == 0 {
		return "none", "missing"
	}
	if len(values) != 1 {
		return "multiple", "other"
	}
	if len(values[0]) > 128 {
		return "one", "other"
	}
	media, parameters, err := mime.ParseMediaType(values[0])
	if err != nil || media != "application/json" {
		return "one", "other"
	}
	if len(parameters) == 0 {
		return "one", "json-bare"
	}
	if len(parameters) == 1 && strings.EqualFold(parameters["charset"], "utf-8") {
		return "one", "json-utf8"
	}
	return "one", "other"
}

func grokProjectRouteTestRequest(t *testing.T, method, uri string) *http.Request {
	t.Helper()
	request, err := http.NewRequestWithContext(context.Background(), method, uri, http.NoBody)
	if err != nil {
		t.Fatal(err)
	}
	request.RequestURI = uri
	return request
}

func TestGrokProjectRouteObservationIsFiniteAndNeverAdmitsMIMEVariants(t *testing.T) {
	for _, test := range []struct {
		values      []string
		count, kind string
		exact       bool
	}{
		{nil, "none", "missing", false},
		{[]string{"application/json"}, "one", "json-bare", true},
		{[]string{"Application/JSON"}, "one", "json-bare", false},
		{[]string{"application/json; charset=UTF-8"}, "one", "json-utf8", false},
		{[]string{"application/json; charset=ascii"}, "one", "other", false},
		{[]string{"application/json; private=secret"}, "one", "other", false},
		{[]string{"application/json; charset=utf-8; extra=secret"}, "one", "other", false},
		{[]string{"application/json", "application/json"}, "multiple", "other", false},
		{[]string{"application/json" + strings.Repeat(" ", 113)}, "one", "other", false},
		{[]string{""}, "one", "other", false},
	} {
		request := grokProjectRouteTestRequest(t, http.MethodPost, "/v1/chat/completions")
		request.Host, request.RemoteAddr = "127.0.0.1:12345", "127.0.0.1:12346"
		request.Header["Content-Type"] = test.values
		observation := grokProjectObserveRoute(request, request.Host)
		if observation.ContentTypeCount != test.count || observation.ContentTypeKind != test.kind || observation.ContentTypeExact != test.exact || grokProjectProviderRoute(request, request.Host) != test.exact {
			t.Fatal("diagnostic classification widened the unchanged literal route")
		}
	}
}

func TestGrokProjectRouteObservationKeepsOnlyClosedMethodPathAndPredicates(t *testing.T) {
	request := grokProjectRouteTestRequest(t, http.MethodPost, "/v1/chat/completions")
	request.Host, request.RemoteAddr = "127.0.0.1:12345", "127.0.0.1:12346"
	request.Header.Set("Content-Type", "application/json")
	want := grokProjectRouteObservation{Method: "POST", URI: "/v1/chat/completions", PeerLoopback: true, HostMatches: true,
		RelativeURL: true, EmptyURLHost: true, EmptyRawPath: true, ContentTypeExact: true, ContentTypeCount: "one", ContentTypeKind: "json-bare"}
	if got := grokProjectObserveRoute(request, request.Host); got != want {
		t.Fatal("literal route facts differ")
	}
	for _, uri := range []string{"/v1/chat/completions", "/chat/completions", "/v1/models", "/models", "/v1/responses", "/responses"} {
		request.RequestURI = uri
		if got := grokProjectObserveRoute(request, request.Host); got.URI != uri {
			t.Fatal("fixed route label differs")
		}
	}
	request.Method = http.MethodGet
	if grokProjectObserveRoute(request, request.Host).Method != "GET" {
		t.Fatal("fixed GET label differs")
	}
	request.Method, request.RequestURI = "PRIVATE-METHOD", "/private-path?credential=private-query"
	request.Host, request.RemoteAddr = "private-host", "192.0.2.1:12346"
	request.URL.Scheme, request.URL.Host, request.URL.RawPath = "private-scheme", "private-url-host", "/private-raw-path"
	request.Header.Set("Content-Type", "private-content-type")
	request.Header.Set("Authorization", "Bearer private-credential")
	got := grokProjectObserveRoute(request, "127.0.0.1:12345")
	want = grokProjectRouteObservation{Method: "other", URI: "other", ContentTypeCount: "one", ContentTypeKind: "other"}
	if got != want {
		t.Fatal("unknown route must expose only finite false predicates")
	}
	raw, err := json.Marshal(got)
	if err != nil || bytes.Contains(raw, []byte("private")) || len(raw) > 512 {
		t.Fatal("private route attribute escaped bounded observation")
	}
}

func TestGrokProjectRouteFailureRecordsOnlyTheFirstActualRejectedExchange(t *testing.T) {
	p := grokProjectContractProvider(t)
	r := grokProjectContractRequest(t, p, grokProjectContractBody(t, p))
	r.Header.Set("Content-Type", "application/json; charset=utf-8")
	status, _ := grokProjectContractSend(t, r)
	p.stop()
	if status != http.StatusBadRequest || p.receipt.Failure != "route" || p.receipt.RouteObservation == nil || p.receipt.Requests != 0 || p.receipt.Responses != 0 || p.receipt.CredentialMatched || p.request != nil {
		t.Fatal("refused real route changed request admission or omitted its observation")
	}
	want := *p.receipt.RouteObservation
	if want.ContentTypeKind != "json-utf8" || want.ContentTypeExact || !want.PeerLoopback || !want.HostMatches || !want.RelativeURL || !want.EmptyURLHost || !want.EmptyRawPath {
		t.Fatal("actual inert loopback refusal facts differ")
	}
	p.failWithRequest("route", grokProjectRouteTestRequest(t, http.MethodGet, "/models"))
	p.fail("cleanup")
	if p.receipt.Failure != "route" || !reflect.DeepEqual(*p.receipt.RouteObservation, want) {
		t.Fatal("later failures replaced the original route facts")
	}
}

func TestGrokProjectNonRouteFirstFailureCannotAcquireRouteMetadata(t *testing.T) {
	for _, reason := range []string{"listen", "serve", "deadline", "credential", "duplicate", "body", "request-shape", "response-write", "cleanup"} {
		p := grokProjectContractProvider(t)
		p.fail(reason)
		p.failWithRequest("route", grokProjectRouteTestRequest(t, http.MethodGet, "/models"))
		if p.receipt.Failure != reason || p.receipt.RouteObservation != nil {
			t.Fatal("later route failure replaced an earlier failure")
		}
		p.stop()
	}
}

func TestGrokProjectOriginGETIsObservedWhileAdmissionStillRefuses(t *testing.T) {
	p := grokProjectContractProvider(t)
	r := grokProjectContractRequest(t, p, grokProjectContractBody(t, p))
	r.Method, r.URL.Path = http.MethodGet, "/"
	r.Body, r.ContentLength = http.NoBody, 0
	r.Header.Del("Content-Type")
	r.Header.Set("Authorization", "Bearer private-origin-header")
	status, _ := grokProjectContractSend(t, r)
	p.stop()
	if status != http.StatusBadRequest || p.ctx.Err() == nil || p.receipt.Failure != "route" || p.receipt.Requests != 0 || p.receipt.Responses != 0 || p.receipt.CredentialMatched || p.request != nil || p.reply != nil {
		t.Fatal("origin diagnostic changed rejection, cancellation or provider admission")
	}
	want := grokProjectRouteObservation{Method: "GET", URI: "/", PeerLoopback: true, HostMatches: true,
		RelativeURL: true, EmptyURLHost: true, EmptyRawPath: true, ContentTypeCount: "none", ContentTypeKind: "missing"}
	if p.receipt.RouteObservation == nil || *p.receipt.RouteObservation != want {
		t.Fatal("actual inert rejected origin request lost its finite facts")
	}
	p.failWithRequest("route", grokProjectRouteTestRequest(t, http.MethodGet, "/models"))
	p.fail("cleanup")
	if p.receipt.Failure != "route" || *p.receipt.RouteObservation != want {
		t.Fatal("later failure replaced the first rejected origin request")
	}
	raw, err := json.Marshal(p.receipt.RouteObservation)
	if err != nil || len(raw) > 512 || bytes.Contains(raw, []byte("private")) {
		t.Fatal("origin observation exceeded bounds or exposed a private header")
	}
}

func TestGrokProjectOriginObservationDoesNotNormalizePrivateRootLikeURIs(t *testing.T) {
	for _, uri := range []string{"/?credential=private-query", "//", "/v1/", "/%2F", "http://private-host/", "/private-root"} {
		r := grokProjectRouteTestRequest(t, http.MethodGet, uri)
		r.Host, r.RemoteAddr = "127.0.0.1:12345", "127.0.0.1:12346"
		r.Header.Set("Authorization", "Bearer private-root-header")
		got := grokProjectObserveRoute(r, r.Host)
		if got.URI != "other" || grokProjectProviderRoute(r, r.Host) {
			t.Fatal("root-like request was normalized or admitted")
		}
		raw, err := json.Marshal(got)
		if err != nil || len(raw) > 512 || bytes.Contains(raw, []byte("private")) {
			t.Fatal("root-like request exposed private attributes")
		}
	}
}
