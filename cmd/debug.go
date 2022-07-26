package cmd

import (
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

func NewCmdDebug() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "debug",
		Short: "Internal testing",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			log := ctx.Value(logKey).(*zap.Logger)

			log.Info("unimplemented")
			return nil
			// netLoc, err := manager.CreateDeployment(ctx, 1, "abc")
			// if err != nil {
			// 	return err
			// }

			// log.Info("debug result", zap.String("host", netLoc.Host))
			// return nil

			// err := manager.DeleteDeployment(ctx, "abc")
			// if err != nil {
			// 	return err
			// }

			// log.Info("debug result")
			// return nil
		},
	}

	return cmd
}
