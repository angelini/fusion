package podproxy

import (
	"context"
	"fmt"
	"time"

	"github.com/angelini/fusion/internal/pb"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func NewClient(ctx context.Context, log *zap.Logger, server string) (pb.ManagerClient, error) {
	connectCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(connectCtx, server, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("cannot connect to grpc server %v: %w", server, err)
	}

	return pb.NewManagerClient(conn), nil
}
