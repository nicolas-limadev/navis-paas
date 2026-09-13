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
	RabbitMQNamespace = "rabbitmq"
)

type RabbitMQOffer struct{}

func (r *RabbitMQOffer) GetDefinition() models.OfferDefinition {
	return models.OfferDefinition{
		ID:          "rabbitmq",
		Name:        "RabbitMQ Message Broker",
		Category:    "Messaging",
		Description: "RabbitMQ - Flexible message broker with AMQP, MQTT, and STOMP protocol support. Perfect for task queues and async communication.",
		Version:     "4.0",
		Icon:        "rabbitmq",
		Parameters: []models.OfferParameter{
			{
				Key:         "defaultUser",
				Label:       "Default User",
				Type:        "string",
				Default:     "guest",
				Description: "RabbitMQ default admin username",
				Required:    false,
			},
			{
				Key:         "defaultVhost",
				Label:       "Default Virtual Host",
				Type:        "string",
				Default:     "/",
				Description: "Default virtual host for message routing",
				Required:    false,
			},
		},
	}
}

func (r *RabbitMQOffer) IsInstalled(ctx context.Context, cm *k8s.ClientManager) (bool, error) {
	if !cm.Connected {
		return false, nil
	}
	_, err := cm.Clientset.AppsV1().Deployments(RabbitMQNamespace).Get(ctx, "rabbitmq", metav1.GetOptions{})
	if err == nil {
		return true, nil
	}
	return false, nil
}

func (r *RabbitMQOffer) Install(ctx context.Context, cm *k8s.ClientManager) error {
	if !cm.Connected {
		return fmt.Errorf("kubernetes cluster not connected")
	}

	ns := RabbitMQNamespace
	// 1. Create namespace
	_, err := cm.Clientset.CoreV1().Namespaces().Get(ctx, ns, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		nsObj := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: ns,
				Labels: map[string]string{
					"name":                      ns,
					"app.kubernetes.io/name":    "rabbitmq",
					"app.kubernetes.io/part-of": "navispaas",
					"navispaas.io/managed-by":   "navispaas",
					"navispaas.io/tier":         "messaging",
				},
				Annotations: map[string]string{
					"navispaas.io/description": "RabbitMQ message broker managed by NavisPaaS",
				},
			},
		}
		_, err = cm.Clientset.CoreV1().Namespaces().Create(ctx, nsObj, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create rabbitmq namespace: %w", err)
		}
	}

	// 2. RabbitMQ ConfigMap
	rabbitConfig := `
loopback_users.guest = false
listeners.tcp.default = 5672
management.tcp.port = 15672
`
	rabbitCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "rabbitmq-config",
			Namespace: ns,
		},
		Data: map[string]string{
			"rabbitmq.conf": rabbitConfig,
		},
	}
	_, err = cm.Clientset.CoreV1().ConfigMaps(ns).Get(ctx, "rabbitmq-config", metav1.GetOptions{})
	if errors.IsNotFound(err) {
		_, _ = cm.Clientset.CoreV1().ConfigMaps(ns).Create(ctx, rabbitCM, metav1.CreateOptions{})
	}

	// 3. RabbitMQ Deployment
	rabbitReplicas := int32(1)
	rabbitDep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "rabbitmq",
			Namespace: ns,
			Labels:    map[string]string{"app": "rabbitmq", "navispaas.io/managed-by": "navispaas"},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &rabbitReplicas,
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "rabbitmq"}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "rabbitmq"},
					Annotations: map[string]string{
						"navispaas.io/managed-by": "navispaas",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "rabbitmq",
							Image: "rabbitmq:4.0-management",
							Env: []corev1.EnvVar{
								{Name: "RABBITMQ_DEFAULT_USER", Value: "guest"},
								{Name: "RABBITMQ_DEFAULT_PASS", Value: "guest"},
								{Name: "RABBITMQ_DEFAULT_VHOST", Value: "/"},
							},
							Ports: []corev1.ContainerPort{
								{Name: "amqp", ContainerPort: 5672},
								{Name: "management", ContainerPort: 15672},
							},
							VolumeMounts: []corev1.VolumeMount{
								{Name: "rabbitmq-config", MountPath: "/etc/rabbitmq/rabbitmq.conf", SubPath: "rabbitmq.conf"},
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									TCPSocket: &corev1.TCPSocketAction{
										Port: intstr.FromInt(5672),
									},
								},
								InitialDelaySeconds: 10,
								PeriodSeconds:       5,
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "rabbitmq-config",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{Name: "rabbitmq-config"},
								},
							},
						},
					},
				},
			},
		},
	}
	_, err = cm.Clientset.AppsV1().Deployments(ns).Get(ctx, "rabbitmq", metav1.GetOptions{})
	if errors.IsNotFound(err) {
		_, _ = cm.Clientset.AppsV1().Deployments(ns).Create(ctx, rabbitDep, metav1.CreateOptions{})
	}

	// 4. RabbitMQ AMQP Service (NodePort)
	rabbitAMQPSvc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "rabbitmq-amqp",
			Namespace: ns,
			Labels:    map[string]string{"app": "rabbitmq"},
		},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeNodePort,
			Selector: map[string]string{"app": "rabbitmq"},
			Ports: []corev1.ServicePort{
				{Name: "amqp", Port: 5672, TargetPort: intstr.FromInt(5672), NodePort: 30672},
			},
		},
	}
	_, err = cm.Clientset.CoreV1().Services(ns).Get(ctx, "rabbitmq-amqp", metav1.GetOptions{})
	if errors.IsNotFound(err) {
		_, _ = cm.Clientset.CoreV1().Services(ns).Create(ctx, rabbitAMQPSvc, metav1.CreateOptions{})
	}

	// 5. RabbitMQ Management Service (NodePort)
	rabbitMgmtSvc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "rabbitmq-management",
			Namespace: ns,
			Labels:    map[string]string{"app": "rabbitmq"},
		},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeNodePort,
			Selector: map[string]string{"app": "rabbitmq"},
			Ports: []corev1.ServicePort{
				{Name: "management", Port: 15672, TargetPort: intstr.FromInt(15672), NodePort: 30673},
			},
		},
	}
	_, err = cm.Clientset.CoreV1().Services(ns).Get(ctx, "rabbitmq-management", metav1.GetOptions{})
	if errors.IsNotFound(err) {
		_, _ = cm.Clientset.CoreV1().Services(ns).Create(ctx, rabbitMgmtSvc, metav1.CreateOptions{})
	}

	return nil
}

func (r *RabbitMQOffer) Bind(ctx context.Context, cm *k8s.ClientManager, app *models.Application, params map[string]string) (map[string]string, error) {
	if !cm.Connected {
		return nil, fmt.Errorf("kubernetes cluster not connected")
	}

	dep, err := cm.Clientset.AppsV1().Deployments(app.Namespace).Get(ctx, app.Name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("application deployment not found: %w", err)
	}

	// Get custom parameters with defaults
	user := params["username"]
	if user == "" {
		user = "guest"
	}
	password := params["password"]
	if password == "" {
		password = "guest"
	}
	vhost := params["vhost"]
	if vhost == "" {
		vhost = "/"
	}
	port := params["port"]
	if port == "" {
		port = "5672"
	}

	// Inject environment variables
	injectedEnvs := map[string]string{
		"RABBITMQ_HOST":     "rabbitmq-amqp.rabbitmq.svc.cluster.local",
		"RABBITMQ_PORT":     port,
		"RABBITMQ_USER":     user,
		"RABBITMQ_PASSWORD": password,
		"RABBITMQ_VHOST":    vhost,
		"RABBITMQ_URL":      fmt.Sprintf("amqp://%s:%s@rabbitmq-amqp.rabbitmq.svc.cluster.local:%s/%s", user, password, port, vhost),
		"RABBITMQ_MGMT_URL": "http://rabbitmq-management.rabbitmq.svc.cluster.local:15672",
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
		OfferID:   "rabbitmq",
		OfferName: "RabbitMQ Message Broker",
		BoundAt:   time.Now(),
		Config:    injectedEnvs,
	}

	updated := false
	for i, o := range linkedOffers {
		if o.OfferID == "rabbitmq" {
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
		return nil, fmt.Errorf("failed to bind rabbitmq offer: %w", err)
	}

	return injectedEnvs, nil
}

func (r *RabbitMQOffer) Unbind(ctx context.Context, cm *k8s.ClientManager, app *models.Application) error {
	if !cm.Connected {
		return fmt.Errorf("kubernetes cluster not connected")
	}

	dep, err := cm.Clientset.AppsV1().Deployments(app.Namespace).Get(ctx, app.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	// Remove env vars
	rabbitKeys := map[string]bool{
		"RABBITMQ_HOST":     true,
		"RABBITMQ_PORT":     true,
		"RABBITMQ_USER":     true,
		"RABBITMQ_PASSWORD": true,
		"RABBITMQ_VHOST":    true,
		"RABBITMQ_URL":      true,
		"RABBITMQ_MGMT_URL": true,
	}

	if len(dep.Spec.Template.Spec.Containers) > 0 {
		var filteredEnv []corev1.EnvVar
		for _, env := range dep.Spec.Template.Spec.Containers[0].Env {
			if !rabbitKeys[env.Name] {
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
			if o.OfferID != "rabbitmq" {
				filteredOffers = append(filteredOffers, o)
			}
		}
		bytes, _ := json.Marshal(filteredOffers)
		dep.Annotations[k8s.AnnotationOffers] = string(bytes)
	}

	_, err = cm.Clientset.AppsV1().Deployments(app.Namespace).Update(ctx, dep, metav1.UpdateOptions{})
	return err
}