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
	PostgresNamespace = "postgres"
)

type PostgresOffer struct{}

func (p *PostgresOffer) GetDefinition() models.OfferDefinition {
	return models.OfferDefinition{
		ID:          "postgresql",
		Name:        "PostgreSQL Relational DB",
		Category:    "Database",
		Description: "Reliable ACID-compliant relational SQL database with automatic credential injection.",
		Version:     "16-alpine",
		Icon:        "postgresql",
		Parameters: []models.OfferParameter{
			{
				Key:         "databaseName",
				Label:       "Database Name",
				Type:        "string",
				Default:     "appdb",
				Description: "Initial database name to be created",
				Required:    true,
			},
			{
				Key:         "username",
				Label:       "DB Username",
				Type:        "string",
				Default:     "postgres",
				Description: "PostgreSQL master username",
				Required:    false,
			},
		},
	}
}

func (p *PostgresOffer) IsInstalled(ctx context.Context, cm *k8s.ClientManager) (bool, error) {
	if !cm.Connected {
		return false, nil
	}
	// Check primary postgres deployment
	_, err := cm.Clientset.AppsV1().Deployments(PostgresNamespace).Get(ctx, "postgres", metav1.GetOptions{})
	if err == nil {
		return true, nil
	}
	// Fallback to legacy namespace if present
	_, err = cm.Clientset.AppsV1().Deployments("data").Get(ctx, "postgres", metav1.GetOptions{})
	return err == nil, nil
}

func (p *PostgresOffer) Install(ctx context.Context, cm *k8s.ClientManager) error {
	if !cm.Connected {
		return fmt.Errorf("kubernetes cluster not connected")
	}
	ns := PostgresNamespace

	// 1. Ensure well-described namespace with cloud-native labels and annotations
	_, err := cm.Clientset.CoreV1().Namespaces().Get(ctx, ns, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		nsObj := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: ns,
				Labels: map[string]string{
					"name":                      ns,
					"app.kubernetes.io/name":    "postgresql",
					"app.kubernetes.io/part-of": "navispaas",
					"navispaas.io/managed-by":   "navispaas",
					"navispaas.io/tier":         "database",
					"navispaas.io/component":    "postgresql",
				},
				Annotations: map[string]string{
					"navispaas.io/description": "PostgreSQL Relational Database Service managed by NavisPaaS",
				},
			},
		}
		_, err = cm.Clientset.CoreV1().Namespaces().Create(ctx, nsObj, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create postgres namespace: %w", err)
		}
	}

	replicas := int32(1)
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "postgres",
			Namespace: ns,
			Labels: map[string]string{
				"app":                       "postgres",
				"app.kubernetes.io/name":    "postgres",
				"app.kubernetes.io/part-of": "navispaas",
				"navispaas.io/managed-by":   "navispaas",
				"navispaas.io/component":    "database",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "postgres"}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":                       "postgres",
						"app.kubernetes.io/name":    "postgres",
						"app.kubernetes.io/part-of": "navispaas",
						"navispaas.io/managed-by":   "navispaas",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "postgres",
							Image: "postgres:16-alpine",
							Env: []corev1.EnvVar{
								{Name: "POSTGRES_DB", Value: "postgres"},
								{Name: "POSTGRES_USER", Value: "postgres"},
								{Name: "POSTGRES_PASSWORD", Value: "navispass123"},
							},
							Ports: []corev1.ContainerPort{{ContainerPort: 5432, Name: "postgres"}},
						},
					},
				},
			},
		},
	}
	_, err = cm.Clientset.AppsV1().Deployments(ns).Get(ctx, "postgres", metav1.GetOptions{})
	if errors.IsNotFound(err) {
		_, _ = cm.Clientset.AppsV1().Deployments(ns).Create(ctx, dep, metav1.CreateOptions{})
	}

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "postgres-service",
			Namespace: ns,
			Labels: map[string]string{
				"app":                       "postgres",
				"app.kubernetes.io/name":    "postgres",
				"navispaas.io/managed-by":   "navispaas",
			},
		},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeClusterIP,
			Selector: map[string]string{"app": "postgres"},
			Ports:    []corev1.ServicePort{{Name: "postgres", Port: 5432, TargetPort: intstr.FromInt(5432)}},
		},
	}
	_, err = cm.Clientset.CoreV1().Services(ns).Get(ctx, "postgres-service", metav1.GetOptions{})
	if errors.IsNotFound(err) {
		_, _ = cm.Clientset.CoreV1().Services(ns).Create(ctx, svc, metav1.CreateOptions{})
	}
	return nil
}

func (p *PostgresOffer) Bind(ctx context.Context, cm *k8s.ClientManager, app *models.Application, params map[string]string) (map[string]string, error) {
	if !cm.Connected {
		return nil, fmt.Errorf("kubernetes cluster not connected")
	}
	dbName := params["databaseName"]
	if dbName == "" {
		dbName = "postgres"
	}
	user := params["username"]
	if user == "" {
		user = "postgres"
	}
	pass := params["password"]
	if pass == "" {
		pass = "navispass123"
	}

	// Check if postgres service is in dedicated 'postgres' namespace or fallback
	targetNS := PostgresNamespace
	if _, err := cm.Clientset.CoreV1().Services(PostgresNamespace).Get(ctx, "postgres-service", metav1.GetOptions{}); err != nil {
		if _, err := cm.Clientset.CoreV1().Services("data").Get(ctx, "postgres-service", metav1.GetOptions{}); err == nil {
			targetNS = "data"
		}
	}
	host := fmt.Sprintf("postgres-service.%s.svc.cluster.local", targetNS)

	injectedEnvs := map[string]string{
		"DATABASE_HOST":     host,
		"DATABASE_PORT":     "5432",
		"DATABASE_NAME":     dbName,
		"DATABASE_USER":     user,
		"DATABASE_PASSWORD": pass,
		"DB_HOST":           host,
		"DB_PORT":           "5432",
		"DB_NAME":           dbName,
		"DB_USER":           user,
		"DB_PASSWORD":       pass,
		"POSTGRES_HOST":     host,
		"POSTGRES_PORT":     "5432",
		"POSTGRES_DB":       dbName,
		"POSTGRES_USER":     user,
		"POSTGRES_PASSWORD": pass,
		"DATABASE_URL":      fmt.Sprintf("postgres://%s:%s@%s:5432/%s?sslmode=disable", user, pass, host, dbName),
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
		OfferID:   "postgresql",
		OfferName: "PostgreSQL Relational DB",
		BoundAt:   time.Now(),
		Config:    injectedEnvs,
	}
	updated := false
	for i, o := range linkedOffers {
		if o.OfferID == "postgresql" {
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

func (p *PostgresOffer) Unbind(ctx context.Context, cm *k8s.ClientManager, app *models.Application) error {
	if !cm.Connected {
		return fmt.Errorf("kubernetes cluster not connected")
	}
	dep, err := cm.Clientset.AppsV1().Deployments(app.Namespace).Get(ctx, app.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	dbKeys := map[string]bool{
		"DATABASE_HOST": true, "DATABASE_PORT": true, "DATABASE_NAME": true, "DATABASE_USER": true, "DATABASE_PASSWORD": true,
		"DB_HOST": true, "DB_PORT": true, "DB_NAME": true, "DB_USER": true, "DB_PASSWORD": true,
		"POSTGRES_HOST": true, "POSTGRES_PORT": true, "POSTGRES_DB": true, "POSTGRES_USER": true, "POSTGRES_PASSWORD": true,
		"DATABASE_URL": true,
	}

	// Read and extract all keys dynamically from linked offer config
	var linkedOffers []models.LinkedOffer
	var filteredOffers []models.LinkedOffer
	if dep.Annotations != nil && dep.Annotations[k8s.AnnotationOffers] != "" {
		_ = json.Unmarshal([]byte(dep.Annotations[k8s.AnnotationOffers]), &linkedOffers)
		for _, o := range linkedOffers {
			if o.OfferID == "postgresql" {
				for k := range o.Config {
					dbKeys[k] = true
				}
			} else {
				filteredOffers = append(filteredOffers, o)
			}
		}
		bytes, _ := json.Marshal(filteredOffers)
		dep.Annotations[k8s.AnnotationOffers] = string(bytes)
	}

	if len(dep.Spec.Template.Spec.Containers) > 0 {
		var filteredEnv []corev1.EnvVar
		for _, env := range dep.Spec.Template.Spec.Containers[0].Env {
			if !dbKeys[env.Name] {
				filteredEnv = append(filteredEnv, env)
			}
		}
		dep.Spec.Template.Spec.Containers[0].Env = filteredEnv
	}

	_, err = cm.Clientset.AppsV1().Deployments(app.Namespace).Update(ctx, dep, metav1.UpdateOptions{})
	return err
}
