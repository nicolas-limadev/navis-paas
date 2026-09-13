.PHONY: build test run clean minikube-start tunnel backstage

BINARY_NAME=bin/navispaas

build:
	go build -o $(BINARY_NAME) cmd/server/main.go

test:
	go test ./... -v

run: build
	./$(BINARY_NAME)

minikube-start:
	minikube start --driver=docker --cpus=2 --memory=4096

tunnel:
	@echo "Opening Minikube Tunnel for LoadBalancer services..."
	@echo "Keep this terminal running. Applications will be directly accessible via External IP!"
	minikube tunnel

backstage:
	@./scripts/start-backstage.sh

clean:
	rm -rf bin/
