package loadbalancer

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/computer-technology-team/distributed-kvstore/api/common"
	"github.com/computer-technology-team/distributed-kvstore/api/database"
	kvstoreAPI "github.com/computer-technology-team/distributed-kvstore/api/kvstore"
	"github.com/samber/lo"
)

// SetValue implements LoadBalancer.
func (s *server) SetValue(ctx context.Context,
	request kvstoreAPI.SetValueRequestObject) (kvstoreAPI.SetValueResponseObject, error) {
	partition, err := s.statePtr.Load().GetPartition(request.Key)
	if err != nil {
		slog.ErrorContext(ctx, "could not get partition", "method", "set", "error", err)
		return kvstoreAPI.SetValuedefaultJSONResponse{
			Body: common.ErrorResponse{
				Error: "could not get partition",
			},
			StatusCode: http.StatusInternalServerError,
		}, nil
	}

	masterReplica, found := lo.Find(s.statePtr.Load().Nodes, func(node common.Node) bool {
		return node.Id == partition.MasterNodeId
	})

	if !found {
		slog.ErrorContext(ctx, "master node not found", "method", "set",
			"partition_id", partition.Id)
		return kvstoreAPI.SetValuedefaultJSONResponse{
			Body: common.ErrorResponse{
				Error: "master node not found",
			},
			StatusCode: http.StatusInternalServerError,
		}, nil
	}

	// Check if the node has this partition and it's healthy
	if masterReplica.Partitions == nil {
		slog.ErrorContext(ctx, "master node has no partitions", "method", "set",
			"partition_id", partition.Id, "node_id", masterReplica.Id)
		return kvstoreAPI.SetValuedefaultJSONResponse{
			Body: common.ErrorResponse{
				Error: "master node has no partitions",
			},
			StatusCode: http.StatusInternalServerError,
		}, nil
	}

	partitionRole, exists := masterReplica.Partitions[partition.Id]
	if !exists || !partitionRole.IsMaster {
		slog.ErrorContext(ctx, "node is not master for this partition", "method", "set",
			"partition_id", partition.Id, "node_id", masterReplica.Id)
		return kvstoreAPI.SetValuedefaultJSONResponse{
			Body: common.ErrorResponse{
				Error: "node is not master for this partition",
			},
			StatusCode: http.StatusInternalServerError,
		}, nil
	}

	if partitionRole.Status != common.Healthy {
		slog.ErrorContext(ctx, "master partition not healthy", "method", "set",
			"partition_id", partition.Id, "node_id", masterReplica.Id)
		return kvstoreAPI.SetValuedefaultJSONResponse{
			Body: common.ErrorResponse{
				Error: "master partition not healthy",
			},
			StatusCode: http.StatusServiceUnavailable,
		}, nil
	}

	client, err := database.NewClientWithResponses("http://"+masterReplica.Address,
		database.WithHTTPClient(s.httpClient))
	if err != nil {
		slog.ErrorContext(ctx, "could not create client", "method", "set", "error", err)
		return kvstoreAPI.SetValuedefaultJSONResponse{
			Body: common.ErrorResponse{
				Error: "could not create client",
			},
			StatusCode: http.StatusInternalServerError,
		}, nil
	}

	// Create the database request body
	dbRequestBody := database.SetValueInPartitionJSONRequestBody{
		Value: request.Body.Value,
	}

	// Call the database API to set the value in the partition
	resp, err := client.SetValueInPartitionWithResponse(ctx, partition.Id, request.Key, dbRequestBody)
	if err != nil {
		slog.ErrorContext(ctx, "error in set value", "method", "set", "error", err)
		return kvstoreAPI.SetValuedefaultJSONResponse{
			Body: common.ErrorResponse{
				Error: "could not set value",
			},
			StatusCode: http.StatusInternalServerError,
		}, nil
	}

	if resp.JSON200 != nil {
		return kvstoreAPI.SetValue200JSONResponse(*resp.JSON200), nil
	} else {
		slog.ErrorContext(ctx, "unexpected response from server", "method", "set", "error", resp.JSON500.Error)
		return kvstoreAPI.SetValuedefaultJSONResponse{
			Body: common.ErrorResponse{
				Error: "unexpected response from server",
			},
			StatusCode: http.StatusInternalServerError,
		}, nil
	}
}
