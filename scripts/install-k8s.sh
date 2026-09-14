#!/usr/bin/env bash
set -e

echo "====================================================="
echo " ☸️  NavisPaaS - Automatic Local Kubernetes Installer"
echo "====================================================="
echo "This script will download and install Minikube and Kubectl"
echo "on your machine so you can run NavisPaaS local workloads."
echo "====================================================="

OS_TYPE="$(uname -s)"
ARCH_TYPE="$(uname -m)"

# 1. Detect Docker dependency
if ! command -v docker &> /dev/null; then
  echo "⚠️ Warning: Docker was not found. Minikube requires a container driver (Docker is highly recommended)."
  echo "Please install Docker from https://docs.docker.com/get-docker/ before starting Minikube."
  read -p "Do you want to continue anyway? (y/N) " -n 1 -r
  echo ""
  if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    exit 1
  fi
fi

# 2. Install Minikube
if command -v minikube &> /dev/null; then
  echo "✅ Minikube is already installed ($(minikube version --short))"
else
  echo "📥 Installing Minikube..."
  
  if [ "$OS_TYPE" = "Linux" ]; then
    if [ "$ARCH_TYPE" = "x86_64" ]; then
      curl -LO https://storage.googleapis.com/minikube/releases/latest/minikube-linux-amd64
      sudo install minikube-linux-amd64 /usr/local/bin/minikube
      rm minikube-linux-amd64
    elif [ "$ARCH_TYPE" = "aarch64" ] || [ "$ARCH_TYPE" = "arm64" ]; then
      curl -LO https://storage.googleapis.com/minikube/releases/latest/minikube-linux-arm64
      sudo install minikube-linux-arm64 /usr/local/bin/minikube
      rm minikube-linux-arm64
    else
      echo "❌ Unsupported architecture: $ARCH_TYPE"
      exit 1
    fi
  elif [ "$OS_TYPE" = "Darwin" ]; then
    if command -v brew &> /dev/null; then
      brew install minikube
    else
      if [ "$ARCH_TYPE" = "x86_64" ]; then
        curl -LO https://storage.googleapis.com/minikube/releases/latest/minikube-darwin-amd64
        sudo install minikube-darwin-amd64 /usr/local/bin/minikube
        rm minikube-darwin-amd64
      else
        curl -LO https://storage.googleapis.com/minikube/releases/latest/minikube-darwin-arm64
        sudo install minikube-darwin-arm64 /usr/local/bin/minikube
        rm minikube-darwin-arm64
      fi
    fi
  else
    echo "❌ Unsupported OS: $OS_TYPE. Please install Minikube manually from: https://minikube.sigs.k8s.io/docs/start/"
    exit 1
  fi
  
  echo "✅ Minikube installed successfully!"
fi

# 3. Install Kubectl
if command -v kubectl &> /dev/null; then
  echo "✅ Kubectl is already installed"
else
  echo "📥 Installing Kubectl..."
  
  if [ "$OS_TYPE" = "Linux" ]; then
    if [ "$ARCH_TYPE" = "x86_64" ]; then
      curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"
    else
      curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/arm64/kubectl"
    fi
    sudo install -o root -g root -m 0755 kubectl /usr/local/bin/kubectl
    rm kubectl
  elif [ "$OS_TYPE" = "Darwin" ]; then
    if command -v brew &> /dev/null; then
      brew install kubernetes-cli
    else
      if [ "$ARCH_TYPE" = "x86_64" ]; then
        curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/darwin/amd64/kubectl"
      else
        curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/darwin/arm64/kubectl"
      fi
      chmod +x ./kubectl
      sudo mv ./kubectl /usr/local/bin/kubectl
    fi
  fi
  
  echo "✅ Kubectl installed successfully!"
fi

# 4. Start Minikube
echo ""
echo "🚀 Everything is ready! Starting Minikube cluster with docker driver..."
echo "This might take a minute depending on your internet connection."
echo ""

minikube start --driver=docker --cpus=2 --memory=4096

echo ""
echo "====================================================="
echo " 🎉 Minikube is now running!"
echo "====================================================="
echo "👉 Run: make tunnel (in a separate terminal) to enable LoadBalancer routing"
echo "👉 Run: navis start  to launch the NavisPaaS platform!"
echo "====================================================="
