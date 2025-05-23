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

// DeleteKey implements LoadBalancer.
func (s *server) DeleteKey(ctx context.Context,
	request kvstoreAPI.DeleteKeyRequestObject) (kvstoreAPI.DeleteKeyResponseObject, error) {
	partition, err := s.statePtr.Load().GetPartition(request.Key)
	if err != nil {
		slog.ErrorContext(ctx, "could not get partition", "method", "delete", "error", err)
		return kvstoreAPI.DeleteKeydefaultJSONResponse{
			Body: common.ErrorResponse{
				Error: "could not get partition",
			},
			StatusCode: http.StatusInternalServerError,
		}, nil
	}

	masterNode, found := lo.Find(s.statePtr.Load().Nodes, func(node common.Node) bool {
		return node.Id == partition.MasterNodeId
	})

	if !found {
		slog.ErrorContext(ctx, "master node not found", "method", "delete",
			"partition_id", partition.Id)
		return kvstoreAPI.DeleteKeydefaultJSONResponse{
			Body: common.ErrorResponse{
				Error: "master node not found",
			},
			StatusCode: http.StatusInternalServerError,
		}, nil
	}

	// Check if the node has this partition and it's healthy
	if masterNode.Partitions == nil {
		slog.ErrorContext(ctx, "master node has no partitions", "method", "delete",
			"partition_id", partition.Id, "node_id", masterNode.Id)
		return kvstoreAPI.DeleteKeydefaultJSONResponse{
			Body: common.ErrorResponse{
				Error: "master node has no partitions",
			},
			StatusCode: http.StatusInternalServerError,
		}, nil
	}

	partitionRole, exists := masterNode.Partitions[partition.Id]
	if !exists || !partitionRole.IsMaster {
		slog.ErrorContext(ctx, "node is not master for this partition", "method", "delete",
			"partition_id", partition.Id, "node_id", masterNode.Id)
		return kvstoreAPI.DeleteKeydefaultJSONResponse{
			Body: common.ErrorResponse{
				Error: "node is not master for this partition",
			},
			StatusCode: http.StatusInternalServerError,
		}, nil
	}

	if partitionRole.Status != common.Healthy {
		slog.ErrorContext(ctx, "master partition not healthy", "method", "delete",
			"partition_id", partition.Id, "node_id", masterNode.Id)
		return kvstoreAPI.DeleteKeydefaultJSONResponse{
			Body: common.ErrorResponse{
				Error: "master partition not healthy",
			},
			StatusCode: http.StatusServiceUnavailable,
		}, nil
	}

	client, err := database.NewClientWithResponses("http://"+masterNode.Address,
		database.WithHTTPClient(s.httpClient))
	if err != nil {
		slog.ErrorContext(ctx, "could not create client", "method", "delete", "error", err)
		return kvstoreAPI.DeleteKeydefaultJSONResponse{
			Body: common.ErrorResponse{
				Error: "could not create client",
			},
			StatusCode: http.StatusInternalServerError,
		}, nil
	}

	// Call the database API to delete the key from the partition
	resp, err := client.DeleteKeyFromPartitionWithResponse(ctx, partition.Id, request.Key)
	if err != nil {
		slog.ErrorContext(ctx, "error in delete key", "method", "delete", "error", err)
		return kvstoreAPI.DeleteKeydefaultJSONResponse{
			Body: common.ErrorResponse{
				Error: "could not delete key",
			},
			StatusCode: http.StatusInternalServerError,
		}, nil
	}

	if resp.JSON200 != nil {
		return kvstoreAPI.DeleteKey200JSONResponse(*resp.JSON200), nil
	} else if resp.JSON404 != nil {
		return kvstoreAPI.DeleteKey404JSONResponse(*resp.JSON404), nil
	} else if resp.JSON500 != nil {
		slog.ErrorContext(ctx, "unexpected response from server", "method", "delete", "error", resp.JSON500.Error)
	} else {
		slog.ErrorContext(ctx, "unexpected response from server", "method", "delete")
	}

	return kvstoreAPI.DeleteKeydefaultJSONResponse{
		Body: common.ErrorResponse{
			Error: "unexpected response from server",
		},
		StatusCode: http.StatusInternalServerError,
	}, nil
}
