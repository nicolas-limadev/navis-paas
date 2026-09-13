package offers

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"navispaas/pkg/models"
	"navispaas/pkg/k8s"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	RedisNamespace = "redis"
)

type RedisOffer struct{}

func (r *RedisOffer) GetDefinition() models.OfferDefinition {
	return models.OfferDefinition{
		ID:          "redis",
		Name:        "Redis In-Memory Cache",
		Category:    "Database",
		Description: "Ultra-fast in-memory key-value data store for caching and session management.",
		Version:     "7.2-alpine",
		Icon:        "redis",
		Parameters: []models.OfferParameter{
			{
				Key:         "dbIndex",
				Label:       "Database Index",
				Type:        "number",
				Default:     "0",
				Description: "Redis DB number (0-15)",
				Required:    false,
			},
		},
	}
}

func (r *RedisOffer) IsInstalled(ctx context.Context, cm *k8s.ClientManager) (bool, error) {
	if !cm.Connected {
		return false, nil
	}
	// Check primary redis deployment
	_, err := cm.Clientset.AppsV1().Deployments(RedisNamespace).Get(ctx, "redis", metav1.GetOptions{})
	if err == nil {
		return true, nil
	}
	// Fallback to legacy namespace if present
	_, err = cm.Clientset.AppsV1().Deployments("data").Get(ctx, "redis", metav1.GetOptions{})
	return err == nil, nil
}

func (r *RedisOffer) Install(ctx context.Context, cm *k8s.ClientManager) error {
	if !cm.Connected {
		return fmt.Errorf("kubernetes cluster not connected")
	}
	ns := RedisNamespace

	// 1. Ensure well-described namespace with cloud-native labels and annotations
	_, err := cm.Clientset.CoreV1().Namespaces().Get(ctx, ns, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		nsObj := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: ns,
				Labels: map[string]string{
					"name":                      ns,
					"app.kubernetes.io/name":    "redis",
					"app.kubernetes.io/part-of": "navispaas",
					"navispaas.io/managed-by":   "navispaas",
					"navispaas.io/tier":         "cache",
					"navispaas.io/component":    "redis",
				},
				Annotations: map[string]string{
					"navispaas.io/description": "Redis in-memory cache and data store managed by NavisPaaS",
				},
			},
		}
		_, err = cm.Clientset.CoreV1().Namespaces().Create(ctx, nsObj, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create redis namespace: %w", err)
		}
	}

	replicas := int32(1)
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "redis",
			Namespace: ns,
			Labels: map[string]string{
				"app":                       "redis",
				"app.kubernetes.io/name":    "redis",
				"app.kubernetes.io/part-of": "navispaas",
				"navispaas.io/managed-by":   "navispaas",
				"navispaas.io/component":    "redis",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "redis"}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":                       "redis",
						"app.kubernetes.io/name":    "redis",
						"app.kubernetes.io/part-of": "navispaas",
						"navispaas.io/managed-by":   "navispaas",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "redis",
							Image: "redis:7.2-alpine",
							Ports: []corev1.ContainerPort{{ContainerPort: 6379, Name: "redis"}},
						},
					},
				},
			},
		},
	}
	_, err = cm.Clientset.AppsV1().Deployments(ns).Get(ctx, "redis", metav1.GetOptions{})
	if errors.IsNotFound(err) {
		_, _ = cm.Clientset.AppsV1().Deployments(ns).Create(ctx, dep, metav1.CreateOptions{})
	}

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "redis-service",
			Namespace: ns,
			Labels: map[string]string{
				"app":                       "redis",
				"app.kubernetes.io/name":    "redis",
				"navispaas.io/managed-by":   "navispaas",
			},
		},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeClusterIP,
			Selector: map[string]string{"app": "redis"},
			Ports:    []corev1.ServicePort{{Name: "redis", Port: 6379, TargetPort: intstr.FromInt(6379)}},
		},
	}
	_, err = cm.Clientset.CoreV1().Services(ns).Get(ctx, "redis-service", metav1.GetOptions{})
	if errors.IsNotFound(err) {
		_, _ = cm.Clientset.CoreV1().Services(ns).Create(ctx, svc, metav1.CreateOptions{})
	}
	return nil
}

func (r *RedisOffer) Bind(ctx context.Context, cm *k8s.ClientManager, app *models.Application, params map[string]string) (map[string]string, error) {
	if !cm.Connected {
		return nil, fmt.Errorf("kubernetes cluster not connected")
	}
	db := params["dbIndex"]
	if db == "" {
		db = "0"
	}

	targetNS := RedisNamespace
	if _, err := cm.Clientset.CoreV1().Services(RedisNamespace).Get(ctx, "redis-service", metav1.GetOptions{}); err != nil {
		if _, err := cm.Clientset.CoreV1().Services("data").Get(ctx, "redis-service", metav1.GetOptions{}); err == nil {
			targetNS = "data"
		}
	}
	host := fmt.Sprintf("redis-service.%s.svc.cluster.local", targetNS)
	injectedEnvs := map[string]string{
		"REDIS_HOST": host,
		"REDIS_PORT": "6379",
		"REDIS_DB":   db,
		"REDIS_URL":  fmt.Sprintf("redis://%s:6379/%s", host, db),
	}

	dep, err := cm.Clientset.AppsV1().Deployments(app.Namespace).Get(ctx, app.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
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
				container.Env = append(container.Env, corev1.EnvVar{Name: key, Value: val})
			}
		}
	}

	var linkedOffers []models.LinkedOffer
	if dep.Annotations != nil && dep.Annotations[k8s.AnnotationOffers] != "" {
		_ = json.Unmarshal([]byte(dep.Annotations[k8s.AnnotationOffers]), &linkedOffers)
	}
	newOffer := models.LinkedOffer{
		OfferID:   "redis",
		OfferName: "Redis In-Memory Cache",
		BoundAt:   time.Now(),
		Config:    injectedEnvs,
	}
	updated := false
	for i, o := range linkedOffers {
		if o.OfferID == "redis" {
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
	return injectedEnvs, err
}

func (r *RedisOffer) Unbind(ctx context.Context, cm *k8s.ClientManager, app *models.Application) error {
	if !cm.Connected {
		return fmt.Errorf("kubernetes cluster not connected")
	}
	dep, err := cm.Clientset.AppsV1().Deployments(app.Namespace).Get(ctx, app.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	redisKeys := map[string]bool{"REDIS_HOST": true, "REDIS_PORT": true, "REDIS_DB": true, "REDIS_URL": true}
	if len(dep.Spec.Template.Spec.Containers) > 0 {
		var filteredEnv []corev1.EnvVar
		for _, env := range dep.Spec.Template.Spec.Containers[0].Env {
			if !redisKeys[env.Name] {
				filteredEnv = append(filteredEnv, env)
			}
		}
		dep.Spec.Template.Spec.Containers[0].Env = filteredEnv
	}
	var linkedOffers []models.LinkedOffer
	if dep.Annotations != nil && dep.Annotations[k8s.AnnotationOffers] != "" {
		_ = json.Unmarshal([]byte(dep.Annotations[k8s.AnnotationOffers]), &linkedOffers)
		var filteredOffers []models.LinkedOffer
		for _, o := range linkedOffers {
			if o.OfferID != "redis" {
				filteredOffers = append(filteredOffers, o)
			}
		}
		bytes, _ := json.Marshal(filteredOffers)
		dep.Annotations[k8s.AnnotationOffers] = string(bytes)
	}
	_, err = cm.Clientset.AppsV1().Deployments(app.Namespace).Update(ctx, dep, metav1.UpdateOptions{})
	return err
}
