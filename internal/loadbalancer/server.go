package loadbalancer

import (
	"context"
	"fmt"
	"net/http"
	"sync/atomic"

	"github.com/computer-technology-team/distributed-kvstore/api/common"
	"github.com/computer-technology-team/distributed-kvstore/api/controller"
	kvstoreAPI "github.com/computer-technology-team/distributed-kvstore/api/kvstore"
	"github.com/computer-technology-team/distributed-kvstore/api/loadbalancer"
)

type LoadBalancer interface {
	kvstoreAPI.StrictServerInterface
	loadbalancer.StrictServerInterface
}

type server struct {
	statePtr   atomic.Pointer[common.State]
	httpClient *http.Client
}

// SetState implements LoadBalancer.
func (s *server) SetState(ctx context.Context, request loadbalancer.SetStateRequestObject) (loadbalancer.SetStateResponseObject, error) {
	s.statePtr.Store(request.Body)
	return loadbalancer.SetState200JSONResponse{}, nil
}

// PingServer implements LoadBalancer.
func (s *server) PingServer(ctx context.Context,
	request kvstoreAPI.PingServerRequestObject) (kvstoreAPI.PingServerResponseObject, error) {
	return kvstoreAPI.PingServer200JSONResponse{Ping: "Pong"}, nil
}

func NewServer(ctx context.Context, controllerClient controller.ClientWithResponsesInterface) (LoadBalancer, error) {
	resp, err := controllerClient.GetStateWithResponse(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get state from controller: %w", err)
	}

	srv := &server{
		httpClient: http.DefaultClient,
	}
	srv.statePtr.Store(resp.JSON200)
	return srv, nil
}
