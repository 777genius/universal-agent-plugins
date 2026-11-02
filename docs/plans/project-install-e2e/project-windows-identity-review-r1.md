# Project namespace Windows correction independent review R1

Reviewer: cursor_clients, 2026-10-23. Reviewed the working diff in
`work/uap-core-publication` for `installer/project_namespace.go`, its existing
namespace contract tests, and `.github/workflows/core-fast.yml`. The author
subsequently froze the same reviewed code plus its implementation evidence at
`c49a50542da071223e9a858096ca9f3508f2ac53`.

## Decision: GO for the narrow correction

The namespace guard now retains open-handle `File.Stat` identity for both the
selected project root and `.uap`, matching the established nativeconfig lock
directory identity pattern in this repository. This addresses Windows' lazy
path-based file-ID population in an earlier retained `Lstat` FileInfo: a later
SameFile call must compare the originally captured physical directory rather
than reopen its old name after replacement.

The helper keeps the before-open Lstat real-directory/no-symlink admission,
compares the opened handle against that observation, rejects an unexpected
directory change with ErrPlanChanged, handles Stat/Close failures, and returns
the eager handle snapshot for subsequent guard comparisons. Existing exact
namespace layout and containment checks are preserved. A missing namespace
remains permissible until first creation; once pinned, disappearance or
replacement refuses. Both original failing contracts remain intact, and the
new selected-project replacement test additionally proves refusal before a
new `.uap` is created in the replacement root.

The checkout change sets core.autocrlf=false for the project-contract checkout,
preserving Git bytes for embedded schema digest checks on Windows. It changes
neither schema bytes nor native execution/approval guards.

Independent verification: complete `installer -run '^TestProjectNamespace'
-count=1` selection PASS, 0.010s on Linux with Go 1.26.10. Inspected the new
helper alongside `nativeconfig/lock.go`'s existing handle identity capture.
No Windows runtime was available to this review; the genuine hosted Windows
result remains the platform verification gate, not a claim supplied here.
