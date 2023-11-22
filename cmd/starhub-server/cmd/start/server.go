package start

import (
	"context"
	"fmt"
	"time"

	"git-devops.opencsg.com/product/community/starhub-server/pkg/httpbase"
	"github.com/spf13/cobra"
)

var (
	serverEnableTunnel bool
	enableOpenBrowser  bool
	enableSwagger      bool
	enableUI           bool
)

var Initializer func(ctx context.Context) (*httpbase.GracefulServer, error)

func init() {
	serverCmd.Flags().BoolVar(&enableSwagger, "swagger", false, "Start swagger help docs")
	serverCmd.Flags().BoolVar(&enableUI, "ui", false, "enable frontend ui")
	serverCmd.Flags().BoolVar(&serverEnableTunnel, "tunnel", false, "automatic connection to UltraFox dev tunnel, and modifies the externalhost configuration")
	serverCmd.Flags().BoolVar(&enableOpenBrowser, "open-browser", false, "auto open swagger and ui in browser")
	Cmd.AddCommand(serverCmd)
}

var serverCmd = &cobra.Command{
	Use:     "server",
	Short:   "Start the API server",
	Example: serverExample(),
	RunE: func(cmd *cobra.Command, args []string) (err error) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		server, err := Initializer(ctx)
		if err != nil {
			return
		}

		err = server.ListenAndServe()
		if err != nil {
			err = fmt.Errorf("starting API server: %w", err)
			return
		}

		go waitExitSignal(func() {
			exitCtx, cancelExitCtx := context.WithTimeout(ctx, 10*time.Second)
			defer cancelExitCtx()
			// server.Shutdown already logs
			_ = server.Shutdown(exitCtx)
		})
		return
	},
}

func serverExample() string {
	return `
# for development
starhub-server start server
`
}
