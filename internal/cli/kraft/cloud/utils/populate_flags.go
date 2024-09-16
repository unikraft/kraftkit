package utils

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"kraftkit.sh/config"
	"kraftkit.sh/log"
	"kraftkit.sh/tui/selection"

	unikraftcloud "sdk.kraft.cloud"
)

type Metro struct {
	Location string
	Code     string
}

var _ fmt.Stringer = (*Metro)(nil)

func (m Metro) String() string {
	return fmt.Sprintf("%s (%s)", m.Code, m.Location)
}

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

		if *metro == "" && !config.G[config.KraftKit](cmd.Context()).NoPrompt {
			client := unikraftcloud.NewMetrosClient()

			metros, err := client.List(cmd.Context(), false)
			if err != nil {
				return fmt.Errorf("could not list metros: %w", err)
			}

			candidates := make([]Metro, len(metros))
			for i, m := range metros {
				candidates[i].Code = m.Code
				candidates[i].Location = m.Location
			}

			candidate, err := selection.Select("metro not explicitly set: which one would you like to use?", candidates...)
			if err != nil {
				return err
			}

			*metro = candidate.Code

			log.G(cmd.Context()).Infof("run `export UKC_METRO=%s` or use the `--metro` flag to skip this prompt in the future", *metro)
		}

		if *metro == "" {
			return fmt.Errorf("unikraft cloud metro is unset, try setting `UKC_METRO`, or use the `--metro` flag")
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
