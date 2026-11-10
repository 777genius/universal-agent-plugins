package report

// policyAssessments visits all policy lanes in their stable report order.
// Normalization and public ID projection must use exactly the same set.
func (r *Report) policyAssessments() []*Assessment {
	return []*Assessment{
		&r.Loadability, &r.Conformance, &r.HostSafety, &r.Readiness,
		&r.Release, &r.Runtime, &r.Compatibility, &r.Toolchain,
	}
}
