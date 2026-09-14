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
	return m.ConnectWithContext("")
}

// ConnectWithContext connects using a specific context name or default
func (m *ClientManager) ConnectWithContext(targetContext string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var config *rest.Config
	var err error
	var contextName string

	// 1. Try In-Cluster Config if targetContext is "in-cluster" or empty and in-cluster succeeds
	if targetContext == "in-cluster" || targetContext == "" {
		inClusterCfg, inClusterErr := rest.InClusterConfig()
		if inClusterErr == nil {
			config = inClusterCfg
			contextName = "in-cluster"
		}
	}

	if config == nil {
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
			if targetContext != "" {
				configOverrides.CurrentContext = targetContext
			}

			clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)

			rawConfig, rawErr := clientConfig.RawConfig()
			if rawErr == nil {
				if targetContext != "" {
					contextName = targetContext
				} else {
					contextName = rawConfig.CurrentContext
				}
			}

			config, err = clientConfig.ClientConfig()
		}
	}

	if err != nil || config == nil {
		m.Connected = false
		if err == nil {
			err = fmt.Errorf("no valid kubeconfig or in-cluster config found")
		}
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

// SetKubeconfigContent connects to a cluster using a raw Kubeconfig string (e.g. Raspberry Pi or external K8s)
func (m *ClientManager) SetKubeconfigContent(rawKubeconfig []byte, customContextName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	clientConfig, err := clientcmd.NewClientConfigFromBytes(rawKubeconfig)
	if err != nil {
		return fmt.Errorf("invalid kubeconfig format: %w", err)
	}

	rawConfig, err := clientConfig.RawConfig()
	if err != nil {
		return fmt.Errorf("failed to parse kubeconfig raw config: %w", err)
	}

	contextName := customContextName
	if contextName == "" {
		contextName = rawConfig.CurrentContext
	}
	if contextName == "" {
		contextName = "external-cluster"
	}

	config, err := clientConfig.ClientConfig()
	if err != nil {
		return fmt.Errorf("failed to build client config from bytes: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create clientset: %w", err)
	}

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create dynamic client: %w", err)
	}

	// Verify connectivity to the external cluster (e.g. Raspberry Pi)
	_, err = clientset.Discovery().ServerVersion()
	if err != nil {
		m.Connected = false
		m.LastError = fmt.Errorf("external cluster unreachable: %w", err)
		return m.LastError
	}

	// Save custom kubeconfig to ~/.kube/navis-external.yaml for persistence
	homeDir, err := os.UserHomeDir()
	if err == nil {
		navisKubeDir := filepath.Join(homeDir, ".kube")
		_ = os.MkdirAll(navisKubeDir, 0755)
		_ = os.WriteFile(filepath.Join(navisKubeDir, "navis-external.yaml"), rawKubeconfig, 0600)
	}

	m.Config = config
	m.Clientset = clientset
	m.DynamicClient = dynamicClient
	m.Connected = true
	m.ContextName = contextName
	m.LastError = nil

	return nil
}

// GetAvailableContexts lists all available contexts in ~/.kube/config
func (m *ClientManager) GetAvailableContexts() ([]string, string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		homeDir, err := os.UserHomeDir()
		if err == nil {
			kubeconfig = filepath.Join(homeDir, ".kube", "config")
		}
	}

	if kubeconfig == "" {
		return []string{}, m.ContextName, nil
	}

	loadingRules := &clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfig}
	configOverrides := &clientcmd.ConfigOverrides{}
	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)

	rawConfig, err := clientConfig.RawConfig()
	if err != nil {
		return []string{}, m.ContextName, err
	}

	contexts := make([]string, 0, len(rawConfig.Contexts))
	for name := range rawConfig.Contexts {
		contexts = append(contexts, name)
	}

	return contexts, m.ContextName, nil
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
