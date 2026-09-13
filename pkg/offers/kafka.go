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

type KafkaOffer struct{}

func (k *KafkaOffer) GetDefinition() models.OfferDefinition {
	return models.OfferDefinition{
		ID:          "kafka",
		Name:        "Apache Kafka (Event Streaming)",
		Category:    "Messaging",
		Description: "Distributed event streaming platform with KRaft mode and Strimzi support for ultra-low latency messaging.",
		Version:     "3.8.0",
		Icon:        "kafka",
		Parameters: []models.OfferParameter{
			{
				Key:         "topic",
				Label:       "Topic Name",
				Type:        "string",
				Default:     "events",
				Description: "Kafka topic name for this application to produce/consume",
				Required:    true,
			},
			{
				Key:         "consumerGroup",
				Label:       "Consumer Group ID",
				Type:        "string",
				Default:     "default-group",
				Description: "Kafka consumer group identifier",
				Required:    false,
			},
			{
				Key:         "partitions",
				Label:       "Partitions Count",
				Type:        "number",
				Default:     "3",
				Description: "Number of topic partitions for parallel consumption",
				Required:    false,
			},
		},
	}
}

func (k *KafkaOffer) IsInstalled(ctx context.Context, cm *k8s.ClientManager) (bool, error) {
	if !cm.Connected {
		return false, nil
	}
	// Check if kafka namespace and service exist
	_, err := cm.Clientset.CoreV1().Services("kafka").Get(ctx, "kafka-service", metav1.GetOptions{})
	if err == nil {
		return true, nil
	}
	return false, nil
}

// Install provisions a lightweight KRaft Apache Kafka cluster in the "kafka" namespace
func (k *KafkaOffer) Install(ctx context.Context, cm *k8s.ClientManager) error {
	if !cm.Connected {
		return fmt.Errorf("kubernetes cluster not connected")
	}

	ns := "kafka"
	// 1. Create namespace
	_, err := cm.Clientset.CoreV1().Namespaces().Get(ctx, ns, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		_, err = cm.Clientset.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: ns,
				Labels: map[string]string{
					"navispaas.io/managed-by": "navispaas",
				},
			},
		}, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create kafka namespace: %w", err)
		}
	}

	// 2. Service
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kafka-service",
			Namespace: ns,
			Labels: map[string]string{
				"app": "kafka",
			},
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
			Selector: map[string]string{
				"app": "kafka",
			},
			Ports: []corev1.ServicePort{
				{
					Name:       "client",
					Port:       9092,
					TargetPort: intstr.FromInt(9092),
				},
			},
		},
	}
	_, err = cm.Clientset.CoreV1().Services(ns).Get(ctx, "kafka-service", metav1.GetOptions{})
	if errors.IsNotFound(err) {
		_, err = cm.Clientset.CoreV1().Services(ns).Create(ctx, svc, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create kafka service: %w", err)
		}
	}

	// 3. Kafka KRaft single-node Deployment (fast and resource-lean for Minikube)
	replicas := int32(1)
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kafka-broker",
			Namespace: ns,
			Labels: map[string]string{
				"app":                     "kafka",
				"navispaas.io/managed-by": "navispaas",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "kafka"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "kafka"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "kafka",
							Image: "apache/kafka:3.8.0",
							Ports: []corev1.ContainerPort{
								{ContainerPort: 9092, Name: "client"},
							},
							Env: []corev1.EnvVar{
								{Name: "KAFKA_NODE_ID", Value: "1"},
								{Name: "KAFKA_PROCESS_ROLES", Value: "broker,controller"},
								{Name: "KAFKA_LISTENERS", Value: "PLAINTEXT://:9092,CONTROLLER://:9093"},
								{Name: "KAFKA_ADVERTISED_LISTENERS", Value: "PLAINTEXT://kafka-service.kafka.svc.cluster.local:9092"},
								{Name: "KAFKA_CONTROLLER_LISTENER_NAMES", Value: "CONTROLLER"},
								{Name: "KAFKA_LISTENER_SECURITY_PROTOCOL_MAP", Value: "CONTROLLER:PLAINTEXT,PLAINTEXT:PLAINTEXT"},
								{Name: "KAFKA_CONTROLLER_QUORUM_VOTERS", Value: "1@localhost:9093"},
								{Name: "KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR", Value: "1"},
								{Name: "KAFKA_TRANSACTION_STATE_LOG_REPLICATION_FACTOR", Value: "1"},
								{Name: "KAFKA_TRANSACTION_STATE_LOG_MIN_ISR", Value: "1"},
								{Name: "KAFKA_GROUP_INITIAL_REBALANCE_DELAY_MS", Value: "0"},
								{Name: "KAFKA_NUM_PARTITIONS", Value: "3"},
							},
						},
					},
				},
			},
		},
	}

	_, err = cm.Clientset.AppsV1().Deployments(ns).Get(ctx, "kafka-broker", metav1.GetOptions{})
	if errors.IsNotFound(err) {
		_, err = cm.Clientset.AppsV1().Deployments(ns).Create(ctx, dep, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to deploy kafka: %w", err)
		}
	}

	return nil
}

// Bind injects Kafka configurations and environment variables into the application deployment
func (k *KafkaOffer) Bind(ctx context.Context, cm *k8s.ClientManager, app *models.Application, params map[string]string) (map[string]string, error) {
	if !cm.Connected {
		return nil, fmt.Errorf("kubernetes cluster not connected")
	}

	topic := params["topic"]
	if topic == "" {
		topic = fmt.Sprintf("%s-events", app.Name)
	}

	consumerGroup := params["consumerGroup"]
	if consumerGroup == "" {
		consumerGroup = fmt.Sprintf("%s-group", app.Name)
	}

	bootstrapServers := "kafka-service.kafka.svc.cluster.local:9092"

	injectedEnvs := map[string]string{
		"KAFKA_BOOTSTRAP_SERVERS": bootstrapServers,
		"KAFKA_TOPIC":             topic,
		"KAFKA_CONSUMER_GROUP":    consumerGroup,
		"KAFKA_CLIENT_ID":         app.Name,
		"KAFKA_SECURITY_PROTOCOL": "PLAINTEXT",
	}

	// 1. Create or update ConfigMap in the application's namespace
	cmName := fmt.Sprintf("%s-kafka-binding", app.Name)
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cmName,
			Namespace: app.Namespace,
			Labels: map[string]string{
				"navispaas.io/managed-by": "navispaas",
				"navispaas.io/app":        app.Name,
				"navispaas.io/offer":      "kafka",
			},
		},
		Data: injectedEnvs,
	}

	_, err := cm.Clientset.CoreV1().ConfigMaps(app.Namespace).Get(ctx, cmName, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		_, err = cm.Clientset.CoreV1().ConfigMaps(app.Namespace).Create(ctx, configMap, metav1.CreateOptions{})
	} else {
		_, err = cm.Clientset.CoreV1().ConfigMaps(app.Namespace).Update(ctx, configMap, metav1.UpdateOptions{})
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create kafka binding configmap: %w", err)
	}

	// 2. Patch application deployment with env variables and linked offer annotation
	dep, err := cm.Clientset.AppsV1().Deployments(app.Namespace).Get(ctx, app.Name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("application deployment not found: %w", err)
	}

	// Add/Update env vars in container
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

	// Replace or append kafka offer
	updated := false
	newOffer := models.LinkedOffer{
		OfferID:   "kafka",
		OfferName: "Apache Kafka",
		BoundAt:   time.Now(),
		Config:    injectedEnvs,
	}
	for i, o := range linkedOffers {
		if o.OfferID == "kafka" {
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
		return nil, fmt.Errorf("failed to update deployment with kafka binding: %w", err)
	}

	return injectedEnvs, nil
}

// Unbind removes Kafka binding and injected variables from the application
func (k *KafkaOffer) Unbind(ctx context.Context, cm *k8s.ClientManager, app *models.Application) error {
	if !cm.Connected {
		return fmt.Errorf("kubernetes cluster not connected")
	}

	cmName := fmt.Sprintf("%s-kafka-binding", app.Name)
	_ = cm.Clientset.CoreV1().ConfigMaps(app.Namespace).Delete(ctx, cmName, metav1.DeleteOptions{})

	dep, err := cm.Clientset.AppsV1().Deployments(app.Namespace).Get(ctx, app.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	// Remove Kafka env vars
	kafkaKeys := map[string]bool{
		"KAFKA_BOOTSTRAP_SERVERS": true,
		"KAFKA_TOPIC":             true,
		"KAFKA_CONSUMER_GROUP":    true,
		"KAFKA_CLIENT_ID":         true,
		"KAFKA_SECURITY_PROTOCOL": true,
	}

	if len(dep.Spec.Template.Spec.Containers) > 0 {
		var filteredEnv []corev1.EnvVar
		for _, env := range dep.Spec.Template.Spec.Containers[0].Env {
			if !kafkaKeys[env.Name] {
				filteredEnv = append(filteredEnv, env)
			}
		}
		dep.Spec.Template.Spec.Containers[0].Env = filteredEnv
	}

	// Remove from annotations
	var linkedOffers []models.LinkedOffer
	if dep.Annotations != nil && dep.Annotations[k8s.AnnotationOffers] != "" {
		_ = json.Unmarshal([]byte(dep.Annotations[k8s.AnnotationOffers]), &linkedOffers)
		var filteredOffers []models.LinkedOffer
		for _, o := range linkedOffers {
			if o.OfferID != "kafka" {
				filteredOffers = append(filteredOffers, o)
			}
		}
		bytes, _ := json.Marshal(filteredOffers)
		dep.Annotations[k8s.AnnotationOffers] = string(bytes)
	}

	_, err = cm.Clientset.AppsV1().Deployments(app.Namespace).Update(ctx, dep, metav1.UpdateOptions{})
	return err
}
