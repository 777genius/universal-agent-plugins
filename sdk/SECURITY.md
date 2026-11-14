# Security

For vulnerability reports, use GitHub **Security Advisories** (if enabled on the repository) or contact the maintainers privately. Do not open a public issue for undisclosed security problems.

Hook plugins receive JSON from Claude Code on stdin. Treat path-like fields and free-text fields (`reason`, `prompt`, tool arguments) as **untrusted** input. Validate paths before using them, avoid shell interpolation, and prefer checksum-verified release assets such as the documented `checksums.txt` install flow.
