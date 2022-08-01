package cmd

import (
	"fmt"
	"net"

	"github.com/angelini/fusion/pkg/manager"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

func NewCmdManager() *cobra.Command {
	var port int

	cmd := &cobra.Command{
		Use:   "manager",
		Short: "Manage booting and tearing down sandboxes",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			log := ctx.Value(logKey).(*zap.Logger)

			socket, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
			if err != nil {
				return fmt.Errorf("failed to listen on TCP port %d: %w", port, err)
			}

			server, err := manager.NewServer(log)
			if err != nil {
				return err
			}

			log.Info("start manager", zap.Int("port", port))
			return server.Serve(socket)
		},
	}

	cmd.PersistentFlags().IntVarP(&port, "port", "p", 5152, "Manager port")

	return cmd
}
