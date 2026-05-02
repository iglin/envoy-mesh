package xds

// Blank imports register Envoy extension proto types in Go's global protobuf
// registry so that protojson.Unmarshal can resolve google.protobuf.Any fields
// that reference these types. Add an import here for every extension type that
// appears in any xDS resource CR managed by this control-plane.
import (
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/router/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/stateful_session/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/http/stateful_session/cookie/v3"
)
