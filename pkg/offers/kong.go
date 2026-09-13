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
	KongNamespace = "kong"
)

type KongGatewayOffer struct{}

func (k *KongGatewayOffer) GetDefinition() models.OfferDefinition {
	return models.OfferDefinition{
		ID:          "kong",
		Name:        "Kong API Gateway",
		Category:    "Networking",
		Description: "Kong Gateway - The most popular open-source API Gateway with rate limiting, authentication, and plugin ecosystem.",
		Version:     "3.9",
		Icon:        "kong",
		Parameters: []models.OfferParameter{
			{
				Key:         "enableRateLimiting",
				Label:       "Enable Rate Limiting",
				Type:        "string",
				Default:     "true",
				Description: "Enable rate limiting plugin (100 req/min by default)",
				Required:    false,
			},
			{
				Key:         "enableAuth",
				Label:       "Enable Authentication",
				Type:        "string",
				Default:     "false",
				Description: "Enable key-auth plugin for API key authentication",
				Required:    false,
			},
		},
	}
}

func (k *KongGatewayOffer) IsInstalled(ctx context.Context, cm *k8s.ClientManager) (bool, error) {
	if !cm.Connected {
		return false, nil
	}
	_, err := cm.Clientset.AppsV1().Deployments(KongNamespace).Get(ctx, "kong-gateway", metav1.GetOptions{})
	if err == nil {
		return true, nil
	}
	return false, nil
}

func (k *KongGatewayOffer) Install(ctx context.Context, cm *k8s.ClientManager) error {
	if !cm.Connected {
		return fmt.Errorf("kubernetes cluster not connected")
	}

	ns := KongNamespace
	// 1. Create namespace
	_, err := cm.Clientset.CoreV1().Namespaces().Get(ctx, ns, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		nsObj := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: ns,
				Labels: map[string]string{
					"name":                      ns,
					"app.kubernetes.io/name":    "kong",
					"app.kubernetes.io/part-of": "navispaas",
					"navispaas.io/managed-by":   "navispaas",
					"navispaas.io/tier":         "networking",
				},
				Annotations: map[string]string{
					"navispaas.io/description": "Kong API Gateway managed by NavisPaaS",
				},
			},
		}
		_, err = cm.Clientset.CoreV1().Namespaces().Create(ctx, nsObj, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create kong namespace: %w", err)
		}
	}

	// 2. Kong ConfigMap
	kongConfig := `
prefix: /usr/local/kong
proxy_access_log: /dev/stdout
proxy_error_log: /dev/stderr
admin_access_log: /dev/stdout
admin_error_log: /dev/stderr
nginx_worker_processes: auto
proxy_listen: 0.0.0.0:8000, 0.0.0.0:8443 ssl
admin_listen: 0.0.0.0:8001
database: off
declarative_config: /etc/kong/kong.yml
`
	kongCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kong-config",
			Namespace: ns,
		},
		Data: map[string]string{
			"kong.conf": kongConfig,
		},
	}
	_, err = cm.Clientset.CoreV1().ConfigMaps(ns).Get(ctx, "kong-config", metav1.GetOptions{})
	if errors.IsNotFound(err) {
		_, _ = cm.Clientset.CoreV1().ConfigMaps(ns).Create(ctx, kongCM, metav1.CreateOptions{})
	}

	// 3. Kong Declarative Config (kong.yml)
	kongDeclarative := `
_format_version: "3.0"
_transform: true

services:
- name: kong-placeholder
  url: http://localhost:8001
  routes:
  - name: placeholder
    paths:
    - /placeholder
    plugins:
    - name: rate-limiting
      config:
        minute: 100
        policy: local

plugins:
- name: cors
  config:
    origins:
    - "*"
    methods:
    - GET
    - POST
    - PUT
    - DELETE
    - PATCH
    - OPTIONS
    headers:
    - Authorization
    - Content-Type
    - Accept
    exposed_headers:
    - X-Kong-Upstream-Latency
    - X-Kong-Proxy-Latency
    max_age: 3600
    credentials: true
`
	kongDeclCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kong-declarative",
			Namespace: ns,
		},
		Data: map[string]string{
			"kong.yml": kongDeclarative,
		},
	}
	_, err = cm.Clientset.CoreV1().ConfigMaps(ns).Get(ctx, "kong-declarative", metav1.GetOptions{})
	if errors.IsNotFound(err) {
		_, _ = cm.Clientset.CoreV1().ConfigMaps(ns).Create(ctx, kongDeclCM, metav1.CreateOptions{})
	}

	// 4. Kong Deployment
	kongReplicas := int32(1)
	kongDep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kong-gateway",
			Namespace: ns,
			Labels:    map[string]string{"app": "kong-gateway", "navispaas.io/managed-by": "navispaas"},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &kongReplicas,
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "kong-gateway"}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "kong-gateway"},
					Annotations: map[string]string{
						"navispaas.io/managed-by": "navispaas",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "proxy",
							Image: "kong/kong-gateway:3.9",
							Env: []corev1.EnvVar{
								{Name: "KONG_DATABASE", Value: "off"},
								{Name: "KONG_DECLARATIVE_CONFIG", Value: "/etc/kong/kong.yml"},
								{Name: "KONG_PROXY_LISTEN", Value: "0.0.0.0:8000, 0.0.0.0:8443 ssl"},
								{Name: "KONG_ADMIN_LISTEN", Value: "0.0.0.0:8001"},
								{Name: "KONG_LOG_LEVEL", Value: "info"},
							},
							Ports: []corev1.ContainerPort{
								{Name: "proxy", ContainerPort: 8000},
								{Name: "proxy-ssl", ContainerPort: 8443},
								{Name: "admin", ContainerPort: 8001},
							},
							VolumeMounts: []corev1.VolumeMount{
								{Name: "kong-config", MountPath: "/etc/kong/kong.conf", SubPath: "kong.conf"},
								{Name: "kong-declarative", MountPath: "/etc/kong/kong.yml", SubPath: "kong.yml"},
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/status",
										Port: intstr.FromInt(8001),
									},
								},
								InitialDelaySeconds: 10,
								PeriodSeconds:       5,
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "kong-config",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{Name: "kong-config"},
								},
							},
						},
						{
							Name: "kong-declarative",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{Name: "kong-declarative"},
								},
							},
						},
					},
				},
			},
		},
	}
	_, err = cm.Clientset.AppsV1().Deployments(ns).Get(ctx, "kong-gateway", metav1.GetOptions{})
	if errors.IsNotFound(err) {
		_, _ = cm.Clientset.AppsV1().Deployments(ns).Create(ctx, kongDep, metav1.CreateOptions{})
	}

	// 5. Kong Proxy Service (NodePort)
	kongProxySvc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kong-proxy",
			Namespace: ns,
			Labels:    map[string]string{"app": "kong-gateway"},
		},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeNodePort,
			Selector: map[string]string{"app": "kong-gateway"},
			Ports: []corev1.ServicePort{
				{Name: "http", Port: 80, TargetPort: intstr.FromInt(8000), NodePort: 30080},
				{Name: "https", Port: 443, TargetPort: intstr.FromInt(8443), NodePort: 30443},
			},
		},
	}
	_, err = cm.Clientset.CoreV1().Services(ns).Get(ctx, "kong-proxy", metav1.GetOptions{})
	if errors.IsNotFound(err) {
		_, _ = cm.Clientset.CoreV1().Services(ns).Create(ctx, kongProxySvc, metav1.CreateOptions{})
	}

	// 6. Kong Admin Service (NodePort)
	kongAdminSvc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kong-admin",
			Namespace: ns,
			Labels:    map[string]string{"app": "kong-gateway"},
		},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeNodePort,
			Selector: map[string]string{"app": "kong-gateway"},
			Ports: []corev1.ServicePort{
				{Name: "admin", Port: 8001, TargetPort: intstr.FromInt(8001), NodePort: 30001},
			},
		},
	}
	_, err = cm.Clientset.CoreV1().Services(ns).Get(ctx, "kong-admin", metav1.GetOptions{})
	if errors.IsNotFound(err) {
		_, _ = cm.Clientset.CoreV1().Services(ns).Create(ctx, kongAdminSvc, metav1.CreateOptions{})
	}

	return nil
}

func (k *KongGatewayOffer) Bind(ctx context.Context, cm *k8s.ClientManager, app *models.Application, params map[string]string) (map[string]string, error) {
	if !cm.Connected {
		return nil, fmt.Errorf("kubernetes cluster not connected")
	}

	dep, err := cm.Clientset.AppsV1().Deployments(app.Namespace).Get(ctx, app.Name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("application deployment not found: %w", err)
	}

	// Get custom parameters with defaults
	appPort := app.Port
	if appPort == 0 {
		appPort = 8080
	}

	rateLimit := params["rateLimit"]
	if rateLimit == "" {
		rateLimit = "100"
	}

	gatewayPath := params["gatewayPath"]
	if gatewayPath == "" {
		gatewayPath = "/" + app.Name
	}

	// Inject environment variables
	injectedEnvs := map[string]string{
		"KONG_PROXY_URL":     "http://kong-proxy.kong.svc.cluster.local:80",
		"KONG_ADMIN_URL":     "http://kong-admin.kong.svc.cluster.local:8001",
		"KONG_SERVICE_NAME":  app.Name,
		"KONG_UPSTREAM_URL":  fmt.Sprintf("http://%s.%s.svc.cluster.local:%d", app.Name, app.Namespace, appPort),
		"KONG_GATEWAY_PATH":  gatewayPath,
		"KONG_RATE_LIMIT":    rateLimit,
		"KONG_PUBLIC_URL":    fmt.Sprintf("http://192.168.49.2:30080%s", gatewayPath),
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
		OfferID:   "kong",
		OfferName: "Kong API Gateway",
		BoundAt:   time.Now(),
		Config:    injectedEnvs,
	}

	updated := false
	for i, o := range linkedOffers {
		if o.OfferID == "kong" {
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
		return nil, fmt.Errorf("failed to bind kong offer: %w", err)
	}

	return injectedEnvs, nil
}

func (k *KongGatewayOffer) Unbind(ctx context.Context, cm *k8s.ClientManager, app *models.Application) error {
	if !cm.Connected {
		return fmt.Errorf("kubernetes cluster not connected")
	}

	dep, err := cm.Clientset.AppsV1().Deployments(app.Namespace).Get(ctx, app.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	// Remove env vars
	kongKeys := map[string]bool{
		"KONG_PROXY_URL":    true,
		"KONG_ADMIN_URL":    true,
		"KONG_SERVICE_NAME": true,
		"KONG_UPSTREAM_URL": true,
		"KONG_GATEWAY_PATH": true,
		"KONG_RATE_LIMIT":   true,
		"KONG_PUBLIC_URL":   true,
	}

	if len(dep.Spec.Template.Spec.Containers) > 0 {
		var filteredEnv []corev1.EnvVar
		for _, env := range dep.Spec.Template.Spec.Containers[0].Env {
			if !kongKeys[env.Name] {
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
			if o.OfferID != "kong" {
				filteredOffers = append(filteredOffers, o)
			}
		}
		bytes, _ := json.Marshal(filteredOffers)
		dep.Annotations[k8s.AnnotationOffers] = string(bytes)
	}

	_, err = cm.Clientset.AppsV1().Deployments(app.Namespace).Update(ctx, dep, metav1.UpdateOptions{})
	return err
}