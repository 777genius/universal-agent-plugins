package main

import "github.com/spf13/cobra"

func newPublicationDoctorCmd(runner inspectRunner) *cobra.Command {
	flags := newPublicationDoctorFlags()
	cmd := &cobra.Command{
		Use:           "doctor [path]",
		Short:         "Inspect publication readiness without mutating files",
		Long:          "Read-only publication readiness check for package-capable targets and authored publish/... channels.",
		Args:          cobra.MaximumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE:          publicationDoctorRunE(runner, &flags),
	}
	configurePublicationDoctorCmd(cmd, &flags)
	return cmd
}

func newPublicationDoctorFlags() publicationDoctorFlags {
	return publicationDoctorFlags{}
}

func publicationDoctorRunE(runner inspectRunner, flags *publicationDoctorFlags) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		return runPublicationDoctor(cmd, runner, publicationDoctorInput(*flags, args))
	}
}

func configurePublicationDoctorCmd(cmd *cobra.Command, flags *publicationDoctorFlags) {
	bindPublicationDoctorFlags(cmd, flags)
}
