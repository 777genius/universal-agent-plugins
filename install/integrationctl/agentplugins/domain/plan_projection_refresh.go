package domain

// RequiresNativeProjectionRefresh reports an ephemeral adapter decision that
// explicit update must stage a changed host projection at the same revision.
// It grants no ownership and does not change repeated-add or repair policy.
func (plan DeliveryPlan) RequiresNativeProjectionRefresh() bool {
	return plan.requiresNativeProjectionRefresh
}

// WithNativeProjectionRefresh returns a plan copy with the current decision.
// Trusted custom planners must compute or preserve this decision when the
// prepared host requires different projection bytes. It is never serialized.
func (plan DeliveryPlan) WithNativeProjectionRefresh(required bool) DeliveryPlan {
	plan.requiresNativeProjectionRefresh = required
	return plan
}
