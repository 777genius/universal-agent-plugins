// Command agentplugins-registry-mirror verifies the signed public Directory,
// Discovery, and Security feeds, then copies their byte-exact artifacts into a
// staging tree.
// It is intentionally a build-time tool: it never signs, executes, or mutates
// an installed client configuration.
package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/directoryv1"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/discoveryv1"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/securityv1"
)

const (
	defaultPagesOrigin = "https://777genius.github.io/universal-agent-plugins-registry/"
	defaultRegistry    = "777genius/universal-agent-plugins-registry"
	maxTrustBytes      = 64 << 10
	directoryKeyID     = "uap-directory-2026-01"
	directoryPublicKey = "HalXARjat+v3ylTPLMAnvuavRo4ZfrF+DbWwsjlp2bI="
	discoveryKeyID     = "uap-discovery-2026-01"
	discoveryPublicKey = "IxWvGuscXR9crlCrGyBQZNqroYNVPbBA1B3pnjSffhc="
)

var shaPattern = regexp.MustCompile(`^[0-9a-f]{40}$`)

type trustedKeysDocument struct {
	SchemaVersion int `json:"schema_version"`
	Keys          []struct {
		ID        string `json:"key_id"`
		PublicKey string `json:"public_key"`
		State     string `json:"state,omitempty"`
	} `json:"keys"`
}

type mirrorMetadata struct {
	SchemaVersion      int    `json:"schema_version"`
	RegistryRepository string `json:"registry_repository"`
	DirectorySequence  uint64 `json:"directory_sequence"`
	DirectoryDigest    string `json:"directory_digest"`
	DirectoryCommit    string `json:"directory_source_commit"`
	DiscoverySequence  uint64 `json:"discovery_sequence"`
	DiscoveryDigest    string `json:"discovery_digest"`
	DiscoveryCommit    string `json:"discovery_source_commit"`
	SecuritySequence   uint64 `json:"security_sequence"`
	SecurityDigest     string `json:"security_digest"`
	SecurityCommit     string `json:"security_source_commit"`
	GeneratedFiles     int    `json:"generated_files"`
}

func main() {
	var (
		registry = flag.String("registry-repository", defaultRegistry, "registry GitHub owner/repository")
		origin   = flag.String("pages-origin", defaultPagesOrigin, "published registry Pages origin")
		output   = flag.String("output", "", "empty staging directory to populate")
		previous = flag.String("previous-marker", "", "optional previous MIRROR_METADATA.json")
	)
	flag.Parse()
	if strings.TrimSpace(*output) == "" {
		flag.Usage()
		os.Exit(2)
	}
	if err := run(*registry, *origin, *output, *previous); err != nil {
		fmt.Fprintf(os.Stderr, "agentplugins-registry-mirror: %v\n", err)
		os.Exit(1)
	}
}

func run(registry, origin, output, previous string) error {
	base, err := safeOrigin(origin)
	if err != nil {
		return err
	}
	if !strings.Contains(registry, "/") || strings.ContainsAny(registry, "?\\#") {
		return fmt.Errorf("invalid registry repository %q", registry)
	}
	client := &http.Client{Timeout: 30 * time.Second}
	client.CheckRedirect = sameOriginRedirect(base)

	directory, err := fetchDirectory(client, base, registry)
	if err != nil {
		return fmt.Errorf("directory: %w", err)
	}
	discovery, err := fetchDiscovery(client, base, registry)
	if err != nil {
		return fmt.Errorf("discovery: %w", err)
	}
	security, err := fetchSecurity(client, base, registry)
	if err != nil {
		return fmt.Errorf("security: %w", err)
	}
	metadata := mirrorMetadata{
		SchemaVersion:      1,
		RegistryRepository: registry,
		DirectorySequence:  directory.bundle.Snapshot.Sequence,
		DirectoryDigest:    directory.bundle.Digest,
		DirectoryCommit:    directory.bundle.Snapshot.SourceCommit,
		DiscoverySequence:  discovery.bundle.Snapshot.Sequence,
		DiscoveryDigest:    discovery.bundle.Digest,
		DiscoveryCommit:    discovery.bundle.Snapshot.SourceCommit,
		SecuritySequence:   security.snapshot.Sequence,
		SecurityDigest:     security.digest,
		SecurityCommit:     security.snapshot.SourceCommit,
		GeneratedFiles:     0,
	}
	if previous != "" {
		if err := enforceMonotonic(previous, metadata); err != nil {
			return err
		}
	}

	stage := output + ".staging"
	if err := os.RemoveAll(stage); err != nil {
		return fmt.Errorf("reset staging directory: %w", err)
	}
	if err := os.MkdirAll(stage, 0o755); err != nil {
		return fmt.Errorf("create staging directory: %w", err)
	}
	files := stagedFiles(directory, discovery, security)
	for _, file := range files {
		if err := writeStaged(stage, file.rel, file.body); err != nil {
			return err
		}
	}
	metadata.GeneratedFiles = len(files) + 1
	metadataBytes, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("encode metadata: %w", err)
	}
	metadataBytes = append(metadataBytes, '\n')
	if err := writeStaged(stage, "MIRROR_METADATA.json", metadataBytes); err != nil {
		return err
	}
	if _, err := os.Stat(output); err == nil {
		return fmt.Errorf("output path already exists; provide a fresh staging path: %s", output)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("check output path: %w", err)
	}
	if err := os.Rename(stage, output); err != nil {
		return fmt.Errorf("publish staging tree: %w", err)
	}
	fmt.Printf("verified Directory seq %d, Discovery seq %d, and Security seq %d; mirrored %d files to %s\n", metadata.DirectorySequence, metadata.DiscoverySequence, metadata.SecuritySequence, metadata.GeneratedFiles, output)
	return nil
}

type mirrorFile struct {
	rel  string
	body []byte
}

func stagedFiles(directory directoryResult, discovery discoveryResult, security securityResult) []mirrorFile {
	return []mirrorFile{
		{rel: "registry/schemas/1/latest.json", body: directory.pointerBytes},
		{rel: "registry/schemas/1/snapshots/" + directory.stem + ".json", body: directory.snapshotBytes},
		{rel: "registry/schemas/1/snapshots/" + directory.stem + ".envelope.json", body: directory.envelopeBytes},
		{rel: "registry/publication/trusted-keys.json", body: directory.trustBytes},
		{rel: "discovery/latest.json", body: discovery.pointerBytes},
		{rel: "discovery/snapshots/" + discovery.stem + ".json", body: discovery.snapshotBytes},
		{rel: "discovery/snapshots/" + discovery.stem + ".envelope.json", body: discovery.envelopeBytes},
		{rel: "discovery/search/" + discovery.stem + ".json", body: discovery.searchBytes},
		{rel: "discovery/trusted-keys.json", body: discovery.trustBytes},
		{rel: "security/latest.json", body: security.pointerBytes},
		{rel: "security/snapshots/" + security.stem + ".json", body: security.snapshotBytes},
		{rel: "security/snapshots/" + security.stem + ".envelope.json", body: security.envelopeBytes},
	}
}

type directoryResult struct {
	pointerBytes, snapshotBytes, envelopeBytes, trustBytes []byte
	stem                                                   string
	bundle                                                 directoryv1.VerifiedBundle
}

func fetchDirectory(client *http.Client, base *url.URL, registry string) (directoryResult, error) {
	pointerBytes, err := get(client, base.ResolveReference(&url.URL{Path: "registry/schemas/1/latest.json"}).String(), directoryv1.MaxLatestBytes)
	if err != nil {
		return directoryResult{}, err
	}
	pointer, err := directoryv1.ParsePointer(pointerBytes)
	if err != nil {
		return directoryResult{}, err
	}
	if len(pointerBytes) > pointer.FetchContract.LatestMaxBytes {
		return directoryResult{}, errors.New("latest pointer exceeds declared limit")
	}
	getArtifact := func(relative string, limit int) ([]byte, error) {
		return get(client, base.ResolveReference(&url.URL{Path: "registry/schemas/1/" + relative}).String(), limit)
	}
	snapshotBytes, err := getArtifact(pointer.SnapshotPath, pointer.FetchContract.SnapshotMaxBytes)
	if err != nil {
		return directoryResult{}, err
	}
	envelopeBytes, err := getArtifact(pointer.EnvelopePath, pointer.FetchContract.EnvelopeMaxBytes)
	if err != nil {
		return directoryResult{}, err
	}
	snapshot, err := directoryv1.ParseSnapshot(snapshotBytes)
	if err != nil {
		return directoryResult{}, err
	}
	trustBytes, trust, err := fetchDirectoryTrust(registry, snapshot.SourceCommit)
	if err != nil {
		return directoryResult{}, err
	}
	bundle, err := directoryv1.VerifyPointerBundle(pointer, snapshotBytes, envelopeBytes, trust)
	if err != nil {
		return directoryResult{}, err
	}
	return directoryResult{pointerBytes, snapshotBytes, envelopeBytes, trustBytes, fmt.Sprintf("%020d", pointer.Sequence), bundle}, nil
}

type discoveryResult struct {
	pointerBytes, snapshotBytes, envelopeBytes, searchBytes, trustBytes []byte
	stem                                                                string
	bundle                                                              discoveryv1.VerifiedBundle
}

func fetchDiscovery(client *http.Client, base *url.URL, registry string) (discoveryResult, error) {
	pointerBytes, err := get(client, base.ResolveReference(&url.URL{Path: "discovery/latest.json"}).String(), discoveryv1.MaxLatestBytes)
	if err != nil {
		return discoveryResult{}, err
	}
	pointer, err := discoveryv1.ParsePointer(pointerBytes)
	if err != nil {
		return discoveryResult{}, err
	}
	getArtifact := func(relative string, limit int) ([]byte, error) {
		return get(client, base.ResolveReference(&url.URL{Path: "discovery/" + relative}).String(), limit)
	}
	snapshotBytes, err := getArtifact(pointer.SnapshotPath, pointer.FetchContract.SnapshotMaxBytes)
	if err != nil {
		return discoveryResult{}, err
	}
	envelopeBytes, err := getArtifact(pointer.EnvelopePath, pointer.FetchContract.EnvelopeMaxBytes)
	if err != nil {
		return discoveryResult{}, err
	}
	searchBytes, err := getArtifact(pointer.SearchPath, pointer.FetchContract.SearchMaxBytes)
	if err != nil {
		return discoveryResult{}, err
	}
	snapshot, err := parseDiscoverySnapshot(snapshotBytes)
	if err != nil {
		return discoveryResult{}, err
	}
	trustBytes, trust, err := fetchDiscoveryTrust(registry, snapshot.SourceCommit)
	if err != nil {
		return discoveryResult{}, err
	}
	bundle, err := discoveryv1.VerifyBundle(pointerBytes, snapshotBytes, envelopeBytes, searchBytes, trust)
	if err != nil {
		return discoveryResult{}, err
	}
	return discoveryResult{pointerBytes, snapshotBytes, envelopeBytes, searchBytes, trustBytes, fmt.Sprintf("%020d", pointer.Sequence), bundle}, nil
}

func parseDiscoverySnapshot(body []byte) (discoveryv1.Snapshot, error) {
	// VerifyBundle performs the full strict schema check. This small decode only
	// obtains source_commit before the trust document can be selected.
	var snapshot discoveryv1.Snapshot
	if err := json.Unmarshal(body, &snapshot); err != nil {
		return snapshot, err
	}
	if !shaPattern.MatchString(snapshot.SourceCommit) {
		return snapshot, fmt.Errorf("malformed source_commit %q", snapshot.SourceCommit)
	}
	return snapshot, nil
}

type securityResult struct {
	pointerBytes, snapshotBytes, envelopeBytes []byte
	stem                                       string
	digest                                     string
	snapshot                                   securityv1.Snapshot
}

func fetchSecurity(client *http.Client, base *url.URL, registry string) (securityResult, error) {
	pointerBytes, err := get(client, base.ResolveReference(&url.URL{Path: "security/latest.json"}).String(), securityv1.MaxLatestBytes)
	if err != nil {
		return securityResult{}, err
	}
	pointer, err := securityv1.ParsePointer(pointerBytes)
	if err != nil {
		return securityResult{}, err
	}
	getArtifact := func(relative string, limit int) ([]byte, error) {
		return get(client, base.ResolveReference(&url.URL{Path: "security/" + relative}).String(), limit)
	}
	snapshotBytes, err := getArtifact(pointer.SnapshotPath, pointer.FetchContract.SnapshotMaxBytes)
	if err != nil {
		return securityResult{}, err
	}
	envelopeBytes, err := getArtifact(pointer.EnvelopePath, pointer.FetchContract.EnvelopeMaxBytes)
	if err != nil {
		return securityResult{}, err
	}
	snapshotIdentity, err := parseSecuritySnapshot(snapshotBytes)
	if err != nil {
		return securityResult{}, err
	}
	trust, err := fetchSecurityTrust(registry, snapshotIdentity.SourceCommit)
	if err != nil {
		return securityResult{}, err
	}
	snapshot, err := securityv1.Verify(pointerBytes, snapshotBytes, envelopeBytes, trust)
	if err != nil {
		return securityResult{}, err
	}
	if err := securityv1.ValidityError(snapshot, time.Now().UTC()); err != nil {
		return securityResult{}, err
	}
	digest := sha256.Sum256(snapshotBytes)
	return securityResult{
		pointerBytes:  pointerBytes,
		snapshotBytes: snapshotBytes,
		envelopeBytes: envelopeBytes,
		stem:          fmt.Sprintf("%020d", pointer.Sequence),
		digest:        fmt.Sprintf("sha256:%x", digest[:]),
		snapshot:      snapshot,
	}, nil
}

func parseSecuritySnapshot(body []byte) (securityv1.Snapshot, error) {
	// Verify performs the full strict schema check after the source commit has
	// selected the immutable trust document.
	var snapshot securityv1.Snapshot
	if err := json.Unmarshal(body, &snapshot); err != nil {
		return snapshot, err
	}
	if !shaPattern.MatchString(snapshot.SourceCommit) {
		return snapshot, fmt.Errorf("malformed source_commit %q", snapshot.SourceCommit)
	}
	return snapshot, nil
}

func fetchTrustBytes(registry, revision, relative string) ([]byte, trustedKeysDocument, error) {
	if !shaPattern.MatchString(revision) {
		return nil, trustedKeysDocument{}, fmt.Errorf("invalid trust revision %q", revision)
	}
	endpoint := "https://raw.githubusercontent.com/" + registry + "/" + revision + "/" + relative
	rawOrigin, err := safeOrigin("https://raw.githubusercontent.com/")
	if err != nil {
		return nil, trustedKeysDocument{}, err
	}
	rawClient := &http.Client{Timeout: 30 * time.Second, CheckRedirect: sameOriginRedirect(rawOrigin)}
	body, err := get(rawClient, endpoint, maxTrustBytes)
	if err != nil {
		return nil, trustedKeysDocument{}, err
	}
	var document trustedKeysDocument
	if err := strictDecode(body, &document); err != nil {
		return nil, trustedKeysDocument{}, fmt.Errorf("trust document: %w", err)
	}
	if document.SchemaVersion != 1 || len(document.Keys) == 0 {
		return nil, trustedKeysDocument{}, errors.New("trust document has no schema-1 keys")
	}
	return body, document, nil
}

func fetchDirectoryTrust(registry, revision string) ([]byte, directoryv1.TrustStore, error) {
	body, document, err := fetchTrustBytes(registry, revision, "registry/publication/trusted-keys.json")
	if err != nil {
		return nil, directoryv1.TrustStore{}, err
	}
	keys := make([]directoryv1.TrustedKey, 0, len(document.Keys))
	bootstrapFound := false
	for _, item := range document.Keys {
		key, err := decodeKey(item.ID, item.PublicKey, item.State)
		if err != nil {
			return nil, directoryv1.TrustStore{}, err
		}
		if item.ID == directoryKeyID && item.PublicKey == directoryPublicKey {
			bootstrapFound = true
		}
		keys = append(keys, key)
	}
	if !bootstrapFound {
		return nil, directoryv1.TrustStore{}, errors.New("directory trust document is not anchored to the release bootstrap key")
	}
	return body, directoryv1.TrustStore{Keys: keys}, nil
}

func decodeKey(id, encoded, state string) (directoryv1.TrustedKey, error) {
	raw, err := base64.StdEncoding.Strict().DecodeString(encoded)
	if err != nil || len(raw) != ed25519.PublicKeySize {
		return directoryv1.TrustedKey{}, fmt.Errorf("invalid public key for %q", id)
	}
	if state == "" {
		state = string(directoryv1.KeyCurrent)
	}
	return directoryv1.TrustedKey{ID: id, PublicKey: ed25519.PublicKey(raw), State: directoryv1.KeyState(state)}, nil
}

func fetchDiscoveryTrust(registry, revision string) ([]byte, discoveryv1.TrustStore, error) {
	body, document, err := fetchTrustBytes(registry, revision, "registry/discovery/trusted-keys.json")
	if err != nil {
		return nil, discoveryv1.TrustStore{}, err
	}
	keys := make([]discoveryv1.TrustedKey, 0, len(document.Keys))
	bootstrapFound := false
	for _, item := range document.Keys {
		raw, err := base64.StdEncoding.Strict().DecodeString(item.PublicKey)
		if err != nil || len(raw) != ed25519.PublicKeySize {
			return nil, discoveryv1.TrustStore{}, fmt.Errorf("invalid public key for %q", item.ID)
		}
		state := item.State
		if state == "" {
			state = string(discoveryv1.KeyCurrent)
		}
		if item.ID == discoveryKeyID && item.PublicKey == discoveryPublicKey {
			bootstrapFound = true
		}
		keys = append(keys, discoveryv1.TrustedKey{ID: item.ID, PublicKey: ed25519.PublicKey(raw), State: discoveryv1.KeyState(state)})
	}
	if !bootstrapFound {
		return nil, discoveryv1.TrustStore{}, errors.New("Discovery trust document is not anchored to the release bootstrap key")
	}
	return body, discoveryv1.TrustStore{Keys: keys}, nil
}

func fetchSecurityTrust(registry, revision string) (securityv1.TrustStore, error) {
	_, document, err := fetchTrustBytes(registry, revision, "registry/discovery/trusted-keys.json")
	if err != nil {
		return securityv1.TrustStore{}, err
	}
	for _, item := range document.Keys {
		if item.ID != discoveryKeyID || item.PublicKey != discoveryPublicKey || (item.State != "" && item.State != "current") {
			continue
		}
		raw, err := base64.StdEncoding.Strict().DecodeString(item.PublicKey)
		if err != nil || len(raw) != ed25519.PublicKeySize {
			return securityv1.TrustStore{}, fmt.Errorf("invalid public key for %q", item.ID)
		}
		return securityv1.TrustStore{KeyID: item.ID, PublicKey: ed25519.PublicKey(raw)}, nil
	}
	return securityv1.TrustStore{}, errors.New("Security trust document is not anchored to the current release bootstrap key")
}

func strictDecode(body []byte, destination any) error {
	if len(body) == 0 || !json.Valid(body) {
		return errors.New("empty or invalid JSON")
	}
	if err := rejectDuplicateKeys(body); err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return errors.New("trailing JSON value")
	}
	return nil
}

func rejectDuplicateKeys(body []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	if err := scanJSONValue(decoder); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return errors.New("trailing JSON")
	}
	return nil
}

func scanJSONValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	if token == nil {
		return errors.New("JSON null is not allowed")
	}
	delimiter, compound := token.(json.Delim)
	if !compound {
		return nil
	}
	switch delimiter {
	case '{':
		keys := map[string]struct{}{}
		for decoder.More() {
			key, err := decoder.Token()
			if err != nil {
				return err
			}
			name, ok := key.(string)
			if !ok {
				return errors.New("object key is not a string")
			}
			if _, exists := keys[name]; exists {
				return fmt.Errorf("duplicate JSON key %q", name)
			}
			keys[name] = struct{}{}
			if err := scanJSONValue(decoder); err != nil {
				return err
			}
		}
		end, err := decoder.Token()
		if err != nil || end != json.Delim('}') {
			return errors.New("unterminated object")
		}
	case '[':
		for decoder.More() {
			if err := scanJSONValue(decoder); err != nil {
				return err
			}
		}
		end, err := decoder.Token()
		if err != nil || end != json.Delim(']') {
			return errors.New("unterminated array")
		}
	default:
		return errors.New("unexpected JSON delimiter")
	}
	return nil
}

func safeOrigin(raw string) (*url.URL, error) {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, fmt.Errorf("pages origin must be an HTTPS URL without credentials/query: %q", raw)
	}
	if !strings.HasSuffix(parsed.Path, "/") {
		parsed.Path += "/"
	}
	clean := strings.TrimSuffix(parsed.Path, "/")
	if clean == "" {
		clean = "/"
	}
	if strings.Contains(strings.ToLower(parsed.EscapedPath()), "%2f") || strings.Contains(strings.ToLower(parsed.EscapedPath()), "%5c") || strings.Contains(strings.ToLower(parsed.EscapedPath()), "%2e") || path.Clean(parsed.Path) != clean {
		return nil, fmt.Errorf("pages origin contains an unsafe path: %q", raw)
	}
	return parsed, nil
}

func sameOriginRedirect(origin *url.URL) func(*http.Request, []*http.Request) error {
	return func(request *http.Request, via []*http.Request) error {
		if len(via) > 2 || request.URL.Scheme != origin.Scheme || !strings.EqualFold(request.URL.Host, origin.Host) || !strings.HasPrefix(request.URL.Path, origin.Path) || request.URL.User != nil {
			return errors.New("unsafe Pages redirect")
		}
		return nil
	}
}

func get(client *http.Client, endpoint string, limit int) ([]byte, error) {
	request, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "application/json")
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s returned HTTP %d", endpoint, response.StatusCode)
	}
	reader := io.LimitReader(response.Body, int64(limit)+1)
	body, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	if len(body) > limit {
		return nil, fmt.Errorf("GET %s exceeded %d bytes", endpoint, limit)
	}
	return body, nil
}

func writeStaged(root, relative string, body []byte) error {
	if relative == "" || filepath.IsAbs(relative) || path.Clean(relative) != relative || strings.HasPrefix(relative, "../") || strings.Contains(relative, "\\") {
		return fmt.Errorf("unsafe mirror path %q", relative)
	}
	full := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return fmt.Errorf("create %s: %w", relative, err)
	}
	temporary, err := os.CreateTemp(filepath.Dir(full), ".mirror-*")
	if err != nil {
		return fmt.Errorf("stage %s: %w", relative, err)
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	if err := temporary.Chmod(0o644); err != nil {
		temporary.Close()
		return fmt.Errorf("mode %s: %w", relative, err)
	}
	if _, err := temporary.Write(body); err != nil {
		temporary.Close()
		return fmt.Errorf("write %s: %w", relative, err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close %s: %w", relative, err)
	}
	if err := os.Rename(temporaryName, full); err != nil {
		return fmt.Errorf("publish %s: %w", relative, err)
	}
	return nil
}

func enforceMonotonic(previousPath string, candidate mirrorMetadata) error {
	body, err := os.ReadFile(previousPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("read previous marker: %w", err)
	}
	var previous mirrorMetadata
	if err := strictDecode(body, &previous); err != nil {
		return fmt.Errorf("previous marker: %w", err)
	}
	if previous.SchemaVersion != 1 {
		return errors.New("previous marker has unsupported schema")
	}
	if previous.RegistryRepository != "" && previous.RegistryRepository != candidate.RegistryRepository {
		return fmt.Errorf("candidate registry %q differs from previous marker %q", candidate.RegistryRepository, previous.RegistryRepository)
	}
	if candidate.DirectorySequence < previous.DirectorySequence || candidate.DiscoverySequence < previous.DiscoverySequence || candidate.SecuritySequence < previous.SecuritySequence {
		return fmt.Errorf("candidate feed regresses previous marker (directory %d/%d, discovery %d/%d, security %d/%d)", candidate.DirectorySequence, previous.DirectorySequence, candidate.DiscoverySequence, previous.DiscoverySequence, candidate.SecuritySequence, previous.SecuritySequence)
	}
	if candidate.DirectorySequence == previous.DirectorySequence && candidate.DirectoryDigest != previous.DirectoryDigest {
		return errors.New("Directory sequence has conflicting authenticated bytes")
	}
	if candidate.DiscoverySequence == previous.DiscoverySequence && candidate.DiscoveryDigest != previous.DiscoveryDigest {
		return errors.New("Discovery sequence has conflicting authenticated bytes")
	}
	if previous.SecuritySequence > 0 && candidate.SecuritySequence == previous.SecuritySequence && candidate.SecurityDigest != previous.SecurityDigest {
		return errors.New("Security sequence has conflicting authenticated bytes")
	}
	return nil
}
