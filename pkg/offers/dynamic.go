package offers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nicolas-limadev/navis-paas/pkg/models"
	"github.com/nicolas-limadev/navis-paas/pkg/k8s"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"gopkg.in/yaml.v3"
)

// DynamicOfferDefinition is the schema for user-created custom YAML offers
type DynamicOfferDefinition struct {
	ID          string            `yaml:"id"`
	Name        string            `yaml:"name"`
	Category    string            `yaml:"category"`
	Description string            `yaml:"description"`
	Version     string            `yaml:"version"`
	Namespace   string            `yaml:"namespace"`
	Image       string            `yaml:"image"`
	Port        int               `yaml:"port"`
	EnvPrefix   string            `yaml:"envPrefix"`
	Parameters  []models.OfferParameter `yaml:"parameters,omitempty"`
}

type DynamicOffer struct {
	Def DynamicOfferDefinition
}

func (d *DynamicOffer) GetDefinition() models.OfferDefinition {
	return models.OfferDefinition{
		ID:          d.Def.ID,
		Name:        d.Def.Name,
		Category:    d.Def.Category,
		Description: d.Def.Description,
		Version:     d.Def.Version,
		Icon:        "custom",
		Parameters:  d.Def.Parameters,
	}
}

func (d *DynamicOffer) IsInstalled(ctx context.Context, cm *k8s.ClientManager) (bool, error) {
	if !cm.Connected {
		return false, nil
	}
	_, err := cm.Clientset.AppsV1().Deployments(d.Def.Namespace).Get(ctx, d.Def.ID, metav1.GetOptions{})
	if err == nil {
		return true, nil
	}
	return false, nil
}

func (d *DynamicOffer) Install(ctx context.Context, cm *k8s.ClientManager) error {
	if !cm.Connected {
		return fmt.Errorf("kubernetes cluster not connected")
	}

	ns := d.Def.Namespace
	// 1. Create namespace
	_, err := cm.Clientset.CoreV1().Namespaces().Get(ctx, ns, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		nsObj := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: ns,
				Labels: map[string]string{
					"name":                      ns,
					"app.kubernetes.io/name":    d.Def.ID,
					"app.kubernetes.io/part-of": "navispaas",
					"navispaas.io/managed-by":   "navispaas",
				},
				Annotations: map[string]string{
					"navispaas.io/description": fmt.Sprintf("%s managed by NavisPaaS Dynamic Engine", d.Def.Name),
				},
			},
		}
		_, err = cm.Clientset.CoreV1().Namespaces().Create(ctx, nsObj, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create namespace: %w", err)
		}
	}

	// 2. Deployment
	replicas := int32(1)
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      d.Def.ID,
			Namespace: ns,
			Labels:    map[string]string{"app": d.Def.ID, "navispaas.io/managed-by": "navispaas"},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": d.Def.ID}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": d.Def.ID}},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  d.Def.ID,
							Image: d.Def.Image,
							Ports: []corev1.ContainerPort{
								{Name: "http", ContainerPort: int32(d.Def.Port)},
							},
						},
					},
				},
			},
		},
	}
	_, err = cm.Clientset.AppsV1().Deployments(ns).Get(ctx, d.Def.ID, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		_, _ = cm.Clientset.AppsV1().Deployments(ns).Create(ctx, dep, metav1.CreateOptions{})
	}

	// 3. Service (NodePort)
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      d.Def.ID + "-service",
			Namespace: ns,
			Labels:    map[string]string{"app": d.Def.ID},
		},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeNodePort,
			Selector: map[string]string{"app": d.Def.ID},
			Ports: []corev1.ServicePort{
				{Name: "http", Port: int32(d.Def.Port), TargetPort: intstr.FromInt(d.Def.Port)},
			},
		},
	}
	_, err = cm.Clientset.CoreV1().Services(ns).Get(ctx, d.Def.ID+"-service", metav1.GetOptions{})
	if errors.IsNotFound(err) {
		_, _ = cm.Clientset.CoreV1().Services(ns).Create(ctx, svc, metav1.CreateOptions{})
	}

	return nil
}

func (d *DynamicOffer) Bind(ctx context.Context, cm *k8s.ClientManager, app *models.Application, params map[string]string) (map[string]string, error) {
	if !cm.Connected {
		return nil, fmt.Errorf("kubernetes cluster not connected")
	}

	dep, err := cm.Clientset.AppsV1().Deployments(app.Namespace).Get(ctx, app.Name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("application deployment not found: %w", err)
	}

	// Generate environment variables dynamically based on EnvPrefix
	prefix := strings.ToUpper(d.Def.EnvPrefix)
	if prefix == "" {
		prefix = strings.ToUpper(d.Def.ID)
	}

	host := fmt.Sprintf("%s-service.%s.svc.cluster.local", d.Def.ID, d.Def.Namespace)
	injectedEnvs := map[string]string{
		prefix + "_HOST": host,
		prefix + "_PORT": fmt.Sprintf("%d", d.Def.Port),
		prefix + "_URL":  fmt.Sprintf("http://%s:%d", host, d.Def.Port),
	}

	// Merge any user-submitted parameter overrides
	for k, v := range params {
		injectedEnvs[prefix+"_PARAM_"+strings.ToUpper(k)] = v
	}

	if len(dep.Spec.Template.Spec.Containers) > 0 {
		container := &dep.Spec.Template.Spec.Containers[0]
		existingEnvMap := make(map[string]int)
		for i, env := range container.Env {
			existingEnvMap[env.Name] = i
		}

		for key, val := range injectedEnvs {
			if idx, exists := existingEnvMap[key]; exists {
				container.Env[idx].Value = val
			} else {
				container.Env = append(container.Env, corev1.EnvVar{
					Name:  key,
					Value: val,
				})
			}
		}
	}

	// Update LinkedOffer annotation
	var linkedOffers []models.LinkedOffer
	if dep.Annotations != nil && dep.Annotations[k8s.AnnotationOffers] != "" {
		_ = json.Unmarshal([]byte(dep.Annotations[k8s.AnnotationOffers]), &linkedOffers)
	}

	newOffer := models.LinkedOffer{
		OfferID:   d.Def.ID,
		OfferName: d.Def.Name,
		BoundAt:   time.Now(),
		Config:    injectedEnvs,
	}

	updated := false
	for i, o := range linkedOffers {
		if o.OfferID == d.Def.ID {
			linkedOffers[i] = newOffer
			updated = true
			break
		}
	}
	if !updated {
		linkedOffers = append(linkedOffers, newOffer)
	}

	if dep.Annotations == nil {
		dep.Annotations = make(map[string]string)
	}
	bytes, _ := json.Marshal(linkedOffers)
	dep.Annotations[k8s.AnnotationOffers] = string(bytes)

	_, err = cm.Clientset.AppsV1().Deployments(app.Namespace).Update(ctx, dep, metav1.UpdateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to bind custom offer: %w", err)
	}

	return injectedEnvs, nil
}

func (d *DynamicOffer) Unbind(ctx context.Context, cm *k8s.ClientManager, app *models.Application) error {
	if !cm.Connected {
		return fmt.Errorf("kubernetes cluster not connected")
	}

	dep, err := cm.Clientset.AppsV1().Deployments(app.Namespace).Get(ctx, app.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	prefix := strings.ToUpper(d.Def.EnvPrefix)
	if prefix == "" {
		prefix = strings.ToUpper(d.Def.ID)
	}

	// Remove dynamic env vars
	if len(dep.Spec.Template.Spec.Containers) > 0 {
		var filteredEnv []corev1.EnvVar
		for _, env := range dep.Spec.Template.Spec.Containers[0].Env {
			if !strings.HasPrefix(env.Name, prefix+"_") {
				filteredEnv = append(filteredEnv, env)
			}
		}
		dep.Spec.Template.Spec.Containers[0].Env = filteredEnv
	}

	// Update LinkedOffer annotation
	var linkedOffers []models.LinkedOffer
	if dep.Annotations != nil && dep.Annotations[k8s.AnnotationOffers] != "" {
		_ = json.Unmarshal([]byte(dep.Annotations[k8s.AnnotationOffers]), &linkedOffers)
		var filteredOffers []models.LinkedOffer
		for _, o := range linkedOffers {
			if o.OfferID != d.Def.ID {
				filteredOffers = append(filteredOffers, o)
			}
		}
		bytes, _ := json.Marshal(filteredOffers)
		dep.Annotations[k8s.AnnotationOffers] = string(bytes)
	}

	_, err = cm.Clientset.AppsV1().Deployments(app.Namespace).Update(ctx, dep, metav1.UpdateOptions{})
	return err
}

// SaveCustomOffer writes a custom offer definition to the local custom-offers directory
func SaveCustomOffer(def DynamicOfferDefinition) error {
	if def.ID == "" || def.Name == "" {
		return fmt.Errorf("offer ID and Name are required")
	}

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	dir := filepath.Join(cwd, "custom-offers")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create custom-offers directory: %w", err)
	}

	filePath := filepath.Join(dir, def.ID+".yaml")
	data, err := yaml.Marshal(def)
	if err != nil {
		return fmt.Errorf("failed to marshal offer to YAML: %w", err)
	}

	return os.WriteFile(filePath, data, 0644)
}

// LoadCustomOffers scans a directory for custom offer YAML definitions
func LoadCustomOffers(dir string) ([]OfferHandler, error) {
	var list []OfferHandler

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return list, nil // Directory not found, skip silently
	}

	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		if !file.IsDir() && (strings.HasSuffix(file.Name(), ".yaml") || strings.HasSuffix(file.Name(), ".yml")) {
			path := filepath.Join(dir, file.Name())
			data, err := os.ReadFile(path)
			if err != nil {
				log.Printf("⚠️ Failed to read custom offer file %s: %v", path, err)
				continue
			}

			var def DynamicOfferDefinition
			if err := yaml.Unmarshal(data, &def); err != nil {
				log.Printf("⚠️ Failed to parse custom offer YAML %s: %v", path, err)
				continue
			}

			if def.ID != "" && def.Name != "" {
				log.Printf("✅ Loaded custom offer: %s (%s)", def.Name, def.ID)
				list = append(list, &DynamicOffer{Def: def})
			} else {
				log.Printf("⚠️ Custom offer %s has empty ID or Name", path)
			}
		}
	}

	return list, nil
}
