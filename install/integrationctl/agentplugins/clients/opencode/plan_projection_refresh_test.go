package opencode

import (
	"errors"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/contracttest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// Pure prepared-host fixtures exercise the adapter decision, not host probing.
type projectionDialectHost struct {
	contracttest.OpenCodeV1Host
	dialect string
}

func (h projectionDialectHost) ConfigDialect() string { return h.dialect }

type projectionHostWithoutDialect struct{}

func (projectionHostWithoutDialect) ValidateNative(bool, []string) error { return nil }

func TestPlanNativeProjectionRefreshDecision(t *testing.T) {
	for _, tc := range []struct {
		name     string
		dialect  string
		kinds    []string
		want     bool
		noMethod bool
	}{
		{name: "V1 matching", dialect: "opencode_v1", kinds: []string{OpenCodeMCPObjectKind}},
		{name: "V2 matching", dialect: "opencode_v2", kinds: []string{OpenCodeV2MCPObjectKind}},
		{name: "V1 to V2", dialect: "opencode_v2", kinds: []string{OpenCodeMCPObjectKind}, want: true},
		{name: "V2 to V1", dialect: "opencode_v1", kinds: []string{OpenCodeV2MCPObjectKind}, want: true},
		{name: "mixed to V1", dialect: "opencode_v1", kinds: []string{OpenCodeMCPObjectKind, OpenCodeV2MCPObjectKind}, want: true},
		{name: "mixed to V2", dialect: "opencode_v2", kinds: []string{OpenCodeV2MCPObjectKind, OpenCodeMCPObjectKind}, want: true},
		{name: "empty", dialect: "opencode_v2"},
		{name: "only package and skill", dialect: "opencode_v2", kinds: []string{"managed_package_directory", openCodeSkillKind}},
		{name: "missing host", kinds: []string{OpenCodeMCPObjectKind}},
		{name: "no dialect accessor", kinds: []string{OpenCodeMCPObjectKind}, noMethod: true},
		{name: "unknown dialect", dialect: "opencode_future", kinds: []string{OpenCodeMCPObjectKind}},
		{name: "unknown receipt", dialect: "opencode_v2", kinds: []string{"opencode_future_mcp_server"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			previous := make([]domain.NativeObjectOwnership, 0, len(tc.kinds))
			for _, kind := range tc.kinds {
				previous = append(previous, domain.NativeObjectOwnership{Kind: kind})
			}
			frozen := append([]domain.NativeObjectOwnership{}, previous...)
			var host domain.OpenCodeHostAuthority
			if tc.dialect != "" {
				host = projectionDialectHost{dialect: tc.dialect}
			} else if tc.noMethod {
				host = projectionHostWithoutDialect{}
			}
			input := clients.PlanInput{PreviousNativeObjects: previous, Client: domain.DetectedClient{OpenCodeHost: host}}
			// A reused plan must not retain an earlier decision. No components
			// means this also retains hostless metadata-only admission.
			plan := domain.DeliveryPlan{}.WithNativeProjectionRefresh(true)
			if err := (&Adapter{}).RefinePlan(t.Context(), input, &plan); err != nil {
				t.Fatal(err)
			}
			if plan.RequiresNativeProjectionRefresh() != tc.want || !reflect.DeepEqual(previous, frozen) {
				t.Fatalf("decision=%t want=%t, previous changed=%t", plan.RequiresNativeProjectionRefresh(), tc.want, !reflect.DeepEqual(previous, frozen))
			}
		})
	}
}

func TestPlanProjectionRefreshCannotAuthorizeMissingHost(t *testing.T) {
	input := clients.PlanInput{PreviousNativeObjects: []domain.NativeObjectOwnership{{Kind: OpenCodeMCPObjectKind}}}
	plan := (domain.DeliveryPlan{Components: []domain.ComponentDecision{{Kind: domain.ComponentMCPServer, Name: "docs", Support: domain.SupportPrepared}}}).WithNativeProjectionRefresh(true)
	if err := (&Adapter{}).RefinePlan(t.Context(), input, &plan); !errors.Is(err, clients.ErrOpenCodeAdapterUnavailable) || plan.RequiresNativeProjectionRefresh() {
		t.Fatalf("missing host admitted: plan=%+v, %v", plan, err)
	}
}
