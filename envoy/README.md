# envoy-mesh — Envoy proxy

Dockerfile and Helm chart for an Envoy proxy managed by the
[envoy-mesh control plane](../control-plane) via xDS/ADS.

Envoy connects to the control plane on gRPC port 18000. The bootstrap config
sets `node.id = <name>.<namespace>`, which must match an `EnvoyProxy` CR in
the same namespace.

## Build & push image

```bash
cd envoy
make docker-build-push IMG=<registry>/envoy-mesh-envoy:<tag>
```

## Deploy

```bash
# From local chart
helm install envoy ./helm -n default --create-namespace \
  --set image.repository=<registry>/envoy-mesh-envoy \
  --set image.tag=0.0.5

# From OCI registry
helm install envoy oci://ghcr.io/iglin/envoy-mesh/charts/envoy \
  --version 0.0.5 -n default --create-namespace
```

Use `--set name=<value>` to rename the proxy (changes Deployment, Service,
ConfigMap, and `node.id` in one go). Must match the `EnvoyProxy` CR name.

## Publish Helm chart

```bash
make helm-push HELM_REGISTRY=ghcr.io/<org>/envoy-mesh/charts
```

Or push a `envoy/v<semver>` git tag to let
[GitHub Actions](../.github/workflows/publish-envoy.yml) build and publish
both the image and chart automatically. Current published version: `0.0.5`.

## Key values

| Value | Default | Description |
| --- | --- | --- |
| `name` | `envoy` | Name for all resources and `node.id` |
| `image.repository` | `envoy-mesh-envoy` | Image repository |
| `image.tag` | `latest` | Image tag |
| `replicaCount` | `1` | Number of Envoy pods |
| `service.type` | `ClusterIP` | Kubernetes Service type |
| `service.port` | `8080` | Service port |
| `xds.controlPlaneHost` | `envoy-mesh-control-plane` | Control-plane hostname |
| `xds.controlPlanePort` | `18000` | Control-plane xDS gRPC port |
| `admin.port` | `9901` | Envoy admin port (pod-internal) |
