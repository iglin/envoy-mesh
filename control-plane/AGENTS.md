# control-plane — AI Agent Guide

Go module: `github.com/iglin/envoy-mesh/control-plane`. Run all `go` / `make` commands from this directory.

## Key files

| File | Purpose |
|------|---------|
| `cmd/main.go` | Binary entry point — starts controller-runtime manager + xDS gRPC server |
| `api/v1alpha1/*_types.go` | CRD Go types (group `mesh.iglin.io`) |
| `api/v1alpha1/zz_generated.deepcopy.go` | Auto-generated — do not edit |
| `internal/controller/envoyproxy_controller.go` | Single reconciler, watches all xDS types + EndpointSlices |
| `internal/xds/server.go` | go-control-plane gRPC xDS server + connection tracking |
| `internal/xds/snapshot.go` | protojson → Envoy proto → xDS snapshot builder |
| `internal/xds/eds.go` | EDS helpers: `ClusterName`, `SynthesizeCLA`, `resolvePort` |
| `helm/` | Hand-authored Helm chart — do not overwrite with kubebuilder tooling |

## After editing `api/v1alpha1/`

```bash
# Regenerate DeepCopy methods
controller-gen object:headerFile="hack/boilerplate.go.txt" paths="./api/..."

# Verify
go build ./...
go vet ./...
```

## After editing `internal/`

```bash
go build ./...
go vet ./...
```

## Run locally

```bash
go run ./cmd/main.go \
  --xds-bind-address=:18000 \
  --health-probe-bind-address=:8081
```

## Critical rules

- `zz_generated.deepcopy.go` is auto-generated — always re-run `controller-gen` after type changes.
- `helm/` is hand-authored — do not regenerate with kubebuilder.
- Status fields belong only on `EnvoyProxy`, not on xDS resource CRs.
- `kubernetesServiceRef` on `Cluster` is a Kubernetes-only field; inject its CRD schema via `crd-gen/cmd/root.go` (`kindExtraProps`), not via proto or kubebuilder markers.
- RBAC for `discovery.k8s.io/endpointslices` must be present in `helm/templates/clusterrole.yaml` (get;list;watch).
- EDS auto-synthesis (`eds.go`) only runs when `kubernetesServiceRef` is set and no manual `ClusterLoadAssignment` CR exists for that cluster name.
