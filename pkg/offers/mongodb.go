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
	MongoDBNamespace = "mongodb"
)

type MongoDBOffer struct{}

func (m *MongoDBOffer) GetDefinition() models.OfferDefinition {
	return models.OfferDefinition{
		ID:          "mongodb",
		Name:        "MongoDB NoSQL Database",
		Category:    "Database",
		Description: "MongoDB - Document-oriented NoSQL database with auto-injected connection parameters and authentication.",
		Version:     "7.0",
		Icon:        "mongodb",
		Parameters: []models.OfferParameter{
			{
				Key:         "databaseName",
				Label:       "Database Name",
				Type:        "string",
				Default:     "navisdb",
				Description: "Default database to create",
				Required:    false,
			},
			{
				Key:         "username",
				Label:       "Root Username",
				Type:        "string",
				Default:     "admin",
				Description: "MongoDB root username",
				Required:    false,
			},
			{
				Key:         "password",
				Label:       "Root Password",
				Type:        "string",
				Default:     "secret123",
				Description: "MongoDB root password",
				Required:    false,
			},
		},
	}
}

func (m *MongoDBOffer) IsInstalled(ctx context.Context, cm *k8s.ClientManager) (bool, error) {
	if !cm.Connected {
		return false, nil
	}
	_, err := cm.Clientset.AppsV1().Deployments(MongoDBNamespace).Get(ctx, "mongodb", metav1.GetOptions{})
	if err == nil {
		return true, nil
	}
	return false, nil
}

func (m *MongoDBOffer) Install(ctx context.Context, cm *k8s.ClientManager) error {
	if !cm.Connected {
		return fmt.Errorf("kubernetes cluster not connected")
	}

	ns := MongoDBNamespace
	// 1. Create namespace
	_, err := cm.Clientset.CoreV1().Namespaces().Get(ctx, ns, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		nsObj := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: ns,
				Labels: map[string]string{
					"name":                      ns,
					"app.kubernetes.io/name":    "mongodb",
					"app.kubernetes.io/part-of": "navispaas",
					"navispaas.io/managed-by":   "navispaas",
					"navispaas.io/tier":         "database",
				},
				Annotations: map[string]string{
					"navispaas.io/description": "MongoDB Database managed by NavisPaaS",
				},
			},
		}
		_, err = cm.Clientset.CoreV1().Namespaces().Create(ctx, nsObj, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create mongodb namespace: %w", err)
		}
	}

	// 2. MongoDB Deployment
	mongoReplicas := int32(1)
	mongoDep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mongodb",
			Namespace: ns,
			Labels:    map[string]string{"app": "mongodb", "navispaas.io/managed-by": "navispaas"},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &mongoReplicas,
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "mongodb"}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "mongodb"},
					Annotations: map[string]string{
						"navispaas.io/managed-by": "navispaas",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "mongodb",
							Image: "mongo:7.0",
							Env: []corev1.EnvVar{
								{Name: "MONGO_INITDB_ROOT_USERNAME", Value: "admin"},
								{Name: "MONGO_INITDB_ROOT_PASSWORD", Value: "secret123"},
								{Name: "MONGO_INITDB_DATABASE", Value: "navisdb"},
							},
							Ports: []corev1.ContainerPort{{ContainerPort: 27017, Name: "mongodb"}},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									TCPSocket: &corev1.TCPSocketAction{
										Port: intstr.FromInt(27017),
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
	_, err = cm.Clientset.AppsV1().Deployments(ns).Get(ctx, "mongodb", metav1.GetOptions{})
	if errors.IsNotFound(err) {
		_, _ = cm.Clientset.AppsV1().Deployments(ns).Create(ctx, mongoDep, metav1.CreateOptions{})
	}

	// 3. MongoDB Service (NodePort)
	mongoSvc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mongodb-service",
			Namespace: ns,
			Labels:    map[string]string{"app": "mongodb"},
		},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeNodePort,
			Selector: map[string]string{"app": "mongodb"},
			Ports: []corev1.ServicePort{
				{Name: "mongodb", Port: 27017, TargetPort: intstr.FromInt(27017), NodePort: 32017},
			},
		},
	}
	_, err = cm.Clientset.CoreV1().Services(ns).Get(ctx, "mongodb-service", metav1.GetOptions{})
	if errors.IsNotFound(err) {
		_, _ = cm.Clientset.CoreV1().Services(ns).Create(ctx, mongoSvc, metav1.CreateOptions{})
	}

	return nil
}

func (m *MongoDBOffer) Bind(ctx context.Context, cm *k8s.ClientManager, app *models.Application, params map[string]string) (map[string]string, error) {
	if !cm.Connected {
		return nil, fmt.Errorf("kubernetes cluster not connected")
	}

	dbName := params["databaseName"]
	if dbName == "" {
		dbName = "navisdb"
	}
	user := params["username"]
	if user == "" {
		user = "admin"
	}
	pass := params["password"]
	if pass == "" {
		pass = "secret123"
	}

	host := "mongodb-service.mongodb.svc.cluster.local"

	injectedEnvs := map[string]string{
		"MONGODB_HOST":     host,
		"MONGODB_PORT":     "27017",
		"MONGODB_DATABASE": dbName,
		"MONGODB_USER":     user,
		"MONGODB_PASSWORD": pass,
		"MONGODB_URL":      fmt.Sprintf("mongodb://%s:%s@%s:27017/%s?authSource=admin", user, pass, host, dbName),
		"MONGO_URL":        fmt.Sprintf("mongodb://%s:%s@%s:27017/%s?authSource=admin", user, pass, host, dbName),
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
		OfferID:   "mongodb",
		OfferName: "MongoDB NoSQL Database",
		BoundAt:   time.Now(),
		Config:    injectedEnvs,
	}

	updated := false
	for i, o := range linkedOffers {
		if o.OfferID == "mongodb" {
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

func (m *MongoDBOffer) Unbind(ctx context.Context, cm *k8s.ClientManager, app *models.Application) error {
	if !cm.Connected {
		return fmt.Errorf("kubernetes cluster not connected")
	}

	dep, err := cm.Clientset.AppsV1().Deployments(app.Namespace).Get(ctx, app.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	mongoKeys := map[string]bool{
		"MONGODB_HOST": true, "MONGODB_PORT": true, "MONGODB_DATABASE": true, "MONGODB_USER": true, "MONGODB_PASSWORD": true, "MONGODB_URL": true, "MONGO_URL": true,
	}

	if len(dep.Spec.Template.Spec.Containers) > 0 {
		var filteredEnv []corev1.EnvVar
		for _, env := range dep.Spec.Template.Spec.Containers[0].Env {
			if !mongoKeys[env.Name] {
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
			if o.OfferID != "mongodb" {
				filteredOffers = append(filteredOffers, o)
			}
		}
		bytes, _ := json.Marshal(filteredOffers)
		dep.Annotations[k8s.AnnotationOffers] = string(bytes)
	}

	_, err = cm.Clientset.AppsV1().Deployments(app.Namespace).Update(ctx, dep, metav1.UpdateOptions{})
	return err
}
