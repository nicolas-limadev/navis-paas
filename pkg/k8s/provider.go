package k8s

import (
	"context"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Provider string

const (
	ProviderMinikube       Provider = "minikube"
	ProviderK3d            Provider = "k3d"
	ProviderKind           Provider = "kind"
	ProviderDockerDesktop  Provider = "docker-desktop"
	ProviderRancherDesktop Provider = "rancher-desktop"
	ProviderMicroK8s       Provider = "microk8s"
	ProviderUnknown        Provider = "unknown"
)

type ProviderInfo struct {
	Name            Provider
	Version         string
	ExposeCommand   string
	LoadBalancerIP  string
	NodePortRange   string
	SupportsTunnel  bool
}

// DetectProvider identifies the Kubernetes provider from the cluster context
func (m *ClientManager) DetectProvider(ctx context.Context) *ProviderInfo {
	if m.Clientset == nil {
		return &ProviderInfo{Name: ProviderUnknown}
	}

	// Check node labels and annotations to detect provider
	nodes, err := m.Clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil || len(nodes.Items) == 0 {
		return &ProviderInfo{Name: ProviderUnknown}
	}

	node := nodes.Items[0]
	labels := node.Labels
	annotations := node.Annotations

	// Minikube detection
	if strings.Contains(m.ContextName, "minikube") ||
		labels["minikube.k8s.io/version"] != "" ||
		annotations["minikube.k8s.io/primary-ip"] != "" {
		return &ProviderInfo{
			Name:           ProviderMinikube,
			ExposeCommand:  "minikube tunnel",
			LoadBalancerIP: "192.168.49.2",
			NodePortRange:  "30000-32767",
			SupportsTunnel: true,
		}
	}

	// k3d detection
	if strings.Contains(m.ContextName, "k3d") ||
		labels["node-role.kubernetes.io/control-plane"] != "" && annotations["k3s.io/hostname"] != "" ||
		annotations["k3d.io/cluster"] != "" {
		return &ProviderInfo{
			Name:           ProviderK3d,
			ExposeCommand:  "k3d node list",
			LoadBalancerIP: "0.0.0.0",
			NodePortRange:  "30000-32767",
			SupportsTunnel: false,
		}
	}

	// Kind detection
	if strings.Contains(m.ContextName, "kind") ||
		labels["node-role.kubernetes.io/control-plane"] != "" && annotations["io.x-k8s.kind/node"] != "" {
		return &ProviderInfo{
			Name:           ProviderKind,
			ExposeCommand:  "kubectl port-forward",
			LoadBalancerIP: "127.0.0.1",
			NodePortRange:  "30000-32767",
			SupportsTunnel: false,
		}
	}

	// Docker Desktop detection
	if strings.Contains(m.ContextName, "docker-desktop") ||
		labels["node-role.kubernetes.io/control-plane"] != "" && strings.Contains(node.Status.NodeInfo.OSImage, "Docker Desktop") {
		return &ProviderInfo{
			Name:           ProviderDockerDesktop,
			ExposeCommand:  "kubectl port-forward",
			LoadBalancerIP: "127.0.0.1",
			NodePortRange:  "30000-32767",
			SupportsTunnel: false,
		}
	}

	// Rancher Desktop detection
	if strings.Contains(m.ContextName, "rancher-desktop") ||
		annotations["rancher.io/node-name"] != "" {
		return &ProviderInfo{
			Name:           ProviderRancherDesktop,
			ExposeCommand:  "kubectl port-forward",
			LoadBalancerIP: "127.0.0.1",
			NodePortRange:  "30000-32767",
			SupportsTunnel: false,
		}
	}

	// MicroK8s detection
	if strings.Contains(m.ContextName, "microk8s") ||
		labels["node.kubernetes.io/microk8s-controlplane"] != "" {
		return &ProviderInfo{
			Name:           ProviderMicroK8s,
			ExposeCommand:  "microk8s kubectl port-forward",
			LoadBalancerIP: "127.0.0.1",
			NodePortRange:  "30000-32767",
			SupportsTunnel: false,
		}
	}

	// Generic Kubernetes
	return &ProviderInfo{
		Name:           ProviderUnknown,
		ExposeCommand:  "kubectl port-forward",
		LoadBalancerIP: "127.0.0.1",
		NodePortRange:  "30000-32767",
		SupportsTunnel: false,
	}
}

// GetProvider returns the detected provider info
func (m *ClientManager) GetProvider() *ProviderInfo {
	ctx := context.Background()
	return m.DetectProvider(ctx)
}
