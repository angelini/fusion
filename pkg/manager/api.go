package manager

import (
	"context"

	"github.com/angelini/fusion/internal/pb"
	"go.uber.org/zap"
)

type ManagerApi struct {
	pb.UnimplementedManagerServer

	log   *zap.Logger
	epoch int64
}

func (m *ManagerApi) GetRoute(ctx context.Context, req *pb.GetRouteRequest) (*pb.GetRouteResponse, error) {
	return nil, nil
}
