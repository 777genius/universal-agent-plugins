package app

func (s Service) Validate(opts ValidateOptions) (ValidationReport, error) {
	return s.validateSkills(opts)
}
