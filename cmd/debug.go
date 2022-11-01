package cmd

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"time"

	"github.com/angelini/fusion/internal/pb"
	dlc "github.com/gadget-inc/dateilager/pkg/client"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
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

			managerClient, err := newManagerClient(ctx, log, "fusion-manager.localdomain:443")
			if err != nil {
				return fmt.Errorf("failed to create manager client: %w", err)
			}

			err = dlClient.NewProject(ctx, 123, nil, "")
			if err != nil {
				return err
			}

			version, _, err := dlClient.Update(ctx, 123, "example.mjs")
			if err != nil {
				return err
			}

			// FIXME: Include the DL_TOKEN in this sandbox
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
				return err
			}

			healthResp, err := managerClient.CheckHealth(ctx, &pb.CheckHealthRequest{
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

func newManagerClient(ctx context.Context, log *zap.Logger, server string) (pb.ManagerClient, error) {
	connectCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	pool, err := x509.SystemCertPool()
	if err != nil {
		return nil, fmt.Errorf("load system cert pool: %w", err)
	}

	creds := credentials.NewTLS(&tls.Config{RootCAs: pool})

	conn, err := grpc.DialContext(connectCtx, server, grpc.WithTransportCredentials(creds))
	if err != nil {
		return nil, fmt.Errorf("cannot connect to grpc server %v: %w", server, err)
	}

	return pb.NewManagerClient(conn), nil
}
