package kimi

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

var kimiLocalReady = regexp.MustCompile(`^  Local:    (http://127\.0\.0\.1:([1-9][0-9]{0,4}))/#token=([A-Za-z0-9_-]{43})$`)
var kimiTokenReady = regexp.MustCompile(`^ {2}Token: {4}([A-Za-z0-9_-]{43})$`)

// The bearer belongs only to this finite exchange. Never include native output,
// HTTP bodies or transport diagnostics in a public error or persisted outcome.
type kimiObservationError struct{ cause error }

func (e kimiObservationError) Error() string {
	return "Kimi native Skills observation did not complete"
}
func (e kimiObservationError) Unwrap() error { return e.cause }

type kimiSkillAPI struct {
	ctx          context.Context
	transport    http.RoundTripper
	origin       string
	token        string
	remaining    int64
	registration domain.NativeEffectState
}

func observeKimiSkills(ctx context.Context, env clients.Env, request domain.ActivationRequest, command ports.Command, expected map[string]kimiSkillMetadata) (domain.NativeEffectState, shared.NativeFileSnapshot, error) {
	runner, ok := env.Runner.(ports.CompletionCommandRunner)
	if !ok || env.NativeHTTP == nil {
		return domain.NativeEffectUnchanged, shared.NativeFileSnapshot{}, fmt.Errorf("kimi finite native observation is unavailable")
	}
	registration := domain.NativeEffectUnchanged
	var token shared.NativeFileSnapshot
	err := runner.RunDuplexToCompletion(ctx, command, func(_ io.Writer, stdout io.Reader) error {
		api := kimiSkillAPI{ctx: ctx, transport: env.NativeHTTP, remaining: 1 << 20}
		scanner := bufio.NewScanner(&kimiBudgetReader{reader: stdout, remaining: &api.remaining})
		scanner.Buffer(make([]byte, 4096), 64*1024)
		if err := api.ready(scanner, request.Client.Version); err != nil {
			return err
		}
		issued, err := kimiIssuedToken(env, request.Client.ConfigRoot, api.token)
		if err != nil {
			return err
		}
		token = issued
		observed := api.catalog(request, expected)
		if api.registration != "" {
			registration = api.registration
		}
		// Shutdown is still attempted after a closed protocol failure. Only the
		// selected, authenticated process origin is ever addressed.
		shutdown, shutdownErr := api.call(http.MethodPost, "/shutdown", map[string]any{})
		if shutdownErr == nil && (len(shutdown) != 1 || shutdown["ok"] != true) {
			shutdownErr = fmt.Errorf("kimi shutdown was not acknowledged")
		}
		for scanner.Scan() {
			if strings.Contains(scanner.Text(), "Kimi server ready") || kimiLocalReady.MatchString(scanner.Text()) || kimiTokenReady.MatchString(scanner.Text()) {
				observed = errors.Join(observed, fmt.Errorf("kimi readiness identity was repeated"))
			}
		}
		_, tokenErr := kimiIssuedToken(env, request.Client.ConfigRoot, api.token)
		return errors.Join(observed, shutdownErr, scanner.Err(), ctx.Err(), tokenErr)
	})
	if err == nil {
		return registration, token, ctx.Err()
	}
	return registration, token, shared.CompletionProbeFailure(kimiObservationError{cause: err})
}

func (api *kimiSkillAPI) ready(scanner *bufio.Scanner, version string) error {
	header, local := false, false
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "Kimi server ready") {
			if header || !strings.HasSuffix(line, "Kimi server ready  "+version) {
				return fmt.Errorf("kimi readiness version differs")
			}
			header = true
		}
		if match := kimiLocalReady.FindStringSubmatch(line); match != nil {
			port, err := strconv.Atoi(match[2])
			if local || err != nil || port > 65535 {
				return fmt.Errorf("kimi readiness origin differs")
			}
			api.origin, api.token, local = match[1], match[3], true
		}
		if match := kimiTokenReady.FindStringSubmatch(line); match != nil {
			if !header || !local || match[1] != api.token {
				return fmt.Errorf("kimi readiness bearer differs")
			}
			return nil
		}
	}
	return errors.Join(fmt.Errorf("kimi readiness is unavailable"), scanner.Err(), api.ctx.Err())
}

func (api *kimiSkillAPI) call(method, path string, value map[string]any) (map[string]any, error) {
	var body io.Reader
	if value != nil {
		encoded, err := json.Marshal(value)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(encoded)
	}
	request, err := http.NewRequestWithContext(api.ctx, method, api.origin+"/api/v1"+path, body)
	if err != nil || request.URL.Scheme != "http" || request.URL.Hostname() != "127.0.0.1" || request.URL.User != nil || request.URL.RawQuery != "" || request.URL.Fragment != "" {
		return nil, fmt.Errorf("kimi native API origin differs")
	}
	request.Header.Set("Authorization", "Bearer "+api.token)
	request.Header.Set("Content-Type", "application/json")
	response, err := api.transport.RoundTrip(request)
	if err != nil {
		return nil, fmt.Errorf("kimi native API request failed")
	}
	if response == nil || response.Body == nil {
		return nil, fmt.Errorf("kimi native API response is unavailable")
	}
	raw, readErr := io.ReadAll(&kimiBudgetReader{reader: response.Body, remaining: &api.remaining})
	closeErr := response.Body.Close()
	if readErr != nil || closeErr != nil || response.StatusCode != http.StatusOK || response.Header.Get("Location") != "" {
		return nil, fmt.Errorf("kimi native API status or bound differs")
	}
	return kimiSkillAPIData(raw)
}

func kimiSkillAPIData(raw []byte) (map[string]any, error) {
	envelope, err := shared.DecodeStrictJSONObject(raw)
	if err != nil || envelope["code"] != json.Number("0") || envelope["msg"] != "success" || !kimiText(envelope["request_id"], 256, false) || envelope["details"] != nil || envelope["stack"] != nil {
		return nil, fmt.Errorf("kimi native API envelope differs")
	}
	data, ok := envelope["data"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("kimi native API data differs")
	}
	return data, nil
}

type kimiBudgetReader struct {
	reader    io.Reader
	remaining *int64
}

func (r *kimiBudgetReader) Read(buffer []byte) (int, error) {
	if *r.remaining < 0 {
		return 0, fmt.Errorf("kimi native observation exceeds output bound")
	}
	if int64(len(buffer)) > *r.remaining+1 {
		buffer = buffer[:*r.remaining+1]
	}
	n, err := r.reader.Read(buffer)
	*r.remaining -= int64(n)
	if *r.remaining < 0 {
		return n, fmt.Errorf("kimi native observation exceeds output bound")
	}
	return n, err
}

func kimiText(value any, maximum int, empty bool) bool {
	text, ok := value.(string)
	return ok && len(text) <= maximum && (empty || text != "") && !strings.ContainsRune(text, 0)
}
