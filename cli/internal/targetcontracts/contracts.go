package targetcontracts

type Entry struct {
	Target                 string            `json:"target"`
	PlatformFamily         string            `json:"platform_family"`
	TargetClass            string            `json:"target_class"`
	LauncherRequirement    string            `json:"launcher_requirement"`
	TargetNoun             string            `json:"target_noun,omitempty"`
	ProductionClass        string            `json:"production_class"`
	RuntimeContract        string            `json:"runtime_contract"`
	InstallModel           string            `json:"install_model,omitempty"`
	DevModel               string            `json:"dev_model,omitempty"`
	ActivationModel        string            `json:"activation_model,omitempty"`
	NativeRoot             string            `json:"native_root,omitempty"`
	ImportSupport          bool              `json:"import_support"`
	GenerateSupport        bool              `json:"generate_support"`
	ValidateSupport        bool              `json:"validate_support"`
	PortableComponentKinds []string          `json:"portable_component_kinds"`
	TargetComponentKinds   []string          `json:"target_component_kinds"`
	NativeDocs             []string          `json:"native_docs,omitempty"`
	NativeDocPaths         map[string]string `json:"native_doc_paths,omitempty"`
	NativeSurfaces         []Surface         `json:"native_surfaces,omitempty"`
	NativeSurfaceTiers     map[string]string `json:"native_surface_tiers,omitempty"`
	ManagedArtifactRules   []ManagedArtifact `json:"managed_artifact_rules,omitempty"`
	ManagedArtifacts       []string          `json:"managed_artifacts"`
	Summary                string            `json:"summary"`
}

type Surface struct {
	Kind string `json:"kind"`
	Tier string `json:"tier"`
}

type ManagedArtifact struct {
	Path      string `json:"path"`
	Condition string `json:"condition,omitempty"`
}
