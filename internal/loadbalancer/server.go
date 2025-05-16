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
	openapi_types "github.com/oapi-codegen/runtime/types"
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

func replicaPredicate(replicaId openapi_types.UUID) func(common.Node) bool {
	return func(node common.Node) bool {
		return node.ReplicaID != nil && *node.ReplicaID == replicaId
	}
}

func balanceReplicaIter(replicaIds []openapi_types.UUID, nodes []common.Node) iter.Seq[common.Node] {
	selectedReplicaIdx := rand.IntN(len(replicaIds))
	return func(yield func(common.Node) bool) {
		for i := range len(replicaIds) {
			replicaIdx := (i + selectedReplicaIdx) % len(replicaIds)
			replicaId := replicaIds[replicaIdx]
			
			// Find the node with this replica ID
			for _, node := range nodes {
				if node.ReplicaID != nil && *node.ReplicaID == replicaId && node.Status == common.Healthy {
					if !yield(node) {
						return
					}
					break
				}
			}
		}
	}
}

func getNodesForReplicaIds(replicaIds []openapi_types.UUID, nodes []common.Node) []common.Node {
	result := make([]common.Node, 0, len(replicaIds))
	
	for _, replicaId := range replicaIds {
		for _, node := range nodes {
			if node.ReplicaID != nil && *node.ReplicaID == replicaId {
				result = append(result, node)
				break
			}
		}
	}
	
	return result
}

func filterHealthyNodes(nodes []common.Node) []common.Node {
	return lo.Filter(nodes, func(node common.Node, _ int) bool {
		return node.Status == common.Healthy
	})
}
