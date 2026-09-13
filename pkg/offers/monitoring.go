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

type MonitoringOffer struct{}

func (m *MonitoringOffer) GetDefinition() models.OfferDefinition {
	return models.OfferDefinition{
		ID:          "monitoring",
		Name:        "Observability Stack (Prometheus + Grafana + OTel)",
		Category:    "Observability",
		Description: "Instant metrics scraping, Grafana dashboards, and OpenTelemetry distributed tracing integration.",
		Version:     "2.54.0 / Grafana 11.2",
		Icon:        "observability",
		Parameters: []models.OfferParameter{
			{
				Key:         "metricsPath",
				Label:       "Metrics Endpoint Path",
				Type:        "string",
				Default:     "/metrics",
				Description: "HTTP endpoint where your application exposes Prometheus metrics",
				Required:    true,
			},
			{
				Key:         "scrapeInterval",
				Label:       "Scrape Interval",
				Type:        "string",
				Default:     "15s",
				Description: "How frequently Prometheus should poll your application",
				Required:    false,
			},
			{
				Key:         "enableOTel",
				Label:       "Enable OpenTelemetry Tracing",
				Type:        "boolean",
				Default:     "true",
				Description: "Auto-inject OTel SDK Collector endpoints for distributed traces",
				Required:    false,
			},
		},
	}
}

func (m *MonitoringOffer) IsInstalled(ctx context.Context, cm *k8s.ClientManager) (bool, error) {
	if !cm.Connected {
		return false, nil
	}
	_, err := cm.Clientset.CoreV1().Services("monitoring").Get(ctx, "prometheus-service", metav1.GetOptions{})
	if err == nil {
		return true, nil
	}
	return false, nil
}

// Install provisions Prometheus and Grafana in the "monitoring" namespace
func (m *MonitoringOffer) Install(ctx context.Context, cm *k8s.ClientManager) error {
	if !cm.Connected {
		return fmt.Errorf("kubernetes cluster not connected")
	}

	ns := "monitoring"
	// 1. Create namespace
	_, err := cm.Clientset.CoreV1().Namespaces().Get(ctx, ns, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		_, err = cm.Clientset.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
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
					"navispaas.io/description": "Prometheus, Grafana and OpenTelemetry observability stack managed by NavisPaaS",
				},
			},
		}, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create monitoring namespace: %w", err)
		}
	}

	// 2. Prometheus ConfigMap with Pod Scrape Discovery
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
	_, err = cm.Clientset.CoreV1().ConfigMaps(ns).Get(ctx, "prometheus-config", metav1.GetOptions{})
	if errors.IsNotFound(err) {
		_, _ = cm.Clientset.CoreV1().ConfigMaps(ns).Create(ctx, promCM, metav1.CreateOptions{})
	}

	// 3. Prometheus Deployment & Service
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

	// 4. Grafana Config & Datasource
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
	_, err = cm.Clientset.CoreV1().ConfigMaps(ns).Get(ctx, "grafana-datasources", metav1.GetOptions{})
	if errors.IsNotFound(err) {
		_, _ = cm.Clientset.CoreV1().ConfigMaps(ns).Create(ctx, grafanaCM, metav1.CreateOptions{})
	}

	// 5. Grafana Deployment & Service
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

// Bind annotates the application deployment for Prometheus scraping and injects OTel configuration
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

	// 1. Pod annotations for Prometheus Scraper and OTel
	if dep.Spec.Template.Annotations == nil {
		dep.Spec.Template.Annotations = make(map[string]string)
	}
	dep.Spec.Template.Annotations["prometheus.io/scrape"] = "true"
	dep.Spec.Template.Annotations["prometheus.io/path"] = metricsPath
	dep.Spec.Template.Annotations["prometheus.io/port"] = strconv.Itoa(int(appPort))
	dep.Spec.Template.Annotations["instrumentation.opentelemetry.io/inject-sdk"] = "true"

	// 2. Inject environment variables into application container
	injectedEnvs := map[string]string{
		"METRICS_PATH":                metricsPath,
		"PROMETHEUS_SCRAPE_PORT":      strconv.Itoa(int(appPort)),
		"OTEL_SERVICE_NAME":           app.Name,
		"OTEL_EXPORTER_OTLP_ENDPOINT": "http://otel-collector.monitoring.svc.cluster.local:4317",
		"GRAFANA_DASHBOARD_URL":       "http://localhost:30080",
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

	// 3. Update LinkedOffer annotation
	var linkedOffers []models.LinkedOffer
	if dep.Annotations != nil && dep.Annotations[k8s.AnnotationOffers] != "" {
		_ = json.Unmarshal([]byte(dep.Annotations[k8s.AnnotationOffers]), &linkedOffers)
	}

	newOffer := models.LinkedOffer{
		OfferID:   "monitoring",
		OfferName: "Prometheus & Grafana Observability",
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

// Unbind removes Prometheus annotations and monitoring variables
func (m *MonitoringOffer) Unbind(ctx context.Context, cm *k8s.ClientManager, app *models.Application) error {
	if !cm.Connected {
		return fmt.Errorf("kubernetes cluster not connected")
	}

	dep, err := cm.Clientset.AppsV1().Deployments(app.Namespace).Get(ctx, app.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	// Remove pod annotations
	if dep.Spec.Template.Annotations != nil {
		delete(dep.Spec.Template.Annotations, "prometheus.io/scrape")
		delete(dep.Spec.Template.Annotations, "prometheus.io/path")
		delete(dep.Spec.Template.Annotations, "prometheus.io/port")
		delete(dep.Spec.Template.Annotations, "instrumentation.opentelemetry.io/inject-sdk")
	}

	// Remove env vars
	monitoringKeys := map[string]bool{
		"METRICS_PATH":                true,
		"PROMETHEUS_SCRAPE_PORT":      true,
		"OTEL_SERVICE_NAME":           true,
		"OTEL_EXPORTER_OTLP_ENDPOINT": true,
		"GRAFANA_DASHBOARD_URL":       true,
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

	// Update LinkedOffer annotation
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
