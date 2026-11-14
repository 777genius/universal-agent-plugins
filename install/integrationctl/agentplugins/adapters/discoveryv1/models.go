// Package discoveryv1 authenticates and validates the static unreviewed Agent
// Plugins Discovery Index. It never acquires or executes package content.
package discoveryv1

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	SchemaVersion    = 1
	SignatureDomain  = "UAP-DISCOVERY-INDEX-ED25519-V1"
	MaxLatestBytes   = 16 << 10
	MaxEnvelopeBytes = 16 << 10
	MaxSnapshotBytes = 16 << 20
	MaxSearchBytes   = 10 << 20
)

type Pointer struct {
	PointerSchemaVersion  int           `json:"pointer_schema_version"`
	SnapshotSchemaVersion int           `json:"snapshot_schema_version"`
	Sequence              uint64        `json:"sequence"`
	SnapshotPath          string        `json:"snapshot_path"`
	EnvelopePath          string        `json:"envelope_path"`
	SearchPath            string        `json:"search_path"`
	FetchContract         FetchContract `json:"fetch_contract"`
}

type FetchContract struct {
	MaxRedirects     int `json:"max_redirects"`
	LatestMaxBytes   int `json:"latest_max_bytes"`
	SnapshotMaxBytes int `json:"snapshot_max_bytes"`
	EnvelopeMaxBytes int `json:"envelope_max_bytes"`
	SearchMaxBytes   int `json:"search_max_bytes"`
	RetryAttempts    int `json:"retry_attempts"`
}

type Envelope struct {
	EnvelopeSchemaVersion int    `json:"envelope_schema_version"`
	SnapshotSchemaVersion int    `json:"snapshot_schema_version"`
	Sequence              uint64 `json:"sequence"`
	KeyID                 string `json:"key_id"`
	Algorithm             string `json:"algorithm"`
	SignatureDomain       string `json:"signature_domain"`
	SnapshotDigest        string `json:"snapshot_digest"`
	Signature             string `json:"signature"`
}

type Partition struct {
	Query      string `json:"query"`
	SizeMin    int    `json:"size_min"`
	SizeMax    int    `json:"size_max"`
	TotalCount int    `json:"total_count"`
}

type SearchProjection struct {
	Path        string `json:"path"`
	Digest      string `json:"digest"`
	RecordCount int    `json:"record_count"`
}

type Components struct {
	Extensions int `json:"extensions"`
	MCP        int `json:"mcp"`
	Skills     int `json:"skills"`
}

type Author struct {
	Name  string `json:"name"`
	Email string `json:"email,omitempty"`
	URL   string `json:"url,omitempty"`
}

type Record struct {
	Slug                   string     `json:"slug"`
	Name                   string     `json:"name"`
	Description            string     `json:"description"`
	Owner                  string     `json:"owner"`
	Repository             string     `json:"repository"`
	PackagePath            string     `json:"package_path"`
	Revision               string     `json:"revision"`
	Version                *string    `json:"version"`
	License                *string    `json:"license"`
	SchemaVersion          string     `json:"schema_version"`
	Components             Components `json:"components"`
	MCPTransports          []string   `json:"mcp_transports"`
	CompatibleClients      []string   `json:"compatible_clients"`
	Authentication         string     `json:"authentication"`
	Status                 string     `json:"status"`
	RuntimeReviewed        bool       `json:"runtime_reviewed"`
	TreeDigest             string     `json:"tree_digest"`
	ManifestDigest         string     `json:"manifest_digest"`
	Stars                  int        `json:"stars"`
	RepositoryUpdatedAt    string     `json:"repository_updated_at"`
	ReviewedDistributionID *string    `json:"reviewed_distribution_id"`
	Availability           string     `json:"availability"`
	Author                 *Author    `json:"author,omitempty"`
	FirstSeen              string     `json:"first_seen,omitempty"`
	LastSeen               string     `json:"last_seen,omitempty"`
}

type Snapshot struct {
	DiscoverySchemaVersion int              `json:"discovery_schema_version"`
	Sequence               uint64           `json:"sequence"`
	PublicationID          string           `json:"publication_id"`
	SourceCommit           string           `json:"source_commit"`
	GeneratedAt            string           `json:"generated_at"`
	ExpiresAt              string           `json:"expires_at"`
	Complete               bool             `json:"complete"`
	QueryManifestDigest    string           `json:"query_manifest_digest"`
	Partitions             []Partition      `json:"partitions"`
	SearchProjection       SearchProjection `json:"search_projection"`
	Records                []Record         `json:"records"`
}

type Search struct {
	SearchSchemaVersion int      `json:"search_schema_version"`
	Sequence            uint64   `json:"sequence"`
	GeneratedAt         string   `json:"generated_at"`
	Records             []Record `json:"records"`
}

type KeyState string

const (
	KeyCurrent KeyState = "current"
	KeyNext    KeyState = "next"
	KeyRetired KeyState = "retired"
)

type TrustedKey struct {
	ID        string
	PublicKey ed25519.PublicKey
	State     KeyState
}

type TrustStore struct{ Keys []TrustedKey }

type BundleSource string

const (
	BundleSourceVerified BundleSource = "verified"
	BundleSourceRemote   BundleSource = "remote"
	BundleSourceCache    BundleSource = "cache"
)

type VerifiedBundle struct {
	PointerBytes  []byte
	SnapshotBytes []byte
	EnvelopeBytes []byte
	SearchBytes   []byte
	Pointer       Pointer
	Snapshot      Snapshot
	Envelope      Envelope
	Search        Search
	Digest        string
	Source        BundleSource
}

var (
	ErrStrictJSON       = errors.New("Discovery JSON is not strict schema 1")
	ErrUnknownKey       = errors.New("unknown Discovery signing key")
	ErrRetiredKey       = errors.New("retired Discovery signing key")
	ErrNoTrustedKeys    = errors.New("no trusted Discovery signing keys configured")
	ErrDigestMismatch   = errors.New("Discovery artifact digest mismatch")
	ErrInvalidSignature = errors.New("invalid Discovery snapshot signature")
	ErrSequenceMismatch = errors.New("Discovery schema or sequence mismatch")
	ErrExpired          = errors.New("Discovery snapshot is expired")
	ErrClockSkew        = errors.New("local clock is before Discovery generation time")
	ErrRollback         = errors.New("Discovery snapshot sequence is below local floor")
	ErrSequenceConflict = errors.New("Discovery sequence has conflicting authenticated bytes")
	ErrUnavailable      = errors.New("no valid Discovery snapshot is available")
)

var (
	keyIDPattern       = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9._-]*[a-z0-9])?$`)
	publicationPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)
	repositoryPattern  = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*/[a-z0-9][a-z0-9._-]*$`)
	packagePathPattern = regexp.MustCompile(`^(?:|[A-Za-z0-9._-]+(?:/[A-Za-z0-9._-]+)*)$`)
	shaPattern         = regexp.MustCompile(`^[0-9a-f]{40}$`)
	digestPattern      = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
	timestampPattern   = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z$`)
)

var signaturePrefix = []byte(SignatureDomain + "\x00")

func ParsePointer(body []byte) (Pointer, error) {
	var value Pointer
	if err := strictDecode(body, &value, MaxLatestBytes); err != nil {
		return value, strictError("pointer: %v", err)
	}
	if err := exactObject(body, []string{"pointer_schema_version", "snapshot_schema_version", "sequence", "snapshot_path", "envelope_path", "search_path", "fetch_contract"}); err != nil {
		return value, strictError("pointer: %v", err)
	}
	var root map[string]json.RawMessage
	_ = json.Unmarshal(body, &root)
	if err := exactObject(root["fetch_contract"], []string{"max_redirects", "latest_max_bytes", "snapshot_max_bytes", "envelope_max_bytes", "search_max_bytes", "retry_attempts"}); err != nil {
		return value, strictError("fetch contract: %v", err)
	}
	stem := fmt.Sprintf("%020d", value.Sequence)
	if value.PointerSchemaVersion != 1 || value.SnapshotSchemaVersion != 1 || value.Sequence < 1 ||
		exactPath(value.SnapshotPath, "snapshots/"+stem+".json") != nil ||
		exactPath(value.EnvelopePath, "snapshots/"+stem+".envelope.json") != nil ||
		exactPath(value.SearchPath, "search/"+stem+".json") != nil {
		return value, strictError("invalid pointer identity")
	}
	c := value.FetchContract
	if c.MaxRedirects != 0 || c.RetryAttempts < 1 || c.RetryAttempts > 3 || c.LatestMaxBytes < 1 || c.LatestMaxBytes > MaxLatestBytes ||
		c.SnapshotMaxBytes < 1 || c.SnapshotMaxBytes > MaxSnapshotBytes || c.EnvelopeMaxBytes < 1 || c.EnvelopeMaxBytes > MaxEnvelopeBytes ||
		c.SearchMaxBytes < 1 || c.SearchMaxBytes > MaxSearchBytes {
		return value, strictError("unsafe fetch contract")
	}
	return value, nil
}

func VerifyBundle(pointerBytes, snapshotBytes, envelopeBytes, searchBytes []byte, trust TrustStore) (VerifiedBundle, error) {
	if len(trust.Keys) == 0 {
		return VerifiedBundle{}, ErrNoTrustedKeys
	}
	pointer, err := ParsePointer(pointerBytes)
	if err != nil {
		return VerifiedBundle{}, err
	}
	var envelope Envelope
	if err := strictDecode(envelopeBytes, &envelope, MaxEnvelopeBytes); err != nil {
		return VerifiedBundle{}, strictError("envelope: %v", err)
	}
	if err := exactObject(envelopeBytes, []string{"envelope_schema_version", "snapshot_schema_version", "sequence", "key_id", "algorithm", "signature_domain", "snapshot_digest", "signature"}); err != nil {
		return VerifiedBundle{}, strictError("envelope: %v", err)
	}
	if envelope.EnvelopeSchemaVersion != 1 || envelope.SnapshotSchemaVersion != 1 || envelope.Sequence < 1 || !keyIDPattern.MatchString(envelope.KeyID) ||
		envelope.Algorithm != "Ed25519" || envelope.SignatureDomain != SignatureDomain || !digestPattern.MatchString(envelope.SnapshotDigest) {
		return VerifiedBundle{}, strictError("invalid envelope")
	}
	sum := sha256.Sum256(snapshotBytes)
	digest := "sha256:" + hex.EncodeToString(sum[:])
	if digest != envelope.SnapshotDigest {
		return VerifiedBundle{}, ErrDigestMismatch
	}
	key, err := trustedKey(trust, envelope.KeyID)
	if err != nil {
		return VerifiedBundle{}, err
	}
	signature, err := base64.StdEncoding.Strict().DecodeString(envelope.Signature)
	if err != nil || len(signature) != ed25519.SignatureSize || !ed25519.Verify(key.PublicKey, SignatureMessage(snapshotBytes), signature) {
		return VerifiedBundle{}, ErrInvalidSignature
	}
	snapshot, err := parseSnapshot(snapshotBytes)
	if err != nil {
		return VerifiedBundle{}, err
	}
	search, err := parseSearch(searchBytes)
	if err != nil {
		return VerifiedBundle{}, err
	}
	if pointer.Sequence != snapshot.Sequence || envelope.Sequence != snapshot.Sequence || search.Sequence != snapshot.Sequence ||
		pointer.SnapshotSchemaVersion != snapshot.DiscoverySchemaVersion || envelope.SnapshotSchemaVersion != snapshot.DiscoverySchemaVersion ||
		pointer.SearchPath != snapshot.SearchProjection.Path || search.GeneratedAt != snapshot.GeneratedAt || len(search.Records) != snapshot.SearchProjection.RecordCount {
		return VerifiedBundle{}, ErrSequenceMismatch
	}
	searchSum := sha256.Sum256(searchBytes)
	if "sha256:"+hex.EncodeToString(searchSum[:]) != snapshot.SearchProjection.Digest {
		return VerifiedBundle{}, ErrDigestMismatch
	}
	for index := range snapshot.Records {
		expected := snapshot.Records[index]
		expected.Author, expected.FirstSeen, expected.LastSeen = nil, "", ""
		if !reflect.DeepEqual(expected, search.Records[index]) {
			return VerifiedBundle{}, strictError("search projection does not match signed records")
		}
	}
	return VerifiedBundle{
		PointerBytes: append([]byte(nil), pointerBytes...), SnapshotBytes: append([]byte(nil), snapshotBytes...),
		EnvelopeBytes: append([]byte(nil), envelopeBytes...), SearchBytes: append([]byte(nil), searchBytes...),
		Pointer: pointer, Snapshot: snapshot, Envelope: envelope, Search: search, Digest: digest, Source: BundleSourceVerified,
	}, nil
}

func SignatureMessage(snapshot []byte) []byte {
	message := make([]byte, 0, len(signaturePrefix)+8+len(snapshot))
	message = append(message, signaturePrefix...)
	message = binary.BigEndian.AppendUint64(message, uint64(len(snapshot)))
	return append(message, snapshot...)
}

func parseSnapshot(body []byte) (Snapshot, error) {
	var value Snapshot
	if err := strictDecode(body, &value, MaxSnapshotBytes); err != nil {
		return value, strictError("snapshot: %v", err)
	}
	rootFields := []string{"discovery_schema_version", "sequence", "publication_id", "source_commit", "generated_at", "expires_at", "complete", "query_manifest_digest", "partitions", "search_projection", "records"}
	if err := exactObject(body, rootFields); err != nil {
		return value, strictError("snapshot: %v", err)
	}
	var root map[string]json.RawMessage
	_ = json.Unmarshal(body, &root)
	if err := validateNested(root, true); err != nil {
		return value, err
	}
	if value.DiscoverySchemaVersion != 1 || value.Sequence < 1 || !publicationPattern.MatchString(value.PublicationID) || !shaPattern.MatchString(value.SourceCommit) ||
		!value.Complete || !digestPattern.MatchString(value.QueryManifestDigest) || value.Partitions == nil || value.Records == nil || len(value.Records) > 10_000 {
		return value, strictError("invalid snapshot identity")
	}
	generated, err := parseTimestamp(value.GeneratedAt)
	if err != nil {
		return value, strictError("generated_at: %v", err)
	}
	expires, err := parseTimestamp(value.ExpiresAt)
	if err != nil || !expires.After(generated) || expires.Sub(generated) > 7*24*time.Hour {
		return value, strictError("invalid snapshot lifetime")
	}
	stem := fmt.Sprintf("%020d", value.Sequence)
	if exactPath(value.SearchProjection.Path, "search/"+stem+".json") != nil || !digestPattern.MatchString(value.SearchProjection.Digest) ||
		value.SearchProjection.RecordCount != len(value.Records) {
		return value, strictError("invalid search projection")
	}
	for index, partition := range value.Partitions {
		if partition.Query == "" || partition.SizeMin < 0 || partition.SizeMax < partition.SizeMin || partition.SizeMax > 1<<20 || partition.TotalCount < 0 || partition.TotalCount > 1000 {
			return value, strictError("invalid partition %d", index)
		}
	}
	if err := validateRecords(value.Records, generated, true); err != nil {
		return value, err
	}
	return value, nil
}

func parseSearch(body []byte) (Search, error) {
	var value Search
	if err := strictDecode(body, &value, MaxSearchBytes); err != nil {
		return value, strictError("search: %v", err)
	}
	if err := exactObject(body, []string{"search_schema_version", "sequence", "generated_at", "records"}); err != nil {
		return value, strictError("search: %v", err)
	}
	var root map[string]json.RawMessage
	_ = json.Unmarshal(body, &root)
	if err := validateNested(root, false); err != nil {
		return value, err
	}
	if value.SearchSchemaVersion != 1 || value.Sequence < 1 || value.Records == nil || len(value.Records) > 10_000 {
		return value, strictError("invalid search projection identity")
	}
	generated, err := parseTimestamp(value.GeneratedAt)
	if err != nil {
		return value, strictError("search generated_at: %v", err)
	}
	if err := validateRecords(value.Records, generated, false); err != nil {
		return value, err
	}
	return value, nil
}

func validateNested(root map[string]json.RawMessage, snapshot bool) error {
	if snapshot {
		if err := exactObject(root["search_projection"], []string{"path", "digest", "record_count"}); err != nil {
			return strictError("search projection: %v", err)
		}
		var partitions []json.RawMessage
		if err := json.Unmarshal(root["partitions"], &partitions); err != nil {
			return strictError("partitions: %v", err)
		}
		for _, item := range partitions {
			if err := exactObject(item, []string{"query", "size_min", "size_max", "total_count"}); err != nil {
				return strictError("partition: %v", err)
			}
		}
	}
	var records []json.RawMessage
	if err := json.Unmarshal(root["records"], &records); err != nil {
		return strictError("records: %v", err)
	}
	common := []string{"slug", "name", "description", "owner", "repository", "package_path", "revision", "version", "license", "schema_version", "components", "mcp_transports", "compatible_clients", "authentication", "status", "runtime_reviewed", "tree_digest", "manifest_digest", "stars", "repository_updated_at", "reviewed_distribution_id", "availability"}
	for _, raw := range records {
		fields := append([]string(nil), common...)
		if snapshot {
			fields = append(fields, "author", "first_seen", "last_seen")
		}
		if err := exactObject(raw, fields); err != nil {
			return strictError("record: %v", err)
		}
		var record map[string]json.RawMessage
		_ = json.Unmarshal(raw, &record)
		if err := exactObject(record["components"], []string{"extensions", "mcp", "skills"}); err != nil {
			return strictError("components: %v", err)
		}
		if snapshot && string(record["author"]) != "null" {
			var author map[string]json.RawMessage
			if err := json.Unmarshal(record["author"], &author); err != nil {
				return strictError("author: %v", err)
			}
			allowed := map[string]bool{"name": true, "email": true, "url": true}
			if len(author) < 1 || len(author) > 3 || author["name"] == nil {
				return strictError("author fields are invalid")
			}
			for key := range author {
				if !allowed[key] {
					return strictError("unknown author field %q", key)
				}
			}
		}
	}
	return nil
}

func validateRecords(records []Record, generated time.Time, snapshot bool) error {
	seenIdentity, seenSlug := map[string]bool{}, map[string]bool{}
	for index, record := range records {
		if !repositoryPattern.MatchString(record.Repository) || record.Owner != strings.SplitN(record.Repository, "/", 2)[0] || !packagePathPattern.MatchString(record.PackagePath) || !shaPattern.MatchString(record.Revision) ||
			record.SchemaVersion != "1.0.0" || !digestPattern.MatchString(record.TreeDigest) || !digestPattern.MatchString(record.ManifestDigest) || record.Stars < 0 ||
			record.Status != "conformant_unreviewed" || record.RuntimeReviewed || (record.Availability != "available" && record.Availability != "unavailable") {
			return strictError("invalid record %d identity", index)
		}
		expectedSlug := "discovery:" + record.Repository
		if record.PackagePath != "" {
			expectedSlug += "//" + record.PackagePath
		}
		identity := strings.ToLower(record.Repository + "\x00" + record.PackagePath)
		if record.Slug != expectedSlug || seenIdentity[identity] || seenSlug[record.Slug] {
			return strictError("duplicate or non-canonical record %q", record.Slug)
		}
		seenIdentity[identity], seenSlug[record.Slug] = true, true
		if utf8.RuneCountInString(record.Name) < 1 || utf8.RuneCountInString(record.Name) > 64 || utf8.RuneCountInString(record.Description) > 500 ||
			record.Components.Extensions < 0 || record.Components.MCP < 0 || record.Components.Skills < 0 {
			return strictError("invalid record %q metadata", record.Slug)
		}
		if err := enumList(record.MCPTransports, []string{"sse", "stdio", "streamable-http"}, true); err != nil {
			return strictError("record %q transports: %v", record.Slug, err)
		}
		// ChatGPT compatibility requires a separately registered app binding.
		// Unreviewed Discovery conformance cannot establish that trust boundary.
		if err := enumList(record.CompatibleClients, []string{"codex", "cursor", "copilot", "vscode", "kiro"}, false); err != nil {
			return strictError("record %q clients: %v", record.Slug, err)
		}
		if record.Authentication != "required" && record.Authentication != "not_required" && record.Authentication != "unknown" {
			return strictError("record %q authentication is invalid", record.Slug)
		}
		if _, err := parseTimestamp(record.RepositoryUpdatedAt); err != nil {
			return strictError("record %q repository_updated_at is invalid", record.Slug)
		}
		if snapshot {
			first, firstErr := parseTimestamp(record.FirstSeen)
			last, lastErr := parseTimestamp(record.LastSeen)
			if firstErr != nil || lastErr != nil || first.After(last) || last.After(generated) || (record.Author != nil && strings.TrimSpace(record.Author.Name) == "") {
				return strictError("record %q seen interval or author is invalid", record.Slug)
			}
		} else if record.Author != nil || record.FirstSeen != "" || record.LastSeen != "" {
			return strictError("search record %q contains snapshot-only fields", record.Slug)
		}
	}
	if !sort.SliceIsSorted(records, func(i, j int) bool {
		if records[i].Repository != records[j].Repository {
			return records[i].Repository < records[j].Repository
		}
		left, right := strings.ToLower(records[i].PackagePath), strings.ToLower(records[j].PackagePath)
		if left != right {
			return left < right
		}
		return records[i].Slug < records[j].Slug
	}) {
		return strictError("records are not canonically ordered")
	}
	return nil
}

func enumList(values, allowed []string, lexical bool) error {
	if values == nil {
		return errors.New("array is null")
	}
	positions := map[string]int{}
	for index, value := range allowed {
		positions[value] = index
	}
	previous := -1
	for _, value := range values {
		position, ok := positions[value]
		if !ok || position <= previous {
			return errors.New("values are unknown, duplicated, or out of canonical order")
		}
		previous = position
	}
	if lexical && !sort.StringsAreSorted(values) {
		return errors.New("values are not sorted")
	}
	return nil
}

func ValidityError(snapshot Snapshot, now time.Time) error {
	generated, err := parseTimestamp(snapshot.GeneratedAt)
	if err != nil {
		return err
	}
	expires, err := parseTimestamp(snapshot.ExpiresAt)
	if err != nil {
		return err
	}
	if now.UTC().Before(generated) {
		return ErrClockSkew
	}
	if !now.UTC().Before(expires) {
		return ErrExpired
	}
	return nil
}

func trustedKey(store TrustStore, id string) (TrustedKey, error) {
	seen := map[string]bool{}
	for _, key := range store.Keys {
		if seen[key.ID] || !keyIDPattern.MatchString(key.ID) || len(key.PublicKey) != ed25519.PublicKeySize || (key.State != KeyCurrent && key.State != KeyNext && key.State != KeyRetired) {
			return TrustedKey{}, fmt.Errorf("%w: malformed configured key %s", ErrUnknownKey, key.ID)
		}
		seen[key.ID] = true
	}
	for _, key := range store.Keys {
		if key.ID == id {
			if key.State == KeyRetired {
				return TrustedKey{}, fmt.Errorf("%w: %s", ErrRetiredKey, id)
			}
			return key, nil
		}
	}
	return TrustedKey{}, fmt.Errorf("%w: %s", ErrUnknownKey, id)
}

func strictDecode(body []byte, destination any, limit int) error {
	if len(body) == 0 || len(body) > limit || !json.Valid(body) {
		return errors.New("empty, oversized, or invalid JSON")
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
	if _, err := decoder.Token(); err != io.EOF {
		return errors.New("trailing JSON")
	}
	return nil
}

func scanJSONValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, compound := token.(json.Delim)
	if !compound {
		return nil
	}
	switch delimiter {
	case '{':
		keys := map[string]bool{}
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return err
			}
			key, ok := keyToken.(string)
			if !ok || keys[key] {
				return fmt.Errorf("duplicate or invalid JSON key %q", key)
			}
			keys[key] = true
			if err := scanJSONValue(decoder); err != nil {
				return err
			}
		}
		_, err = decoder.Token()
		return err
	case '[':
		for decoder.More() {
			if err := scanJSONValue(decoder); err != nil {
				return err
			}
		}
		_, err = decoder.Token()
		return err
	default:
		return errors.New("unexpected JSON delimiter")
	}
}

func exactObject(body []byte, fields []string) error {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(body, &object); err != nil {
		return err
	}
	if len(object) != len(fields) {
		return errors.New("object fields do not match schema")
	}
	for _, field := range fields {
		if _, ok := object[field]; !ok {
			return fmt.Errorf("missing field %q", field)
		}
	}
	return nil
}

func exactPath(actual, expected string) error {
	if actual != expected || strings.ContainsAny(actual, `\:`) || strings.HasPrefix(actual, "/") || path.Clean(actual) != actual {
		return errors.New("unsafe artifact path")
	}
	return nil
}

func parseTimestamp(value string) (time.Time, error) {
	if !timestampPattern.MatchString(value) {
		return time.Time{}, errors.New("timestamp must use second-precision UTC")
	}
	return time.Parse(time.RFC3339, value)
}

func strictError(format string, arguments ...any) error {
	return fmt.Errorf("%w: %s", ErrStrictJSON, fmt.Sprintf(format, arguments...))
}
