package loadbalancer

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/computer-technology-team/distributed-kvstore/api/common"
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
			Body: kvstoreAPI.ErrorResponse{
				Error: "could not get partition",
			},
			StatusCode: http.StatusInternalServerError,
		}, nil
	}

	masterReplica, found := lo.Find(s.statePtr.Load().Nodes, replicaPredicate(partition.MasterReplicaId))
	if !found {
		slog.ErrorContext(ctx, "master replica not found", "method", "set",
			"partition_id", partition.Id)
		return kvstoreAPI.SetValuedefaultJSONResponse{
			Body: kvstoreAPI.ErrorResponse{
				Error: "master replica not found",
			},
			StatusCode: http.StatusInternalServerError,
		}, nil
	}

	if masterReplica.Status != common.Healthy {
		slog.ErrorContext(ctx, "master replica not healthy", "method", "set",
			"partition_id", partition.Id, "replica_id", masterReplica.Id)
		return kvstoreAPI.SetValuedefaultJSONResponse{
			Body: kvstoreAPI.ErrorResponse{
				Error: "master replica not healthy",
			},
			StatusCode: http.StatusServiceUnavailable,
		}, nil
	}

	client, err := kvstoreAPI.NewClientWithResponses("http://" + masterReplica.Address,
		kvstoreAPI.WithHTTPClient(s.httpClient))
	if err != nil {
		slog.ErrorContext(ctx, "could not create client", "method", "set", "error", err)
		return kvstoreAPI.SetValuedefaultJSONResponse{
			Body: kvstoreAPI.ErrorResponse{
				Error: "could not create client",
			},
			StatusCode: http.StatusInternalServerError,
		}, nil
	}

	resp, err := client.SetValueWithResponse(ctx, request.Key, *request.Body)
	if err != nil {
		slog.ErrorContext(ctx, "error in set value", "method", "set", "error", err)
		return kvstoreAPI.SetValuedefaultJSONResponse{
			Body: kvstoreAPI.ErrorResponse{
				Error: "could not set value",
			},
			StatusCode: http.StatusInternalServerError,
		}, nil
	}

	if resp.JSON200 != nil {
		return kvstoreAPI.SetValue200JSONResponse(*resp.JSON200), nil
	} else {
		slog.ErrorContext(ctx, "unexpected response from server", "method", "set", "error", resp.JSONDefault.Error)
		return kvstoreAPI.SetValuedefaultJSONResponse{
			Body: kvstoreAPI.ErrorResponse{
				Error: "unexpected response from server",
			},
			StatusCode: http.StatusInternalServerError,
		}, nil
	}
}
