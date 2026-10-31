package domain

// DirectoryResolveLockedRestore permits restoring an exact user-scope lock
// without installed-state proof. Only the superseded-release gate is relaxed;
// the current signed publication must still authorize an ordinary install.
const DirectoryResolveLockedRestore DirectoryResolvePurpose = "locked_restore"
