package installer

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/clientdetect"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/opencode"
)

// Old source refuses the second Prepare. Stored receipt IDs must survive both
// directions through the actual facade and remain usable without any host.
func TestOpenCodeNativeFacadeDurableRoundTrip(t *testing.T) {
	root := openCodeTestRoot(t)
	v1 := buildOpenCodeTarget(t, filepath.Join(root, "v1"), "1.18.34", "ok")
	v2 := buildOpenCodeTarget(t, filepath.Join(root, "v2"), "2.0.21", "ok")
	req := openCodeRequest(t, root, v1)
	req.InstallationID = "TEST-durable-roundtrip"
	cfg := Config{StateRoot: filepath.Join(root, "state"), HelperExecutable: v1, OpenCodeProbeEnvironment: []string{"PATH="}}
	engine := openCodeEngine(t, cfg)
	var ids []string
	for i, host := range []string{v1, v2, v1} {
		req.ClientExecutable = host
		if i > 0 {
			req.Operation = OpUpdate
		}
		handle, err := engine.Prepare(testCtx(t), req)
		if err != nil {
			t.Fatal(err)
		}
		result, err := engine.Apply(testCtx(t), handle, Decision{Confirmed: true})
		closeOpenCodeTestHandle(t, handle)
		if err != nil {
			t.Fatal(err)
		}
		req.InstallationID = result.InstallationID
		state, err := engine.store.Load()
		if err != nil {
			t.Fatal(err)
		}
		for _, b := range state.Installations[0].Clients {
			if b.NativeActivationAttempt != "" {
				t.Fatal("attempt not durably cleared")
			}
			if err := opencode.VerifyOpenCodeNativeObjects(req.ClientConfigRoot, b.TargetLocator, b.NativeObjects, nativeconfig.New(), false); err != nil {
				t.Fatal(err)
			}
			var current []string
			for _, o := range b.NativeObjects {
				codec, mcp, _ := nativeconfig.OpenCodeCodecForKind(o.Kind)
				if !mcp {
					continue
				}
				current = append(current, o.ObjectID)
				want := nativeconfig.CodecOpenCode
				if i == 1 {
					want = nativeconfig.CodecOpenCodeV2
				}
				if codec != want {
					t.Fatalf("generation %d: confirmed %s, want %s", i, codec, want)
				}
			}
			if i == 0 {
				ids = current
			} else {
				if len(ids) != len(current) {
					t.Fatal("logical identity count changed")
				}
				for j := range ids {
					if ids[j] != current[j] {
						t.Fatal("logical identity changed")
					}
				}
			}
		}
		// A fresh composition root must see neither a journal nor an attempt.
		engine = openCodeEngine(t, cfg)
		view, err := engine.Inspect(testCtx(t))
		if err != nil || view.Recovery.Required {
			t.Fatalf("reopened service: %+v %v", view, err)
		}
	}
	cfg.Runner = forbiddenV2Runner{t}
	cfg.EnableNativeObserver = true
	cfg.OpenCodeProbe = func(context.Context, clientdetect.ProbeTarget) (clientdetect.ProbeEvidence, error) {
		t.Fatal("stored operation probed host")
		return clientdetect.ProbeEvidence{}, errors.New("forbidden")
	}
	engine = openCodeEngine(t, cfg)
	if _, err := engine.RecoverCurrent(testCtx(t)); err != nil {
		t.Fatal(err)
	}
	req.Operation = OpRemove
	req.ClientExecutable = ""
	handle, err := engine.Prepare(testCtx(t), req)
	if err != nil {
		t.Fatal(err)
	}
	defer closeOpenCodeTestHandle(t, handle)
	if _, err := engine.Apply(testCtx(t), handle, Decision{Confirmed: true}); err != nil {
		t.Fatal(err)
	}
}

// A same-ID proposal is only a candidate: equal foreign destination, edited
// source, retained foreign source leaves and ambiguous config variants refuse
// during Prepare and preserve all config/skills/package/state bytes.
func TestOpenCodeNativeFacadeTransitionRefusesForeignFacts(t *testing.T) {
	for _, scenario := range []string{"equal-destination", "edited-source", "foreign-source", "ambiguous"} {
		t.Run(scenario, func(t *testing.T) {
			root := openCodeTestRoot(t)
			v1 := buildOpenCodeTarget(t, filepath.Join(root, "v1"), "1.18.34", "ok")
			v2 := buildOpenCodeTarget(t, filepath.Join(root, "v2"), "2.0.21", "ok")
			req := openCodeRequest(t, root, v1)
			req.InstallationID = "TEST-foreign-transition"
			engine := openCodeEngine(t, Config{StateRoot: filepath.Join(root, "state"), HelperExecutable: v1, OpenCodeProbeEnvironment: []string{"PATH="}})
			first, err := engine.Prepare(testCtx(t), req)
			if err != nil {
				t.Fatal(err)
			}
			result, err := engine.Apply(testCtx(t), first, Decision{Confirmed: true})
			closeOpenCodeTestHandle(t, first)
			if err != nil {
				t.Fatal(err)
			}
			paths := nativeconfig.Paths{JSON: filepath.Join(req.ClientConfigRoot, "opencode.json"), JSONC: filepath.Join(req.ClientConfigRoot, "opencode.jsonc")}
			body, err := os.ReadFile(paths.JSON)
			if err != nil {
				t.Fatal(err)
			}
			var cfg map[string]any
			if err := json.Unmarshal(body, &cfg); err != nil {
				t.Fatal(err)
			}
			mcp := cfg["mcp"].(map[string]any)
			switch scenario {
			case "equal-destination":
				old := mcp["sample-notify"].(map[string]any)
				mcp["servers"] = map[string]any{"sample-notify": map[string]any{"type": old["type"], "command": old["command"], "environment": old["environment"], "disabled": false}}
			case "edited-source":
				mcp["sample-notify"].(map[string]any)["foreign"] = true
			case "foreign-source":
				mcp["foreign"] = map[string]any{"type": "remote", "url": "https://foreign.invalid", "enabled": false}
			case "ambiguous":
				mustWriteV2(t, paths.JSONC, []byte(`{}`))
			}
			if scenario != "ambiguous" {
				edited, _ := json.Marshal(cfg)
				mustWriteV2(t, paths.JSON, edited)
			}
			before := v2EffectSnapshot(t, engine)
			req.Operation, req.InstallationID, req.ClientExecutable = OpUpdate, result.InstallationID, v2
			next, err := engine.Prepare(testCtx(t), req)
			if next != nil {
				closeOpenCodeTestHandle(t, next)
			}
			if err == nil {
				t.Fatal("foreign transition facts admitted")
			}
			if before != v2EffectSnapshot(t, engine) {
				t.Fatal("foreign refusal changed effects")
			}
		})
	}
}
