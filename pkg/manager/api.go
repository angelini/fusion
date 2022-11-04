package manager

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/angelini/fusion/internal/pb"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	HEALTH_CHECK_ATTEMPTS = 5
)

type ManagerApi struct {
	pb.UnimplementedManagerServer

	log        *zap.Logger
	epoch      int64
	namespace  string
	image      string
	kubeClient *KubeClient
}

func NewManagerApi(log *zap.Logger, epoch int64, namespace, image, dlServer string) (*ManagerApi, error) {
	kubeClient, err := NewKubeClient(epoch, namespace, image)
	if err != nil {
		return nil, fmt.Errorf("failed to create k8s client %v [%v]: %w", namespace, image, err)
	}

	return &ManagerApi{
		log:        log,
		epoch:      epoch,
		namespace:  namespace,
		image:      image,
		kubeClient: kubeClient,
	}, nil
}

func (m *ManagerApi) BootSandbox(ctx context.Context, req *pb.BootSandboxRequest) (*pb.BootSandboxResponse, error) {
	m.log.Info("boot sandbox", zap.Int64("project", req.Project))
	name := m.name(req.Project)

	err := m.kubeClient.CreateDeployment(ctx, name, req.Project)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Manager.BootSandbox failed to boot %v: %v", name, err)
	}

	err = m.kubeClient.WaitForEndpoint(ctx, name)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Manager.BootSandbox failed to wait for %v: %v", name, err)
	}

	err = m.updateAllEndpoints(ctx, name, req.Version)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Manager.BootSandbox failed to update versions %v: %v", name, err)
	}

	return &pb.BootSandboxResponse{
		Epoch: m.epoch,
		Host:  m.hostname(name),
	}, nil
}

func (m *ManagerApi) SetVersion(ctx context.Context, req *pb.SetVersionRequest) (*pb.SetVersionResponse, error) {
	m.log.Info("set version", zap.Int64("project", req.Project), zap.Int64p("version", req.Version))
	name := m.name(req.Project)

	err := m.updateAllEndpoints(ctx, name, req.Version)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Manager failed to update versions %v: %v", name, err)
	}

	return &pb.SetVersionResponse{}, nil
}

func (m *ManagerApi) CheckHealth(ctx context.Context, req *pb.CheckHealthRequest) (*pb.CheckHealthResponse, error) {
	m.log.Info("check health", zap.Int64("project", req.Project))
	name := m.name(req.Project)
	client := &http.Client{
		Timeout: 200 * time.Millisecond,
	}

	var resp *http.Response
	var err error

	for idx := 0; idx < HEALTH_CHECK_ATTEMPTS; idx++ {
		resp, err = client.Get(fmt.Sprintf("http://%s/health", m.hostname(name)))
		if err == nil {
			break
		}
		if os.IsTimeout(err) && idx < HEALTH_CHECK_ATTEMPTS-1 {
			continue
		}
		return nil, status.Errorf(codes.Internal, "Manager failed to run health check %v: %v", name, err)
	}

	status := pb.CheckHealthResponse_UNHEALTHY
	if resp.StatusCode < 300 {
		status = pb.CheckHealthResponse_HEALTHY
	}

	return &pb.CheckHealthResponse{
		Status:  status,
		Version: -1,
	}, nil
}

func (m *ManagerApi) hostname(name string) string {
	return fmt.Sprintf("%s.%s.svc.cluster.local", name, m.namespace)
}

func (m *ManagerApi) name(project int64) string {
	return fmt.Sprintf("s-%d", project)
}

func (m *ManagerApi) updateAllEndpoints(ctx context.Context, name string, version *int64) error {
	ips, err := m.kubeClient.GetAllEndpoints(ctx, name)
	if err != nil {
		return err
	}

	client := &http.Client{}
	group, _ := errgroup.WithContext(ctx)

	for _, ip := range ips {
		ip := ip
		if ip == "" {
			continue
		}

		version, err := json.Marshal(map[string]*int64{"version": version})
		if err != nil {
			return err
		}
		body := bytes.NewBuffer(version)

		group.Go(func() error {
			_, err = client.Post(fmt.Sprintf("http://%s:5152/__meta__/version", ip), "application/json", body)
			return err
		})
	}

	return group.Wait()
}
