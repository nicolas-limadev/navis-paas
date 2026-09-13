package k8s

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type ClientManager struct {
	mu            sync.RWMutex
	Clientset     kubernetes.Interface
	DynamicClient dynamic.Interface
	Config        *rest.Config
	Connected     bool
	ContextName   string
	LastError     error
}

var instance *ClientManager
var once sync.Once

// GetClientManager returns the singleton ClientManager
func GetClientManager() *ClientManager {
	once.Do(func() {
		instance = &ClientManager{}
		_ = instance.Connect()
	})
	return instance
}

// Connect attempts to establish connection to the Kubernetes cluster
func (m *ClientManager) Connect() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var config *rest.Config
	var err error
	var contextName string

	// 1. Try In-Cluster Config
	config, err = rest.InClusterConfig()
	if err == nil {
		contextName = "in-cluster"
	} else {
		// 2. Try Kubeconfig file
		kubeconfig := os.Getenv("KUBECONFIG")
		if kubeconfig == "" {
			homeDir, err := os.UserHomeDir()
			if err == nil {
				kubeconfig = filepath.Join(homeDir, ".kube", "config")
			}
		}

		if kubeconfig != "" {
			loadingRules := &clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfig}
			configOverrides := &clientcmd.ConfigOverrides{}
			clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)

			rawConfig, rawErr := clientConfig.RawConfig()
			if rawErr == nil {
				contextName = rawConfig.CurrentContext
			}

			config, err = clientConfig.ClientConfig()
		}
	}

	if err != nil {
		m.Connected = false
		m.LastError = fmt.Errorf("failed to load kubeconfig: %w", err)
		return m.LastError
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		m.Connected = false
		m.LastError = fmt.Errorf("failed to create kubernetes clientset: %w", err)
		return m.LastError
	}

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		m.Connected = false
		m.LastError = fmt.Errorf("failed to create dynamic client: %w", err)
		return m.LastError
	}

	// Verify connectivity
	_, err = clientset.Discovery().ServerVersion()
	if err != nil {
		m.Connected = false
		m.LastError = fmt.Errorf("kubernetes cluster unreachable: %w", err)
		return m.LastError
	}

	m.Config = config
	m.Clientset = clientset
	m.DynamicClient = dynamicClient
	m.Connected = true
	m.ContextName = contextName
	m.LastError = nil

	return nil
}

// CheckConnectivity verifies and refreshes connectivity status
func (m *ClientManager) CheckConnectivity(ctx context.Context) (bool, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.Clientset == nil {
		// Attempt reconnect
		m.mu.Unlock()
		_ = m.Connect()
		m.mu.Lock()
	}

	if !m.Connected || m.Clientset == nil {
		return false, "", m.LastError
	}

	version, err := m.Clientset.Discovery().ServerVersion()
	if err != nil {
		m.Connected = false
		m.LastError = err
		return false, "", err
	}

	return true, version.GitVersion, nil
}
