package cmd

import (
	"fmt"
	"strconv"

	"github.com/angelini/fusion/pkg/sandbox"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

func NewCmdSandbox() *cobra.Command {
	var port int

	cmd := &cobra.Command{
		Use:   "sandbox",
		Short: "Start the sandbox process pool",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			log := ctx.Value(logKey).(*zap.Logger)

			log.Info("start sandbox", zap.Int("port", port))

			project, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("cannot parse <project> arg: %w", err)
			}

			command := sandbox.NewCommand("node", []string{"/tmp/fusion/script.mjs"}, "/tmp/fusion")
			controller, err := sandbox.NewController(ctx, log, "127.0.0.1", "dateilager-service.fusion.svc.cluster.local:5051", project, command, 8000)
			if err != nil {
				return err
			}

			return sandbox.StartProxy(ctx, log, controller, port)
		},
	}

	cmd.PersistentFlags().IntVarP(&port, "port", "p", 5152, "Sandbox proxy port")

	return cmd
}
