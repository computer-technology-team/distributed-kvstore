package loadbalancer

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/computer-technology-team/distributed-kvstore/api/common"
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
			Body: kvstoreAPI.ErrorResponse{
				Error: "could not get partition",
			},
			StatusCode: http.StatusInternalServerError,
		}, nil
	}

	masterReplica, found := lo.Find(s.statePtr.Load().Nodes, replicaPredicate(partition.MasterReplicaId))
	if !found {
		slog.ErrorContext(ctx, "master replica not found", "method", "delete",
			"partition_id", partition.Id)
		return kvstoreAPI.DeleteKeydefaultJSONResponse{
			Body: kvstoreAPI.ErrorResponse{
				Error: "master replica not found",
			},
			StatusCode: http.StatusInternalServerError,
		}, nil
	}

	if masterReplica.Status != common.Healthy {
		slog.ErrorContext(ctx, "master replica not healthy", "method", "delete",
			"partition_id", partition.Id, "replica_id", masterReplica.Id)
		return kvstoreAPI.DeleteKeydefaultJSONResponse{
			Body: kvstoreAPI.ErrorResponse{
				Error: "master replica not healthy",
			},
			StatusCode: http.StatusServiceUnavailable,
		}, nil
	}

	client, err := kvstoreAPI.NewClientWithResponses("http://"+masterReplica.Address,
		kvstoreAPI.WithHTTPClient(s.httpClient))
	if err != nil {
		slog.ErrorContext(ctx, "could not create client", "method", "delete", "error", err)
		return kvstoreAPI.DeleteKeydefaultJSONResponse{
			Body: kvstoreAPI.ErrorResponse{
				Error: "could not create client",
			},
			StatusCode: http.StatusInternalServerError,
		}, nil
	}

	resp, err := client.DeleteKeyWithResponse(ctx, request.Key)
	if err != nil {
		slog.ErrorContext(ctx, "error in delete key", "method", "delete", "error", err)
		return kvstoreAPI.DeleteKeydefaultJSONResponse{
			Body: kvstoreAPI.ErrorResponse{
				Error: "could not delete key",
			},
			StatusCode: http.StatusInternalServerError,
		}, nil
	}

	if resp.JSON200 != nil {
		return kvstoreAPI.DeleteKey200JSONResponse(*resp.JSON200), nil
	} else if resp.JSON404 != nil {
		return kvstoreAPI.DeleteKey404JSONResponse(*resp.JSON404), nil
	} else {
		slog.ErrorContext(ctx, "unexpected response from server", "method", "delete", "error", resp.JSONDefault.Error)
		return kvstoreAPI.DeleteKeydefaultJSONResponse{
			Body: kvstoreAPI.ErrorResponse{
				Error: "unexpected response from server",
			},
			StatusCode: http.StatusInternalServerError,
		}, nil
	}
}
