package platformexec

func rejectOpenCodeCompatSkillRoots(roots []openCodeCompatSkillRoot) error {
	for _, reject := range roots {
		if err := rejectOpenCodeCompatSkillRoot(reject.full, reject.display); err != nil {
			return err
		}
	}
	return nil
}
