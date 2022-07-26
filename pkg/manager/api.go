package manager

import (
	"context"
	"fmt"

	"github.com/angelini/fusion/internal/pb"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	NAMESPACE = "fusion"
	IMAGE     = "localhost/fusion:latest"
)

type ManagerApi struct {
	pb.UnimplementedManagerServer

	log   *zap.Logger
	epoch int64
}

func (m *ManagerApi) BootSandbox(ctx context.Context, req *pb.BootSandboxRequest) (*pb.BootSandboxResponse, error) {
	client, err := NewKubeClient(m.epoch, NAMESPACE, IMAGE)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Manager failed to create kube client: %v", err)
	}

	err = client.CreateDeployment(ctx, req.Key)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Manager failed to boot %v: %v", req.Key, err)
	}

	err = client.WaitForEndpoint(ctx, req.Key)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Manager failed to wait for %v: %v", req.Key, err)
	}

	return &pb.BootSandboxResponse{
		Epoch: m.epoch,
		Host:  fmt.Sprintf("%s.%s.svc.cluster.local", req.Key, NAMESPACE),
	}, nil
}
