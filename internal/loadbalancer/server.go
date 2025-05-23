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

// This function is no longer needed as we check nodes by ID directly
// We'll keep it for now but it's not used
func replicaPredicate(id openapi_types.UUID) func(common.Node) bool {
	return func(node common.Node) bool {
		return node.Id == id
	}
}

func balanceReplicaIter(replicaIds []openapi_types.UUID, nodes []common.Node, partitionId string) iter.Seq[common.Node] {
	selectedReplicaIdx := rand.IntN(len(replicaIds))
	return func(yield func(common.Node) bool) {
		for i := range len(replicaIds) {
			replicaIdx := (i + selectedReplicaIdx) % len(replicaIds)
			replicaId := replicaIds[replicaIdx]

			// Find the node with this replica ID
			for _, node := range nodes {
				if node.Id == replicaId && node.Partitions != nil {
					// Check if this node has the partition and it's healthy
					if partitionRole, exists := node.Partitions[partitionId]; exists && partitionRole.Status == common.Healthy {
						if !yield(node) {
							return
						}
						break
					}
				}
			}
		}
	}
}

func getNodesForReplicaIds(replicaIds []openapi_types.UUID, nodes []common.Node) []common.Node {
	result := make([]common.Node, 0, len(replicaIds))

	for _, replicaId := range replicaIds {
		for _, node := range nodes {
			if node.Id == replicaId {
				result = append(result, node)
				break
			}
		}
	}

	return result
}

// This function is no longer needed as we check health status per partition
// We'll keep it for now but it's not used
func filterHealthyNodes(nodes []common.Node) []common.Node {
	return lo.Filter(nodes, func(node common.Node, _ int) bool {
		// A node is considered healthy if it has at least one healthy partition
		if node.Partitions == nil {
			return false
		}

		for _, partitionRole := range node.Partitions {
			if partitionRole.Status == common.Healthy {
				return true
			}
		}
		return false
	})
}
