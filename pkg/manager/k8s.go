package manager

import (
	"context"
	"fmt"
	"strconv"

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
	NAMESPACE     = "fusion"
	K8S_CONFIG    = "/etc/rancher/k3s/k3s.yaml"
	IMAGE         = "localhost/fusion:latest"
)

func CreateDeployment(ctx context.Context, epoch int64, key string) (NetLocation, error) {
	client, err := k8sClient()
	if err != nil {
		return NetLocation{}, err
	}

	deployment, err := client.AppsV1().
		Deployments(NAMESPACE).
		Apply(ctx, genDeployment(key, IMAGE, epoch), meta.ApplyOptions{FieldManager: FIELD_MANAGER})
	if err != nil {
		return NetLocation{}, fmt.Errorf("cannot apply deployment %v: %w", key, err)
	}

	_, err = client.CoreV1().
		Services(NAMESPACE).
		Apply(ctx, genService(key, epoch), meta.ApplyOptions{FieldManager: FIELD_MANAGER})
	if err != nil {
		return NetLocation{}, fmt.Errorf("cannot apply service %v: %w", key, err)
	}

	return NetLocation{
		Host: deployment.GetName(),
		Port: 3333,
	}, nil
}

func DeleteDeployment(ctx context.Context, key string) error {
	client, err := k8sClient()
	if err != nil {
		return err
	}

	return client.AppsV1().Deployments(NAMESPACE).Delete(ctx, key, meta.DeleteOptions{})
}

func genDeployment(name, image string, epoch int64) *appsconf.DeploymentApplyConfiguration {
	labels := map[string]string{
		"fusion/type":  "node",
		"fusion/name":  name,
		"fusion/epoch": strconv.FormatInt(epoch, 10),
	}

	return appsconf.Deployment(name, NAMESPACE).
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
								WithContainers(genContainer(image)),
						),
				),
		)
}

func genService(name string, epoch int64) *coreconf.ServiceApplyConfiguration {
	labels := map[string]string{
		"fusion/type":  "node",
		"fusion/name":  name,
		"fusion/epoch": strconv.FormatInt(epoch, 10),
	}

	return coreconf.Service(name, NAMESPACE).
		WithSpec(
			coreconf.ServiceSpec().
				WithSelector(labels).
				WithPorts(
					coreconf.ServicePort().
						WithPort(80).
						WithTargetPort(intstr.FromInt(5152)),
				),
		)
}

func genContainer(image string) *coreconf.ContainerApplyConfiguration {
	port := coreconf.ContainerPort().
		WithContainerPort(5152)

	return coreconf.Container().
		WithName("sandbox").
		WithImage(image).
		WithImagePullPolicy(core.PullNever).
		WithPorts(port).
		WithCommand("./fusion", "sandbox", "-p", "5152")
}

func k8sClient() (*kubernetes.Clientset, error) {
	config, err := clientcmd.BuildConfigFromFlags("", K8S_CONFIG)
	if err != nil {
		return nil, fmt.Errorf("cannot load kubeconfig %v: %w", K8S_CONFIG, err)
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("cannot build clientset: %w", err)
	}

	return client, nil
}
