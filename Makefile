.PHONY: build build-cli test run clean minikube-start k3d-start kind-start tunnel backstage

BINARY_NAME=bin/navispaas
CLI_BINARY=bin/navis

build:
	go build -o $(BINARY_NAME) cmd/server/main.go

build-cli:
	go build -o $(CLI_BINARY) ./cmd/navis

test:
	go test ./... -v

run: build
	./$(BINARY_NAME)

# Minikube
minikube-start:
	minikube start --driver=docker --cpus=2 --memory=4096

tunnel:
	@echo "Opening Minikube Tunnel for LoadBalancer services..."
	@echo "Keep this terminal running. Applications will be directly accessible via External IP!"
	minikube tunnel

# k3d (K3s in Docker)
k3d-start:
	@echo "Starting k3d cluster..."
	k3d cluster create navispaas --servers 1 --agents 1 --port "8080:80@loadbalancer" --port "8443:443@loadbalancer" --k3s-arg "--disable=traefik@server:0"

k3d-stop:
	k3d cluster delete navispaas

# Kind (Kubernetes in Docker)
kind-start:
	@echo "Starting Kind cluster..."
	kind create cluster --name navispaas --config kind-config.yaml 2>/dev/null || kind create cluster --name navispaas

kind-stop:
	kind delete cluster --name navispaas

# Docker Desktop (manual setup)
docker-desktop-info:
	@echo "Docker Desktop Kubernetes:"
	@echo "1. Enable Kubernetes in Docker Desktop settings"
	@echo "2. Run: kubectl config use-context docker-desktop"
	@echo "3. Run: make run"

# Rancher Desktop (manual setup)
rancher-desktop-info:
	@echo "Rancher Desktop Kubernetes:"
	@echo "1. Enable Kubernetes in Rancher Desktop settings"
	@echo "2. Run: kubectl config use-context rancher-desktop"
	@echo "3. Run: make run"

# MicroK8s (manual setup)
microk8s-info:
	@echo "MicroK8s Kubernetes:"
	@echo "1. Run: microk8s enable dns storage"
	@echo "2. Run: microk8s config > ~/.kube/config"
	@echo "3. Run: make run"

# Universal: Auto-detect provider and show info
provider-info:
	@echo "Detecting Kubernetes provider..."
	@./$(BINARY_NAME) --detect-provider 2>/dev/null || echo "Run 'make run' to start NavisPaaS"

backstage:
	@./scripts/start-backstage.sh

clean:
	rm -rf bin/
