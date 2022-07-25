package cmd

import (
	"github.com/angelini/fusion/pkg/podproxy"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

func NewCmdPodProxy() *cobra.Command {
	var port int

	cmd := &cobra.Command{
		Use:   "podproxy",
		Short: "Proxies requests to the correct pod",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			log := ctx.Value(logKey).(*zap.Logger)

			log.Info("start pod proxy", zap.Int("port", port))
			return podproxy.StartProxy(ctx, log, port)
		},
	}

	cmd.PersistentFlags().IntVarP(&port, "port", "p", 5152, "Pod proxy port")

	return cmd
}
