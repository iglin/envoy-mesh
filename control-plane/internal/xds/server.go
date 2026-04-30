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

// Package xds wraps go-control-plane to serve xDS configuration to Envoy proxies.
package xds

import (
	"context"
	"fmt"
	"net"
	"sync"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	discoveryv3 "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	golog "github.com/envoyproxy/go-control-plane/pkg/log"
	cachev3 "github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	serverv3 "github.com/envoyproxy/go-control-plane/pkg/server/v3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
	ctrl "sigs.k8s.io/controller-runtime"
)

var log = ctrl.Log.WithName("xds")

// cacheLogger adapts controller-runtime's logr to go-control-plane's Logger interface.
type cacheLogger struct{}

func (cacheLogger) Debugf(format string, args ...interface{}) {
	log.V(2).Info(fmt.Sprintf(format, args...))
}
func (cacheLogger) Infof(format string, args ...interface{}) {
	log.V(1).Info(fmt.Sprintf(format, args...))
}
func (cacheLogger) Warnf(format string, args ...interface{}) {
	log.Info(fmt.Sprintf(format, args...))
}
func (cacheLogger) Errorf(format string, args ...interface{}) {
	log.Error(fmt.Errorf(format, args...), "xDS cache error")
}

var _ golog.Logger = cacheLogger{}

// Server wraps the go-control-plane xDS gRPC server and snapshot cache.
type Server struct {
	// Cache is the xDS snapshot cache. Controllers write snapshots here.
	Cache     cachev3.SnapshotCache
	connected sync.Map // nodeID string → struct{}{}
}

// NewServer creates a new xDS Server with an ADS-enabled snapshot cache.
func NewServer() *Server {
	s := &Server{}
	s.Cache = cachev3.NewSnapshotCache(
		false,        // ads=false: Envoy can request individual resource types
		cachev3.IDHash{},
		cacheLogger{},
	)
	return s
}

// IsConnected reports whether an Envoy proxy with the given nodeID is actively streaming.
func (s *Server) IsConnected(nodeID string) bool {
	_, ok := s.connected.Load(nodeID)
	return ok
}

// Start runs the gRPC xDS server on addr. It blocks until ctx is cancelled.
func (s *Server) Start(ctx context.Context, addr string) error {
	cb := serverv3.CallbackFuncs{
		StreamRequestFunc: func(id int64, req *discoveryv3.DiscoveryRequest) error {
			if req.Node != nil && req.Node.Id != "" {
				if _, loaded := s.connected.LoadOrStore(req.Node.Id, struct{}{}); !loaded {
					log.Info("Envoy connected", "nodeID", req.Node.Id, "streamID", id)
				}
			}
			return nil
		},
		StreamClosedFunc: func(id int64, node *corev3.Node) {
			if node != nil && node.Id != "" {
				s.connected.Delete(node.Id)
				log.Info("Envoy disconnected", "nodeID", node.Id, "streamID", id)
			}
		},
	}

	xdsSrv := serverv3.NewServer(ctx, s.Cache, cb)

	grpcSrv := grpc.NewServer(
		grpc.KeepaliveParams(keepalive.ServerParameters{}),
		grpc.MaxConcurrentStreams(1000),
	)
	discoveryv3.RegisterAggregatedDiscoveryServiceServer(grpcSrv, xdsSrv)

	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("xDS listen on %s: %w", addr, err)
	}
	log.Info("xDS server listening", "addr", addr)

	errCh := make(chan error, 1)
	go func() { errCh <- grpcSrv.Serve(lis) }()

	select {
	case <-ctx.Done():
		grpcSrv.GracefulStop()
		return nil
	case err := <-errCh:
		return err
	}
}
