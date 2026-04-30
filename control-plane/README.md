# envoy-mesh — control-plane

Kubernetes operator (kubebuilder v4) that reconciles Envoy xDS CRs into live
xDS snapshots served over gRPC/ADS on port 18000.

## How it works

1. User applies xDS resource CRs (`Listener`, `Cluster`, `RouteConfiguration`,
   `ScopedRouteConfiguration`, `ClusterLoadAssignment`) with a `targetRef`
   pointing to an `EnvoyProxy` CR.
2. The `EnvoyProxyReconciler` watches all resource types, groups them by
   `targetRef`, and builds one xDS snapshot per `EnvoyProxy`.
3. Snapshots are pushed to connected Envoy instances via a single
   [go-control-plane](https://github.com/envoyproxy/go-control-plane) ADS
   stream. Node ID = `{EnvoyProxy.name}.{EnvoyProxy.namespace}`.
4. `EnvoyProxy` status conditions reflect connection state (`Connected`) and
   whether a snapshot has been pushed (`Ready`).

## Prerequisites

- go v1.24+, docker, kubectl, access to a Kubernetes cluster.
- CRDs applied before deploying the operator (see below).

## Build & push image

```bash
cd control-plane
make docker-build docker-push IMG=<registry>/envoy-mesh-control-plane:<tag>
```

## Deploy via Helm

### Apply CRDs first

```bash
kubectl apply -f crds/
```

### From local chart

```bash
helm install control-plane ./helm \
  --namespace envoy-mesh-system --create-namespace \
  --set image.repository=<registry>/envoy-mesh-control-plane \
  --set image.tag=<tag>
```

### From OCI registry (after publishing)

```bash
helm install control-plane \
  oci://ghcr.io/iglin/envoy-mesh/charts/envoy-mesh-control-plane \
  --version <version> \
  --namespace envoy-mesh-system --create-namespace
```

## Publish Helm chart

```bash
cd control-plane
make helm-push HELM_REGISTRY=ghcr.io/<org>/envoy-mesh/charts
```

Or push a `control-plane/v<semver>` git tag to let
[GitHub Actions](../.github/workflows/publish-control-plane.yml) build and
publish both the image and chart automatically:

```bash
git tag control-plane/v1.0.0
git push origin control-plane/v1.0.0
```

## Run locally

```bash
cd control-plane
go run ./cmd/main.go \
  --xds-bind-address=:18000 \
  --health-probe-bind-address=:8081
```

## Key values

| Value | Default | Description |
|---|---|---|
| `image.repository` | `envoy-mesh-control-plane` | Image repository |
| `image.tag` | `latest` | Image tag |
| `replicaCount` | `1` | Number of operator pods |
| `xds.port` | `18000` | gRPC xDS server port |
| `health.port` | `8081` | Health/readiness probe port |
| `leaderElection.enabled` | `true` | Enable leader election |
| `service.type` | `ClusterIP` | Service type for the xDS port |
| `service.port` | `18000` | Service port |
