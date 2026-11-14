package main

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

func writePublicationDoctorJSON(cmd *cobra.Command, report publicationDoctorJSONReport) error {
	body, err := marshalPublicationDoctorJSON(report)
	if err != nil {
		return err
	}
	return writePublicationDoctorJSONBody(cmd, body)
}

func marshalPublicationDoctorJSON(report publicationDoctorJSONReport) ([]byte, error) {
	return json.MarshalIndent(report, "", "  ")
}

func writePublicationDoctorJSONBody(cmd *cobra.Command, body []byte) error {
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s\n", body)
	return nil
}
