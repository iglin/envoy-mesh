IMG          ?= envoy-mesh-envoy:latest
HELM_REGISTRY ?= ghcr.io/iglin/envoy-mesh/charts
CHART_VERSION := $(shell grep '^version:' helm/Chart.yaml | awk '{print $$2}')

.PHONY: docker-build
docker-build:
	docker build -t $(IMG) .

.PHONY: docker-push
docker-push:
	docker push $(IMG)

.PHONY: docker-build-push
docker-build-push: docker-build docker-push

.PHONY: helm-package
helm-package:
	mkdir -p dist
	helm package helm/ --destination dist/

.PHONY: helm-push
helm-push: helm-package
	helm push dist/envoy-$(CHART_VERSION).tgz oci://$(HELM_REGISTRY)

.PHONY: helm-build-push
helm-build-push: helm-package helm-push
