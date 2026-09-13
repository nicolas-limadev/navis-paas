.PHONY: build test run clean minikube-start

BINARY_NAME=bin/navispaas

build:
	go build -o $(BINARY_NAME) cmd/server/main.go

test:
	go test ./... -v

run: build
	./$(BINARY_NAME)

minikube-start:
	minikube start --driver=docker --cpus=2 --memory=4096

clean:
	rm -rf bin/
