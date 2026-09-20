package pluginmanifest

type CompatibilityStatus string

const (
	CompatibilityFull        CompatibilityStatus = "full"
	CompatibilityPartial     CompatibilityStatus = "partial"
	CompatibilityUnsupported CompatibilityStatus = "unsupported"
)

type SourceCompatibility struct {
	Target           string              `json:"target"`
	Status           CompatibilityStatus `json:"status"`
	SupportedKinds   []string            `json:"supported_kinds"`
	UnsupportedKinds []string            `json:"unsupported_kinds"`
	Notes            []string            `json:"notes,omitempty"`
}

type SourceInspection struct {
	RequestedSource     string                `json:"requested_source"`
	ResolvedSource      string                `json:"resolved_source"`
	SourceKind          string                `json:"source_kind"`
	SourceDigest        string                `json:"source_digest"`
	CanonicalPackage    bool                  `json:"canonical_package"`
	ImportSource        string                `json:"import_source,omitempty"`
	DetectedImportKinds []string              `json:"detected_import_kinds,omitempty"`
	DroppedKinds        []string              `json:"dropped_kinds,omitempty"`
	OriginTargets       []string              `json:"origin_targets"`
	Inspection          Inspection            `json:"inspection"`
	Compatibility       []SourceCompatibility `json:"compatibility"`
}

func InspectSource(sourceRef string, from string, target string, includeUserScope bool) (SourceInspection, []Warning, error) {
	resolved, cleanup, err := resolveSourceRef(sourceRef)
	if err != nil {
		return SourceInspection{}, nil, err
	}
	defer cleanup()

	if isPackageStandardSource(resolved.LocalPath) {
		return inspectCanonicalSource(sourceRef, resolved, target)
	}
	return inspectImportedSource(sourceRef, resolved, from, target, includeUserScope)
}

func ImportSource(root string, sourceRef string, from string, force bool, includeUserScope bool) (Manifest, []Warning, error) {
	resolved, cleanup, err := resolveSourceRef(sourceRef)
	if err != nil {
		return Manifest{}, nil, err
	}
	defer cleanup()
	if isPackageStandardSource(resolved.LocalPath) {
		return Manifest{}, nil, errCanonicalSourceImport
	}
	prepared, err := prepareImportFromRoot(resolved.LocalPath, from, includeUserScope)
	if err != nil {
		return Manifest{}, prepared.Warnings, err
	}
	if err := writePreparedImport(root, prepared, force); err != nil {
		return prepared.Manifest, prepared.Warnings, err
	}
	return prepared.Manifest, prepared.Warnings, nil
}
