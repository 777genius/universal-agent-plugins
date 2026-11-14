//go:build linux

package process

import (
	"context"
	"errors"

	"golang.org/x/sys/unix"
)

// The fixed OS runtime view retains original alias metadata and bounded text;
// it does not reproduce every host OS alias target. Outside targets remain
// unavailable; compatibility requires the measured native operation. No
// external target is opened or mounted.
func (census *readonlyRuntimeCensus) requireRoots(ctx context.Context, roots [4]readonlyRuntimeRoot) error {
	for i, visible := range readonlyRuntimePaths() {
		if err := ctx.Err(); err != nil {
			return err
		}
		root := roots[i]
		if root.visible != visible || !readonlyAbsolute(root.object.path) || !readonlyWithin(root.object.path, "/usr") {
			return errors.New("readonly runtime visible root differs")
		}
		expected, err := readonlyRuntimeFacts(root.object.facts, census.uid, census.gid)
		row, exists := census.rows[root.object.path]
		if err != nil || !exists || row.facts != expected || row.link != "" || expected.Mode&unix.S_IFMT != unix.S_IFDIR {
			return errors.Join(err, errors.New("readonly runtime visible root is outside the captured closure"))
		}
	}
	return ctx.Err()
}
