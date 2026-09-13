package models

import "time"

// Application represents a user application managed by NavisPaaS
type Application struct {
	Name         string            `json:"name"`
	Namespace    string            `json:"namespace"`
	Image        string            `json:"image"`
	Port         int32             `json:"port"`
	Replicas     int32             `json:"replicas"`
	ReadyCount   int32             `json:"readyCount"`
	Status       string            `json:"status"` // Running, Pending, Degraded, etc.
	NodePort     int32             `json:"nodePort,omitempty"`
	ClusterIP    string            `json:"clusterIP,omitempty"`
	EnvVars      map[string]string `json:"envVars,omitempty"`
	LinkedOffers []LinkedOffer     `json:"linkedOffers"`
	CreatedAt    time.Time         `json:"createdAt"`
}

// LinkedOffer represents an offer currently attached to an application
type LinkedOffer struct {
	OfferID   string            `json:"offerId"`   // e.g., "kafka", "monitoring"
	OfferName string            `json:"offerName"` // Display name
	BoundAt   time.Time         `json:"boundAt"`
	Config    map[string]string `json:"config"` // injected variables or configuration
}

// CreateAppRequest payload for creating a new application
type CreateAppRequest struct {
	Name      string            `json:"name" binding:"required"`
	Namespace string            `json:"namespace,omitempty"` // defaults to "navis-apps"
	Image     string            `json:"image" binding:"required"`
	Port      int32             `json:"port" binding:"required"`
	Replicas  int32             `json:"replicas,omitempty"` // defaults to 1
	EnvVars   map[string]string `json:"envVars,omitempty"`
	Offers    []string          `json:"offers,omitempty"` // optional list of offers to auto-bind
}

// UpdateAppRequest payload for updating an existing application
type UpdateAppRequest struct {
	Image    string            `json:"image,omitempty"`
	Port     int32             `json:"port,omitempty"`
	Replicas *int32            `json:"replicas,omitempty"`
	EnvVars  map[string]string `json:"envVars,omitempty"`
}

// PodInfo holds basic information about a running pod
type PodInfo struct {
	Name      string    `json:"name"`
	Status    string    `json:"status"`
	Restarts  int32     `json:"restarts"`
	IP        string    `json:"ip"`
	Node      string    `json:"node"`
	StartTime time.Time `json:"startTime"`
}

// AppDetailResponse includes full app metadata and pod statuses
type AppDetailResponse struct {
	Application Application `json:"application"`
	Pods        []PodInfo   `json:"pods"`
}

// OfferDefinition represents an addon available in the NavisPaaS catalog
type OfferDefinition struct {
	ID          string           `json:"id"`       // "kafka", "monitoring", etc.
	Name        string           `json:"name"`     // "Apache Kafka"
	Category    string           `json:"category"` // "Messaging", "Observability", "Database"
	Description string           `json:"description"`
	Version     string           `json:"version"`
	Icon        string           `json:"icon"`      // SVG or icon class
	Installed   bool             `json:"installed"` // Cluster-level operator installed
	Parameters  []OfferParameter `json:"parameters"`
}

// OfferParameter configures an offer binding
type OfferParameter struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	Type        string `json:"type"` // "string", "number", "boolean", "select"
	Default     string `json:"default"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
}

// LinkOfferRequest payload to link an offer to an application
type LinkOfferRequest struct {
	OfferID    string            `json:"offerId" binding:"required"`
	Parameters map[string]string `json:"parameters,omitempty"`
}

// ClusterStatus represents health and connectivity to the Kubernetes cluster
type ClusterStatus struct {
	Connected      bool   `json:"connected"`
	ClusterVersion string `json:"clusterVersion,omitempty"`
	Context        string `json:"context,omitempty"`
	ServerURL      string `json:"serverUrl,omitempty"`
	MinikubeActive bool   `json:"minikubeActive"`
	ErrorMessage   string `json:"errorMessage,omitempty"`
}
