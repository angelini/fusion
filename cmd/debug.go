package cmd

import (
	"fmt"
	"os"

	"github.com/angelini/fusion/internal/pb"
	"github.com/angelini/fusion/pkg/manager"
	dlc "github.com/gadget-inc/dateilager/pkg/client"
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

			dlClient, err := dlc.NewClient(ctx, "dateilager.localdomain:443")
			if err != nil {
				return fmt.Errorf("failed to create dl client: %w", err)
			}

			managerClient, err := manager.NewClient(ctx, log, "fusion-manager.localdomain:443")
			if err != nil {
				return fmt.Errorf("failed to create manager client: %w", err)
			}

			err = dlClient.NewProject(ctx, 123, nil, nil)
			if err != nil {
				return err
			}

			os.RemoveAll("./example/.dl")
			version, _, err := dlClient.Update(ctx, 123, "./example")
			if err != nil {
				return err
			}

			log.Info("DL project updated", zap.Int("project", 123), zap.Int64("version", version))

			bootResp, err := managerClient.BootSandbox(ctx, &pb.BootSandboxRequest{
				Project: 123,
			})
			if err != nil {
				return fmt.Errorf("failed to boot sandbox: %w", err)
			}

			log.Info("sandbox booted", zap.Int64("epoch", bootResp.Epoch), zap.String("host", bootResp.Host), zap.Int32("port", bootResp.Port))

			_, err = managerClient.SetVersion(ctx, &pb.SetVersionRequest{
				Project: 123,
				Version: version,
			})
			if err != nil {
				return fmt.Errorf("failed to set sandbox version: %w", err)
			}

			healthResp, err := managerClient.CheckHealth(ctx, &pb.CheckHealthRequest{
				Project: 123,
			})
			if err != nil {
				return fmt.Errorf("failed to check sandbox health: %w", err)
			}

			status := "unhealthy"
			if healthResp.Status == pb.CheckHealthResponse_HEALTHY {
				status = "healthy"
			}

			log.Info("sandbox health", zap.String("status", status))

			version, _, err = dlClient.Update(ctx, 123, "./example-2")
			if err != nil {
				return err
			}

			log.Info("DL project updated", zap.Int("project", 123), zap.Int64("version", version))

			_, err = managerClient.SetVersion(ctx, &pb.SetVersionRequest{
				Project: 123,
				Version: version,
			})
			if err != nil {
				return fmt.Errorf("failed to set sandbox version: %w", err)
			}

			healthResp, err = managerClient.CheckHealth(ctx, &pb.CheckHealthRequest{
				Project: 123,
			})
			if err != nil {
				return fmt.Errorf("failed to check sandbox health: %w", err)
			}

			status = "unhealthy"
			if healthResp.Status == pb.CheckHealthResponse_HEALTHY {
				status = "healthy"
			}

			log.Info("sandbox health", zap.String("status", status))

			return nil
		},
	}

	return cmd
}
