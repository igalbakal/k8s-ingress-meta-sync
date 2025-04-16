\\
10# Makefile for K8s Ingress Meta Sync

# Image URL to use all building/pushing image targets
IMG ?= k8s-ingress-meta-sync:latest
# Kubernetes namespace to deploy the controller
NAMESPACE ?= ingress-meta-sync-system

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

.PHONY: all
all: build

##@ Development

.PHONY: fmt
fmt: ## Run go fmt against code
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code
	go vet ./...

.PHONY: test
test: fmt vet ## Run tests
	go test ./... -coverprofile cover.out

.PHONY: test-integration
test-integration: ## Run integration tests
	go test ./... -tags=integration

.PHONY: build
build: fmt vet ## Build binary
	go build -o bin/manager cmd/manager/main.go

.PHONY: run
run: fmt vet ## Run from source
	go run ./cmd/manager/main.go

##@ Deployment

.PHONY: docker-build
docker-build: ## Build docker image
	docker build -t ${IMG} .

.PHONY: docker-push
docker-push: ## Push docker image
	docker push ${IMG}

.PHONY: deploy-crds
deploy-crds: ## Install CRDs into the K8s cluster
	kubectl apply -f config/crds/providerconfig.yaml
	kubectl apply -f config/crds/ingressconfig.yaml
	kubectl apply -f config/crds/syncconfig.yaml

.PHONY: deploy-rbac
deploy-rbac: ## Deploy RBAC resources to the K8s cluster
	kubectl apply -f config/rbac.yaml

.PHONY: deploy
deploy: deploy-crds deploy-rbac ## Deploy to the K8s cluster
	cd config && kubectl apply -f deployment.yaml

.PHONY: undeploy
undeploy: ## Undeploy from the K8s cluster
	kubectl delete -f config/deployment.yaml
	kubectl delete -f config/rbac.yaml
	kubectl delete -f config/crds/syncconfig.yaml
	kubectl delete -f config/crds/ingressconfig.yaml
	kubectl delete -f config/crds/providerconfig.yaml

##@ Examples

.PHONY: deploy-example-github-cloudflare
deploy-example-github-cloudflare: deploy-crds ## Deploy GitHub to Cloudflare example
	kubectl apply -f examples/github-to-cloudflare.yaml

.PHONY: deploy-example-github-istio
deploy-example-github-istio: deploy-crds ## Deploy GitHub to Istio example
	kubectl apply -f examples/github-to-istio.yaml

##@ Documentation

.PHONY: generate-diagrams
generate-diagrams: ## Generate diagrams from Mermaid sources
	@echo "Open docs/render-diagrams.html in a browser to generate and save diagrams"

##@ Helpers

.PHONY: help
help: ## Display this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)
