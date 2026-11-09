package agentpluginscli

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

// The SBOM describes validated, pinned dependency inventory. It deliberately
// does not turn UAP's canonical tree digest into a raw file SHA-256 checksum,
// attest publisher identity, or claim that an MCP server is safe at runtime.
func newDependencySBOMCommand(app App, opts *options) *cobra.Command {
	var kind string
	command := &cobra.Command{
		Use: "sbom",
		Short: "Export a deterministic CycloneDX or SPDX inventory of the verified local dependency lock",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := validateDepsOptions(opts); err != nil {
				return err
			}
			selected, err := app.selectScopeForOptions(opts)
			if err != nil {
				return err
			}
			if err := selected.checkProjectNamespacePaths(); err != nil {
				return err
			}
			report := inspectLocalDependencies(cmd.Context(), selected)
			if report.Outcome != dependencyCheckValid {
				return fmt.Errorf("%w: %s; run agentplugins deps check --format json for details", errDependencyCheckFailed, report.Outcome)
			}
			agreement, checks := inspectDependencyAgreement(cmd.Context(), selected)
			for _, check := range checks {
				if check.Outcome != dependencyCheckValid {
					return fmt.Errorf("dependency agreement changed before SBOM export: %s", check.Code)
				}
			}
			body, err := marshalDependencySBOM(agreement.lock, kind)
			if err != nil {
				return err
			}
			if issue := agreement.recheck(cmd.Context()); issue != nil {
				return fmt.Errorf("dependency agreement changed during SBOM export: %s", issue.Code)
			}
			_, err = cmd.OutOrStdout().Write(append(body, '\n'))
			return err
		},
	}
	command.Flags().StringVar(&kind, "type", "cyclonedx", "SBOM format: cyclonedx or spdx")
	return command
}

type sbomProperty struct {
	Name string `json:"name"`
	Value string `json:"value"`
}

type cdxComponent struct {
	Type string `json:"type"`
	Ref string `json:"bom-ref"`
	Name string `json:"name"`
	Version string `json:"version,omitempty"`
	Properties []sbomProperty `json:"properties,omitempty"`
}

type cdxDependency struct {
	Ref string `json:"ref"`
	DependsOn []string `json:"dependsOn"`
}

type cyclonedxDocument struct {
	BOMFormat string `json:"bomFormat"`
	SpecVersion string `json:"specVersion"`
	Version int `json:"version"`
	Components []cdxComponent `json:"components"`
	Dependencies []cdxDependency `json:"dependencies"`
}

type spdxCreationInfo struct {
	Creators []string `json:"creators"`
	Created string `json:"created"`
	Comment string `json:"comment,omitempty"`
}

type spdxPackage struct {
	Name string `json:"name"`
	ID string `json:"SPDXID"`
	Version string `json:"versionInfo,omitempty"`
	DownloadLocation string `json:"downloadLocation"`
	FilesAnalyzed bool `json:"filesAnalyzed"`
	LicenseConcluded string `json:"licenseConcluded"`
	LicenseDeclared string `json:"licenseDeclared"`
	Comment string `json:"comment,omitempty"`
}

type spdxRelationship struct {
	Element string `json:"spdxElementId"`
	Type string `json:"relationshipType"`
	Related string `json:"relatedSpdxElement"`
}

type spdxDocument struct {
	SPDXVersion string `json:"spdxVersion"`
	DataLicense string `json:"dataLicense"`
	ID string `json:"SPDXID"`
	Name string `json:"name"`
	Namespace string `json:"documentNamespace"`
	CreationInfo spdxCreationInfo `json:"creationInfo"`
	Packages []spdxPackage `json:"packages"`
	Relationships []spdxRelationship `json:"relationships"`
}

func orderedLockedPackages(lock environment.Lock) []environment.LockedPackage {
	packages := append([]environment.LockedPackage(nil), lock.Packages...)
	sort.Slice(packages, func(i, j int) bool { return packages[i].Alias < packages[j].Alias })
	return packages
}

func sbomDependencies(packages []environment.LockedPackage) map[string][]string {
	children := make(map[string][]string, len(packages))
	for _, pin := range packages {
		if _, ok := children[pin.Alias]; !ok {
			children[pin.Alias] = []string{}
		}
		for _, parent := range pin.RequiredBy {
			children[parent.Alias] = append(children[parent.Alias], pin.Alias)
		}
	}
	for key := range children {
		sort.Strings(children[key])
	}
	return children
}

func sbomSource(pin environment.LockedPackage) string {
	switch {
	case pin.Source.Git != nil:
		s := pin.Source.Git
		if s.Repository != "" {
			return "git:" + s.Repository + "@" + s.Commit
		}
		return "git-commit:" + s.Commit
	case pin.Source.Registry != nil:
		s := pin.Source.Registry
		return "directory:" + s.Distribution + "@" + s.Commit
	case pin.Source.Archive != nil:
		return "archive-sha256:" + pin.Source.Archive.SHA256
	case pin.Source.Web != nil:
		return "web-skill"
	default:
		return "unknown"
	}
}

func marshalDependencySBOM(lock environment.Lock, kind string) ([]byte, error) {
	switch kind {
	case "cyclonedx":
		doc := cyclonedxDocument{BOMFormat: "CycloneDX", SpecVersion: "1.5", Version: 1,
			Components: []cdxComponent{}, Dependencies: []cdxDependency{}}
		packages := orderedLockedPackages(lock)
		children := sbomDependencies(packages)
		for _, pin := range packages {
			ref := "urn:uap:" + pin.Alias
			doc.Components = append(doc.Components, cdxComponent{Type: "library", Ref: ref,
				Name: pin.Alias, Version: pin.Version, Properties: []sbomProperty{
					{Name: "uap:source", Value: sbomSource(pin)},
					{Name: "uap:tree-digest", Value: pin.TreeDigest},
					{Name: "uap:tree-digest-algorithm", Value: pin.DigestAlgorithm},
				}})
			dep := cdxDependency{Ref: ref, DependsOn: []string{}}
			for _, alias := range children[pin.Alias] {
				dep.DependsOn = append(dep.DependsOn, "urn:uap:"+alias)
			}
			doc.Dependencies = append(doc.Dependencies, dep)
		}
		return json.MarshalIndent(doc, "", "  ")
	case "spdx":
		// A fixed creation marker makes the inventory reproducible from its
		// lock alone; it is not the time of scanning or of the source commit.
		encoded, err := json.Marshal(lock)
		if err != nil {
			return nil, err
		}
		hash := sha256.Sum256(encoded)
		doc := spdxDocument{
			SPDXVersion: "SPDX-2.3", DataLicense: "CC0-1.0",
			ID: "SPDXRef-DOCUMENT", Name: "UAP dependency inventory",
			Namespace: "https://universal-agent-plugins.com/spdx/sha256-" + hex.EncodeToString(hash[:]),
			CreationInfo: spdxCreationInfo{Creators: []string{"Tool: universal-agent-plugins"},
				Created: "1970-01-01T00:00:00Z", Comment: "Reproducible creation marker, not an assessment timestamp."},
			Packages: []spdxPackage{}, Relationships: []spdxRelationship{},
		}
		packages := orderedLockedPackages(lock)
		children := sbomDependencies(packages)
		for _, pin := range packages {
			id := "SPDXRef-Package-" + pin.Alias
			doc.Packages = append(doc.Packages, spdxPackage{Name: pin.Alias, ID: id,
				Version: pin.Version, DownloadLocation: "NOASSERTION", FilesAnalyzed: false,
				LicenseConcluded: "NOASSERTION", LicenseDeclared: "NOASSERTION",
				Comment: "Source: " + sbomSource(pin) + "; UAP canonical tree digest (" + pin.DigestAlgorithm + "): " + pin.TreeDigest})
			doc.Relationships = append(doc.Relationships, spdxRelationship{
				Element: "SPDXRef-DOCUMENT", Type: "DESCRIBES", Related: id,
			})
			for _, child := range children[pin.Alias] {
				doc.Relationships = append(doc.Relationships, spdxRelationship{
					Element: id, Type: "DEPENDS_ON", Related: "SPDXRef-Package-" + child,
				})
			}
		}
		return json.MarshalIndent(doc, "", "  ")
	default:
		return nil, errors.New("unsupported SBOM type: choose cyclonedx or spdx")
	}
}
