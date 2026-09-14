package k8s

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/nicolas-limadev/navis-paas/pkg/models"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	DefaultRegistrySecretName = "navis-registry-secret"
)

type DockerConfigEntry struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Email    string `json:"email,omitempty"`
	Auth     string `json:"auth"`
}

type DockerConfigJSON struct {
	Auths map[string]DockerConfigEntry `json:"auths"`
}

type RegistryService struct {
	mu            sync.RWMutex
	clientManager *ClientManager
	config        models.RegistryConfig
}

func NewRegistryService(cm *ClientManager) *RegistryService {
	return &RegistryService{
		clientManager: cm,
		config: models.RegistryConfig{
			SecretName: DefaultRegistrySecretName,
			Enabled:    false,
		},
	}
}

// GetConfig returns the current registry config
func (r *RegistryService) GetConfig() models.RegistryConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.config
}

// GetStatus returns the safe (masked password) status of the registry
func (r *RegistryService) GetStatus() models.RegistryStatusResponse {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return models.RegistryStatusResponse{
		Server:        r.config.Server,
		Username:      r.config.Username,
		HasPassword:   r.config.Password != "",
		DefaultPrefix: r.config.DefaultPrefix,
		Enabled:       r.config.Enabled,
		SecretName:    r.config.SecretName,
		UpdatedAt:     r.config.UpdatedAt,
	}
}

// SaveConfig updates the registry credentials and syncs the secret to Kubernetes
func (r *RegistryService) SaveConfig(ctx context.Context, cfg models.RegistryConfig) error {
	r.mu.Lock()
	if cfg.SecretName == "" {
		cfg.SecretName = DefaultRegistrySecretName
	}
	// If password wasn't passed in update but existed before, keep previous
	if cfg.Password == "" && r.config.Password != "" {
		cfg.Password = r.config.Password
	}
	cfg.UpdatedAt = time.Now()
	r.config = cfg
	r.mu.Unlock()

	if cfg.Enabled && cfg.Username != "" && cfg.Password != "" && r.clientManager.Connected {
		// Sync secret to default namespace
		if err := r.EnsureRegistrySecret(ctx, DefaultNamespace); err != nil {
			return fmt.Errorf("failed to sync registry secret to namespace %s: %w", DefaultNamespace, err)
		}
	}

	return nil
}

// ResolveImage prepends default prefix if the image has no registry/organization prefix
func (r *RegistryService) ResolveImage(image string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if !r.config.Enabled || r.config.DefaultPrefix == "" {
		return image
	}

	// If image doesn't contain a slash (e.g. "orders:v1.0" or "nginx"), prepend the default prefix
	if !strings.Contains(image, "/") {
		return fmt.Sprintf("%s/%s", strings.TrimRight(r.config.DefaultPrefix, "/"), image)
	}

	return image
}

// EnsureRegistrySecret creates or updates the dockerconfigjson Secret in the specified namespace
func (r *RegistryService) EnsureRegistrySecret(ctx context.Context, namespace string) error {
	r.mu.RLock()
	cfg := r.config
	r.mu.RUnlock()

	if !cfg.Enabled || cfg.Username == "" || cfg.Password == "" {
		return nil
	}

	if !r.clientManager.Connected {
		return fmt.Errorf("kubernetes cluster not connected")
	}

	if namespace == "" {
		namespace = DefaultNamespace
	}

	server := cfg.Server
	if server == "" {
		server = "https://index.docker.io/v1/"
	}

	// Build auth string
	authStr := fmt.Sprintf("%s:%s", cfg.Username, cfg.Password)
	authEncoded := base64.StdEncoding.EncodeToString([]byte(authStr))

	dockerConfig := DockerConfigJSON{
		Auths: map[string]DockerConfigEntry{
			server: {
				Username: cfg.Username,
				Password: cfg.Password,
				Email:    cfg.Email,
				Auth:     authEncoded,
			},
		},
	}

	configJSONBytes, err := json.Marshal(dockerConfig)
	if err != nil {
		return fmt.Errorf("failed to encode dockerconfigjson: %w", err)
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cfg.SecretName,
			Namespace: namespace,
			Labels: map[string]string{
				LabelManagedBy: ManagedByValue,
			},
		},
		Type: corev1.SecretTypeDockerConfigJson,
		Data: map[string][]byte{
			corev1.DockerConfigJsonKey: configJSONBytes,
		},
	}

	existing, err := r.clientManager.Clientset.CoreV1().Secrets(namespace).Get(ctx, cfg.SecretName, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		_, err = r.clientManager.Clientset.CoreV1().Secrets(namespace).Create(ctx, secret, metav1.CreateOptions{})
	} else if err == nil {
		secret.ResourceVersion = existing.ResourceVersion
		_, err = r.clientManager.Clientset.CoreV1().Secrets(namespace).Update(ctx, secret, metav1.UpdateOptions{})
	}

	return err
}

// GetImagePullSecrets returns the secret reference if private registry is active
func (r *RegistryService) GetImagePullSecrets() []corev1.LocalObjectReference {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if !r.config.Enabled || r.config.Username == "" {
		return nil
	}

	return []corev1.LocalObjectReference{
		{Name: r.config.SecretName},
	}
}
