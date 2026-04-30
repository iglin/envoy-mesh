# envoy-mesh

Kubernetes Service Mesh built on [Envoy Proxy](https://www.envoyproxy.io/). Envoy configuration types are exposed as native Kubernetes CRDs; an operator reconciles those CRs into live Envoy xDS config.

## Structure

```
envoy-mesh/
├── proto/                    # Envoy protobuf definitions — source of truth for all config types
│   └── download.sh           # downloads proto files from a given envoyproxy/envoy git tag
├── crds/                     # Generated CRD manifests (YAML) — do not edit by hand
├── crd-gen/                  # Go CLI that generates crds/ from proto/
├── envoy/                    # Envoy proxy image and Helm chart
│   ├── Dockerfile            # wraps envoyproxy/envoy; config mounted from ConfigMap
│   ├── Makefile              # docker-build-push, helm-package, helm-push
│   └── helm/                 # Helm chart: Deployment + Service + ConfigMap (bootstrap config)
│       ├── Chart.yaml
│       ├── values.yaml       # name, image, xds.controlPlaneHost/Port, service.port, admin.port
│       └── templates/
│           ├── configmap.yaml    # Envoy bootstrap config — node.id = <name>.<namespace>
│           ├── deployment.yaml
│           └── service.yaml
├── control-plane/            # Kubernetes operator (kubebuilder v4, go module: github.com/iglin/envoy-mesh/control-plane)
│   ├── api/v1alpha1/         # Go types for all CRDs (group: mesh.envoy.io)
│   │   ├── envoyproxy_types.go
│   │   ├── {listener,cluster,route*,clusterloadassignment}_types.go
│   │   └── zz_generated.deepcopy.go
│   ├── internal/
│   │   ├── controller/
│   │   │   └── envoyproxy_controller.go  # single reconciler, watches all xDS resource types
│   │   └── xds/
│   │       ├── server.go     # go-control-plane gRPC xDS server + connection tracking
│   │       └── snapshot.go   # protojson → Envoy proto → xDS snapshot
│   ├── cmd/main.go           # starts controller-runtime manager + xDS gRPC server
│   └── helm/                 # Helm chart: Deployment + Service + RBAC
│       ├── Chart.yaml
│       ├── values.yaml       # image, xds.port, health.port, leaderElection, service, resources
│       └── templates/
│           ├── deployment.yaml
│           ├── service.yaml          # ClusterIP on port 18000 (xds-grpc)
│           ├── serviceaccount.yaml
│           ├── clusterrole.yaml      # mesh.envoy.io RBAC from kubebuilder markers
│           ├── clusterrolebinding.yaml
│           ├── role.yaml             # leader-election (configmaps/leases/events)
│           └── rolebinding.yaml
└── .github/workflows/
    ├── publish-control-plane.yml  # triggered by control-plane/v* tags
    └── publish-envoy.yml          # triggered by envoy/v* tags
```

## CRD model

There are two categories of CRDs:

**`EnvoyProxy`** (`crds/envoyproxies.mesh.envoy.io.yaml`) — hand-authored, represents one Envoy proxy instance managed by the control-plane. Spec is intentionally empty; the control-plane derives xDS node identifiers from CR metadata:
- `node.id = {name}.{namespace}`
- `node.cluster = {name}.{namespace}`

Status conditions: `Connected` (Envoy is streaming xDS), `Ready` (first snapshot pushed).

**xDS resource CRDs** (`Listener`, `Cluster`, `RouteConfiguration`, `ScopedRouteConfiguration`, `ClusterLoadAssignment`) — generated from Envoy proto. Each carries a required `targetRef` field pointing to an `EnvoyProxy` CR:

```yaml
targetRef:
  name: edge-proxy       # EnvoyProxy name (required)
  namespace: default     # EnvoyProxy namespace (optional, defaults to this CR's namespace)
spec:
  # ... Envoy config fields ...
```

The control-plane groups xDS resource CRs by `targetRef` and builds one xDS snapshot per `EnvoyProxy`.

## How it works

```
User CR applied → EnvoyProxyReconciler triggered (via Watches on all xDS resource types)
  → collect all xDS CRs with matching targetRef
  → protojson.Unmarshal each CR spec → typed Envoy proto
  → cache.NewSnapshot(version) → SnapshotCache.SetSnapshot(nodeID)
  → Envoy proxies pull updated config via gRPC ADS (:18000)
  → EnvoyProxy status updated (Connected / Ready conditions)
```

### control-plane internals

- **Single reconciler** (`EnvoyProxyReconciler`) owns the full reconcile loop. It watches `EnvoyProxy` directly and all five xDS resource types via `handler.EnqueueRequestsFromMapFunc`, mapping each CR's `targetRef` back to its owning `EnvoyProxy`.
- **Spec storage**: xDS resource CR specs are stored as `runtime.RawExtension` (raw JSON). At reconcile time `protojson.Unmarshal` (with `DiscardUnknown: true`) converts them to typed Envoy proto messages.
- **Snapshot versioning**: first 16 hex chars of SHA-256 over all `resourceVersion` strings — unchanged configs never push a new snapshot.
- **xDS server**: a single `go-control-plane` gRPC server per process. `CallbackFuncs.StreamRequestFunc` marks a node connected on first request; `StreamClosedFunc` unmarks it. Connection state is reflected in the `Connected` status condition.

### Envoy bootstrap config

The `envoy/helm` ConfigMap renders `node.id` and `node.cluster` as
`{{ .Values.name }}.{{ .Release.Namespace }}` at Helm install time. This must
match the `EnvoyProxy` CR name and namespace. The static `xds_cluster` points
at `{{ .Values.xds.controlPlaneHost }}:{{ .Values.xds.controlPlanePort }}`
(default `envoy-mesh-control-plane:18000`) using HTTP/2 for gRPC.

## Workflows

### Update proto sources
```bash
./proto/download.sh v1.37.2   # fetches envoy API protos, strips Bazel BUILD files
```

### Regenerate CRDs
```bash
cd crd-gen
go run . \
  -m envoy.config.listener.v3.Listener \
  -m envoy.config.cluster.v3.Cluster \
  -m envoy.config.route.v3.RouteConfiguration \
  -m envoy.config.route.v3.ScopedRouteConfiguration \
  -m envoy.config.endpoint.v3.ClusterLoadAssignment
```
`envoy.config.bootstrap.v3.Bootstrap` is permanently blocked — Bootstrap is proxy startup config, not an xDS resource.

Always commit `proto/` and `crds/` together. Apply updated CRDs to the cluster before deploying a new operator version.

### Regenerate control-plane DeepCopy methods
Run this after modifying any type in `control-plane/api/`:
```bash
cd control-plane
controller-gen object:headerFile="hack/boilerplate.go.txt" paths="./api/..."
```

### Run the operator locally
```bash
cd control-plane
go run ./cmd/main.go \
  --xds-bind-address=:18000 \
  --health-probe-bind-address=:8081
```

### Build & push images
```bash
# Operator
cd control-plane
make docker-build docker-push IMG=<registry>/envoy-mesh-control-plane:<tag>

# Envoy
cd envoy
make docker-build-push IMG=<registry>/envoy-mesh-envoy:<tag>
```

### Deploy to cluster
```bash
# 1. Apply CRDs (always before a new operator version)
kubectl apply -f crds/

# 2. Deploy operator via Helm
helm install control-plane control-plane/helm \
  --namespace envoy-mesh-system --create-namespace \
  --set image.repository=<registry>/envoy-mesh-control-plane \
  --set image.tag=<tag>

# 3. Deploy Envoy proxy via Helm (one release per EnvoyProxy CR)
helm install envoy envoy/helm \
  --namespace default --create-namespace \
  --set image.repository=<registry>/envoy-mesh-envoy \
  --set image.tag=<tag>
```

### Publish Helm charts
```bash
cd control-plane && make helm-push HELM_REGISTRY=ghcr.io/<org>/envoy-mesh/charts
cd envoy         && make helm-push HELM_REGISTRY=ghcr.io/<org>/envoy-mesh/charts
```

Or push a versioned tag to trigger GitHub Actions:
```bash
git tag control-plane/v1.0.0 && git push origin control-plane/v1.0.0
git tag envoy/v1.0.0         && git push origin envoy/v1.0.0
```

## Rules

- `crds/` xDS CRDs are fully generated — always edit `proto/` and re-run `crd-gen`.
- `crds/envoyproxies.mesh.envoy.io.yaml` is hand-authored — do not overwrite with crd-gen.
- Every xDS resource CR must have a `targetRef` pointing to an existing `EnvoyProxy`.
- CRDs must be applied to the cluster before deploying a new operator version.
- `Bootstrap` must never become a CRD.
- After modifying `control-plane/api/` types, always re-run `controller-gen` to regenerate `zz_generated.deepcopy.go`.
- The control-plane Go module is `github.com/iglin/envoy-mesh/control-plane` (separate from any root module). Run `go` commands from inside `control-plane/`.
- Do not add a `Status` subresource or status fields to xDS resource CRs (Listener, Cluster, etc.) — status lives only on `EnvoyProxy`.
- `envoy/helm` and `control-plane/helm` are hand-authored — do not overwrite with kubebuilder or crd-gen tooling.
- The `name` value in `envoy/helm/values.yaml` must match the `EnvoyProxy` CR name deployed in the same namespace.

## Key libraries

| Layer | Library |
|-------|---------|
| Operator scaffolding | [kubebuilder v4](https://github.com/kubernetes-sigs/kubebuilder) |
| Operator runtime | [controller-runtime v0.23](https://github.com/kubernetes-sigs/controller-runtime) |
| xDS server + cache | [go-control-plane v0.13](https://github.com/envoyproxy/go-control-plane) |
| Envoy API proto types | [go-control-plane/envoy](https://github.com/envoyproxy/go-control-plane/tree/main/envoy) |
| Proto → JSON unmarshalling | [google.golang.org/protobuf/encoding/protojson](https://pkg.go.dev/google.golang.org/protobuf/encoding/protojson) |
| Proto parser (crd-gen) | [emicklei/proto](https://github.com/emicklei/proto) |
| Proto source | [envoyproxy/envoy API](https://github.com/envoyproxy/envoy/tree/main/api) |
