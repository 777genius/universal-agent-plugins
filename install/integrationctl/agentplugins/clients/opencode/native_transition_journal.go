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
	if err := ctx.Err(); err != nil {
		return err
	}
	if t.State == nil {
		return fmt.Errorf("native transition state authority missing")
	}
	before, err := t.State.Load()
	if err != nil {
		return err
	}
	i, b, err := transitionBinding(before, id)
	if err != nil {
		return err
	}
	if id.OperationID == "" || b.NativeActivationAttempt != id.OperationID {
		return fmt.Errorf("native attempt identity changed")
	}
	if !reflect.DeepEqual(OpenCodeObjects(b.NativeObjects), OpenCodeObjects(previous)) {
		return fmt.Errorf("native source ownership changed")
	}
	// Preserve the directory-committed package object. It is already authoritative
	// and distinct from the external native leaves this transition owns.
	objects := append([]domain.NativeObjectOwnership(nil), OpenCodeObjects(desired)...)
	for _, o := range b.NativeObjects {
		if o.Kind == "managed_package_directory" {
			objects = append(objects, o)
		}
	}
	target := cloneTransitionState(before)
	b.NativeObjects = objects
	b.NativeActivationAttempt = ""
	b.Activation = domain.ActivationActive
	b.Verification = domain.VerificationInstalled
	b.Materialization = domain.MaterializationMaterialized
	b.Policy = domain.PolicyAllowed
	target.Installations[i].Clients[id.BindingID] = b
	if t.PackageVerifier == nil {
		return fmt.Errorf("native package verifier missing")
	}
	if err := t.PackageVerifier.Verify(ctx, b.TargetLocator, shared.ManagedPackageDigest(b)); err != nil {
		return err
	}
	projection, err := readTransitionFile(b.TargetLocator, filepath.Join(b.TargetLocator, OpenCodeProjectionFile), nativeconfig.MaxTransitionConfigBytes)
	if err != nil {
		return err
	}
	r := transitionRecord{Version: 1, Type: "opencode_native_transition", Phase: "prepared", Identity: id, Root: root, ProjectionHash: transitionHash(projection), Native: d, OldState: cloneTransitionState(before), TargetState: target}
	previousByID, desiredByID := shared.ObjectMap(OpenCodeObjects(previous)), shared.ObjectMap(OpenCodeObjects(desired))
	names := map[string]bool{}
	for id, o := range previousByID {
		if o.Kind == openCodeSkillKind {
			names[id] = true
		}
	}
	for id, o := range desiredByID {
		if o.Kind == openCodeSkillKind {
			names[id] = true
		}
	}
	for id := range names {
		s := transitionSkill{ID: id}
		if old, ok := previousByID[id]; ok {
			s.Old = &old
			s.Name = old.LogicalName
			s.Target = old.Path
		}
		if next, ok := desiredByID[id]; ok {
			s.New = &next
			s.Name = next.LogicalName
			s.Target = next.Path
			s.Staged = filepath.Join(root, "new-"+s.Name)
		}
		s.Backup = filepath.Join(root, "old-"+s.Name)
		if s.Old != nil {
			digest, err := shared.DigestSkillDirectory(s.Target)
			if !os.IsNotExist(err) {
				if err != nil || digest != s.Old.ManagedDigest {
					return fmt.Errorf("source skill changed")
				}
				s.OldExists = true
				if err := syncTransitionTree(s.Target); err != nil {
					return err
				}
			}
		}
		r.Skills = append(r.Skills, s)
	}
	sort.Slice(r.Skills, func(i, j int) bool { return r.Skills[i].ID < r.Skills[j].ID })
	if err := validateTransitionRecord(r, root); err != nil {
		return err
	}
	return writeTransitionRecord(r)
}

func validateTransitionRecord(r transitionRecord, root string) error {
	if r.Version != 1 || r.Type != "opencode_native_transition" || r.Root != root || len(r.Skills) > maxTransitionSkills {
		return fmt.Errorf("invalid transition record schema")
	}
	switch r.Phase {
	case "prepared", "native_committed", "state_committed", "cleanup_pending":
	default:
		return fmt.Errorf("invalid transition phase")
	}
	for _, id := range []string{r.Identity.OperationID, r.Identity.InstallationID, r.Identity.BindingID} {
		if err := pathpolicy.ValidateLeafID(id); err != nil {
			return err
		}
	}
	nativeRoot := r.Identity.NativeRoot
	if !filepath.IsAbs(nativeRoot) || filepath.Clean(nativeRoot) != nativeRoot || filepath.Dir(root) != filepath.Join(nativeRoot, "skills") || !strings.HasPrefix(filepath.Base(root), ".agentplugins-native-") {
		return fmt.Errorf("invalid transition root")
	}
	if err := pathpolicy.RequireContainedChild(nativeRoot, root); err != nil {
		return err
	}
	info, err := os.Lstat(root)
	if err != nil || !info.IsDir() || info.Mode().Perm() != 0700 {
		return fmt.Errorf("transition root must be private")
	}
	if err := nativeconfig.ValidatePreparedTransition(transitionPaths(nativeRoot), nativePrepared(r.Native)); err != nil {
		return err
	}
	_, old, err := transitionBinding(r.OldState, r.Identity)
	if err != nil {
		return err
	}
	_, target, err := transitionBinding(r.TargetState, r.Identity)
	if err != nil {
		return err
	}
	if old.NativeActivationAttempt != r.Identity.OperationID || target.NativeActivationAttempt != "" || old.PackageRevision == nil || old.TargetLocator == "" || !filepath.IsAbs(old.TargetLocator) {
		return fmt.Errorf("invalid bound attempt/package")
	}
	oldPackage, targetPackage := []domain.NativeObjectOwnership{}, []domain.NativeObjectOwnership{}
	for _, side := range []struct {
		objects        []domain.NativeObjectOwnership
		packageObjects *[]domain.NativeObjectOwnership
	}{{old.NativeObjects, &oldPackage}, {target.NativeObjects, &targetPackage}} {
		for _, o := range side.objects {
			if o.Kind == "managed_package_directory" {
				*side.packageObjects = append(*side.packageObjects, o)
			} else {
				_, mcp, err := nativeconfig.OpenCodeCodecForKind(o.Kind)
				if err != nil || !mcp && o.Kind != openCodeSkillKind {
					return fmt.Errorf("transition contains unbound native objects")
				}
			}
		}
	}
	if len(oldPackage) != 1 || oldPackage[0].Path != old.TargetLocator || !validTransitionHash(oldPackage[0].ManagedDigest) {
		return fmt.Errorf("transition has no exact directory-committed package")
	}
	if !reflect.DeepEqual(oldPackage, targetPackage) {
		return fmt.Errorf("transition changes directory-committed package ownership")
	}
	expected := cloneTransitionState(r.OldState)
	i, b, _ := transitionBinding(expected, r.Identity)
	b.NativeObjects = target.NativeObjects
	b.NativeActivationAttempt = ""
	b.Activation = domain.ActivationActive
	b.Verification = domain.VerificationInstalled
	b.Materialization = domain.MaterializationMaterialized
	b.Policy = domain.PolicyAllowed
	expected.Installations[i].Clients[r.Identity.BindingID] = b
	if !bytes.Equal(comparableTransitionState(expected), comparableTransitionState(r.TargetState)) {
		return fmt.Errorf("transition changes unbound state")
	}
	for _, side := range []struct {
		objects []domain.NativeObjectOwnership
		codec   string
		source  bool
	}{{old.NativeObjects, r.Native.SourceCodec, true}, {target.NativeObjects, r.Native.TargetCodec, false}} {
		objects := shared.ObjectMap(OpenCodeObjects(side.objects))
		mcpCount := 0
		for _, o := range objects {
			codec, mcp, err := nativeconfig.OpenCodeCodecForKind(o.Kind)
			if err != nil {
				return err
			}
			if err := validateOpenCodeObject(nativeRoot, OpenCodeProjection{}, o); err != nil {
				return err
			}
			if mcp {
				if string(codec) != side.codec {
					return fmt.Errorf("mixed stored codecs")
				}
				mcpCount++
			}
		}
		if mcpCount != len(r.Native.Entries) {
			return fmt.Errorf("incomplete transition ownership")
		}
		for _, entry := range r.Native.Entries {
			o, ok := objects[entry.LogicalID]
			receipt := entry.TargetReceipt
			if side.source {
				receipt = entry.SourceReceipt
			}
			if !ok || o.LogicalName != entry.Name || o.Path != receipt.Path || o.ManagedDigest != receipt.Digest {
				return fmt.Errorf("transition receipt/state mismatch")
			}
		}
	}
	oldObjects, nextObjects := shared.ObjectMap(OpenCodeObjects(old.NativeObjects)), shared.ObjectMap(OpenCodeObjects(target.NativeObjects))
	skillCount := 0
	for _, o := range oldObjects {
		if o.Kind == openCodeSkillKind {
			skillCount++
		}
	}
	for id, o := range nextObjects {
		if o.Kind == openCodeSkillKind {
			if _, ok := oldObjects[id]; !ok {
				skillCount++
			}
		}
	}
	if skillCount != len(r.Skills) {
		return fmt.Errorf("incomplete skill intent")
	}
	for i, s := range r.Skills {
		if err := pathpolicy.ValidateLeafID(s.Name); err != nil {
			return err
		}
		if s.ID != "opencode-skill:"+s.Name || i > 0 && r.Skills[i-1].ID >= s.ID || s.Target != filepath.Join(nativeRoot, "skills", s.Name) || s.Backup != filepath.Join(root, "old-"+s.Name) || s.Old == nil && s.New == nil || s.Old == nil && s.OldExists {
			return fmt.Errorf("invalid skill intent")
		}
		old, oldOK := oldObjects[s.ID]
		next, nextOK := nextObjects[s.ID]
		if oldOK != (s.Old != nil) || nextOK != (s.New != nil) || oldOK && !reflect.DeepEqual(old, *s.Old) || nextOK && !reflect.DeepEqual(next, *s.New) {
			return fmt.Errorf("skill intent/state mismatch")
		}
		if s.New != nil && s.Staged != filepath.Join(root, "new-"+s.Name) || s.New == nil && s.Staged != "" {
			return fmt.Errorf("invalid staged skill path")
		}
		for _, path := range []string{s.Target, s.Backup, s.Staged} {
			if path != "" {
				if err := pathpolicy.RequireContainedChild(nativeRoot, path); err != nil {
					return err
				}
			}
		}
	}
	if !validTransitionHash(r.ProjectionHash) {
		return fmt.Errorf("invalid projection hash")
	}
	return nil
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
