package domain

import "errors"

// ErrManagedSkillChanged identifies an existing owned Skill directory whose
// current bytes no longer match its receipt. Recovery must preserve those bytes
// until the owner explicitly resolves the conflict.
var ErrManagedSkillChanged = errors.New("managed skill changed outside agentplugins")
