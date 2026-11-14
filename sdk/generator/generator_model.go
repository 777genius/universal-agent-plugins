package generator

import (
	"fmt"
	"strings"

	"github.com/777genius/plugin-kit-ai/sdk/internal/descriptors/defs"
	"github.com/777genius/plugin-kit-ai/sdk/internal/runtime"
	"github.com/777genius/plugin-kit-ai/sdk/platformmeta"
)

type model struct {
	profiles    map[runtime.PlatformID]defs.PlatformProfile
	cliProfiles []platformmeta.PlatformProfile
	events      []defs.EventDescriptor
}

func loadModel() (model, error) {
	profiles := defs.Profiles()
	events := defs.Events()
	out := model{
		profiles:    make(map[runtime.PlatformID]defs.PlatformProfile, len(profiles)),
		cliProfiles: append([]platformmeta.PlatformProfile(nil), platformmeta.All()...),
		events:      events,
	}
	for _, p := range profiles {
		if p.Platform == "" {
			return model{}, fmt.Errorf("platform profile missing platform")
		}
		if _, ok := out.profiles[p.Platform]; ok {
			return model{}, fmt.Errorf("duplicate platform profile %s", p.Platform)
		}
		out.profiles[p.Platform] = p
	}
	if err := validateModel(out); err != nil {
		return model{}, err
	}
	return out, nil
}

func validateModel(m model) error {
	seenEvents := make(map[string]struct{}, len(m.events))
	for _, p := range m.profiles {
		if err := validateProfile(p); err != nil {
			return err
		}
	}

	registrars := make(map[string]struct{})
	for _, e := range m.events {
		key := string(e.Platform) + "/" + string(e.Event)
		if _, ok := seenEvents[key]; ok {
			return fmt.Errorf("duplicate event descriptor %s", key)
		}
		seenEvents[key] = struct{}{}
		p, ok := m.profiles[e.Platform]
		if !ok {
			return fmt.Errorf("event descriptor %s references unknown platform profile", key)
		}
		if p.Status != runtime.StatusRuntimeSupported {
			return fmt.Errorf("event descriptor %s targets non-runtime profile %s", key, p.Status)
		}
		if err := validateEventMetadata(e, key); err != nil {
			return err
		}
		regKey := p.PublicPackage + "." + e.Registrar.MethodName
		if _, ok := registrars[regKey]; ok {
			return fmt.Errorf("registrar collision %s", regKey)
		}
		registrars[regKey] = struct{}{}
		switch e.Carrier {
		case runtime.CarrierStdinJSON, runtime.CarrierArgvJSON:
		default:
			return fmt.Errorf("event descriptor %s has unsupported carrier %q", key, e.Carrier)
		}
		switch e.Invocation.Kind {
		case runtime.InvocationArgvCommand, runtime.InvocationArgvCommandCaseFold:
		case runtime.InvocationCustomResolver:
			if strings.TrimSpace(e.Invocation.ResolverRef) == "" {
				return fmt.Errorf("event descriptor %s missing custom resolver ref", key)
			}
		default:
			return fmt.Errorf("event descriptor %s has unsupported invocation kind %q", key, e.Invocation.Kind)
		}
	}
	return nil
}

func validateProfile(p defs.PlatformProfile) error {
	if p.Status == "" {
		return fmt.Errorf("platform profile %s missing status", p.Platform)
	}
	if p.PublicPackage == "" || p.InternalPackage == "" || p.InternalImport == "" {
		return fmt.Errorf("platform profile %s missing package metadata", p.Platform)
	}
	if len(p.TransportModes) == 0 {
		return fmt.Errorf("platform profile %s missing transport modes", p.Platform)
	}
	if p.RuntimeOnly && (len(p.Scaffold.RequiredFiles) > 0 || len(p.Scaffold.TemplateFiles) > 0 || len(p.Validate.RequiredFiles) > 0) {
		return fmt.Errorf("runtime-only platform profile %s has authoring metadata", p.Platform)
	}
	if p.Status != runtime.StatusDeferred && !p.RuntimeOnly {
		if len(p.Scaffold.RequiredFiles) == 0 || len(p.Scaffold.TemplateFiles) == 0 {
			return fmt.Errorf("platform profile %s missing scaffold metadata", p.Platform)
		}
		if len(p.Validate.RequiredFiles) == 0 {
			return fmt.Errorf("platform profile %s missing validate metadata", p.Platform)
		}
	}
	return nil
}

func validateEventMetadata(e defs.EventDescriptor, key string) error {
	if e.Invocation.Kind == "" {
		return fmt.Errorf("event descriptor %s missing invocation kind", key)
	}
	if e.Invocation.Kind != runtime.InvocationCustomResolver && strings.TrimSpace(e.Invocation.Name) == "" {
		return fmt.Errorf("event descriptor %s missing invocation name", key)
	}
	if e.Contract.Maturity == "" {
		return fmt.Errorf("event descriptor %s missing contract maturity", key)
	}
	if e.DecodeFunc == "" || e.EncodeFunc == "" {
		return fmt.Errorf("event descriptor %s missing codec refs", key)
	}
	if e.Registrar.MethodName == "" || e.Registrar.WrapFunc == "" {
		return fmt.Errorf("event descriptor %s missing registrar metadata", key)
	}
	if e.Docs.Summary == "" || e.Docs.SnippetKey == "" || e.Docs.TableGroup == "" {
		return fmt.Errorf("event descriptor %s missing docs metadata", key)
	}
	return nil
}
