package offers

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
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
	MonitoringNamespace = "monitoring"
)

type MonitoringOffer struct{}

func (m *MonitoringOffer) GetDefinition() models.OfferDefinition {
	return models.OfferDefinition{
		ID:          "monitoring",
		Name:        "Observability Stack",
		Category:    "Observability",
		Description: "Prometheus, Grafana and OpenTelemetry for full-stack observability with auto-discovery.",
		Version:     "2.54.0",
		Icon:        "monitoring",
		Parameters: []models.OfferParameter{
			{
				Key:         "scrapeInterval",
				Label:       "Scrape Interval",
				Type:        "string",
				Default:     "15s",
				Description: "How frequently Prometheus should poll your application",
				Required:    false,
			},
			{
				Key:         "installPrometheus",
				Label:       "Install Prometheus",
				Type:        "string",
				Default:     "true",
				Description: "Install Prometheus metrics server",
				Required:    false,
			},
			{
				Key:         "installGrafana",
				Label:       "Install Grafana",
				Type:        "string",
				Default:     "true",
				Description: "Install Grafana dashboards",
				Required:    false,
			},
			{
				Key:         "installOTEL",
				Label:       "Install OpenTelemetry Collector",
				Type:        "string",
				Default:     "false",
				Description: "Install OpenTelemetry Collector for distributed tracing",
				Required:    false,
			},
		},
	}
}

func (m *MonitoringOffer) IsInstalled(ctx context.Context, cm *k8s.ClientManager) (bool, error) {
	if !cm.Connected {
		return false, nil
	}

	// Check if ANY component is installed
	prometheusExists, _ := m.isComponentInstalled(ctx, cm, "prometheus")
	grafanaExists, _ := m.isComponentInstalled(ctx, cm, "grafana")
	otelExists, _ := m.isComponentInstalled(ctx, cm, "otel-collector")

	return prometheusExists || grafanaExists || otelExists, nil
}

func (m *MonitoringOffer) isComponentInstalled(ctx context.Context, cm *k8s.ClientManager, component string) (bool, error) {
	_, err := cm.Clientset.AppsV1().Deployments(MonitoringNamespace).Get(ctx, component, metav1.GetOptions{})
	if err == nil {
		return true, nil
	}
	return false, nil
}

func (m *MonitoringOffer) GetComponentStatus(ctx context.Context, cm *k8s.ClientManager) map[string]bool {
	status := map[string]bool{
		"prometheus": false,
		"grafana":    false,
		"otel":       false,
	}
	if !cm.Connected {
		return status
	}
	prometheusExists, _ := m.isComponentInstalled(ctx, cm, "prometheus")
	grafanaExists, _ := m.isComponentInstalled(ctx, cm, "grafana")
	otelExists, _ := m.isComponentInstalled(ctx, cm, "otel-collector")

	status["prometheus"] = prometheusExists
	status["grafana"] = grafanaExists
	status["otel"] = otelExists
	return status
}

func (m *MonitoringOffer) Install(ctx context.Context, cm *k8s.ClientManager) error {
	return m.InstallWithComponents(ctx, cm, true, true, false)
}

func (m *MonitoringOffer) InstallWithComponents(ctx context.Context, cm *k8s.ClientManager, installPrometheus, installGrafana, installOTEL bool) error {
	if !cm.Connected {
		return fmt.Errorf("kubernetes cluster not connected")
	}

	ns := MonitoringNamespace
	// 1. Create namespace
	_, err := cm.Clientset.CoreV1().Namespaces().Get(ctx, ns, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		nsObj := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: ns,
				Labels: map[string]string{
					"name":                      ns,
					"app.kubernetes.io/name":    "monitoring",
					"app.kubernetes.io/part-of": "navispaas",
					"navispaas.io/managed-by":   "navispaas",
					"navispaas.io/tier":         "observability",
				},
				Annotations: map[string]string{
					"navispaas.io/description": "Observability stack managed by NavisPaaS",
				},
			},
		}
		_, err = cm.Clientset.CoreV1().Namespaces().Create(ctx, nsObj, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create monitoring namespace: %w", err)
		}
	}

	if installPrometheus {
		if err := m.installPrometheus(ctx, cm); err != nil {
			return fmt.Errorf("failed to install prometheus: %w", err)
		}
	}

	if installGrafana {
		if err := m.installGrafana(ctx, cm); err != nil {
			return fmt.Errorf("failed to install grafana: %w", err)
		}
	}

	if installOTEL {
		if err := m.installOTEL(ctx, cm); err != nil {
			return fmt.Errorf("failed to install otel: %w", err)
		}
	}

	return nil
}

func (m *MonitoringOffer) installPrometheus(ctx context.Context, cm *k8s.ClientManager) error {
	ns := MonitoringNamespace

	promConfig := `
global:
  scrape_interval: 15s
  evaluation_interval: 15s

scrape_configs:
  - job_name: 'navis-applications'
    kubernetes_sd_configs:
      - role: pod
    relabel_configs:
      - source_labels: [__meta_kubernetes_pod_annotation_prometheus_io_scrape]
        action: keep
        regex: true
      - source_labels: [__meta_kubernetes_pod_annotation_prometheus_io_path]
        action: replace
        target_label: __metrics_path__
        regex: (.+)
      - source_labels: [__address__, __meta_kubernetes_pod_annotation_prometheus_io_port]
        action: replace
        regex: ([^:]+)(?::\d+)?;(\d+)
        replacement: $1:$2
        target_label: __address__
      - action: labelmap
        regex: __meta_kubernetes_pod_label_(.+)
      - source_labels: [__meta_kubernetes_namespace]
        action: replace
        target_label: kubernetes_namespace
      - source_labels: [__meta_kubernetes_pod_name]
        action: replace
        target_label: kubernetes_pod_name
`
	promCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "prometheus-config",
			Namespace: ns,
		},
		Data: map[string]string{
			"prometheus.yml": promConfig,
		},
	}
	_, err := cm.Clientset.CoreV1().ConfigMaps(ns).Get(ctx, "prometheus-config", metav1.GetOptions{})
	if errors.IsNotFound(err) {
		_, _ = cm.Clientset.CoreV1().ConfigMaps(ns).Create(ctx, promCM, metav1.CreateOptions{})
	}

	promReplicas := int32(1)
	promDep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "prometheus",
			Namespace: ns,
			Labels:    map[string]string{"app": "prometheus", "navispaas.io/managed-by": "navispaas"},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &promReplicas,
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "prometheus"}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "prometheus"}},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "prometheus",
							Image: "prom/prometheus:v2.54.0",
							Args:  []string{"--config.file=/etc/prometheus/prometheus.yml", "--storage.tsdb.retention.time=24h"},
							Ports: []corev1.ContainerPort{{Name: "web", ContainerPort: 9090}},
							VolumeMounts: []corev1.VolumeMount{
								{Name: "config-volume", MountPath: "/etc/prometheus"},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "config-volume",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{Name: "prometheus-config"},
								},
							},
						},
					},
				},
			},
		},
	}
	_, err = cm.Clientset.AppsV1().Deployments(ns).Get(ctx, "prometheus", metav1.GetOptions{})
	if errors.IsNotFound(err) {
		_, _ = cm.Clientset.AppsV1().Deployments(ns).Create(ctx, promDep, metav1.CreateOptions{})
	}

	promSvc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "prometheus-service",
			Namespace: ns,
			Labels:    map[string]string{"app": "prometheus"},
		},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeNodePort,
			Selector: map[string]string{"app": "prometheus"},
			Ports: []corev1.ServicePort{
				{Name: "web", Port: 9090, TargetPort: intstr.FromInt(9090), NodePort: 30090},
			},
		},
	}
	_, err = cm.Clientset.CoreV1().Services(ns).Get(ctx, "prometheus-service", metav1.GetOptions{})
	if errors.IsNotFound(err) {
		_, _ = cm.Clientset.CoreV1().Services(ns).Create(ctx, promSvc, metav1.CreateOptions{})
	}

	return nil
}

func (m *MonitoringOffer) installGrafana(ctx context.Context, cm *k8s.ClientManager) error {
	ns := MonitoringNamespace

	grafanaDatasource := `
apiVersion: 1
datasources:
- name: Prometheus
  type: prometheus
  access: proxy
  url: http://prometheus-service.monitoring.svc.cluster.local:9090
  isDefault: true
`
	grafanaCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "grafana-datasources",
			Namespace: ns,
		},
		Data: map[string]string{
			"datasources.yaml": grafanaDatasource,
		},
	}
	_, err := cm.Clientset.CoreV1().ConfigMaps(ns).Get(ctx, "grafana-datasources", metav1.GetOptions{})
	if errors.IsNotFound(err) {
		_, _ = cm.Clientset.CoreV1().ConfigMaps(ns).Create(ctx, grafanaCM, metav1.CreateOptions{})
	}

	grafanaReplicas := int32(1)
	grafanaDep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "grafana",
			Namespace: ns,
			Labels:    map[string]string{"app": "grafana", "navispaas.io/managed-by": "navispaas"},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &grafanaReplicas,
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "grafana"}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "grafana"}},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "grafana",
							Image: "grafana/grafana:11.2.0",
							Env: []corev1.EnvVar{
								{Name: "GF_SECURITY_ADMIN_USER", Value: "admin"},
								{Name: "GF_SECURITY_ADMIN_PASSWORD", Value: "navispaas"},
								{Name: "GF_USERS_ALLOW_SIGN_UP", Value: "false"},
								{Name: "GF_AUTH_ANONYMOUS_ENABLED", Value: "true"},
								{Name: "GF_AUTH_ANONYMOUS_ORG_ROLE", Value: "Viewer"},
							},
							Ports: []corev1.ContainerPort{{Name: "web", ContainerPort: 3000}},
							VolumeMounts: []corev1.VolumeMount{
								{Name: "datasources", MountPath: "/etc/grafana/provisioning/datasources"},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "datasources",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{Name: "grafana-datasources"},
								},
							},
						},
					},
				},
			},
		},
	}
	_, err = cm.Clientset.AppsV1().Deployments(ns).Get(ctx, "grafana", metav1.GetOptions{})
	if errors.IsNotFound(err) {
		_, _ = cm.Clientset.AppsV1().Deployments(ns).Create(ctx, grafanaDep, metav1.CreateOptions{})
	}

	grafanaSvc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "grafana-service",
			Namespace: ns,
			Labels:    map[string]string{"app": "grafana"},
		},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeNodePort,
			Selector: map[string]string{"app": "grafana"},
			Ports: []corev1.ServicePort{
				{Name: "web", Port: 3000, TargetPort: intstr.FromInt(3000), NodePort: 30080},
			},
		},
	}
	_, err = cm.Clientset.CoreV1().Services(ns).Get(ctx, "grafana-service", metav1.GetOptions{})
	if errors.IsNotFound(err) {
		_, _ = cm.Clientset.CoreV1().Services(ns).Create(ctx, grafanaSvc, metav1.CreateOptions{})
	}

	return nil
}

func (m *MonitoringOffer) installOTEL(ctx context.Context, cm *k8s.ClientManager) error {
	ns := MonitoringNamespace

	otelConfig := `
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317
      http:
        endpoint: 0.0.0.0:4318
  prometheus:
    config:
      scrape_configs:
        - job_name: 'otel-collector'
          static_configs:
            - targets: ['localhost:8888']

processors:
  batch:

exporters:
  prometheus:
    endpoint: "0.0.0.0:8889"
  logging:
    loglevel: debug

service:
  pipelines:
    metrics:
      receivers: [otlp, prometheus]
      processors: [batch]
      exporters: [prometheus]
    traces:
      receivers: [otlp]
      processors: [batch]
      exporters: [logging]
`
	otelCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "otel-collector-config",
			Namespace: ns,
		},
		Data: map[string]string{
			"config.yaml": otelConfig,
		},
	}
	_, err := cm.Clientset.CoreV1().ConfigMaps(ns).Get(ctx, "otel-collector-config", metav1.GetOptions{})
	if errors.IsNotFound(err) {
		_, _ = cm.Clientset.CoreV1().ConfigMaps(ns).Create(ctx, otelCM, metav1.CreateOptions{})
	}

	otelReplicas := int32(1)
	otelDep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "otel-collector",
			Namespace: ns,
			Labels:    map[string]string{"app": "otel-collector", "navispaas.io/managed-by": "navispaas"},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &otelReplicas,
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "otel-collector"}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "otel-collector"}},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "otel-collector",
							Image: "otel/opentelemetry-collector-contrib:0.108.0",
							Args:  []string{"--config=/etc/otel/config.yaml"},
							Ports: []corev1.ContainerPort{
								{Name: "otlp-grpc", ContainerPort: 4317},
								{Name: "otlp-http", ContainerPort: 4318},
								{Name: "metrics", ContainerPort: 8889},
							},
							VolumeMounts: []corev1.VolumeMount{
								{Name: "config", MountPath: "/etc/otel"},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "config",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{Name: "otel-collector-config"},
								},
							},
						},
					},
				},
			},
		},
	}
	_, err = cm.Clientset.AppsV1().Deployments(ns).Get(ctx, "otel-collector", metav1.GetOptions{})
	if errors.IsNotFound(err) {
		_, _ = cm.Clientset.AppsV1().Deployments(ns).Create(ctx, otelDep, metav1.CreateOptions{})
	}

	otelSvc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "otel-collector-service",
			Namespace: ns,
			Labels:    map[string]string{"app": "otel-collector"},
		},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeNodePort,
			Selector: map[string]string{"app": "otel-collector"},
			Ports: []corev1.ServicePort{
				{Name: "otlp-grpc", Port: 4317, TargetPort: intstr.FromInt(4317), NodePort: 30317},
				{Name: "otlp-http", Port: 4318, TargetPort: intstr.FromInt(4318), NodePort: 30318},
				{Name: "metrics", Port: 8889, TargetPort: intstr.FromInt(8889), NodePort: 30889},
			},
		},
	}
	_, err = cm.Clientset.CoreV1().Services(ns).Get(ctx, "otel-collector-service", metav1.GetOptions{})
	if errors.IsNotFound(err) {
		_, _ = cm.Clientset.CoreV1().Services(ns).Create(ctx, otelSvc, metav1.CreateOptions{})
	}

	return nil
}

func (m *MonitoringOffer) UninstallComponent(ctx context.Context, cm *k8s.ClientManager, component string) error {
	if !cm.Connected {
		return fmt.Errorf("kubernetes cluster not connected")
	}

	ns := MonitoringNamespace
	_ = cm.Clientset.AppsV1().Deployments(ns).Delete(ctx, component, metav1.DeleteOptions{})
	_ = cm.Clientset.CoreV1().Services(ns).Delete(ctx, component+"-service", metav1.DeleteOptions{})

	return nil
}

func (m *MonitoringOffer) Bind(ctx context.Context, cm *k8s.ClientManager, app *models.Application, params map[string]string) (map[string]string, error) {
	if !cm.Connected {
		return nil, fmt.Errorf("kubernetes cluster not connected")
	}

	metricsPath := params["metricsPath"]
	if metricsPath == "" {
		metricsPath = "/metrics"
	}

	appPort := app.Port
	if appPort == 0 {
		appPort = 8080
	}

	dep, err := cm.Clientset.AppsV1().Deployments(app.Namespace).Get(ctx, app.Name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("application deployment not found: %w", err)
	}

	if dep.Spec.Template.Annotations == nil {
		dep.Spec.Template.Annotations = make(map[string]string)
	}
	dep.Spec.Template.Annotations["prometheus.io/scrape"] = "true"
	dep.Spec.Template.Annotations["prometheus.io/path"] = metricsPath
	dep.Spec.Template.Annotations["prometheus.io/port"] = strconv.Itoa(int(appPort))

	injectedEnvs := map[string]string{
		"METRICS_PATH":           metricsPath,
		"PROMETHEUS_SCRAPE_PORT": strconv.Itoa(int(appPort)),
		"PROMETHEUS_URL":         "http://prometheus-service.monitoring.svc.cluster.local:9090",
		"GRAFANA_DASHBOARD_URL":  "http://192.168.49.2:30080",
		"OTEL_EXPORTER_OTLP_ENDPOINT": "http://otel-collector-service.monitoring.svc.cluster.local:4317",
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

	var linkedOffers []models.LinkedOffer
	if dep.Annotations != nil && dep.Annotations[k8s.AnnotationOffers] != "" {
		_ = json.Unmarshal([]byte(dep.Annotations[k8s.AnnotationOffers]), &linkedOffers)
	}

	newOffer := models.LinkedOffer{
		OfferID:   "monitoring",
		OfferName: "Observability Stack",
		BoundAt:   time.Now(),
		Config:    injectedEnvs,
	}

	updated := false
	for i, o := range linkedOffers {
		if o.OfferID == "monitoring" {
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
		return nil, fmt.Errorf("failed to bind monitoring offer: %w", err)
	}

	return injectedEnvs, nil
}

func (m *MonitoringOffer) Unbind(ctx context.Context, cm *k8s.ClientManager, app *models.Application) error {
	if !cm.Connected {
		return fmt.Errorf("kubernetes cluster not connected")
	}

	dep, err := cm.Clientset.AppsV1().Deployments(app.Namespace).Get(ctx, app.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	if dep.Spec.Template.Annotations != nil {
		delete(dep.Spec.Template.Annotations, "prometheus.io/scrape")
		delete(dep.Spec.Template.Annotations, "prometheus.io/path")
		delete(dep.Spec.Template.Annotations, "prometheus.io/port")
	}

	monitoringKeys := map[string]bool{
		"METRICS_PATH":                true,
		"PROMETHEUS_SCRAPE_PORT":      true,
		"PROMETHEUS_URL":              true,
		"GRAFANA_DASHBOARD_URL":       true,
		"OTEL_EXPORTER_OTLP_ENDPOINT": true,
	}

	if len(dep.Spec.Template.Spec.Containers) > 0 {
		var filteredEnv []corev1.EnvVar
		for _, env := range dep.Spec.Template.Spec.Containers[0].Env {
			if !monitoringKeys[env.Name] {
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
			if o.OfferID != "monitoring" {
				filteredOffers = append(filteredOffers, o)
			}
		}
		bytes, _ := json.Marshal(filteredOffers)
		dep.Annotations[k8s.AnnotationOffers] = string(bytes)
	}

	_, err = cm.Clientset.AppsV1().Deployments(app.Namespace).Update(ctx, dep, metav1.UpdateOptions{})
	return err
}