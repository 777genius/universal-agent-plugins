package app

import (
	"context"
	"time"
)

func (svc PluginService) runDevWatchLoop(ctx context.Context, root string, interval time.Duration, snapshot devSnapshot, cycle *int, lastPassed *bool, runCycle func(string, []string) (bool, error), emit func(PluginDevUpdate)) (PluginDevSummary, error) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return PluginDevSummary{Cycles: *cycle, LastPassed: *lastPassed}, nil
		case <-ticker.C:
			next, err := takeDevSnapshot(root)
			if err != nil {
				*cycle++
				*lastPassed = false
				emit(devSnapshotFailureUpdate(*cycle, err))
				continue
			}
			changed := devSnapshotChanges(snapshot, next)
			if len(changed) == 0 {
				continue
			}
			*lastPassed, _ = runCycle("watch", changed)
			snapshot, err = takeDevSnapshot(root)
			if err != nil {
				return PluginDevSummary{Cycles: *cycle, LastPassed: *lastPassed}, err
			}
		}
	}
}
