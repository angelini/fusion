package manager

import (
	"context"
	"fmt"
	"strconv"
	"time"

	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	appsconf "k8s.io/client-go/applyconfigurations/apps/v1"
	coreconf "k8s.io/client-go/applyconfigurations/core/v1"
	metaconf "k8s.io/client-go/applyconfigurations/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	FIELD_MANAGER = "fusion/manager"
)

type KubeClient struct {
	epoch     int64
	namespace string
	image     string
	set       *kubernetes.Clientset
}

func NewKubeClient(epoch int64, namespace, image string) (*KubeClient, error) {
	config, err := clientcmd.BuildConfigFromFlags("", "")
	if err != nil {
		return nil, fmt.Errorf("cannot load cluster kubeconfig: %w", err)
	}

	set, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("cannot build clientset: %w", err)
	}

	return &KubeClient{
		epoch:     epoch,
		namespace: namespace,
		image:     image,
		set:       set,
	}, nil
}

func (c *KubeClient) CreateDeployment(ctx context.Context, name string) error {
	_, err := c.set.AppsV1().
		Deployments(c.namespace).
		Apply(ctx, c.genDeployment(name), meta.ApplyOptions{FieldManager: FIELD_MANAGER})
	if err != nil {
		return fmt.Errorf("cannot apply deployment %v: %w", name, err)
	}

	_, err = c.set.CoreV1().
		Services(c.namespace).
		Apply(ctx, c.genService(name), meta.ApplyOptions{FieldManager: FIELD_MANAGER})
	if err != nil {
		return fmt.Errorf("cannot apply service %v: %w", name, err)
	}

	return nil
}

func (c *KubeClient) DeleteDeployment(ctx context.Context, name string) error {
	return c.set.AppsV1().Deployments(c.namespace).Delete(ctx, name, meta.DeleteOptions{})
}

func (c *KubeClient) WaitForEndpoint(ctx context.Context, name string) error {
	selector := fmt.Sprintf("metadata.name=%s", name)
	idx := 0

	for {
		if idx > 20 {
			return fmt.Errorf("too many attempts (%v) to find service", idx)
		}

		list, err := c.set.CoreV1().
			Endpoints(c.namespace).
			List(ctx, meta.ListOptions{FieldSelector: selector})
		if err != nil {
			return fmt.Errorf("cannot list services %v: %w", name, err)
		}

		if len(list.Items) > 0 {
			endpoint := list.Items[0]
			if len(endpoint.Subsets) > 0 {
				// FIXME: Wait for DNS confirmation
				time.Sleep(4 * time.Second)
				return nil
			}
		}

		time.Sleep(500 * time.Millisecond)
		idx += 1
	}
}

func (c *KubeClient) GetAllEndpoints(ctx context.Context, name string) ([]string, error) {
	selector := fmt.Sprintf("metadata.name=%s", name)
	list, err := c.set.CoreV1().
		Endpoints(c.namespace).
		List(ctx, meta.ListOptions{FieldSelector: selector})
	if err != nil {
		return nil, fmt.Errorf("cannot list endpoints of %v: %w", name, err)
	}

	ips := make([]string, len(list.Items))
	for _, endpoint := range list.Items {
		if len(endpoint.Subsets) > 0 {
			if len(endpoint.Subsets[0].Addresses) > 0 {
				ips = append(ips, endpoint.Subsets[0].Addresses[0].IP)
			}
		}
	}

	return ips, nil
}

func (c *KubeClient) genDeployment(name string) *appsconf.DeploymentApplyConfiguration {
	labels := map[string]string{
		"fusion/type":  "node",
		"fusion/name":  name,
		"fusion/epoch": strconv.FormatInt(c.epoch, 10),
	}

	return appsconf.Deployment(name, c.namespace).
		WithLabels(labels).
		WithSpec(
			appsconf.DeploymentSpec().
				WithReplicas(1).
				WithSelector(
					metaconf.LabelSelector().
						WithMatchLabels(labels),
				).
				WithTemplate(
					coreconf.PodTemplateSpec().
						WithLabels(labels).
						WithSpec(
							coreconf.PodSpec().
								WithContainers(c.genContainer()),
						),
				),
		)
}

func (c *KubeClient) genService(name string) *coreconf.ServiceApplyConfiguration {
	labels := map[string]string{
		"fusion/name": name,
	}

	return coreconf.Service(name, c.namespace).
		WithSpec(
			coreconf.ServiceSpec().
				WithSelector(labels).
				WithPorts(
					coreconf.ServicePort().
						WithProtocol(core.ProtocolTCP).
						WithPort(80).
						WithTargetPort(intstr.FromInt(5152)),
				),
		)
}

func (c *KubeClient) genContainer() *coreconf.ContainerApplyConfiguration {
	port := coreconf.ContainerPort().
		WithContainerPort(5152)

	return coreconf.Container().
		WithName("sandbox").
		WithImage(c.image).
		WithImagePullPolicy(core.PullNever).
		WithPorts(port).
		WithCommand("./fusion", "sandbox", "-p", "5152")
}
