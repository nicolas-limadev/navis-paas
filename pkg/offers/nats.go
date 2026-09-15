package offers

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/nicolas-limadev/navis-paas/pkg/models"
	"github.com/nicolas-limadev/navis-paas/pkg/k8s"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	NATSNamespace = "nats"
)

type NATSOffer struct{}

func (n *NATSOffer) GetDefinition() models.OfferDefinition {
	return models.OfferDefinition{
		ID:          "nats",
		Name:        "NATS JetStream Broker",
		Category:    "Messaging",
		Description: "NATS - Ultra-lightweight, high-performance messaging system for pub/sub, request-reply, and distributed queues with JetStream persistence enabled.",
		Version:     "2.10",
		Icon:        "nats",
		Parameters: []models.OfferParameter{
			{
				Key:         "enableJetStream",
				Label:       "Enable JetStream",
				Type:        "string",
				Default:     "true",
				Description: "Enable JetStream persistence engine for streams and key-value store",
				Required:    false,
			},
		},
	}
}

func (n *NATSOffer) IsInstalled(ctx context.Context, cm *k8s.ClientManager) (bool, error) {
	if !cm.Connected {
		return false, nil
	}
	_, err := cm.Clientset.AppsV1().Deployments(NATSNamespace).Get(ctx, "nats", metav1.GetOptions{})
	if err == nil {
		return true, nil
	}
	return false, nil
}

func (n *NATSOffer) Install(ctx context.Context, cm *k8s.ClientManager) error {
	if !cm.Connected {
		return fmt.Errorf("kubernetes cluster not connected")
	}

	ns := NATSNamespace
	// 1. Create namespace
	_, err := cm.Clientset.CoreV1().Namespaces().Get(ctx, ns, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		nsObj := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: ns,
				Labels: map[string]string{
					"name":                      ns,
					"app.kubernetes.io/name":    "nats",
					"app.kubernetes.io/part-of": "navispaas",
					"navispaas.io/managed-by":   "navispaas",
					"navispaas.io/tier":         "messaging",
				},
				Annotations: map[string]string{
					"navispaas.io/description": "NATS JetStream broker managed by NavisPaaS",
				},
			},
		}
		_, err = cm.Clientset.CoreV1().Namespaces().Create(ctx, nsObj, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create nats namespace: %w", err)
		}
	}

	// 2. NATS Deployment
	natsReplicas := int32(1)
	natsDep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "nats",
			Namespace: ns,
			Labels:    map[string]string{"app": "nats", "navispaas.io/managed-by": "navispaas"},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &natsReplicas,
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "nats"}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "nats"},
					Annotations: map[string]string{
						"navispaas.io/managed-by": "navispaas",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "nats",
							Image: "nats:2.10-alpine",
							Args:  []string{"-js", "-m", "8222"}, // -js enables JetStream, -m 8222 enables HTTP monitoring
							Ports: []corev1.ContainerPort{
								{Name: "client", ContainerPort: 4222},
								{Name: "monitor", ContainerPort: 8222},
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									TCPSocket: &corev1.TCPSocketAction{
										Port: intstr.FromInt(4222),
									},
								},
								InitialDelaySeconds: 5,
								PeriodSeconds:       5,
							},
						},
					},
				},
			},
		},
	}
	_, err = cm.Clientset.AppsV1().Deployments(ns).Get(ctx, "nats", metav1.GetOptions{})
	if errors.IsNotFound(err) {
		_, _ = cm.Clientset.AppsV1().Deployments(ns).Create(ctx, natsDep, metav1.CreateOptions{})
	}

	// 3. NATS Service (NodePort)
	natsSvc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "nats-service",
			Namespace: ns,
			Labels:    map[string]string{"app": "nats"},
		},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeNodePort,
			Selector: map[string]string{"app": "nats"},
			Ports: []corev1.ServicePort{
				{Name: "client", Port: 4222, TargetPort: intstr.FromInt(4222), NodePort: 30222},
				{Name: "monitor", Port: 8222, TargetPort: intstr.FromInt(8222), NodePort: 30223},
			},
		},
	}
	_, err = cm.Clientset.CoreV1().Services(ns).Get(ctx, "nats-service", metav1.GetOptions{})
	if errors.IsNotFound(err) {
		_, _ = cm.Clientset.CoreV1().Services(ns).Create(ctx, natsSvc, metav1.CreateOptions{})
	}

	return nil
}

func (n *NATSOffer) Bind(ctx context.Context, cm *k8s.ClientManager, app *models.Application, params map[string]string) (map[string]string, error) {
	if !cm.Connected {
		return nil, fmt.Errorf("kubernetes cluster not connected")
	}

	dep, err := cm.Clientset.AppsV1().Deployments(app.Namespace).Get(ctx, app.Name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("application deployment not found: %w", err)
	}

	// Inject environment variables
	injectedEnvs := map[string]string{
		"NATS_HOST": "nats-service.nats.svc.cluster.local",
		"NATS_PORT": "4222",
		"NATS_URL":  "nats://nats-service.nats.svc.cluster.local:4222",
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
		OfferID:   "nats",
		OfferName: "NATS JetStream Broker",
		BoundAt:   time.Now(),
		Config:    injectedEnvs,
	}

	updated := false
	for i, o := range linkedOffers {
		if o.OfferID == "nats" {
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
		return nil, fmt.Errorf("failed to bind nats offer: %w", err)
	}

	return injectedEnvs, nil
}

func (n *NATSOffer) Unbind(ctx context.Context, cm *k8s.ClientManager, app *models.Application) error {
	if !cm.Connected {
		return fmt.Errorf("kubernetes cluster not connected")
	}

	dep, err := cm.Clientset.AppsV1().Deployments(app.Namespace).Get(ctx, app.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	// Remove env vars
	natsKeys := map[string]bool{
		"NATS_HOST": true,
		"NATS_PORT": true,
		"NATS_URL":  true,
	}

	if len(dep.Spec.Template.Spec.Containers) > 0 {
		var filteredEnv []corev1.EnvVar
		for _, env := range dep.Spec.Template.Spec.Containers[0].Env {
			if !natsKeys[env.Name] {
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
			if o.OfferID != "nats" {
				filteredOffers = append(filteredOffers, o)
			}
		}
		bytes, _ := json.Marshal(filteredOffers)
		dep.Annotations[k8s.AnnotationOffers] = string(bytes)
	}

	_, err = cm.Clientset.AppsV1().Deployments(app.Namespace).Update(ctx, dep, metav1.UpdateOptions{})
	return err
}
