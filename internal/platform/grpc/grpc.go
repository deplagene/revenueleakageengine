// Package grpc contains shared gRPC server and client primitives.
package grpc

import (
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

// NewServer creates the shared gRPC server instance.
func NewServer(options ...grpc.ServerOption) *grpc.Server {
	return grpc.NewServer(options...)
}

// RegisterHealthCheck attaches the standard gRPC health service.
func RegisterHealthCheck(server *grpc.Server) {
	healthpb.RegisterHealthServer(server, health.NewServer())
}
