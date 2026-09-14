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
	MinIONamespace = "minio"
)

type MinIOOffer struct{}

func (m *MinIOOffer) GetDefinition() models.OfferDefinition {
	return models.OfferDefinition{
		ID:          "minio",
		Name:        "MinIO Object Storage (S3)",
		Category:    "Storage",
		Description: "MinIO - High performance, S3-compatible object storage for files, media, and backups.",
		Version:     "RELEASE.2024",
		Icon:        "minio",
		Parameters: []models.OfferParameter{
			{
				Key:         "rootUser",
				Label:       "Root Username",
				Type:        "string",
				Default:     "minioadmin",
				Description: "MinIO root access key",
				Required:    false,
			},
			{
				Key:         "rootPassword",
				Label:       "Root Password",
				Type:        "string",
				Default:     "minioadmin",
				Description: "MinIO root secret key",
				Required:    false,
			},
		},
	}
}

func (m *MinIOOffer) IsInstalled(ctx context.Context, cm *k8s.ClientManager) (bool, error) {
	if !cm.Connected {
		return false, nil
	}
	_, err := cm.Clientset.AppsV1().Deployments(MinIONamespace).Get(ctx, "minio", metav1.GetOptions{})
	if err == nil {
		return true, nil
	}
	return false, nil
}

func (m *MinIOOffer) Install(ctx context.Context, cm *k8s.ClientManager) error {
	if !cm.Connected {
		return fmt.Errorf("kubernetes cluster not connected")
	}

	ns := MinIONamespace
	// 1. Create namespace
	_, err := cm.Clientset.CoreV1().Namespaces().Get(ctx, ns, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		nsObj := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: ns,
				Labels: map[string]string{
					"name":                      ns,
					"app.kubernetes.io/name":    "minio",
					"app.kubernetes.io/part-of": "navispaas",
					"navispaas.io/managed-by":   "navispaas",
					"navispaas.io/tier":         "storage",
				},
				Annotations: map[string]string{
					"navispaas.io/description": "MinIO S3 Storage managed by NavisPaaS",
				},
			},
		}
		_, err = cm.Clientset.CoreV1().Namespaces().Create(ctx, nsObj, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create minio namespace: %w", err)
		}
	}

	// 2. MinIO Deployment
	minioReplicas := int32(1)
	minioDep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "minio",
			Namespace: ns,
			Labels:    map[string]string{"app": "minio", "navispaas.io/managed-by": "navispaas"},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &minioReplicas,
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "minio"}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "minio"},
					Annotations: map[string]string{
						"navispaas.io/managed-by": "navispaas",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "minio",
							Image: "minio/minio:RELEASE.2024-05-28T17-19-04Z",
							Args:  []string{"server", "/data", "--console-address", ":9001"},
							Env: []corev1.EnvVar{
								{Name: "MINIO_ROOT_USER", Value: "minioadmin"},
								{Name: "MINIO_ROOT_PASSWORD", Value: "minioadmin"},
							},
							Ports: []corev1.ContainerPort{
								{ContainerPort: 9000, Name: "api"},
								{ContainerPort: 9001, Name: "console"},
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/minio/health/ready",
										Port: intstr.FromInt(9000),
									},
								},
								InitialDelaySeconds: 10,
								PeriodSeconds:       5,
							},
						},
					},
				},
			},
		},
	}
	_, err = cm.Clientset.AppsV1().Deployments(ns).Get(ctx, "minio", metav1.GetOptions{})
	if errors.IsNotFound(err) {
		_, _ = cm.Clientset.AppsV1().Deployments(ns).Create(ctx, minioDep, metav1.CreateOptions{})
	}

	// 3. MinIO Services (NodePort)
	minioSvc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "minio-service",
			Namespace: ns,
			Labels:    map[string]string{"app": "minio"},
		},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeNodePort,
			Selector: map[string]string{"app": "minio"},
			Ports: []corev1.ServicePort{
				{Name: "api", Port: 9000, TargetPort: intstr.FromInt(9000), NodePort: 30900},
				{Name: "console", Port: 9001, TargetPort: intstr.FromInt(9001), NodePort: 30901},
			},
		},
	}
	_, err = cm.Clientset.CoreV1().Services(ns).Get(ctx, "minio-service", metav1.GetOptions{})
	if errors.IsNotFound(err) {
		_, _ = cm.Clientset.CoreV1().Services(ns).Create(ctx, minioSvc, metav1.CreateOptions{})
	}

	return nil
}

func (m *MinIOOffer) Bind(ctx context.Context, cm *k8s.ClientManager, app *models.Application, params map[string]string) (map[string]string, error) {
	if !cm.Connected {
		return nil, fmt.Errorf("kubernetes cluster not connected")
	}

	user := params["rootUser"]
	if user == "" {
		user = "minioadmin"
	}
	pass := params["rootPassword"]
	if pass == "" {
		pass = "minioadmin"
	}

	host := "minio-service.minio.svc.cluster.local"

	injectedEnvs := map[string]string{
		"MINIO_HOST":            host,
		"MINIO_PORT":            "9000",
		"MINIO_ROOT_USER":       user,
		"MINIO_ROOT_PASSWORD":   pass,
		"S3_ENDPOINT":           fmt.Sprintf("http://%s:9000", host),
		"AWS_ACCESS_KEY_ID":     user,
		"AWS_SECRET_ACCESS_KEY": pass,
		"AWS_DEFAULT_REGION":    "us-east-1",
		"S3_USE_PATH_STYLE":     "true",
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
		OfferID:   "minio",
		OfferName: "MinIO Object Storage (S3)",
		BoundAt:   time.Now(),
		Config:    injectedEnvs,
	}

	updated := false
	for i, o := range linkedOffers {
		if o.OfferID == "minio" {
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

func (m *MinIOOffer) Unbind(ctx context.Context, cm *k8s.ClientManager, app *models.Application) error {
	if !cm.Connected {
		return fmt.Errorf("kubernetes cluster not connected")
	}

	dep, err := cm.Clientset.AppsV1().Deployments(app.Namespace).Get(ctx, app.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	minioKeys := map[string]bool{
		"MINIO_HOST": true, "MINIO_PORT": true, "MINIO_ROOT_USER": true, "MINIO_ROOT_PASSWORD": true,
		"S3_ENDPOINT": true, "AWS_ACCESS_KEY_ID": true, "AWS_SECRET_ACCESS_KEY": true, "AWS_DEFAULT_REGION": true, "S3_USE_PATH_STYLE": true,
	}

	if len(dep.Spec.Template.Spec.Containers) > 0 {
		var filteredEnv []corev1.EnvVar
		for _, env := range dep.Spec.Template.Spec.Containers[0].Env {
			if !minioKeys[env.Name] {
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
			if o.OfferID != "minio" {
				filteredOffers = append(filteredOffers, o)
			}
		}
		bytes, _ := json.Marshal(filteredOffers)
		dep.Annotations[k8s.AnnotationOffers] = string(bytes)
	}

	_, err = cm.Clientset.AppsV1().Deployments(app.Namespace).Update(ctx, dep, metav1.UpdateOptions{})
	return err
}
