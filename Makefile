# Current Operator version
LATEST_TAG ?= $(shell git describe --tags --abbrev=0)
VERSION ?= $(LATEST_TAG).$(shell whoami)-dev.$(shell git --no-pager log --pretty=format:'%h' -n 1)
# # Default bundle image tag
# BUNDLE_IMG ?= controller-bundle:$(VERSION)
# # Options for 'bundle-build'
# ifneq ($(origin CHANNELS), undefined)
# BUNDLE_CHANNELS := --channels=$(CHANNELS)
# endif
# ifneq ($(origin DEFAULT_CHANNEL), undefined)
# BUNDLE_DEFAULT_CHANNEL := --default-channel=$(DEFAULT_CHANNEL)
# endif
# BUNDLE_METADATA_OPTS ?= $(BUNDLE_CHANNELS) $(BUNDLE_DEFAULT_CHANNEL)

IMG_NAME ?= namespace-controller

API_GROUP_LABEL ?= gial.lblw.dev/platform-tool-kapp
APPLICATION_LABEL ?= "label:${API_GROUP_LABEL}=namespace-controller" # used for kapp to get references to the application on a cluster level.

# Image URL to use all building/pushing image targets
IMG ?= gcr.io/REDACTED/namespace-controller:${VERSION} #TODO(rsong): seed in opensource context
# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:trivialVersions=true,preserveUnknownFields=false"

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

all: manager

SHELL = /bin/bash

print-version:
	@echo ${VERSION}
# Run tests
ENVTEST_ASSETS_DIR=$(shell pwd)/testbin
get-envtest: generate fmt vet manifests #TODO: adapt for proper integration testing
	mkdir -p ${ENVTEST_ASSETS_DIR}
	test -f ${ENVTEST_ASSETS_DIR}/setup-envtest.sh || curl -sSLo ${ENVTEST_ASSETS_DIR}/setup-envtest.sh https://raw.githubusercontent.com/kubernetes-sigs/controller-runtime/v0.7.0/hack/setup-envtest.sh

integration-test: get-envtest
	source ${ENVTEST_ASSETS_DIR}/setup-envtest.sh; fetch_envtest_tools $(ENVTEST_ASSETS_DIR); setup_envtest_env $(ENVTEST_ASSETS_DIR); go test ./... -coverprofile cover.out

# use for quick testing.
t:
	go test ./... -coverprofile cover.out

test: generate fmt vet manifests t

cover: test
	go tool cover -html cover.out -o coverage/index.html
	http-server coverage

# Build manager binary
manager: generate fmt vet
	go build -o bin/manager main.go

# Run against the configured Kubernetes cluster in ~/.kube/config
# Note that this does not install the webhook. 
run: generate fmt vet manifests
	go run ./main.go

# Generate manifests e.g. CRD, RBAC etc.
manifests: controller-gen
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=deploy/crd/bases output:rbac:artifacts:config=deploy/rbac

# Run go fmt against code
fmt:
	go fmt ./...

# Run go vet against code
vet:
	go vet ./...

# Generate code
generate: controller-gen
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

# Download controller-gen locally if necessary
CONTROLLER_GEN = $(shell pwd)/bin/controller-gen
controller-gen:
	$(call go-get-tool,$(CONTROLLER_GEN),sigs.k8s.io/controller-tools/cmd/controller-gen@v0.4.1)

# Download kustomize locally if necessary
KUSTOMIZE = $(shell pwd)/bin/kustomize
kustomize:
	$(call go-get-tool,$(KUSTOMIZE),sigs.k8s.io/kustomize/kustomize/v3@v3.8.7)

# go-get-tool will 'go get' any package $2 and install it to $1.
PROJECT_DIR := $(shell dirname $(abspath $(lastword $(MAKEFILE_LIST))))
define go-get-tool
@[ -f $(1) ] || { \
set -e ;\
TMP_DIR=$$(mktemp -d) ;\
cd $$TMP_DIR ;\
go mod init tmp ;\
echo "Downloading $(2)" ;\
GOBIN=$(PROJECT_DIR)/bin go get $(2) ;\
rm -rf $$TMP_DIR ;\
}
endef

# Generate bundle manifests and metadata, then validate generated files.
.PHONY: bundle
bundle: manifests kustomize
	operator-sdk generate kustomize manifests -q
	cd deploy/manager && $(KUSTOMIZE) edit set image controller=$(IMG)
	$(KUSTOMIZE) build deploy/manifests | operator-sdk generate bundle -q --overwrite --version $(VERSION) $(BUNDLE_METADATA_OPTS)
	operator-sdk bundle validate ./bundle

# Build the bundle image.
.PHONY: bundle-build
bundle-build:
	docker build -f bundle.Dockerfile -t $(BUNDLE_IMG) .

docker-build:
	docker build -t $(IMG_NAME):local .

docker-push-dev: docker-build
	docker tag $(IMG_NAME):local gcr.io/REDACTED/$(IMG_NAME):$(VERSION) #TODO(rsong): seed in opensource context
	docker push gcr.io/REDACTED/$(IMG_NAME):$(VERSION) #TODO(rsong): seed in opensource context


docker-push-prod: docker-build
	docker tag $(IMG_NAME):local gcr.io/REDACTED/$(IMG_NAME):$(VERSION) #TODO(rsong): seed in opensource context
	docker push gcr.io/REDACTED/$(IMG_NAME):$(VERSION) #TODO(rsong): seed in opensource context

deploy-local: manifests docker-local-load kustomize
	kustomize build deploy/overlays/local | kapp deploy -y -a ${APPLICATION_LABEL} -f -

deploy-cloud: manifests kustomize
	kustomize build deploy/overlays/cloud | kapp deploy -y -a ${APPLICATION_LABEL} -f -

up: kind-up bootstrap kustomize deploy-local

kind-up:
	kind create cluster --name namespace-controller --config=hack/kind-config.yaml

bootstrap:
	docker pull quay.io/jetstack/cert-manager-cainjector:v1.1.0
	kind load docker-image quay.io/jetstack/cert-manager-cainjector:v1.1.0 --name namespace-controller
	docker pull quay.io/jetstack/cert-manager-controller:v1.1.0
	kind load docker-image quay.io/jetstack/cert-manager-controller:v1.1.0 --name namespace-controller
	docker pull quay.io/jetstack/cert-manager-webhook:v1.1.0 
	kind load docker-image quay.io/jetstack/cert-manager-webhook:v1.1.0 --name namespace-controller
	kapp deploy -y -a label:${API_GROUP_LABEL}=cert-manager -f https://github.com/jetstack/cert-manager/releases/download/v1.1.0/cert-manager.yaml

docker-local-load: docker-build
	kind load docker-image $(IMG_NAME):local --name namespace-controller

down:
	kind delete cluster --name namespace-controller || true
	kubectl config delete-context kind-namespace-controller-john || true

redeploy: deploy-local restart-deployment

restart-deployment: 
	kubectl rollout restart deployment -n namespace-controller-system

# Defaults to patch release. For any other release, set NEW_RELEASE_VERSION. Expects no trailing tags.
#
# The bash version of the command is as follows: let VERSION represent the old semver tag. Then:
# $ MAJOR_MINOR_VERSION = $(echo ${VERSION} | sed -E 's/(.*)\.[0-9]*/\1/g') # discard everything after the last dot
# $ PATCH_VERSION = "$(echo ${VERSION} | sed -E 's/.*\.([0-9]*)/\1/g')" # grab numbers after the last dot
# $ INCREMENTED_PATCH_VERSION = "$(( ${PATCH_VERSION} + 1))" # increment 1 to patch version under arithmetic expansion
# $ NEW_RELEASE_VERSION="${MAJOR_MINOR_VERSION}.${INCREMENTED_PATCH_VERSION}"
NEW_RELEASE_VERSION ?= $(shell echo "$(shell echo ${LATEST_TAG} | sed -E 's/(.*)\.[0-9]*/\1/g').$(shell echo $$(($(shell echo ${LATEST_TAG} | sed -E 's/.*\.([0-9]*)/\1/g')+1)))")
release:
	yq e "(.images[] | select(.name==\"controller\"))  |= { \
    \"name\": .name, \
    \"newTag\": \"$(NEW_RELEASE_VERSION)\" \
    }" --inplace deploy/manager/kustomization.yaml
	git add deploy/manager/kustomization.yaml
	git commit -m "chore: release version ${NEW_RELEASE_VERSION}"
	git tag ${NEW_RELEASE_VERSION}
	git push
	git push --tags

release-container-prod: docker-build
	docker tag $(IMG_NAME):local gcr.io/REDACTED/$(IMG_NAME):$(NEW_RELEASE_VERSION) #TODO(rsong): seed in opensource context
	docker push gcr.io/REDACTED/$(IMG_NAME):$(NEW_RELEASE_VERSION) #TODO(rsong): seed in opensource context
