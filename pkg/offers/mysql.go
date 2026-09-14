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
	MySQLNamespace = "mysql"
)

type MySQLOffer struct{}

func (m *MySQLOffer) GetDefinition() models.OfferDefinition {
	return models.OfferDefinition{
		ID:          "mysql",
		Name:        "MySQL Relational DB",
		Category:    "Database",
		Description: "MySQL - World's most popular open-source relational database with auto-injected connection parameters.",
		Version:     "8.0",
		Icon:        "mysql",
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
				Label:       "Master Username",
				Type:        "string",
				Default:     "root",
				Description: "MySQL master username",
				Required:    false,
			},
			{
				Key:         "password",
				Label:       "Master Password",
				Type:        "string",
				Default:     "navispass123",
				Description: "MySQL master password",
				Required:    false,
			},
		},
	}
}

func (m *MySQLOffer) IsInstalled(ctx context.Context, cm *k8s.ClientManager) (bool, error) {
	if !cm.Connected {
		return false, nil
	}
	_, err := cm.Clientset.AppsV1().Deployments(MySQLNamespace).Get(ctx, "mysql", metav1.GetOptions{})
	if err == nil {
		return true, nil
	}
	return false, nil
}

func (m *MySQLOffer) Install(ctx context.Context, cm *k8s.ClientManager) error {
	if !cm.Connected {
		return fmt.Errorf("kubernetes cluster not connected")
	}

	ns := MySQLNamespace
	// 1. Create namespace
	_, err := cm.Clientset.CoreV1().Namespaces().Get(ctx, ns, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		nsObj := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: ns,
				Labels: map[string]string{
					"name":                      ns,
					"app.kubernetes.io/name":    "mysql",
					"app.kubernetes.io/part-of": "navispaas",
					"navispaas.io/managed-by":   "navispaas",
					"navispaas.io/tier":         "database",
				},
				Annotations: map[string]string{
					"navispaas.io/description": "MySQL Database managed by NavisPaaS",
				},
			},
		}
		_, err = cm.Clientset.CoreV1().Namespaces().Create(ctx, nsObj, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create mysql namespace: %w", err)
		}
	}

	// 2. MySQL Deployment
	mysqlReplicas := int32(1)
	mysqlDep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mysql",
			Namespace: ns,
			Labels:    map[string]string{"app": "mysql", "navispaas.io/managed-by": "navispaas"},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &mysqlReplicas,
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "mysql"}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "mysql"},
					Annotations: map[string]string{
						"navispaas.io/managed-by": "navispaas",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "mysql",
							Image: "mysql:8.0",
							Env: []corev1.EnvVar{
								{Name: "MYSQL_ROOT_PASSWORD", Value: "navispass123"},
								{Name: "MYSQL_DATABASE", Value: "navisdb"},
							},
							Ports: []corev1.ContainerPort{{ContainerPort: 3306, Name: "mysql"}},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									TCPSocket: &corev1.TCPSocketAction{
										Port: intstr.FromInt(3306),
									},
								},
								InitialDelaySeconds: 15,
								PeriodSeconds:       10,
							},
						},
					},
				},
			},
		},
	}
	_, err = cm.Clientset.AppsV1().Deployments(ns).Get(ctx, "mysql", metav1.GetOptions{})
	if errors.IsNotFound(err) {
		_, _ = cm.Clientset.AppsV1().Deployments(ns).Create(ctx, mysqlDep, metav1.CreateOptions{})
	}

	// 3. MySQL Service (NodePort)
	mysqlSvc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mysql-service",
			Namespace: ns,
			Labels:    map[string]string{"app": "mysql"},
		},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeNodePort,
			Selector: map[string]string{"app": "mysql"},
			Ports: []corev1.ServicePort{
				{Name: "mysql", Port: 3306, TargetPort: intstr.FromInt(3306), NodePort: 33060},
			},
		},
	}
	_, err = cm.Clientset.CoreV1().Services(ns).Get(ctx, "mysql-service", metav1.GetOptions{})
	if errors.IsNotFound(err) {
		_, _ = cm.Clientset.CoreV1().Services(ns).Create(ctx, mysqlSvc, metav1.CreateOptions{})
	}

	return nil
}

func (m *MySQLOffer) Bind(ctx context.Context, cm *k8s.ClientManager, app *models.Application, params map[string]string) (map[string]string, error) {
	if !cm.Connected {
		return nil, fmt.Errorf("kubernetes cluster not connected")
	}

	dbName := params["databaseName"]
	if dbName == "" {
		dbName = "navisdb"
	}
	user := params["username"]
	if user == "" {
		user = "root"
	}
	pass := params["password"]
	if pass == "" {
		pass = "navispass123"
	}

	host := "mysql-service.mysql.svc.cluster.local"

	injectedEnvs := map[string]string{
		"MYSQL_HOST":        host,
		"MYSQL_PORT":        "3306",
		"MYSQL_DATABASE":    dbName,
		"MYSQL_USER":        user,
		"MYSQL_PASSWORD":    pass,
		"MYSQL_URL":         fmt.Sprintf("mysql://%s:%s@%s:3306/%s", user, pass, host, dbName),
		"DATABASE_URL":      fmt.Sprintf("mysql://%s:%s@%s:3306/%s", user, pass, host, dbName),
		"DATABASE_HOST":     host,
		"DATABASE_PORT":     "3306",
		"DATABASE_NAME":     dbName,
		"DATABASE_USER":     user,
		"DATABASE_PASSWORD": pass,
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
		OfferID:   "mysql",
		OfferName: "MySQL Relational DB",
		BoundAt:   time.Now(),
		Config:    injectedEnvs,
	}

	updated := false
	for i, o := range linkedOffers {
		if o.OfferID == "mysql" {
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

func (m *MySQLOffer) Unbind(ctx context.Context, cm *k8s.ClientManager, app *models.Application) error {
	if !cm.Connected {
		return fmt.Errorf("kubernetes cluster not connected")
	}

	dep, err := cm.Clientset.AppsV1().Deployments(app.Namespace).Get(ctx, app.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	mysqlKeys := map[string]bool{
		"MYSQL_HOST": true, "MYSQL_PORT": true, "MYSQL_DATABASE": true, "MYSQL_USER": true, "MYSQL_PASSWORD": true, "MYSQL_URL": true,
		"DATABASE_URL": true, "DATABASE_HOST": true, "DATABASE_PORT": true, "DATABASE_NAME": true, "DATABASE_USER": true, "DATABASE_PASSWORD": true,
	}

	if len(dep.Spec.Template.Spec.Containers) > 0 {
		var filteredEnv []corev1.EnvVar
		for _, env := range dep.Spec.Template.Spec.Containers[0].Env {
			if !mysqlKeys[env.Name] {
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
			if o.OfferID != "mysql" {
				filteredOffers = append(filteredOffers, o)
			}
		}
		bytes, _ := json.Marshal(filteredOffers)
		dep.Annotations[k8s.AnnotationOffers] = string(bytes)
	}

	_, err = cm.Clientset.AppsV1().Deployments(app.Namespace).Update(ctx, dep, metav1.UpdateOptions{})
	return err
}
