package providers

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/commandext"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

// commandSkills are the files of every planned command skill, keyed by skill
// name and then by slash path inside the skill directory.
type commandSkills map[string]map[string][]byte

// renderCommandSkills renders before any staging exists, so a refused
// explicit-only command leaves nothing on disk.
func (stager Stager) renderCommandSkills(ctx context.Context, envelope domain.PackageEnvelope, plan domain.DeliveryPlan) (commandSkills, error) {
	if err := stager.requireDeps(); err != nil {
		return nil, err
	}
	definition, _ := domain.ClientDefinitionFor(plan.ClientID)
	decorator, _ := clients.As[clients.ExplicitSkillDecorator](stager.Registry, plan.ClientID)
	return renderCommandSkills(ctx, envelope, plan, definition.Traits.EnforcesExplicitSkillInvocation, decorator)
}

// renderCommandSkills is fail-closed for explicit-only commands: the planner
// already withholds them from a client without the trait, and a plan that
// still carries one, or a client that declares the trait without a decorator,
// is refused here instead of delivering an ordinary skill.
func renderCommandSkills(ctx context.Context, envelope domain.PackageEnvelope, plan domain.DeliveryPlan, enforcesExplicit bool, decorator clients.ExplicitSkillDecorator) (commandSkills, error) {
	rendered := commandSkills{}
	for _, component := range plan.Components {
		if component.Source != domain.ComponentSourceCommand || component.Support == domain.SupportUnsupported {
			continue
		}
		command, err := commandext.PlannedCommand(envelope, component)
		if err != nil {
			return nil, err
		}
		var decorate commandext.Decorate
		if command.ExplicitOnly {
			if !enforcesExplicit {
				return nil, fmt.Errorf("client %q cannot enforce explicit invocation of command %q", plan.ClientID, command.Name)
			}
			if decorator == nil {
				return nil, fmt.Errorf("client %q declares explicit invocation but has no skill decorator", plan.ClientID)
			}
			decorate = decorator.DecorateExplicitSkill
		}
		files, err := commandext.Files(ctx, command, envelope.Manifest.Name, decorate)
		if err != nil {
			return nil, err
		}
		rendered[component.Name] = files
	}
	return rendered, nil
}

// writeCommandSkills adds the rendered skills to a sanitized staging tree. It
// never replaces an entry: the loader already rejected a command whose skill
// name is taken, so an existing directory means the tree is not the one that
// was checked.
func writeCommandSkills(paths ports.PathPolicy, stagingPath string, rendered commandSkills) error {
	if len(rendered) == 0 {
		return nil
	}
	skillsRoot := filepath.Join(stagingPath, "skills")
	if err := os.Mkdir(skillsRoot, 0o755); err != nil && !os.IsExist(err) {
		return fmt.Errorf("create staged skills root: %w", err)
	}
	if info, err := os.Lstat(skillsRoot); err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("staged skills root is not a directory")
	}
	for _, name := range slices.Sorted(maps.Keys(rendered)) {
		directory := filepath.Join(skillsRoot, name)
		if err := paths.RequireContainedChild(skillsRoot, directory); err != nil {
			return fmt.Errorf("unsafe command skill path %q: %w", name, err)
		}
		if err := os.Mkdir(directory, 0o755); err != nil {
			return fmt.Errorf("create command skill %q: %w", name, err)
		}
		for _, file := range slices.Sorted(maps.Keys(rendered[name])) {
			if err := writeNewFile(filepath.Join(directory, filepath.FromSlash(file)), rendered[name][file]); err != nil {
				return fmt.Errorf("write command skill %q: %w", name, err)
			}
		}
	}
	return nil
}

// requireInstalledCommandSkills fails with domain.ErrProjectionRefreshRequired
// unless an installed tree holds exactly the command skills the plan renders,
// byte for byte, and none of the package's other command skills. A tree
// rendered under other rules then never reaches a native projection.
func (stager Stager) requireInstalledCommandSkills(ctx context.Context, root string, envelope domain.PackageEnvelope, plan domain.DeliveryPlan) error {
	rendered, err := stager.renderCommandSkills(ctx, envelope, plan)
	if err != nil {
		return err
	}
	for _, name := range commandext.ProjectedSkillNames(envelope) {
		installed, err := readInstalledSkill(filepath.Join(root, "skills", name))
		if err != nil && !errors.Is(err, errUnknownSkillEntry) {
			return fmt.Errorf("read installed command skill %q: %w", name, err)
		}
		if want, planned := rendered[name]; err != nil || planned != (installed != nil) || !maps.EqualFunc(installed, want, bytes.Equal) {
			return fmt.Errorf("%w: installed skill %q differs from the current command projection", domain.ErrProjectionRefreshRequired, name)
		}
	}
	return nil
}

// errUnknownSkillEntry marks an installed entry the stager never writes for a
// command skill, so the skill cannot be the rendered one.
var errUnknownSkillEntry = errors.New("installed skill holds an entry that is not a directory or regular file")

// readInstalledSkill returns the files below one installed skill directory,
// keyed by slash path, or nil when it is absent. It reads through an os.Root,
// so no link below the directory is followed.
func readInstalledSkill(directory string) (map[string][]byte, error) {
	info, err := os.Lstat(directory)
	switch {
	case os.IsNotExist(err):
		return nil, nil
	case err != nil:
		return nil, err
	case !info.IsDir():
		return nil, errUnknownSkillEntry
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		return nil, err
	}
	defer func() { _ = root.Close() }()
	tree := root.FS()
	files := map[string][]byte{}
	err = fs.WalkDir(tree, ".", func(path string, entry fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case entry.IsDir():
			return nil
		case !entry.Type().IsRegular():
			return errUnknownSkillEntry
		}
		body, err := fs.ReadFile(tree, path)
		files[path] = body
		return err
	})
	return files, err
}

func writeNewFile(path string, body []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	_, err = file.Write(body)
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	return err
}
