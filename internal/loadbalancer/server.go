package loadbalancer

import (
	"context"

	kvstoreAPI "github.com/computer-technology-team/distributed-kvstore/api/kvstore"
	"github.com/computer-technology-team/distributed-kvstore/api/loadbalancer"
)

type LoadBalancer interface {
	kvstoreAPI.StrictServerInterface
	loadbalancer.StrictServerInterface
}

type server struct{}

// GetPing implements LoadBalancer.
func (s *server) GetPing(ctx context.Context, request loadbalancer.GetPingRequestObject) (loadbalancer.GetPingResponseObject, error) {
	panic("unimplemented")
}

// GetState implements LoadBalancer.
func (s *server) GetState(ctx context.Context, request loadbalancer.GetStateRequestObject) (loadbalancer.GetStateResponseObject, error) {
	panic("unimplemented")
}

// SetState implements LoadBalancer.
func (s *server) SetState(ctx context.Context, request loadbalancer.SetStateRequestObject) (loadbalancer.SetStateResponseObject, error) {
	panic("unimplemented")
}

// DeleteKey implements LoadBalancer.
func (s *server) DeleteKey(ctx context.Context,
	request kvstoreAPI.DeleteKeyRequestObject) (kvstoreAPI.DeleteKeyResponseObject, error) {
	panic("unimplemented")
}

// GetValue implements LoadBalancer.
func (s *server) GetValue(ctx context.Context,
	request kvstoreAPI.GetValueRequestObject) (kvstoreAPI.GetValueResponseObject, error) {
	panic("unimplemented")
}

// PingServer implements LoadBalancer.
func (s *server) PingServer(ctx context.Context,
	request kvstoreAPI.PingServerRequestObject) (kvstoreAPI.PingServerResponseObject, error) {
	return kvstoreAPI.PingServer200JSONResponse{Ping: "Pong"}, nil
}

// SetValue implements LoadBalancer.
func (s *server) SetValue(ctx context.Context,
	request kvstoreAPI.SetValueRequestObject) (kvstoreAPI.SetValueResponseObject, error) {
	panic("unimplemented")
}

func NewServer() LoadBalancer {
	return &server{}
}
