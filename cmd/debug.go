package cmd

import (
	"context"
	"fmt"

	"github.com/angelini/fusion/internal/pb"
	"github.com/angelini/fusion/pkg/manager"
	dlc "github.com/gadget-inc/dateilager/pkg/client"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

func createProject(ctx context.Context, log *zap.Logger, dlClient *dlc.Client, managerClient pb.ManagerClient, project int64) error {
	err := dlClient.NewProject(ctx, project, nil, nil)
	if err != nil {
		return err
	}

	log.Info("dl project created", zap.Int64("project", project))

	bootResp, err := managerClient.BootSandbox(ctx, &pb.BootSandboxRequest{
		Project: project,
	})
	if err != nil {
		return fmt.Errorf("failed to boot sandbox: %w", err)
	}

	log.Info("sandbox booted", zap.Int64("epoch", bootResp.Epoch), zap.String("host", bootResp.Host), zap.Int32("port", bootResp.Port))

	return nil
}

func updateProject(ctx context.Context, log *zap.Logger, dlClient *dlc.Client, managerClient pb.ManagerClient, project int64, dir string) error {
	version, _, err := dlClient.Update(ctx, project, dir)
	if err != nil {
		return err
	}

	log.Info("dl fs updated", zap.Int64("project", project), zap.Int64("version", version))

	_, err = managerClient.SetVersion(ctx, &pb.SetVersionRequest{
		Project: project,
		Version: version,
	})
	if err != nil {
		return fmt.Errorf("failed to set sandbox version: %w", err)
	}

	log.Info("sandbox version updated", zap.Int64("project", project), zap.Int64("version", version))

	healthResp, err := managerClient.CheckHealth(ctx, &pb.CheckHealthRequest{
		Project: project,
	})
	if err != nil {
		return fmt.Errorf("failed to check sandbox health: %w", err)
	}

	status := "unhealthy"
	if healthResp.Status == pb.CheckHealthResponse_HEALTHY {
		status = "healthy"
	}

	log.Info("sandbox health", zap.String("status", status))

	return nil
}

func NewCmdDebug() *cobra.Command {
	var (
		mode    string
		project int64
		dir     string
	)

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

			switch mode {
			case "create":
				return createProject(ctx, log, dlClient, managerClient, project)
			case "update":
				if dir == "" {
					log.Fatal("--dir cannot be emtpy")
				}
				return updateProject(ctx, log, dlClient, managerClient, project, dir)
			default:
				log.Fatal("--mode must be either 'create' or 'update'")
			}

			return nil
		},
	}

	flags := cmd.PersistentFlags()

	flags.StringVar(&mode, "mode", "", "Debug mode (create | update)")
	flags.Int64Var(&project, "project", 0, "Project ID")
	flags.StringVar(&dir, "dir", "", "Directory to push to DateiLager")

	cmd.MarkFlagRequired("mode")
	cmd.MarkFlagRequired("project")

	return cmd
}
