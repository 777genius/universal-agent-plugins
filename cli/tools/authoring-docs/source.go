package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// Audited factory identity, independent of the integrated adapter commit.
const factoryBaselineSHA = "070663efb27f69ecae8609e6b839f86f843efbb0"

type sourcePin struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

func git(checkout string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(context.Background(), "git", append([]string{"-C", checkout}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_OPTIONAL_LOCKS=0")
	body, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("source checkout verification failed")
	}
	return body, nil
}
func validateSource(checkout, sha string) ([]sourcePin, error) {
	if !regexp.MustCompile(`^[0-9a-f]{40}$`).MatchString(sha) {
		return nil, fmt.Errorf("source identity must be a lowercase full Git SHA")
	}
	if checkout == "" {
		return nil, fmt.Errorf("--checkout is required")
	}
	head, err := git(checkout, "rev-parse", "HEAD")
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(string(head)) != sha {
		return nil, fmt.Errorf("source identity differs from checkout HEAD")
	}
	// A simple tracked cleanliness check includes staged and unstaged changes.
	changed, err := git(checkout, "status", "--porcelain", "--untracked-files=no")
	if err != nil {
		return nil, err
	}
	if len(bytes.TrimSpace(changed)) != 0 {
		return nil, fmt.Errorf("tracked checkout differs from source identity")
	}
	if err := validateInventory(checkout); err != nil {
		return nil, err
	}
	pins := make([]sourcePin, 0, len(factoryPins))
	for _, pin := range factoryPins {
		if err := validateSourcePin(checkout, pin); err != nil {
			return nil, err
		}
		pins = append(pins, pin)
	}
	return pins, nil
}

func validateSourcePin(checkout string, pin sourcePin) error {
	path := filepath.Join(checkout, filepath.FromSlash(pin.Path))
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("source input unavailable: %s", pin.Path)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("source input is not a regular file: %s", pin.Path)
	}
	if _, err := git(checkout, "ls-files", "--error-unmatch", "--", pin.Path); err != nil {
		return fmt.Errorf("source input not tracked: %s", pin.Path)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("source input unavailable: %s", pin.Path)
	}
	if fmt.Sprintf("%x", sha256.Sum256(body)) != pin.SHA256 {
		return fmt.Errorf("source input mismatch: %s", pin.Path)
	}
	return nil
}

// Pins cover the release-selected construction packages (all non-test Go files,
// including definitions beside callbacks), target registry, composition
// references and all six workspace module controls. Current-only bootstrap
// stays outside this inventory until the released source pin advances.
var factoryPins = []sourcePin{
	{"cli/cmd/agentplugins/release_root.go", "4a7dc6735325c1f1cd65d063f349e04716d36ab7c9eaeb4749ec4f5bcb03cd28"},
	{"cli/cmd/plugin-kit-ai/release_compat.go", "d2f11a845c116160114ddd1b0ba8b825e9e5a4794354742749db7122c0c674c8"},
	{"cli/go.mod", "6bbbc3d2a749bda99a5f426a2f0a5c9d3d8681ac15e0080ac039df735b3547a6"},
	{"cli/go.sum", "ea4cb8206732d6b715288572b619b2c679aefe1a66d84a08d226d5c8daa73b98"},
	{"cli/internal/agentpluginscli/add.go", "b630ec68604d361b28af240fb774d2c76eb967aa7d15ef3988c973d9c7c7eaeb"},
	{"cli/internal/agentpluginscli/add_multi.go", "9d1f4b1a556b1cdd2acccda727a020fa95c21f1d871dea467c6892a61e022d44"},
	{"cli/internal/agentpluginscli/app.go", "92292a21b1439655c60f15715aa13814492b3545ca6d2a35aa09304c02dc58fe"},
	{"cli/internal/agentpluginscli/binding.go", "d642368e4f4d0ef8f228a6939fcf011751ce8d7804506c6fbffd5cb6fbfd0842"},
	{"cli/internal/agentpluginscli/chatgpt_guidance.go", "7dacf7fcb0581cfff8f7aabfc85cd21cef15b93486376f31eba25ad39711e19a"},
	{"cli/internal/agentpluginscli/color_error.go", "950bda3855f187a6670e6de60eda11757169f5d37b3d04fb70811706be4c1930"},
	{"cli/internal/agentpluginscli/engine_boundary.go", "22b81bef6608c5ecf9b17a232b375c0cfde700ec8832f9e5677afe0cb73afc19"},
	{"cli/internal/agentpluginscli/interactive_targets.go", "d09291bb12e303cac81c17ebef279e50e696dd32175311f4aae0c23e6bea3652"},
	{"cli/internal/agentpluginscli/legacy_controller.go", "4ba1296edf3c0cdc6e8caf0b27b577ad8715231f4201d4e8e2f56679f1713326"},
	{"cli/internal/agentpluginscli/lifecycle.go", "4325a3aae281a408ead2eae8a04357d299877a3e707f53bfc4636890b9e5f2f1"},
	{"cli/internal/agentpluginscli/outdated.go", "3a121369fbda57e812254cc38efd4ba1abf86f6369eea771085bd7183c03a7ac"},
	{"cli/internal/agentpluginscli/opencode_runtime_notice.go", "9f147759c3970929c2c966797d350d30634d817b77b88286fce3d73e6c9d0150"},
	{"cli/internal/agentpluginscli/output.go", "571e944a73ac3cc89e8b66b84fb0274943913dd4ca2a28f953c5563d400ce8dc"},
	{"cli/internal/agentpluginscli/plan_review.go", "561370e50ff7d2ffe7e3b41ee6d58b3619a567b7b10814c209a2bf90080378e2"},
	{"cli/internal/agentpluginscli/plan_review_format.go", "f2633e875009e0e0c40aa4d4ca5e29f58346ba2cbfb204258534bf2cab774c6c"},
	{"cli/internal/agentpluginscli/preflight.go", "22708d5f0008418c5c4f2cc959719dfabda0a33432e4cc8bd1bbd386151ae0d3"},
	{"cli/internal/agentpluginscli/prepare.go", "92590c7463c91ec4459318e71085b3d5a511551ce912b172aecb26c6e464bf5a"},
	{"cli/internal/agentpluginscli/prompt/prompt.go", "f00921b2b1e687ccfd2d5e70c108734594267a0daa5f0f15e0a54e29a1160218"},
	{"cli/internal/agentpluginscli/prompts.go", "3f740a18c8484ef38bbf737a64dd15a07f02aa99c087924de89e402067273f96"},
	{"cli/internal/agentpluginscli/read.go", "2323b837f267575c8b9e9d3d47e9ecb763ea3f4b1522f4548936615a6c15e542"},
	{"cli/internal/agentpluginscli/read_directory.go", "a2ba8aa03cd1e8efc2a8b3b9aa8718aceb13822bb19d083d6fa32a76e316b9d1"},
	{"cli/internal/agentpluginscli/read_reconciliation.go", "155d8f1cadda8bf0aadac97100d1991433fd547a74d53992f6b5239df94e22ef"},
	{"cli/internal/agentpluginscli/remove_multi.go", "35b5c172e546c28277029783025b62c98f7e179620a47191180fb3c2d8806b6f"},
	{"cli/internal/agentpluginscli/repair_multi.go", "b4c1d9a953da6f59175b4c97b3fd2ddd6a36e7e9bbbb94975357c18d07f382f3"},
	{"cli/internal/agentpluginscli/root.go", "04a77692a6950c85afa6ba06ea3efb81becf0ddfab21220f00e3dba242e39118"},
	{"cli/internal/agentpluginscli/search.go", "d954c3b68d6bcfe6b821855aed6c14e712b9ff0eff0bf67206a1191f714fa998"},
	{"cli/internal/agentpluginscli/security.go", "2933cca3234ea86ba6a2d47c7f1a01d08fc38f4cbdda7981f3b8e16913421429"},
	{"cli/internal/agentpluginscli/source.go", "69da28ca7324685711eb89961a4fac66bea7284402346c4efa3017736d4dc01b"},
	{"cli/internal/agentpluginscli/state_migration.go", "bd59eeae87605373b0ae4601310c67fb7aa02222c62fa8486f2a04b1a167504e"},
	{"cli/internal/agentpluginscli/target_batch.go", "afa9dc9ba6a417229c57afda8a1eacbc397b38c0802880e1c14f5f8fc98fc81c"},
	{"cli/internal/agentpluginscli/update_all.go", "d7704f6fa234a0385af8ff19fb5aa34f796c247c669340445ec3ab37c3c31ff0"},
	{"cli/internal/agentpluginscli/update_multi.go", "ed113a01337a22111af6ec6d093da851b6247663187010e8cbc104649d77cb62"},
	{"cli/internal/agentpluginscli/validate.go", "8c643a08c657431d623197364f5f47d7f23ed0b4e6cf926f40a521a8bac7ee9d"},
	{"cli/internal/authoring/commands/commands.go", "62fb483ca5bb537e82e371a4268c837b399b77c365057f53fc850f517053bfb6"},
	{"cli/internal/authoring/commands/dev_session.go", "fbfe779c0f376620902a8cfd3c7f37f3e9f4406e4e053d84d24dde02630eae6d"},
	{"cli/internal/authoring/commands/maintenance.go", "7fb03c69b48cb0fb5fdacff08930d4b034895be7a65f336e7f7374e607ed0f5a"},
	{"cli/internal/authoring/commands/public_contract.go", "332d209e5453b51e086cebe96baca7ac4542e1f7e5ad2718c18afda3eeb9c55e"},
	{"cli/internal/authoring/commands/skills.go", "566e36e02d58c5d76e92369987d901feb1bcb6f3e59d3225e02961fa041d606b"},
	{"cli/internal/authoring/commands/version.go", "966a468f07573eed3a3e876c6b5d3a4354332500802b1b24bf6dfedc7b855273"},
	{"cli/internal/authoring/jsonmaint/exchange_darwin.go", "7e2b742a50c4375484c6cb452be8d6adabaf6563c911965707047c5bdd9ff4e2"},
	{"cli/internal/authoring/jsonmaint/exchange_linux.go", "85220531aff483ee14e03fe4216b2bc0900e22258bcf598e8981817022b01e51"},
	{"cli/internal/authoring/jsonmaint/exchange_other.go", "107d32aa90642858171ef540e8834b4e76f3fd82257106315b6af9beb9ae1dec"},
	{"cli/internal/authoring/jsonmaint/json.go", "8a6e2747ea21c913d842a570d75d607483dbf5139fc8bdce12c32bf4f36df338"},
	{"cli/internal/authoring/jsonmaint/nofollow_other.go", "a5e7df5315a68f2ca829a49e0ab43e621e15f3b746a3d5c4494639c53a7b83b4"},
	{"cli/internal/authoring/jsonmaint/nofollow_unix.go", "1bbc8faf63f22d1400eac465159eafec088f627d1f03eb4d074b61a3c170f4b0"},
	{"cli/internal/authoring/jsonmaint/replace_exchange.go", "0d8f7065bb86ade32e91c559ce64ace8373e465b684dffd5d38044dcf70877ce"},
	{"cli/internal/authoring/jsonmaint/replace_other.go", "d6fdce4fa4b4a20ca6d26951f4963cc2738fe8377526ea1a6c7644e25a7a5e97"},
	{"cli/internal/authoring/jsonmaint/replace_windows.go", "74e3e6e63e36bfd06a83167544780ba97fd0401e657a8602208becebd4d1edf8"},
	{"cli/internal/authoring/jsonmaint/sync_other.go", "40d9f089db776abe7b1235bd9e72f6689e16e3bbe8f872a552138d0581bcdd17"},
	{"cli/internal/authoring/jsonmaint/sync_windows.go", "a95e6093b542bfd61ec121344dbbc548fb9a4dfd8d0aa6f9b808ac536d4bd1a0"},
	{"cli/internal/authoring/nativeimport/native.go", "edc46fbe8db91c56df921dd966900cbbd6762aebdda48bb78895afef6499e96f"},
	{"cli/internal/authoring/nativeimport/nofollow_other.go", "e14d367e18567b65da7aabd319e63861e20818b26e91691bf77abf17e137de6b"},
	{"cli/internal/authoring/nativeimport/nofollow_unix.go", "e643bab4b2774ce4de51e1af8061bb09ddd67b454c09496552d44e7e673ed526"},
	{"cli/internal/authoring/nativeimport/output.go", "c20355804b42897c844abad0c9a8d7a3b069e1ca3b85a99f2f81ce1a27645432"},
	{"cli/internal/authoring/report/public.go", "2d8aafbd4833a7982160a3ea8c25564bce1cc8bd39d792bc245fdf9e29cdf6e8"},
	{"cli/internal/authoring/report/report.go", "6952de5556409574f22bd537b0ff4a18af72dcc7e03c3996940b64a26a56a149"},
	{"cli/internal/authoring/scaffold/apply.go", "b9709a2a0f2f454b913eea8bd4df11dfeb96ecc2d5ac6579216215be86a9d42c"},
	{"cli/internal/authoring/scaffold/paths.go", "1eb1d5f09798c43a236f05d4dabf309078506e786b4b25d02c654a8bcd2d2488"},
	{"cli/internal/authoring/scaffold/plan.go", "51c098b203fe3a5d6145b67a73fd8dacd3f8c9df9c5c147fcfe56d0d78591807"},
	{"cli/internal/authoring/scaffold/rename_darwin.go", "2b31a18bf69125a971785662759b1bf2c681b222d1a4b5808fd8442e1ea671b8"},
	{"cli/internal/authoring/scaffold/rename_export.go", "4183e1109d60c2391d12ed725b334cc6bc97b1fdfe16322aeb052a97a135a09b"},
	{"cli/internal/authoring/scaffold/rename_linux.go", "613c5beb3179e41c4adcc1184b4b91027a5d1497880db63d5d3d8bf030eda259"},
	{"cli/internal/authoring/scaffold/rename_package_other.go", "80b2359d7811644e96551fcc68b2fb6b74a13c3e3495e8819f14f056d5c9869a"},
	{"cli/internal/authoring/scaffold/rename_unsupported.go", "594b631c59ac54964568a2f9abf816ab1a2d25cbc7af95e90f74cae0b54f89f0"},
	{"cli/internal/authoring/scaffold/rename_windows.go", "4b255048365418a57c83cc4ce458b522e558edd61127e8cc6d18746f4b3366bf"},
	{"cli/internal/authoring/scaffold/skill_plan.go", "a8e5b6ddb8a638dd729aedc947cf8fef627e992214ea39fe17b0ed6ade5cb481"},
	{"cli/internal/authoring/scaffold/stage_posix.go", "1fca38bb261d637666dbd47a232bda801968662ef55bbd7ee609d122d6c9999e"},
	{"cli/internal/authoring/scaffold/stage_windows.go", "3bced06b906245cfd61ab6d2a453dd6c0fd3f7791b3c76de54bbf2c320d137a5"},
	{"cli/internal/authoring/scaffold/templates.go", "958b895a4e35a95b2ee9a43aaeb45f83f483e4295b2b14dada5226b795bb3835"},
	{"cli/internal/authoringcli/command.go", "ab3ea92ca93a676a49cb99898a23386e1766e6f8becb2468bcfbb84ec22e1ec4"},
	{"cli/internal/authoringcli/flags.go", "5f745c810e90586899cff2170433dbc58483ba17496167738387388f29be49c0"},
	{"cli/internal/authoringcli/release.go", "45cb7a7000a31bb8477d1301fc70ce4521140b9f2a4760d5f84da3283134877f"},
	{"go.mod", "352e528c2b2c34df21276c1f9b54c32f79656dad42c310da6b551f022119ce8c"},
	{"go.work", "9b5094963d3524c4e0ad36b99a86ddb1a66d7a4cdaa6615e1c1e4621a45ba579"},
	{"go.work.sum", "806569ceb74454bbf59a2f7a5de9513276726c204c4972ce303779bad40fba25"},
	{"install/integrationctl/agentplugins/domain/acquisition.go", "ea60178232db888d8df99a7b6722bb26b7a3f1a23776a6be041cf9c84f7a2cb3"},
	{"install/integrationctl/agentplugins/domain/catalog.go", "e70853260635bed305ce77fef0c420e2028a57d77a9ce0565ab9cf8b275a98bc"},
	{"install/integrationctl/agentplugins/domain/chatgpt_mapping.go", "a124fe2ad1c349628b36bd901ea280c6401487aa341a05a0216d379d7cac3669"},
	{"install/integrationctl/agentplugins/domain/clients.go", "be1241c9b995050461254a9659258c1e735f351b7d3e3a8e47158ae17033a126"},
	{"install/integrationctl/agentplugins/domain/directory.go", "fbb49723b1e1b7ebfa19ebb7f872ebda461be535fae57a1767883fc6bd015860"},
	{"install/integrationctl/agentplugins/domain/directory_context7_preparation.go", "7fdd6190751ed53a2b438f58415861bcef086e7097350cc9f2305e98dc88f5c1"},
	{"install/integrationctl/agentplugins/domain/errors.go", "7229bf792c60bcbb290050a6659f129f09e85e22174d32ac681a4c6e5bb6efd8"},
	{"install/integrationctl/agentplugins/domain/identity.go", "85a3ab1c67a3137f9de3492a99c8ad8e005e7723a77952f389f0748eb9f58382"},
	{"install/integrationctl/agentplugins/domain/install_intent.go", "a082b97d07f0bf55d42b5baa173d184707c0053533e53a253108bbfd35c6c351"},
	{"install/integrationctl/agentplugins/domain/planning.go", "bd69ed621f82227541cc02af2274ee77e394b0f6a536313b3b8f332bd2180c34"},
	{"install/integrationctl/agentplugins/domain/security.go", "7ef925b9529d946576253ec2a8f6adf2c0ea93fa33a413899914c35fc6a3be9d"},
	{"install/integrationctl/agentplugins/domain/selection.go", "9d1cde69bfc2829da904e26a3881bc0dabb8f1e143b817391a7e05022d7e4a1f"},
	{"install/integrationctl/agentplugins/domain/state.go", "3410f0a1f9c697f6538682d79ac4d6db7f7e7a29aaf36936cf6298e0e65623a3"},
	{"install/integrationctl/agentplugins/domain/types.go", "5428390af193ec99661203ea07fd103b7ed3823318c249592c2d6dcbd6722442"},
	{"install/integrationctl/agentplugins/go.mod", "7ada43e7cd6dd48a8064cfcba32255c743e55277e4b5e75bc57d244d9eb53b30"},
	{"install/integrationctl/agentplugins/go.sum", "eaa9c28f77e14588df8cdb02b4435a8329edad015f10277008a6f769ab9fc12d"},
	{"install/integrationctl/go.mod", "b79f0e7afac7734397b834940dfd7f6d27120301013c826ca2bdd4b35043ec2a"},
	{"install/integrationctl/go.sum", "91d17efbf1fb87ffc6512b7632b5db5024363cc9d48e5ce5b74139de887763e9"},
	{"plugininstall/go.mod", "7d0745754d1ae04fb31d5056cbd2f4fa82d711d0536c1c868f40452cf5e6bb53"},
	{"sdk/go.mod", "79e8f3d4903364c498fffcf34e9394f84437ae198dcefcd1c0c99cb41c80ba5a"},
}

// Exact direct file sets, regardless of ignore rules or platform suffixes.
// Adapter bytes are reviewed with its commit, not self-hashed into that commit.
var constructionDirs = []string{
	"cli/internal/agentpluginscli",
	"cli/internal/agentpluginscli/prompt",
	"cli/internal/authoring/commands",
	"cli/internal/authoring/jsonmaint",
	"cli/internal/authoring/nativeimport",
	"cli/internal/authoring/report",
	"cli/internal/authoring/scaffold",
	"cli/internal/authoringcli",
	"install/integrationctl/agentplugins/domain",
}
var adapterFiles = []string{"export.go", "export_test.go", "main.go", "source.go", "source_test.go"}

func validateInventory(checkout string) error {
	allowed := map[string]bool{}
	for _, pin := range factoryPins {
		allowed[pin.Path] = true
	}
	const adapter = "cli/tools/authoring-docs"
	for _, name := range adapterFiles {
		allowed[adapter+"/"+name] = true
	}
	for _, dir := range append(append([]string{}, constructionDirs...), adapter) {
		if err := validateSourceDirectory(checkout, dir, dir == adapter, allowed); err != nil {
			return err
		}
	}
	for _, name := range adapterFiles {
		path := adapter + "/" + name
		if _, err := os.Stat(filepath.Join(checkout, path)); err != nil {
			return fmt.Errorf("adapter input unavailable: %s", path)
		}
		if _, err := git(checkout, "ls-files", "--error-unmatch", "--", path); err != nil {
			return fmt.Errorf("adapter input not tracked: %s", path)
		}
	}
	return validateDependencyControls(checkout, allowed)
}

func validateSourceDirectory(checkout, dir string, includeTests bool, allowed map[string]bool) error {
	entries, err := os.ReadDir(filepath.Join(checkout, filepath.FromSlash(dir)))
	if err != nil {
		return fmt.Errorf("source directory unavailable: %s", dir)
	}
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		if !includeTests && strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		name := dir + "/" + entry.Name()
		if !allowed[name] || entry.Type()&os.ModeSymlink != 0 || entry.IsDir() {
			return fmt.Errorf("unexpected Go input: %s", name)
		}
	}
	return nil
}

func validateDependencyControls(checkout string, allowed map[string]bool) error {
	// Absent control files are also part of the effective workspace contract.
	for _, dir := range []string{"", "cli/", "install/integrationctl/", "install/integrationctl/agentplugins/", "plugininstall/", "sdk/"} {
		for _, name := range []string{"go.mod", "go.sum", "go.work", "go.work.sum", "vendor"} {
			path := dir + name
			if allowed[path] {
				continue
			}
			if _, err := os.Lstat(filepath.Join(checkout, path)); !os.IsNotExist(err) {
				return fmt.Errorf("unexpected dependency control: %s", path)
			}
		}
	}
	return nil
}
