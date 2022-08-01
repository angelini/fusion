package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/angelini/fusion/internal/pb"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func NewCmdDebug() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "debug",
		Short: "Internal testing",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			log := ctx.Value(logKey).(*zap.Logger)

			client, err := newGrpcClient(ctx, log, "fusion-manager.localdomain:80")
			if err != nil {
				return err
			}

			bootResp, err := client.BootSandbox(ctx, &pb.BootSandboxRequest{
				Project: 123,
			})
			if err != nil {
				fmt.Println("zzz")
				return err
			}

			log.Info("sandbox booted", zap.Int64("epoch", bootResp.Epoch), zap.String("host", bootResp.Host), zap.Int32("port", bootResp.Port))

			_, err = client.SetVersion(ctx, &pb.SetVersionRequest{
				Project: 123,
				Version: 1,
			})
			if err != nil {
				return err
			}

			healthResp, err := client.CheckHealth(ctx, &pb.CheckHealthRequest{
				Project: 123,
			})
			if err != nil {
				return err
			}

			status := "unhealthy"
			if healthResp.Status == pb.CheckHealthResponse_HEALTHY {
				status = "healthy"
			}

			log.Info("sandbox health", zap.String("status", status))
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

func newGrpcClient(ctx context.Context, log *zap.Logger, server string) (pb.ManagerClient, error) {
	connectCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(connectCtx, server, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("cannot connect to grpc server %v: %w", server, err)
	}

	return pb.NewManagerClient(conn), nil
}
