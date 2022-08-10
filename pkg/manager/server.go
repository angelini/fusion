package manager

import (
	"crypto/tls"
	"time"

	"github.com/angelini/fusion/internal/pb"
	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_zap "github.com/grpc-ecosystem/go-grpc-middleware/logging/zap"
	grpc_recovery "github.com/grpc-ecosystem/go-grpc-middleware/recovery"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

func NewServer(log *zap.Logger, cert *tls.Certificate, namespace, image, dlServer string) (*grpc.Server, error) {
	creds := credentials.NewServerTLSFromCert(cert)

	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(
			grpc_middleware.ChainUnaryServer(
				grpc_recovery.UnaryServerInterceptor(),
				grpc_zap.UnaryServerInterceptor(log),
			),
		),
		grpc.StreamInterceptor(
			grpc_middleware.ChainStreamServer(
				grpc_recovery.StreamServerInterceptor(),
				grpc_zap.StreamServerInterceptor(log),
			),
		),
		grpc.Creds(creds),
	)

	api, err := NewManagerApi(log, time.Now().Unix(), namespace, image, dlServer)
	if err != nil {
		return nil, err
	}

	pb.RegisterManagerServer(grpcServer, api)

	return grpcServer, nil
}
