package start

import (
	"fmt"

	"opencsg.com/csghub-server/mq"

	"github.com/spf13/cobra"
	"opencsg.com/csghub-server/common/config"
)

func init() {
	Cmd.AddCommand(serverCmd)
	Cmd.AddCommand(rproxyCmd)
}

var Cmd = &cobra.Command{
	Use:   "start",
	Short: "Start a service",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		config, err := config.LoadConfig()
		if err != nil {
			return err
		}

		_, err = mq.GetOrInit(config)
		if err != nil {
			return fmt.Errorf("fail to build message queue handler: %w", err)
		}

		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}
