package cmd

import (
	"github.com/angelini/fusion/pkg/router"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

func NewCmdRouter() *cobra.Command {
	var port int

	cmd := &cobra.Command{
		Use:   "router",
		Short: "Route requests to running sandboxes",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			log := ctx.Value(logKey).(*zap.Logger)

			log.Info("start router", zap.Int("port", port))
			return router.StartServer(ctx, log, port)
		},
	}

	cmd.PersistentFlags().IntVarP(&port, "port", "p", 5152, "Router port to listen on")

	return cmd
}
