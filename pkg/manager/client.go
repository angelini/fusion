package manager

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"time"

	"github.com/angelini/fusion/internal/pb"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

func NewClient(ctx context.Context, log *zap.Logger, server string) (pb.ManagerClient, error) {
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
