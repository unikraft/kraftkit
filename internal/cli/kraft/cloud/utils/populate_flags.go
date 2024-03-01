package utils

import (
	"fmt"

	"github.com/spf13/cobra"
	"kraftkit.sh/log"
)

func PopulateMetroToken(cmd *cobra.Command, metro, token *string) error {
	*metro = cmd.Flag("metro").Value.String()
	if *metro == "" {
		return fmt.Errorf("kraftcloud metro is unset, try setting `KRAFTCLOUD_METRO`, or use the `--metro` flag")
	}

	log.G(cmd.Context()).WithField("metro", *metro).Debug("using")

	*token = cmd.Flag("token").Value.String()
	if *token != "" {
		log.G(cmd.Context()).WithField("token", *token).Debug("using")
	}

	return nil
}
