/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package xds

import (
	"fmt"

	clusterv3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	endpointv3 "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	listenerv3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	routev3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	cachev3 "github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	cachetypes "github.com/envoyproxy/go-control-plane/pkg/cache/types"
	resourcev3 "github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

var unmarshaler = protojson.UnmarshalOptions{
	DiscardUnknown: true,
	AllowPartial:   true,
}

// BuildSnapshot builds an xDS snapshot from raw JSON specs.
// version must change whenever the configuration changes.
func BuildSnapshot(
	version string,
	listenerSpecs [][]byte,
	clusterSpecs [][]byte,
	routeSpecs [][]byte,
	scopedRouteSpecs [][]byte,
	endpointSpecs [][]byte,
) (*cachev3.Snapshot, error) {
	listeners, err := unmarshalAll(listenerSpecs, func() proto.Message { return &listenerv3.Listener{} })
	if err != nil {
		return nil, fmt.Errorf("listeners: %w", err)
	}
	clusters, err := unmarshalAll(clusterSpecs, func() proto.Message { return &clusterv3.Cluster{} })
	if err != nil {
		return nil, fmt.Errorf("clusters: %w", err)
	}
	routes, err := unmarshalAll(routeSpecs, func() proto.Message { return &routev3.RouteConfiguration{} })
	if err != nil {
		return nil, fmt.Errorf("routes: %w", err)
	}
	scopedRoutes, err := unmarshalAll(scopedRouteSpecs, func() proto.Message { return &routev3.ScopedRouteConfiguration{} })
	if err != nil {
		return nil, fmt.Errorf("scoped routes: %w", err)
	}
	endpoints, err := unmarshalAll(endpointSpecs, func() proto.Message { return &endpointv3.ClusterLoadAssignment{} })
	if err != nil {
		return nil, fmt.Errorf("endpoints: %w", err)
	}

	snap, err := cachev3.NewSnapshot(version, map[resourcev3.Type][]cachetypes.Resource{
		resourcev3.ListenerType:    listeners,
		resourcev3.ClusterType:     clusters,
		resourcev3.RouteType:       routes,
		resourcev3.ScopedRouteType: scopedRoutes,
		resourcev3.EndpointType:    endpoints,
	})
	if err != nil {
		return nil, fmt.Errorf("new snapshot: %w", err)
	}
	return snap, nil
}

func unmarshalAll(raws [][]byte, newMsg func() proto.Message) ([]cachetypes.Resource, error) {
	out := make([]cachetypes.Resource, 0, len(raws))
	for i, raw := range raws {
		if len(raw) == 0 {
			continue
		}
		msg := newMsg()
		if err := unmarshaler.Unmarshal(raw, msg); err != nil {
			return nil, fmt.Errorf("item %d: %w", i, err)
		}
		out = append(out, msg)
	}
	return out, nil
}
