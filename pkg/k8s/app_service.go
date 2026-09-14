package k8s

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"time"

	"github.com/nicolas-limadev/navis-paas/pkg/models"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	LabelManagedBy     = "navispaas.io/managed-by"
	LabelAppName       = "app.kubernetes.io/name"
	AnnotationOffers   = "navispaas.io/linked-offers"
	DefaultNamespace   = "navis-apps"
	ManagedByValue     = "navispaas"
)

type AppService struct {
	clientManager   *ClientManager
	registryService *RegistryService
}

func NewAppService(cm *ClientManager, regSvc *RegistryService) *AppService {
	return &AppService{
		clientManager:   cm,
		registryService: regSvc,
	}
}

// EnsureNamespace ensures the designated namespace exists
func (s *AppService) EnsureNamespace(ctx context.Context, namespace string) error {
	if namespace == "" {
		namespace = DefaultNamespace
	}
	_, err := s.clientManager.Clientset.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespace,
				Labels: map[string]string{
					LabelManagedBy:     ManagedByValue,
					"navispaas.io/app": namespace,
				},
			},
		}
		_, err = s.clientManager.Clientset.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
	}
	return err
}

// CreateOrUpdateApp deploys an application (Deployment + Service) to Kubernetes
func (s *AppService) CreateOrUpdateApp(ctx context.Context, req models.CreateAppRequest) (*models.Application, error) {
	if !s.clientManager.Connected {
		return nil, fmt.Errorf("cluster disconnected: %w", s.clientManager.LastError)
	}

	// Default to dedicated per-application namespace
	namespace := req.Namespace
	if namespace == "" {
		namespace = req.Name
	}

	if err := s.EnsureNamespace(ctx, namespace); err != nil {
		return nil, fmt.Errorf("failed to ensure namespace %s: %w", namespace, err)
	}

	replicas := req.Replicas
	if replicas <= 0 {
		replicas = 1
	}

	// Prepare container env vars
	var envVars []corev1.EnvVar
	for k, v := range req.EnvVars {
		envVars = append(envVars, corev1.EnvVar{
			Name:  k,
			Value: v,
		})
	}
	sort.Slice(envVars, func(i, j int) bool {
		return envVars[i].Name < envVars[j].Name
	})

	labels := map[string]string{
		LabelManagedBy: ManagedByValue,
		LabelAppName:   req.Name,
		"app":          req.Name,
	}

	// Check if deployment already exists to preserve linked offers
	var existingOffers []models.LinkedOffer
	existingDep, err := s.clientManager.Clientset.AppsV1().Deployments(namespace).Get(ctx, req.Name, metav1.GetOptions{})
	if err == nil && existingDep.Annotations != nil {
		if offersJson, exists := existingDep.Annotations[AnnotationOffers]; exists {
			_ = json.Unmarshal([]byte(offersJson), &existingOffers)
		}
	}

	offersBytes, _ := json.Marshal(existingOffers)
	annotations := map[string]string{
		AnnotationOffers: string(offersBytes),
	}

	image := req.Image
	var imagePullSecrets []corev1.LocalObjectReference
	if s.registryService != nil {
		image = s.registryService.ResolveImage(req.Image)
		_ = s.registryService.EnsureRegistrySecret(ctx, namespace)
		imagePullSecrets = s.registryService.GetImagePullSecrets()
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:        req.Name,
			Namespace:   namespace,
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					LabelAppName: req.Name,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					ImagePullSecrets: imagePullSecrets,
					Containers: []corev1.Container{
						{
							Name:            req.Name,
							Image:           image,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Ports: []corev1.ContainerPort{
								{
									Name:          "http",
									ContainerPort: req.Port,
								},
							},
							Env: envVars,
						},
					},
				},
			},
		},
	}

	if existingDep != nil && existingDep.Name != "" {
		_, err = s.clientManager.Clientset.AppsV1().Deployments(namespace).Update(ctx, deployment, metav1.UpdateOptions{})
	} else {
		_, err = s.clientManager.Clientset.AppsV1().Deployments(namespace).Create(ctx, deployment, metav1.CreateOptions{})
	}
	if err != nil {
		return nil, fmt.Errorf("failed to deploy application: %w", err)
	}

	// Create or update Service (LoadBalancer: works with 'minikube tunnel' and allocates NodePort)
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      req.Name,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeLoadBalancer,
			Selector: map[string]string{
				LabelAppName: req.Name,
			},
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Port:       req.Port,
					TargetPort: intstr.FromInt(int(req.Port)),
				},
			},
		},
	}

	existingSvc, err := s.clientManager.Clientset.CoreV1().Services(namespace).Get(ctx, req.Name, metav1.GetOptions{})
	if err == nil && existingSvc != nil {
		service.ResourceVersion = existingSvc.ResourceVersion
		service.Spec.ClusterIP = existingSvc.Spec.ClusterIP
		// preserve assigned nodePort if present
		if len(existingSvc.Spec.Ports) > 0 && existingSvc.Spec.Ports[0].NodePort > 0 {
			service.Spec.Ports[0].NodePort = existingSvc.Spec.Ports[0].NodePort
		}
		_, err = s.clientManager.Clientset.CoreV1().Services(namespace).Update(ctx, service, metav1.UpdateOptions{})
	} else {
		_, err = s.clientManager.Clientset.CoreV1().Services(namespace).Create(ctx, service, metav1.CreateOptions{})
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create/update service: %w", err)
	}

	return s.GetApp(ctx, namespace, req.Name)
}

// ListApps returns all applications managed by NavisPaaS across namespaces
func (s *AppService) ListApps(ctx context.Context, namespace string) ([]models.Application, error) {
	if !s.clientManager.Connected {
		return nil, fmt.Errorf("cluster disconnected: %w", s.clientManager.LastError)
	}

	listOptions := metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", LabelManagedBy, ManagedByValue),
	}

	deployments, err := s.clientManager.Clientset.AppsV1().Deployments(namespace).List(ctx, listOptions)
	if err != nil {
		return nil, err
	}

	var apps []models.Application
	for _, dep := range deployments.Items {
		app := s.convertDeploymentToApp(&dep)
		// Fetch matching service to grab NodePort, ClusterIP, LoadBalancer IP
		svc, err := s.clientManager.Clientset.CoreV1().Services(dep.Namespace).Get(ctx, dep.Name, metav1.GetOptions{})
		if err == nil {
			app.ClusterIP = svc.Spec.ClusterIP
			if len(svc.Spec.Ports) > 0 {
				app.NodePort = svc.Spec.Ports[0].NodePort
			}
			if len(svc.Status.LoadBalancer.Ingress) > 0 {
				ingress := svc.Status.LoadBalancer.Ingress[0]
				if ingress.IP != "" {
					app.ExternalIP = ingress.IP
					app.AccessURL = fmt.Sprintf("http://%s:%d", ingress.IP, app.Port)
				} else if ingress.Hostname != "" {
					app.ExternalIP = ingress.Hostname
					app.AccessURL = fmt.Sprintf("http://%s:%d", ingress.Hostname, app.Port)
				}
			}
			if app.AccessURL == "" && app.NodePort > 0 {
				app.AccessURL = fmt.Sprintf("http://192.168.49.2:%d", app.NodePort)
			}
		}
		apps = append(apps, *app)
	}

	sort.Slice(apps, func(i, j int) bool {
		return apps[i].Name < apps[j].Name
	})

	return apps, nil
}

// findAppDeployment locates an application deployment across namespaces reliably
func (s *AppService) findAppDeployment(ctx context.Context, namespace, name string) (*appsv1.Deployment, error) {
	// 1. If namespace explicitly provided, look there first
	if namespace != "" {
		dep, err := s.clientManager.Clientset.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
		if err == nil {
			return dep, nil
		}
	}

	// 2. Check namespace matching the app name
	if dep, err := s.clientManager.Clientset.AppsV1().Deployments(name).Get(ctx, name, metav1.GetOptions{}); err == nil {
		return dep, nil
	}

	// 3. Check DefaultNamespace (navis-apps)
	if dep, err := s.clientManager.Clientset.AppsV1().Deployments(DefaultNamespace).Get(ctx, name, metav1.GetOptions{}); err == nil {
		return dep, nil
	}

	// 4. Search across all namespaces by label selector
	list, err := s.clientManager.Clientset.AppsV1().Deployments("").List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", LabelAppName, name),
	})
	if err == nil && len(list.Items) > 0 {
		return &list.Items[0], nil
	}

	// 5. Global scan across all namespaces for exact deployment name
	allDeps, err := s.clientManager.Clientset.AppsV1().Deployments("").List(ctx, metav1.ListOptions{})
	if err == nil {
		for _, item := range allDeps.Items {
			if item.Name == name {
				return &item, nil
			}
		}
	}

	return nil, fmt.Errorf("deployments.apps %q not found in any namespace", name)
}

// GetApp returns the detailed application information
func (s *AppService) GetApp(ctx context.Context, namespace, name string) (*models.Application, error) {
	if !s.clientManager.Connected {
		return nil, fmt.Errorf("cluster disconnected: %w", s.clientManager.LastError)
	}

	dep, err := s.findAppDeployment(ctx, namespace, name)
	if err != nil {
		return nil, err
	}

	app := s.convertDeploymentToApp(dep)
	app.Namespace = dep.Namespace

	// Fetch service in the deployment's actual namespace
	svc, err := s.clientManager.Clientset.CoreV1().Services(dep.Namespace).Get(ctx, name, metav1.GetOptions{})
	if err == nil {
		app.ClusterIP = svc.Spec.ClusterIP
		if len(svc.Spec.Ports) > 0 {
			app.NodePort = svc.Spec.Ports[0].NodePort
		}
		if len(svc.Status.LoadBalancer.Ingress) > 0 {
			ingress := svc.Status.LoadBalancer.Ingress[0]
			if ingress.IP != "" {
				app.ExternalIP = ingress.IP
				app.AccessURL = fmt.Sprintf("http://%s:%d", ingress.IP, app.Port)
			} else if ingress.Hostname != "" {
				app.ExternalIP = ingress.Hostname
				app.AccessURL = fmt.Sprintf("http://%s:%d", ingress.Hostname, app.Port)
			}
		}
		if app.AccessURL == "" && app.NodePort > 0 {
			app.AccessURL = fmt.Sprintf("http://192.168.49.2:%d", app.NodePort)
		}
	}

	return app, nil
}

// GetAppDetails returns application info plus current running pods
func (s *AppService) GetAppDetails(ctx context.Context, namespace, name string) (*models.AppDetailResponse, error) {
	app, err := s.GetApp(ctx, namespace, name)
	if err != nil {
		return nil, err
	}

	podList, err := s.clientManager.Clientset.CoreV1().Pods(app.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", LabelAppName, name),
	})
	if err != nil {
		return nil, err
	}

	var pods []models.PodInfo
	for _, p := range podList.Items {
		var restarts int32
		for _, cs := range p.Status.ContainerStatuses {
			restarts += cs.RestartCount
		}

		startTime := time.Time{}
		if p.Status.StartTime != nil {
			startTime = p.Status.StartTime.Time
		}

		pods = append(pods, models.PodInfo{
			Name:      p.Name,
			Status:    string(p.Status.Phase),
			Restarts:  restarts,
			IP:        p.Status.PodIP,
			Node:      p.Spec.NodeName,
			StartTime: startTime,
		})
	}

	return &models.AppDetailResponse{
		Application: *app,
		Pods:        pods,
	}, nil
}

// DeleteApp deletes the deployment, service, and any linked resources
func (s *AppService) DeleteApp(ctx context.Context, namespace, name string) error {
	if !s.clientManager.Connected {
		return fmt.Errorf("cluster disconnected: %w", s.clientManager.LastError)
	}

	dep, err := s.findAppDeployment(ctx, namespace, name)
	if err != nil {
		return err
	}
	actualNamespace := dep.Namespace

	// Delete Deployment
	_ = s.clientManager.Clientset.AppsV1().Deployments(actualNamespace).Delete(ctx, name, metav1.DeleteOptions{})

	// Delete Service
	_ = s.clientManager.Clientset.CoreV1().Services(actualNamespace).Delete(ctx, name, metav1.DeleteOptions{})

	// If the application was in its own dedicated namespace, remove the namespace cleanly
	if actualNamespace == name {
		_ = s.clientManager.Clientset.CoreV1().Namespaces().Delete(ctx, actualNamespace, metav1.DeleteOptions{})
	}

	return nil
}

// GetAppLogs retrieves recent logs from the first available pod of the app
func (s *AppService) GetAppLogs(ctx context.Context, namespace, name string, tailLines int64) (string, error) {
	if !s.clientManager.Connected {
		return "", fmt.Errorf("cluster disconnected: %w", s.clientManager.LastError)
	}

	dep, err := s.findAppDeployment(ctx, namespace, name)
	if err != nil {
		return "", err
	}
	actualNamespace := dep.Namespace

	if tailLines <= 0 {
		tailLines = 100
	}

	pods, err := s.clientManager.Clientset.CoreV1().Pods(actualNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", LabelAppName, name),
	})
	if err != nil || len(pods.Items) == 0 {
		return "No active pods found for this application.", nil
	}

	podName := pods.Items[0].Name
	logOpts := &corev1.PodLogOptions{
		TailLines: &tailLines,
	}

	req := s.clientManager.Clientset.CoreV1().Pods(namespace).GetLogs(podName, logOpts)
	podLogs, err := req.Stream(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to open log stream: %w", err)
	}
	defer podLogs.Close()

	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, podLogs)
	if err != nil {
		return "", fmt.Errorf("failed to read log buffer: %w", err)
	}

	return buf.String(), nil
}

func (s *AppService) convertDeploymentToApp(dep *appsv1.Deployment) *models.Application {
	var port int32
	var image string
	envVars := make(map[string]string)

	if len(dep.Spec.Template.Spec.Containers) > 0 {
		container := dep.Spec.Template.Spec.Containers[0]
		image = container.Image
		if len(container.Ports) > 0 {
			port = container.Ports[0].ContainerPort
		}
		for _, env := range container.Env {
			envVars[env.Name] = env.Value
		}
	}

	var linkedOffers []models.LinkedOffer
	if dep.Annotations != nil {
		if offersJson, exists := dep.Annotations[AnnotationOffers]; exists {
			_ = json.Unmarshal([]byte(offersJson), &linkedOffers)
		}
	}

	status := "Pending"
	if dep.Status.ReadyReplicas > 0 && dep.Status.ReadyReplicas == *dep.Spec.Replicas {
		status = "Running"
	} else if dep.Status.Replicas > 0 && dep.Status.ReadyReplicas < *dep.Spec.Replicas {
		status = "Degraded"
	}

	replicas := int32(1)
	if dep.Spec.Replicas != nil {
		replicas = *dep.Spec.Replicas
	}

	return &models.Application{
		Name:         dep.Name,
		Namespace:    dep.Namespace,
		Image:        image,
		Port:         port,
		Replicas:     replicas,
		ReadyCount:   dep.Status.ReadyReplicas,
		Status:       status,
		EnvVars:      envVars,
		LinkedOffers: linkedOffers,
		CreatedAt:    dep.CreationTimestamp.Time,
	}
}
