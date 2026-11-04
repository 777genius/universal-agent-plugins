package domain

import "errors"

// ErrManagedSkillChanged identifies an existing owned Skill directory whose
// current bytes no longer match its receipt. Recovery must preserve those bytes
// until the owner explicitly resolves the conflict.
var ErrManagedSkillChanged = errors.New("managed skill changed outside agentplugins")

// ErrForeignSkillDestination identifies an existing project Skill target with
// no matching UAP ownership. It must be left untouched.
var ErrForeignSkillDestination = errors.New("foreign project Skill destination")
