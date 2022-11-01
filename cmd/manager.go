package cmd

import (
	"crypto/tls"
	"fmt"
	"net"

	"github.com/angelini/fusion/pkg/manager"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

func NewCmdManager() *cobra.Command {
	var (
		port     int
		certFile string
		keyFile  string
	)

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

			cert, err := tls.LoadX509KeyPair(certFile, keyFile)
			if err != nil {
				return fmt.Errorf("cannot open TLS cert and key files (%s, %s): %w", certFile, keyFile, err)
			}

			server, err := manager.NewServer(log, &cert, "fusion", "localhost/fusion:latest", "dateilager-server.fusion.svc.cluster.local")
			if err != nil {
				return err
			}

			log.Info("start manager", zap.Int("port", port))
			return server.Serve(socket)
		},
	}

	flags := cmd.PersistentFlags()
	flags.IntVarP(&port, "port", "p", 5152, "Manager port")
	flags.StringVar(&certFile, "cert", "development/server.crt", "TLS cert file")
	flags.StringVar(&keyFile, "key", "development/server.key", "TLS key file")

	return cmd
}
