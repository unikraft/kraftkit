package utils

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"kraftkit.sh/log"
)

func PopulateMetroToken(cmd *cobra.Command, metro, token *string) error {
	*metro = cmd.Flag("metro").Value.String()
	if *metro == "" {
		*metro = os.Getenv("UNIKRAFTCLOUD_METRO")

		if *metro == "" {
			*metro = os.Getenv("KRAFTCLOUD_METRO")
		}

		if *metro == "" {
			*metro = os.Getenv("KC_METRO")
		}

		if *metro == "" {
			*metro = os.Getenv("UKC_METRO")
		}

		if *metro == "" {
			return fmt.Errorf("kraftcloud metro is unset, try setting `UNIKRAFTCLOUD_METRO`, or use the `--metro` flag")
		}
	}

	log.G(cmd.Context()).WithField("metro", *metro).Debug("using")

	*token = cmd.Flag("token").Value.String()
	if *token != "" {
		log.G(cmd.Context()).WithField("token", *token).Debug("using")
	} else {
		*token = os.Getenv("UNIKRAFTCLOUD_TOKEN")

		if *token == "" {
			*token = os.Getenv("KRAFTCLOUD_TOKEN")
		}

		if *token == "" {
			*token = os.Getenv("KC_TOKEN")
		}

		if *token == "" {
			*token = os.Getenv("UKC_TOKEN")
		}

		if *token != "" {
			log.G(cmd.Context()).WithField("token", *token).Debug("using")
		}
	}

	return nil
}
