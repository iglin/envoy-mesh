# envoy-mesh

Kubernetes Service Mesh built on [Envoy Proxy](https://www.envoyproxy.io/).
Envoy configuration types (`Listener`, `Cluster`, `RouteConfiguration`,
`ScopedRouteConfiguration`, `ClusterLoadAssignment`) are exposed as native
Kubernetes CRDs. An operator reconciles those CRs into live Envoy xDS
snapshots served over gRPC/ADS.

```
EnvoyProxy CR  +  xDS resource CRs
        │
        ▼
  control-plane (operator)
        │  gRPC ADS :18000
        ▼
  Envoy proxy pod  ──►  upstream services
```

## Repository layout

| Path | Purpose |
|---|---|
| `proto/` | Vendored Envoy API proto definitions (source of truth for CRD schemas) |
| `crds/` | Generated CRD manifests — apply to cluster before deploying operator |
| `crd-gen/` | Go CLI that generates `crds/` from `proto/` |
| `control-plane/` | Kubernetes operator + Helm chart |
| `envoy/` | Envoy proxy Dockerfile + Helm chart |
| `.github/workflows/` | CI/CD — publish image and Helm chart on version tags |

## Prerequisites

| Tool | Purpose |
|---|---|
| Go 1.24+ | Build the operator and crd-gen |
| Docker | Build container images |
| kubectl | Interact with the cluster |
| Helm 3 | Deploy operator and Envoy proxy |
| controller-gen | Regenerate DeepCopy methods |

## Development

### 1. Update Envoy API protos

```bash
./proto/download.sh v1.37.2
```

Always commit `proto/` and `crds/` together.

### 2. Regenerate CRDs

```bash
cd crd-gen
go run . \
  -m envoy.config.listener.v3.Listener \
  -m envoy.config.cluster.v3.Cluster \
  -m envoy.config.route.v3.RouteConfiguration \
  -m envoy.config.route.v3.ScopedRouteConfiguration \
  -m envoy.config.endpoint.v3.ClusterLoadAssignment
```

### 3. Modify operator types

After editing any file in `control-plane/api/v1alpha1/`:

```bash
cd control-plane
controller-gen object:headerFile="hack/boilerplate.go.txt" paths="./api/..."
```

### 4. Run the operator locally

```bash
cd control-plane
go run ./cmd/main.go \
  --xds-bind-address=:18000 \
  --health-probe-bind-address=:8081
```

### 5. Run tests

```bash
cd control-plane
make test        # unit tests
make test-e2e    # end-to-end tests (requires Kind)
```

## Build

```bash
# Operator image
cd control-plane
make docker-build IMG=<registry>/envoy-mesh-control-plane:<tag>

# Envoy proxy image
cd envoy
make docker-build IMG=<registry>/envoy-mesh-envoy:<tag>
```

## Publish

### Push images

```bash
cd control-plane && make docker-push IMG=<registry>/envoy-mesh-control-plane:<tag>
cd envoy         && make docker-push IMG=<registry>/envoy-mesh-envoy:<tag>
```

### Push Helm charts (OCI)

```bash
cd control-plane && make helm-push HELM_REGISTRY=ghcr.io/<org>/envoy-mesh/charts
cd envoy         && make helm-push HELM_REGISTRY=ghcr.io/<org>/envoy-mesh/charts
```

### Release via GitHub Actions

Push a version tag — the corresponding workflow builds and publishes both the
image and the Helm chart to `ghcr.io` automatically:

```bash
git tag control-plane/v1.0.0 && git push origin control-plane/v1.0.0
git tag envoy/v1.0.0         && git push origin envoy/v1.0.0
```

| Tag pattern | Workflow |
|---|---|
| `control-plane/v*` | `.github/workflows/publish-control-plane.yml` |
| `envoy/v*` | `.github/workflows/publish-envoy.yml` |

## Deployment

### 1. Apply CRDs

CRDs must be applied before the operator is deployed, and re-applied whenever
they change:

```bash
kubectl apply -f crds/
```

### 2. Deploy the operator

```bash
helm install control-plane control-plane/helm \
  --namespace envoy-mesh-system --create-namespace \
  --set image.repository=<registry>/envoy-mesh-control-plane \
  --set image.tag=<tag>
```

Or from the OCI registry:

```bash
helm install control-plane \
  oci://ghcr.io/iglin/envoy-mesh/charts/envoy-mesh-control-plane \
  --version <version> \
  --namespace envoy-mesh-system --create-namespace
```

### 3. Create an EnvoyProxy CR

One CR per Envoy instance, in the namespace where the proxy will run:

```yaml
apiVersion: mesh.iglin.io/v1alpha1
kind: EnvoyProxy
metadata:
  name: edge-proxy
  namespace: default
```

### 4. Deploy an Envoy proxy

The `name` value must match the `EnvoyProxy` CR name:

```bash
helm install edge-proxy envoy/helm \
  --namespace default \
  --set name=edge-proxy \
  --set image.repository=<registry>/envoy-mesh-envoy \
  --set image.tag=<tag> \
  --set xds.controlPlaneHost=control-plane-envoy-mesh-control-plane.envoy-mesh-system
```

Or from the OCI registry:

```bash
helm install edge-proxy \
  oci://ghcr.io/iglin/envoy-mesh/charts/envoy \
  --version <version> \
  --namespace default \
  --set name=edge-proxy \
  --set xds.controlPlaneHost=control-plane-envoy-mesh-control-plane.envoy-mesh-system
```

### 5. Apply xDS resource CRs

```yaml
apiVersion: mesh.iglin.io/v1alpha1
kind: Listener
metadata:
  name: my-listener
  namespace: default
spec:
  name: listener_0
  address:
    socket_address: { address: 0.0.0.0, port_value: 8080 }
  # ...
targetRef:
  name: edge-proxy
```

The operator picks up the CR, rebuilds the xDS snapshot, and pushes it to the
connected Envoy proxy — no proxy restart required.

## Further reading

- [control-plane/README.md](control-plane/README.md) — operator internals, Helm values
- [envoy/README.md](envoy/README.md) — Envoy image and Helm values
- [AGENTS.md](AGENTS.md) — codebase conventions and rules for AI agents
