package loadbalancer

import (
	"context"
	"fmt"
	"iter"
	"math/rand/v2"
	"net/http"
	"sync/atomic"

	"github.com/computer-technology-team/distributed-kvstore/api/common"
	"github.com/computer-technology-team/distributed-kvstore/api/controller"
	kvstoreAPI "github.com/computer-technology-team/distributed-kvstore/api/kvstore"
	"github.com/computer-technology-team/distributed-kvstore/api/loadbalancer"
	"github.com/samber/lo"
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

func replicaPredicate(partition *common.Partition) func(common.Replica) bool {
	return func(replica common.Replica) bool {
		return replica.Id == partition.MasterReplicaId
	}
}

func balanceReplicaIter(replicas []common.Replica) iter.Seq[common.Replica] {
	selectedPartitionIdx := rand.IntN(len(replicas))
	return func(yield func(common.Replica) bool) {
		for i := range len(replicas) {
			idx := (i + selectedPartitionIdx) % len(replicas)
			if !yield(replicas[idx]) {
				return
			}
		}
	}
}

func filterHealthyReplica(replicas []common.Replica) []common.Replica {
	return lo.Filter(replicas, func(replica common.Replica, _ int) bool {
		return replica.Status == common.Healthy
	})
}
