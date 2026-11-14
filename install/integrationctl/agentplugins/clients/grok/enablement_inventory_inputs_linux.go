//go:build linux

package grok

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/directoryidentity"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func captureEnablementInventoryInputs(ctx context.Context, profile *enablementProfile, executable, project, home string) (result *enablementInventoryInputs, resultErr error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if profile == nil || profile.config == nil || profile.parent.IsZero() ||
		profile.parent.Facts().CanonicalRoot != filepath.Join(home, ".grok") {
		return nil, errEnablementInventoryInputs
	}
	for _, path := range []string{home, project, executable} {
		if !inventoryInputAbsolute(path) {
			return nil, errEnablementInventoryInputs
		}
	}
	if err := directoryidentity.Revalidate(ctx, profile.parent); err != nil {
		return nil, err
	}
	lease := &enablementInventoryInputs{profile: profile, remaining: projectDiscoveryLimit}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, lease.close())
		}
	}()
	if err := lease.captureRoles(ctx, executable, project, home); err != nil {
		return nil, err
	}
	if err := lease.captureDiscovery(ctx); err != nil {
		return nil, err
	}
	if err := lease.revalidate(ctx, profile.config.Original()); err != nil {
		return nil, err
	}
	return lease, nil
}

func inventoryInputAbsolute(path string) bool {
	return path != "/" && filepath.IsAbs(path) && filepath.Clean(path) == path &&
		membershipText(path) && strings.Count(path, "/") <= 32
}

func (lease *enablementInventoryInputs) captureRoles(ctx context.Context, executable, project, home string) error {
	roles := []struct {
		path      string
		directory bool
		output    *legacyports.ReadonlyOriginal
	}{{home, true, &lease.inputs.Home}, {project, true, &lease.inputs.Project}, {executable, false, &lease.inputs.Executable}}
	for _, role := range roles {
		if err := ctx.Err(); err != nil {
			return err
		}
		parentPath := filepath.Dir(role.path)
		if role.directory {
			parentPath = role.path
		}
		parent, err := inventoryInputParent(ctx, parentPath)
		if err != nil {
			return err
		}
		file, info, err := openProjectPath(role.path, role.directory, nil)
		if err != nil {
			return err
		}
		lease.files = append(lease.files, file)
		if role.directory {
			current, joinErr := directoryidentity.CaptureHeld(ctx, role.path, file)
			if joinErr != nil || !parent.Equal(current) {
				return errors.Join(joinErr, errEnablementInventoryInputs)
			}
		}
		if err := directoryidentity.Revalidate(ctx, parent); err != nil {
			return err
		}
		lease.parents = append(lease.parents, parent)
		*role.output = legacyports.ReadonlyOriginal{Path: role.path, File: file, Info: info}
	}
	lease.inputs.Worktree = lease.inputs.Project
	return ctx.Err()
}

func inventoryInputParent(ctx context.Context, path string) (directoryidentity.Authority, error) {
	parent, err := directoryidentity.Capture(ctx, path)
	if err != nil {
		return directoryidentity.Authority{}, err
	}
	if parent.Facts().CanonicalRoot != path {
		return directoryidentity.Authority{}, errEnablementInventoryInputs
	}
	return parent, nil
}

func (lease *enablementInventoryInputs) revalidate(ctx context.Context, expected nativeconfig.FileSnapshot) error {
	if err := lease.revalidateRoots(ctx); err != nil {
		return err
	}
	for _, pin := range lease.pins {
		if err := pin.revalidate(ctx); err != nil {
			return err
		}
	}
	if !expected.Exists || len(expected.Body) > grokMembershipLimit {
		return errEnablementInventoryInputs
	}
	path := filepath.Join(lease.inputs.Home.Path, ".grok", "config.toml")
	body, info, err := inventoryInputRead(ctx, path, grokMembershipLimit)
	if err != nil {
		return err
	}
	if info.Mode() != expected.Mode || !bytes.Equal(body, expected.Body) {
		return errEnablementInventoryInputs
	}
	if err := inventoryInputConfig(body); err != nil {
		return err
	}
	return lease.revalidateRoots(ctx)
}

func (lease *enablementInventoryInputs) revalidateRoots(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if lease == nil || lease.closed || lease.profile == nil {
		return errEnablementInventoryInputs
	}
	if err := directoryidentity.Revalidate(ctx, lease.profile.parent); err != nil {
		return err
	}
	for _, parent := range lease.parents {
		if err := directoryidentity.Revalidate(ctx, parent); err != nil {
			return err
		}
	}
	for _, original := range []legacyports.ReadonlyOriginal{lease.inputs.Home, lease.inputs.Project, lease.inputs.Executable} {
		held, heldErr := original.File.Stat()
		named, namedErr := os.Lstat(original.Path)
		if err := errors.Join(heldErr, namedErr); err != nil {
			return err
		}
		if !inventoryInputSame(original.Info, held) || !inventoryInputSame(held, named) {
			return errEnablementInventoryInputs
		}
	}
	return ctx.Err()
}

func inventoryInputSame(a, b os.FileInfo) bool {
	if a == nil || b == nil || !os.SameFile(a, b) || a.Mode() != b.Mode() || a.Size() != b.Size() || !a.ModTime().Equal(b.ModTime()) {
		return false
	}
	x, xok := a.Sys().(*syscall.Stat_t)
	y, yok := b.Sys().(*syscall.Stat_t)
	return xok && yok && x != nil && y != nil && x.Nlink == y.Nlink && x.Uid == y.Uid && x.Gid == y.Gid && x.Ctim == y.Ctim
}
