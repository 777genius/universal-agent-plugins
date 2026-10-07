package opencode

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/atomicfile"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

const transitionRecordFile = "opencode-native-transition.json"
const maxTransitionRecordBytes = 8 << 20
const maxTransitionSkills = 256

type transitionSkill struct {
	ID, Name, Target, Backup, Staged string
	Old, New                         *domain.NativeObjectOwnership
	OldExists                        bool
}
type transitionRecord struct {
	Version               int
	Type, Phase, Hash     string
	Identity              domain.NativeAttemptIdentity
	Root, ProjectionHash  string
	Native                domain.OpenCodeTransitionPrepared
	OldState, TargetState domain.StateFileV2
	Skills                []transitionSkill
}

// NativeTransitions consumes only stored roots and receipt/state authority. It
// never resolves a host and never acquires the outer operation lock itself.
type NativeTransitions struct {
	PackageVerifier interface {
		Verify(context.Context, string, string) error
	}
	State  ports.NativeTransitionState
	Kernel nativeconfig.Kernel
}

func transitionHash(body []byte) string {
	sum := sha256.Sum256(body)
	return fmt.Sprintf("sha256:%x", sum)
}

func transitionAuthorityHash(record transitionRecord) string {
	record.Phase, record.Hash = "", ""
	body, _ := json.Marshal(record)
	return transitionHash(body)
}
func comparableTransitionState(state domain.StateFileV2) []byte {
	state.Installations = append([]domain.Installation(nil), state.Installations...)
	sort.Slice(state.Installations, func(i, j int) bool {
		return state.Installations[i].InstallationID < state.Installations[j].InstallationID
	})
	body, _ := json.Marshal(state)
	return body
}
func transitionBinding(state domain.StateFileV2, id domain.NativeAttemptIdentity) (int, domain.ClientBinding, error) {
	for i, installation := range state.Installations {
		if installation.InstallationID == id.InstallationID {
			b, ok := installation.Clients[id.BindingID]
			if ok && b.ClientBindingID == id.BindingID && b.ClientID == "opencode" && storedTransitionRoot(b) == id.NativeRoot {
				return i, b, nil
			}
		}
	}
	return -1, domain.ClientBinding{}, fmt.Errorf("transition binding identity changed")
}
func cloneTransitionState(state domain.StateFileV2) domain.StateFileV2 {
	var clone domain.StateFileV2
	body, _ := json.Marshal(state)
	_ = json.Unmarshal(body, &clone)
	return clone
}
func sourceTransitionState(r transitionRecord) domain.StateFileV2 {
	state := cloneTransitionState(r.OldState)
	i, b, _ := transitionBinding(state, r.Identity)
	b.NativeActivationAttempt = ""
	state.Installations[i].Clients[r.Identity.BindingID] = b
	return state
}

func nativePrepared(d domain.OpenCodeTransitionPrepared) nativeconfig.PreparedTransition {
	receipt := func(r domain.OpenCodeTransitionReceipt) nativeconfig.Receipt {
		return nativeconfig.Receipt{Version: r.Version, Path: r.Path, Codec: nativeconfig.Codec(r.Codec), Name: r.Name, Digest: r.Digest}
	}
	p := nativeconfig.PreparedTransition{Path: d.Path, Original: nativeconfig.FileSnapshot{Body: d.OriginalBytes, Mode: os.FileMode(d.OriginalMode), Exists: d.OriginalExists}, TargetBytes: d.TargetBytes, TargetHash: d.TargetHash, SourceCodec: nativeconfig.Codec(d.SourceCodec), TargetCodec: nativeconfig.Codec(d.TargetCodec)}
	for _, e := range d.Entries {
		v := nativeconfig.PreparedTransitionEntry{LogicalID: e.LogicalID, Name: e.Name, SourceReceipt: receipt(e.SourceReceipt), TargetReceipt: receipt(e.TargetReceipt)}
		if e.PreviouslyOwnedTarget != nil {
			v.PreviouslyOwnedTarget = new(nativeconfig.Receipt)
			*v.PreviouslyOwnedTarget = receipt(*e.PreviouslyOwnedTarget)
		}
		p.Entries = append(p.Entries, v)
	}
	return p
}
func transitionPaths(root string) nativeconfig.Paths {
	return nativeconfig.Paths{JSON: filepath.Join(root, "opencode.json"), JSONC: filepath.Join(root, "opencode.jsonc")}
}

func (t NativeTransitions) PersistPrepared(ctx context.Context, id domain.NativeAttemptIdentity, root string, d domain.OpenCodeTransitionPrepared, previous, desired []domain.NativeObjectOwnership) error {
	before, binding, err := t.prepareTransitionAuthority(ctx, id, previous)
	if err != nil {
		return err
	}
	target := preparedTransitionState(before, id, binding, desired)
	if t.PackageVerifier == nil {
		return fmt.Errorf("native package verifier missing")
	}
	if err := t.PackageVerifier.Verify(ctx, binding.TargetLocator, shared.ManagedPackageDigest(binding)); err != nil {
		return err
	}
	projection, err := readTransitionFile(binding.TargetLocator, filepath.Join(binding.TargetLocator, OpenCodeProjectionFile), nativeconfig.MaxTransitionConfigBytes)
	if err != nil {
		return err
	}
	r := transitionRecord{Version: 1, Type: "opencode_native_transition", Phase: "prepared", Identity: id, Root: root, ProjectionHash: transitionHash(projection), Native: d, OldState: cloneTransitionState(before), TargetState: target}
	r.Skills, err = prepareTransitionSkills(root, previous, desired)
	if err != nil {
		return err
	}
	if err := validateTransitionRecord(r, root); err != nil {
		return err
	}
	return writeTransitionRecord(r)
}

func (t NativeTransitions) prepareTransitionAuthority(ctx context.Context, id domain.NativeAttemptIdentity, previous []domain.NativeObjectOwnership) (domain.StateFileV2, domain.ClientBinding, error) {
	if err := ctx.Err(); err != nil {
		return domain.StateFileV2{}, domain.ClientBinding{}, err
	}
	if t.State == nil {
		return domain.StateFileV2{}, domain.ClientBinding{}, fmt.Errorf("native transition state authority missing")
	}
	before, err := t.State.Load()
	if err != nil {
		return before, domain.ClientBinding{}, err
	}
	_, binding, err := transitionBinding(before, id)
	if err != nil {
		return before, binding, err
	}
	if id.OperationID == "" || binding.NativeActivationAttempt != id.OperationID {
		return before, binding, fmt.Errorf("native attempt identity changed")
	}
	if !reflect.DeepEqual(OpenCodeObjects(binding.NativeObjects), OpenCodeObjects(previous)) {
		return before, binding, fmt.Errorf("native source ownership changed")
	}
	return before, binding, nil
}

func preparedTransitionState(before domain.StateFileV2, id domain.NativeAttemptIdentity, binding domain.ClientBinding, desired []domain.NativeObjectOwnership) domain.StateFileV2 {
	// The directory-committed package is authoritative and distinct from the
	// external native leaves this transition owns.
	objects := append([]domain.NativeObjectOwnership(nil), OpenCodeObjects(desired)...)
	for _, object := range binding.NativeObjects {
		if object.Kind == "managed_package_directory" {
			objects = append(objects, object)
		}
	}
	return transitionTargetState(before, id, objects)
}

func transitionTargetState(before domain.StateFileV2, id domain.NativeAttemptIdentity, objects []domain.NativeObjectOwnership) domain.StateFileV2 {
	target := cloneTransitionState(before)
	i, binding, _ := transitionBinding(target, id)
	binding.NativeObjects = objects
	binding.NativeActivationAttempt = ""
	binding.Activation = domain.ActivationActive
	binding.Verification = domain.VerificationInstalled
	binding.Materialization = domain.MaterializationMaterialized
	binding.Policy = domain.PolicyAllowed
	target.Installations[i].Clients[id.BindingID] = binding
	return target
}

func prepareTransitionSkills(root string, previous, desired []domain.NativeObjectOwnership) ([]transitionSkill, error) {
	previousByID := shared.ObjectMap(OpenCodeObjects(previous))
	desiredByID := shared.ObjectMap(OpenCodeObjects(desired))
	names := transitionSkillIDs(previousByID, desiredByID)
	var skills []transitionSkill
	for id := range names {
		skill := prepareTransitionSkill(root, id, previousByID, desiredByID)
		if err := snapshotTransitionSkill(&skill); err != nil {
			return nil, err
		}
		skills = append(skills, skill)
	}
	sort.Slice(skills, func(i, j int) bool { return skills[i].ID < skills[j].ID })
	return skills, nil
}

func transitionSkillIDs(previous, desired map[string]domain.NativeObjectOwnership) map[string]bool {
	names := map[string]bool{}
	for _, objects := range []map[string]domain.NativeObjectOwnership{previous, desired} {
		for id, object := range objects {
			if object.Kind == openCodeSkillKind {
				names[id] = true
			}
		}
	}
	return names
}

func prepareTransitionSkill(root, id string, previous, desired map[string]domain.NativeObjectOwnership) transitionSkill {
	skill := transitionSkill{ID: id}
	if old, ok := previous[id]; ok {
		skill.Old = &old
		skill.Name = old.LogicalName
		skill.Target = old.Path
	}
	if next, ok := desired[id]; ok {
		skill.New = &next
		skill.Name = next.LogicalName
		skill.Target = next.Path
		skill.Staged = filepath.Join(root, "new-"+skill.Name)
	}
	skill.Backup = filepath.Join(root, "old-"+skill.Name)
	return skill
}

func snapshotTransitionSkill(skill *transitionSkill) error {
	if skill.Old == nil {
		return nil
	}
	digest, err := shared.DigestSkillDirectory(skill.Target)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil || digest != skill.Old.ManagedDigest {
		return fmt.Errorf("source skill changed")
	}
	skill.OldExists = true
	return syncTransitionTree(skill.Target)
}

func validTransitionHash(value string) bool {
	if len(value) != 71 || !strings.HasPrefix(value, "sha256:") {
		return false
	}
	for _, c := range value[7:] {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}

func writeTransitionRecord(r transitionRecord) error {
	if err := validateTransitionRecord(r, r.Root); err != nil {
		return err
	}
	r.Hash = ""
	body, err := json.Marshal(r)
	if err != nil {
		return err
	}
	r.Hash = transitionHash(body)
	body, err = json.Marshal(r)
	if err != nil {
		return err
	}
	if len(body) > maxTransitionRecordBytes {
		return fmt.Errorf("native transition record exceeds size limit")
	}
	path := filepath.Join(r.Root, transitionRecordFile)
	if err := pathpolicy.RequireContainedChild(r.Root, path); err != nil {
		return err
	}
	if err := atomicfile.Write(path, body, 0600); err != nil {
		return err
	}
	// Persist discoverability of the provider transaction directory too.
	return atomicfile.SyncDirectory(filepath.Dir(r.Root))
}
func readTransitionFile(root, path string, limit int64) ([]byte, error) {
	if err := pathpolicy.RequireContainedChild(root, path); err != nil {
		return nil, err
	}
	return nativeconfig.ReadTransitionFileNoFollow(path, limit)
}
func readTransitionRecord(root string) (transitionRecord, error) {
	var r transitionRecord
	body, err := readTransitionFile(root, filepath.Join(root, transitionRecordFile), maxTransitionRecordBytes)
	if err != nil {
		return r, err
	}
	info, err := os.Lstat(filepath.Join(root, transitionRecordFile))
	if err != nil || info.Mode().Perm() != 0600 {
		return r, fmt.Errorf("transition record must be private")
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&r); err != nil {
		return r, fmt.Errorf("invalid transition record")
	}
	if err := requireJSONEOF(dec); err != nil {
		return r, err
	}
	if err := rejectTransitionDuplicateKeys(json.NewDecoder(bytes.NewReader(body)), 0); err != nil {
		return r, err
	}
	hash := r.Hash
	r.Hash = ""
	unsigned, _ := json.Marshal(r)
	r.Hash = hash
	if hash != transitionHash(unsigned) {
		return r, fmt.Errorf("transition record hash mismatch")
	}
	return r, validateTransitionRecord(r, root)
}

func storedTransitionRoot(b domain.ClientBinding) string {
	root := b.NativeProfileRoot
	for _, o := range OpenCodeObjects(b.NativeObjects) {
		codec, mcp, err := nativeconfig.OpenCodeCodecForKind(o.Kind)
		_ = codec
		if err != nil {
			return ""
		}
		candidate := ""
		if mcp {
			candidate = filepath.Dir(o.Path)
		} else if o.Kind == openCodeSkillKind {
			candidate = filepath.Dir(filepath.Dir(o.Path))
		}
		if candidate == "" {
			continue
		}
		if root != "" && root != candidate {
			return ""
		}
		root = candidate
	}
	return root
}

// A closed private record cannot have two competing values for any field.
// This is record validation, not a document mutation/patch API.
func rejectTransitionDuplicateKeys(dec *json.Decoder, depth int) error {
	if depth > 64 {
		return fmt.Errorf("transition record nesting exceeds limit")
	}
	token, err := dec.Token()
	if err != nil {
		return err
	}
	delimiter, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delimiter {
	case '{':
		seen := map[string]bool{}
		for dec.More() {
			token, err := dec.Token()
			if err != nil {
				return err
			}
			key, ok := token.(string)
			if !ok || seen[key] {
				return fmt.Errorf("ambiguous duplicate transition field")
			}
			seen[key] = true
			if err := rejectTransitionDuplicateKeys(dec, depth+1); err != nil {
				return err
			}
		}
	case '[':
		for dec.More() {
			if err := rejectTransitionDuplicateKeys(dec, depth+1); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("invalid transition delimiter")
	}
	_, err = dec.Token()
	return err
}

// Recovery authority is the selected binding and its package/data identity.
// Unrelated installations and sibling lifecycle timestamps are not rollback
// authority. Each actual Save still uses the existing whole-state ambiguity CAS.
func transitionStateMatches(current, bound domain.StateFileV2, id domain.NativeAttemptIdentity) bool {
	narrow := func(state domain.StateFileV2) (domain.Installation, bool) {
		i, b, err := transitionBinding(state, id)
		if err != nil || state.SchemaVersion != domain.StateSchemaVersion {
			return domain.Installation{}, false
		}
		in := state.Installations[i]
		in.Clients = map[string]domain.ClientBinding{id.BindingID: b}
		in.UpdatedAt = ""
		data := map[string]domain.DataReceipt{}
		if b.DataReceiptID != "" {
			receipt, ok := in.DataReceipts[b.DataReceiptID]
			if !ok {
				return domain.Installation{}, false
			}
			data[b.DataReceiptID] = receipt
		}
		in.DataReceipts = data
		return in, true
	}
	left, lok := narrow(current)
	right, rok := narrow(bound)
	return lok && rok && reflect.DeepEqual(left, right)
}
func transitionPublicationState(current, bound domain.StateFileV2, id domain.NativeAttemptIdentity) (domain.StateFileV2, error) {
	state := cloneTransitionState(current)
	i, _, err := transitionBinding(state, id)
	if err != nil {
		return state, err
	}
	_, binding, err := transitionBinding(bound, id)
	if err != nil {
		return state, err
	}
	state.Installations[i].Clients[id.BindingID] = binding
	return state, nil
}
