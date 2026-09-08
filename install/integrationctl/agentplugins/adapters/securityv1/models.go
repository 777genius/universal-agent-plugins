// Package securityv1 verifies automated LintAI assessments for exact Agent
// Plugins package bytes. A successful check is evidence, not a safety claim.
package securityv1

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
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

const (
	SchemaVersion    = 1
	SignatureDomain  = "UAP-SECURITY-INDEX-ED25519-V1"
	MaxLatestBytes   = 16 << 10
	MaxEnvelopeBytes = 16 << 10
	MaxSnapshotBytes = 8 << 20
	maxRecords       = 10_000
	maxFindings      = 32
)

type Pointer struct {
	PointerSchemaVersion  int           `json:"pointer_schema_version"`
	SnapshotSchemaVersion int           `json:"snapshot_schema_version"`
	Sequence              uint64        `json:"sequence"`
	SnapshotPath          string        `json:"snapshot_path"`
	EnvelopePath          string        `json:"envelope_path"`
	FetchContract         FetchContract `json:"fetch_contract"`
}

type FetchContract struct {
	MaxRedirects     int `json:"max_redirects"`
	LatestMaxBytes   int `json:"latest_max_bytes"`
	SnapshotMaxBytes int `json:"snapshot_max_bytes"`
	EnvelopeMaxBytes int `json:"envelope_max_bytes"`
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

type DiscoveryIdentity struct {
	Sequence       uint64 `json:"sequence"`
	SnapshotDigest string `json:"snapshot_digest"`
}

type Coverage struct {
	Subjects    int `json:"subjects"`
	Checked     int `json:"checked"`
	Unavailable int `json:"unavailable"`
}

type Record struct {
	Subject      domain.SecuritySubject   `json:"subject"`
	Outcome      string                   `json:"outcome"`
	Counts       domain.SecurityCounts    `json:"counts"`
	ScannedFiles int                      `json:"scanned_files"`
	ReportDigest string                   `json:"report_digest,omitempty"`
	ErrorCode    string                   `json:"error_code,omitempty"`
	Findings     []domain.SecurityFinding `json:"findings"`
}

type Snapshot struct {
	SecuritySchemaVersion int                    `json:"security_schema_version"`
	Sequence              uint64                 `json:"sequence"`
	PublicationID         string                 `json:"publication_id"`
	SourceCommit          string                 `json:"source_commit"`
	GeneratedAt           string                 `json:"generated_at"`
	ExpiresAt             string                 `json:"expires_at"`
	Complete              bool                   `json:"complete"`
	Discovery             DiscoveryIdentity      `json:"discovery"`
	Scanner               domain.SecurityScanner `json:"scanner"`
	Policy                domain.SecurityPolicy  `json:"policy"`
	Coverage              Coverage               `json:"coverage"`
	Records               []Record               `json:"records"`
}

type TrustStore struct {
	KeyID     string
	PublicKey ed25519.PublicKey
}

var (
	ErrStrictJSON       = errors.New("Security Index JSON is not strict schema 1")
	ErrUnknownKey       = errors.New("unknown Security Index signing key")
	ErrDigestMismatch   = errors.New("Security Index digest mismatch")
	ErrInvalidSignature = errors.New("invalid Security Index signature")
	ErrSequenceMismatch = errors.New("Security Index sequence mismatch")
	ErrExpired          = errors.New("Security Index is expired")
	ErrClockSkew        = errors.New("local clock is before Security Index generation time")
)

var (
	keyIDPattern       = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9._-]*[a-z0-9])?$`)
	publicationPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,255}$`)
	identifierPattern  = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`)
	versionPattern     = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(?:[-+][0-9A-Za-z.-]+)?$`)
	rulePattern        = regexp.MustCompile(`^[A-Z]+[0-9]+$`)
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
	if err := exactObject(body, []string{"pointer_schema_version", "snapshot_schema_version", "sequence", "snapshot_path", "envelope_path", "fetch_contract"}); err != nil {
		return value, strictError("pointer: %v", err)
	}
	var root map[string]json.RawMessage
	_ = json.Unmarshal(body, &root)
	if err := exactObject(root["fetch_contract"], []string{"max_redirects", "latest_max_bytes", "snapshot_max_bytes", "envelope_max_bytes", "retry_attempts"}); err != nil {
		return value, strictError("fetch contract: %v", err)
	}
	stem := fmt.Sprintf("%020d", value.Sequence)
	c := value.FetchContract
	if value.PointerSchemaVersion != 1 || value.SnapshotSchemaVersion != 1 || value.Sequence < 1 ||
		exactPath(value.SnapshotPath, "snapshots/"+stem+".json") != nil || exactPath(value.EnvelopePath, "snapshots/"+stem+".envelope.json") != nil ||
		c.MaxRedirects != 0 || c.RetryAttempts < 1 || c.RetryAttempts > 3 || c.LatestMaxBytes < 1 || c.LatestMaxBytes > MaxLatestBytes ||
		c.SnapshotMaxBytes < 1 || c.SnapshotMaxBytes > MaxSnapshotBytes || c.EnvelopeMaxBytes < 1 || c.EnvelopeMaxBytes > MaxEnvelopeBytes {
		return value, strictError("invalid pointer identity or fetch contract")
	}
	return value, nil
}

func Verify(pointerBytes, snapshotBytes, envelopeBytes []byte, trust TrustStore) (Snapshot, error) {
	pointer, err := ParsePointer(pointerBytes)
	if err != nil {
		return Snapshot{}, err
	}
	var envelope Envelope
	if err := strictDecode(envelopeBytes, &envelope, MaxEnvelopeBytes); err != nil {
		return Snapshot{}, strictError("envelope: %v", err)
	}
	if err := exactObject(envelopeBytes, []string{"envelope_schema_version", "snapshot_schema_version", "sequence", "key_id", "algorithm", "signature_domain", "snapshot_digest", "signature"}); err != nil {
		return Snapshot{}, strictError("envelope: %v", err)
	}
	if envelope.EnvelopeSchemaVersion != 1 || envelope.SnapshotSchemaVersion != 1 || envelope.Sequence < 1 || !keyIDPattern.MatchString(envelope.KeyID) ||
		envelope.Algorithm != "Ed25519" || envelope.SignatureDomain != SignatureDomain || !digestPattern.MatchString(envelope.SnapshotDigest) {
		return Snapshot{}, strictError("invalid envelope")
	}
	if trust.KeyID != envelope.KeyID || !keyIDPattern.MatchString(trust.KeyID) || len(trust.PublicKey) != ed25519.PublicKeySize {
		return Snapshot{}, ErrUnknownKey
	}
	digest := sha256.Sum256(snapshotBytes)
	if "sha256:"+hex.EncodeToString(digest[:]) != envelope.SnapshotDigest {
		return Snapshot{}, ErrDigestMismatch
	}
	signature, err := base64.StdEncoding.Strict().DecodeString(envelope.Signature)
	if err != nil || len(signature) != ed25519.SignatureSize || !ed25519.Verify(trust.PublicKey, SignatureMessage(snapshotBytes), signature) {
		return Snapshot{}, ErrInvalidSignature
	}
	snapshot, err := parseSnapshot(snapshotBytes)
	if err != nil {
		return Snapshot{}, err
	}
	if pointer.Sequence != snapshot.Sequence || envelope.Sequence != snapshot.Sequence || pointer.SnapshotSchemaVersion != snapshot.SecuritySchemaVersion {
		return Snapshot{}, ErrSequenceMismatch
	}
	return snapshot, nil
}

func SignatureMessage(snapshot []byte) []byte {
	message := make([]byte, 0, len(signaturePrefix)+8+len(snapshot))
	message = append(message, signaturePrefix...)
	message = binary.BigEndian.AppendUint64(message, uint64(len(snapshot)))
	return append(message, snapshot...)
}

func Lookup(snapshot Snapshot, subject domain.SecuritySubject) *domain.SecurityAssessment {
	index := sort.Search(len(snapshot.Records), func(index int) bool {
		return compareSubject(snapshot.Records[index].Subject, subject) >= 0
	})
	if index >= len(snapshot.Records) || snapshot.Records[index].Subject != subject || snapshot.Records[index].Outcome == "check_unavailable" {
		return nil
	}
	record := snapshot.Records[index]
	assessment := domain.SecurityAssessment{
		SchemaVersion: SchemaVersion, Subject: record.Subject, Scanner: snapshot.Scanner, Policy: snapshot.Policy,
		Outcome: domain.SecurityOutcome(record.Outcome), Counts: record.Counts, ScannedFiles: record.ScannedFiles,
		ReportDigest: record.ReportDigest, Findings: append([]domain.SecurityFinding(nil), record.Findings...),
	}
	return &assessment
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

func parseSnapshot(body []byte) (Snapshot, error) {
	var value Snapshot
	if err := strictDecode(body, &value, MaxSnapshotBytes); err != nil {
		return value, strictError("snapshot: %v", err)
	}
	rootFields := []string{"security_schema_version", "sequence", "publication_id", "source_commit", "generated_at", "expires_at", "complete", "discovery", "scanner", "policy", "coverage", "records"}
	if err := exactObject(body, rootFields); err != nil {
		return value, strictError("snapshot: %v", err)
	}
	var root map[string]json.RawMessage
	_ = json.Unmarshal(body, &root)
	for name, fields := range map[string][]string{
		"discovery": {"sequence", "snapshot_digest"}, "scanner": {"id", "version"},
		"policy": {"id", "version", "digest"}, "coverage": {"subjects", "checked", "unavailable"},
	} {
		if err := exactObject(root[name], fields); err != nil {
			return value, strictError("%s: %v", name, err)
		}
	}
	var records []json.RawMessage
	_ = json.Unmarshal(root["records"], &records)
	if len(records) != len(value.Records) {
		return value, strictError("records are invalid")
	}
	for index, raw := range records {
		if err := validateRecordShape(raw, value.Records[index]); err != nil {
			return value, strictError("record %d: %v", index, err)
		}
	}
	if value.SecuritySchemaVersion != 1 || value.Sequence < 1 || !publicationPattern.MatchString(value.PublicationID) || !shaPattern.MatchString(value.SourceCommit) || !value.Complete ||
		value.Discovery.Sequence < 1 || !digestPattern.MatchString(value.Discovery.SnapshotDigest) || !identifierPattern.MatchString(value.Scanner.ID) || !versionPattern.MatchString(value.Scanner.Version) ||
		!identifierPattern.MatchString(value.Policy.ID) || value.Policy.Version < 1 || !digestPattern.MatchString(value.Policy.Digest) || value.Records == nil || len(value.Records) > maxRecords {
		return value, strictError("invalid snapshot identity")
	}
	generated, err := parseTimestamp(value.GeneratedAt)
	if err != nil {
		return value, strictError("generated_at: %v", err)
	}
	expires, err := parseTimestamp(value.ExpiresAt)
	if err != nil || !expires.After(generated) || expires.Sub(generated) > 31*24*time.Hour {
		return value, strictError("invalid snapshot lifetime")
	}
	checked := 0
	for index, record := range value.Records {
		if err := validateRecord(record); err != nil {
			return value, strictError("record %d: %v", index, err)
		}
		if record.Outcome != "check_unavailable" {
			checked++
		}
		if index > 0 && compareSubject(value.Records[index-1].Subject, record.Subject) >= 0 {
			return value, strictError("records are duplicated or not ordered")
		}
	}
	if value.Coverage.Subjects != len(value.Records) || value.Coverage.Checked != checked || value.Coverage.Unavailable != len(value.Records)-checked {
		return value, strictError("coverage is inconsistent")
	}
	return value, nil
}

func validateRecordShape(body []byte, record Record) error {
	fields := []string{"subject", "outcome", "counts", "scanned_files", "findings"}
	if record.Outcome == "check_unavailable" {
		fields = append(fields, "error_code")
	} else {
		fields = append(fields, "report_digest")
	}
	if err := exactObject(body, fields); err != nil {
		return err
	}
	var raw map[string]json.RawMessage
	_ = json.Unmarshal(body, &raw)
	if err := exactObject(raw["subject"], []string{"tree_digest", "manifest_digest"}); err != nil {
		return err
	}
	if err := exactObject(raw["counts"], []string{"blocking", "warnings", "total"}); err != nil {
		return err
	}
	var findings []json.RawMessage
	_ = json.Unmarshal(raw["findings"], &findings)
	if len(findings) != len(record.Findings) {
		return errors.New("findings are invalid")
	}
	for index, finding := range findings {
		fields := []string{"code", "disposition", "severity", "confidence", "category", "path", "message"}
		if record.Findings[index].Line > 0 {
			fields = append(fields, "line")
		}
		if err := exactObject(finding, fields); err != nil {
			return err
		}
	}
	return nil
}

func validateRecord(record Record) error {
	if !digestPattern.MatchString(record.Subject.TreeDigest) || !digestPattern.MatchString(record.Subject.ManifestDigest) || record.Counts.Blocking < 0 || record.Counts.Warnings < 0 ||
		record.Counts.Total != record.Counts.Blocking+record.Counts.Warnings || record.ScannedFiles < 0 || record.Findings == nil || len(record.Findings) > maxFindings {
		return errors.New("invalid subject, counts, or finding projection")
	}
	if record.Outcome == "check_unavailable" {
		if record.Counts.Total != 0 || record.ScannedFiles != 0 || record.ReportDigest != "" || len(record.Findings) != 0 || (record.ErrorCode != "acquisition_failed" && record.ErrorCode != "scan_failed") {
			return errors.New("invalid unavailable result")
		}
		return nil
	}
	if record.ErrorCode != "" || !digestPattern.MatchString(record.ReportDigest) {
		return errors.New("invalid checked result")
	}
	expected := "no_blocking_findings"
	if record.Counts.Blocking > 0 {
		expected = "blocking_findings"
	} else if record.Counts.Warnings > 0 {
		expected = "warnings"
	}
	if record.Outcome != expected {
		return errors.New("outcome does not match counts")
	}
	projectedBlocking, projectedWarnings := 0, 0
	for index, finding := range record.Findings {
		if !rulePattern.MatchString(finding.Code) || utf8.RuneCountInString(finding.Message) < 1 || len(finding.Message) > 2000 || len(finding.Path) > 1024 || finding.Line < 0 ||
			(finding.Disposition != "blocking" && finding.Disposition != "warning") || (finding.Severity != "allow" && finding.Severity != "warn" && finding.Severity != "deny") ||
			(finding.Confidence != "low" && finding.Confidence != "medium" && finding.Confidence != "high") || strings.TrimSpace(finding.Category) == "" || len(finding.Category) > 64 {
			return fmt.Errorf("invalid finding %d", index)
		}
		if finding.Disposition == "blocking" {
			projectedBlocking++
		} else {
			projectedWarnings++
		}
		if index > 0 && compareFinding(record.Findings[index-1], finding) > 0 {
			return errors.New("findings are not ordered")
		}
	}
	if projectedBlocking > record.Counts.Blocking || projectedWarnings > record.Counts.Warnings {
		return errors.New("finding projection exceeds counts")
	}
	return nil
}

func compareSubject(left, right domain.SecuritySubject) int {
	if value := strings.Compare(left.TreeDigest, right.TreeDigest); value != 0 {
		return value
	}
	return strings.Compare(left.ManifestDigest, right.ManifestDigest)
}

func compareFinding(left, right domain.SecurityFinding) int {
	leftKey := fmt.Sprintf("%s\x00%s\x00%s\x00%010d\x00%s", left.Disposition, left.Code, left.Path, left.Line, left.Message)
	rightKey := fmt.Sprintf("%s\x00%s\x00%s\x00%010d\x00%s", right.Disposition, right.Code, right.Path, right.Line, right.Message)
	return strings.Compare(leftKey, rightKey)
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
