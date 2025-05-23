package loadbalancer

import (
	"context"
	"log/slog"
	"math/rand/v2"
	"net/http"

	"github.com/computer-technology-team/distributed-kvstore/api/common"
	"github.com/computer-technology-team/distributed-kvstore/api/database"
	kvstoreAPI "github.com/computer-technology-team/distributed-kvstore/api/kvstore"
	"github.com/samber/lo"
)

// GetValue implements LoadBalancer.
func (s *server) GetValue(ctx context.Context,
	request kvstoreAPI.GetValueRequestObject) (kvstoreAPI.GetValueResponseObject, error) {

	partition, err := s.statePtr.Load().GetPartition(request.Key)
	if err != nil {
		slog.ErrorContext(ctx, "could not get partition", "method", "set", "error", err)
		return kvstoreAPI.GetValuedefaultJSONResponse{
			Body: common.ErrorResponse{
				Error: "could not get partition",
			},
			StatusCode: http.StatusInternalServerError,
		}, nil
	}

	// Find nodes that are replicas for this partition
	replicaNodes := lo.Filter(s.statePtr.Load().Nodes, func(node common.Node, _ int) bool {
		if node.Partitions == nil {
			return false
		}
		_, hasPartition := node.Partitions[partition.Id]
		return hasPartition
	})

	// Filter healthy replicas
	healthyReplicas := lo.Filter(replicaNodes, func(node common.Node, _ int) bool {
		partitionRole, exists := node.Partitions[partition.Id]
		return exists && partitionRole.Status == common.Healthy
	})

	if len(healthyReplicas) == 0 {
		return kvstoreAPI.GetValuedefaultJSONResponse{
			Body: common.ErrorResponse{
				Error: "no healthy replica is available",
			},
			StatusCode: http.StatusServiceUnavailable,
		}, nil
	}

	selectedReplicaIdx := rand.IntN(len(healthyReplicas))
	replica := healthyReplicas[selectedReplicaIdx]

	client, err := database.NewClientWithResponses("http://"+replica.Address,
		database.WithHTTPClient(s.httpClient))
	if err != nil {
		slog.ErrorContext(ctx, "could not create client", "method", "get", "error", err)
		return kvstoreAPI.GetValuedefaultJSONResponse{
			Body: common.ErrorResponse{
				Error: "could not create client",
			},
			StatusCode: http.StatusInternalServerError,
		}, nil
	}

	resp, err := client.GetValueFromPartitionWithResponse(ctx, partition.Id, request.Key)
	if err != nil {
		slog.ErrorContext(ctx, "error getting value from replica",
			"method", "get", "error", err)
		return kvstoreAPI.GetValuedefaultJSONResponse{
			Body: common.ErrorResponse{
				Error: "error getting value from replica",
			},
			StatusCode: http.StatusInternalServerError,
		}, nil
	}

	switch {
	case resp.JSON200 != nil:
		return kvstoreAPI.GetValue200JSONResponse(*resp.JSON200), nil
	case resp.JSON404 != nil:
		return kvstoreAPI.GetValue404JSONResponse(*resp.JSON404), nil
	case resp.JSON500 != nil:
		slog.ErrorContext(ctx, "unexpected error in getting value from replica",
			"method", "get", "error", resp.JSON500.Error, "replica_id", replica.Id)
	default:
		slog.ErrorContext(ctx, "unexpected error in getting value from replica",
			"method", "get", "replica_id", replica.Id)
	}

	return kvstoreAPI.GetValuedefaultJSONResponse{
		Body: common.ErrorResponse{
			Error: "unexpected error in retrieving value",
		},
		StatusCode: http.StatusInternalServerError,
	}, nil
}
