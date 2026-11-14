package app

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type PluginDevOptions struct {
	Root      string
	Platform  string
	Event     string
	Fixture   string
	GoldenDir string
	All       bool
	Once      bool
	Interval  time.Duration
}

type PluginDevUpdate struct {
	Cycle  int
	Passed bool
	Lines  []string
}

type PluginDevSummary struct {
	Cycles     int
	LastPassed bool
}

func (svc PluginService) Dev(ctx context.Context, opts PluginDevOptions, emit func(PluginDevUpdate)) (PluginDevSummary, error) {
	root := normalizeDevRoot(opts.Root)
	interval := normalizeDevInterval(opts.Interval)
	emit = normalizeDevEmitter(emit)

	selectedPlatform, err := resolveDevPlatform(root, opts.Platform)
	if err != nil {
		return PluginDevSummary{}, err
	}

	cycle := 0
	runCycle := func(trigger string, changed []string) (bool, error) {
		cycle++
		update := svc.runDevCycle(ctx, root, selectedPlatform, opts, cycle, trigger, changed)
		emit(update)
		return update.Passed, nil
	}

	cycle++
	initialUpdate := svc.runDevCycle(ctx, root, selectedPlatform, opts, cycle, "initial", nil)
	lastPassed := initialUpdate.Passed
	if opts.Once {
		emit(initialUpdate)
		return PluginDevSummary{Cycles: cycle, LastPassed: lastPassed}, nil
	}

	// Capture the generated baseline before publishing the initial update. A caller
	// may edit a fixture as soon as it receives that update; taking the baseline
	// afterward can absorb that edit and leave the watch loop waiting forever.
	snapshot, err := takeDevSnapshot(root)
	if err != nil {
		emit(initialUpdate)
		return PluginDevSummary{}, err
	}
	emit(initialUpdate)

	return svc.runDevWatchLoop(ctx, root, interval, snapshot, &cycle, &lastPassed, runCycle, emit)
}

func normalizeDevRoot(root string) string {
	root = strings.TrimSpace(root)
	if root == "" {
		return "."
	}
	return root
}

func normalizeDevInterval(interval time.Duration) time.Duration {
	if interval <= 0 {
		return 750 * time.Millisecond
	}
	return interval
}

func normalizeDevEmitter(emit func(PluginDevUpdate)) func(PluginDevUpdate) {
	if emit == nil {
		return func(PluginDevUpdate) {}
	}
	return emit
}

func devSnapshotFailureUpdate(cycle int, err error) PluginDevUpdate {
	return PluginDevUpdate{
		Cycle:  cycle,
		Passed: false,
		Lines:  []string{fmt.Sprintf("Cycle %d [watch]: snapshot failed: %v", cycle, err)},
	}
}
