//go:build linux

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// This is the literal research table, not a command/execution endpoint. Private
// callbacks keep its nesting testable without starting any operating-system
// process; production passes only the existing strict runner and fixed probes.
func runDisplayTable(work context.Context, outer func(func(context.Context, isolationProof) error) (childRecord, error), ready func(context.Context, isolationProof) error, probe func(string) error) (childRecord, error) {
	return outer(func(ctx context.Context, proof isolationProof) error {
		if err := ready(ctx, proof); err != nil {
			return err
		}
		for _, label := range probeLabels {
			if err := work.Err(); err != nil {
				return err
			}
			if err := probe(label); err != nil {
				return err
			}
		}
		return nil
	})
}
func requireFreshResultPaths(result string) error {
	for _, path := range []string{result, filepath.Join(filepath.Dir(result), ".observation.pending"), filepath.Join(filepath.Dir(result), "observation.ready")} {
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			return fmt.Errorf("fresh result protocol leaf already exists")
		}
	}
	return nil
}
func admitReadyResult(result string) ([]byte, bool, error) {
	_, err := readOrdinary(filepath.Join(filepath.Dir(result), "observation.ready"), 0)
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	body, err := readOrdinary(result, resultLimit)
	if err != nil {
		return nil, false, fmt.Errorf("ready result is missing/invalid: %w", err)
	}
	return body, true, nil
}
