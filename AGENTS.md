# envoy-mesh

Kubernetes Service Mesh built on [Envoy Proxy](https://www.envoyproxy.io/). Envoy configuration types are exposed as native Kubernetes CRDs; an operator reconciles those CRs into live Envoy xDS config.

## Structure

```
envoy-mesh/
├── proto/          # Envoy protobuf definitions — source of truth for all config types
│   └── download.sh # downloads proto files from a given envoyproxy/envoy git tag
├── crds/           # Generated CRD manifests (YAML) — do not edit by hand
└── crd-gen/        # Go CLI that generates crds/ from proto/
└── control-plane/  # Kubernetes operator: watches CRs, pushes xDS snapshots to Envoy
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
User CR applied → controller reconciles → translation layer builds Envoy protobuf
→ xDS snapshot cache updated → Envoy proxies pull config via gRPC (ADS)
```

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

## Rules

- `crds/` xDS CRDs are fully generated — always edit `proto/` and re-run `crd-gen`.
- `crds/envoyproxies.mesh.envoy.io.yaml` is hand-authored — do not overwrite with crd-gen.
- Every xDS resource CR must have a `targetRef` pointing to an existing `EnvoyProxy`.
- CRDs must be applied to the cluster before deploying a new operator version.
- `Bootstrap` must never become a CRD.

## Key libraries

| Layer | Library |
|-------|---------|
| Operator scaffolding | [kubebuilder](https://github.com/kubernetes-sigs/kubebuilder) |
| Operator runtime | [controller-runtime](https://github.com/kubernetes-sigs/controller-runtime) |
| xDS server | [go-control-plane](https://github.com/envoyproxy/go-control-plane) |
| Proto parser (crd-gen) | [emicklei/proto](https://github.com/emicklei/proto) |
| Proto source | [envoyproxy/envoy API](https://github.com/envoyproxy/envoy/tree/main/api) |
