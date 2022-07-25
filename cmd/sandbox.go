package cmd

import (
	"github.com/angelini/fusion/pkg/sandbox"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

func NewCmdSandbox() *cobra.Command {
	var port int

	cmd := &cobra.Command{
		Use:   "sandbox",
		Short: "Start the sandbox process pool",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			log := ctx.Value(logKey).(*zap.Logger)

			log.Info("start sandbox", zap.Int("port", port))
			manager := sandbox.NewManager(ctx, log, "127.0.0.1", "node", "script.mjs", 8000)
			return sandbox.StartProxy(ctx, log, manager, port)
		},
	}

	cmd.PersistentFlags().IntVarP(&port, "port", "p", 5152, "Sandbox proxy port")

	return cmd
}
