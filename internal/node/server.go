package node

import (
	"context"
	"net/http"

	kvstoreAPI "github.com/computer-technology-team/distributed-kvstore/api/kvstore"
	"github.com/computer-technology-team/distributed-kvstore/internal/kvstore"
	"github.com/google/uuid"
	"github.com/oapi-codegen/runtime/types"
)

type server struct {
	kvStore    *kvstore.KVStore
	id         uuid.UUID
	cancelSync context.CancelFunc
}

func NewServer(id types.UUID) kvstoreAPI.StrictServerInterface {

	return &server{
		kvStore: kvstore.NewKVStore(),
		id:      uuid.UUID(id),
	}
}

func (s *server) PingServer(ctx context.Context, request kvstoreAPI.PingServerRequestObject) (kvstoreAPI.PingServerResponseObject, error) {
	return kvstoreAPI.PingServer200JSONResponse{
		Ping: "pong",
	}, nil
}

func (s *server) GetValue(ctx context.Context, request kvstoreAPI.GetValueRequestObject) (kvstoreAPI.GetValueResponseObject, error) {
	// If the node is not master, it could have old values of the key. We will defensively deny the request.
	if !s.kvStore.IsMaster() {
		return kvstoreAPI.GetValuedefaultJSONResponse{
			Body: kvstoreAPI.ErrorResponse{
				Error: "operation not permitted on non-master node",
			},
			StatusCode: http.StatusUnauthorized,
		}, nil
	}

	key := request.Key
	if value, exists := s.kvStore.Get(key); exists {
		return kvstoreAPI.GetValue200JSONResponse{
			Value: value,
			Key:   key,
		}, nil
	}

	return kvstoreAPI.GetValue404JSONResponse{
		Error: "Key not found",
	}, nil
}

func (s *server) SetValue(ctx context.Context, request kvstoreAPI.SetValueRequestObject) (kvstoreAPI.SetValueResponseObject, error) {
	if !s.kvStore.IsMaster() {
		return kvstoreAPI.SetValuedefaultJSONResponse{
			Body: kvstoreAPI.ErrorResponse{
				Error: "operation not permitted on non-master node",
			},
			StatusCode: http.StatusUnauthorized,
		}, nil
	}

	key := request.Key
	value := request.Body.Value

	s.kvStore.Set(key, value)

	return kvstoreAPI.SetValue200JSONResponse{
		Key:   key,
		Value: value,
	}, nil
}

func (s *server) DeleteKey(ctx context.Context, request kvstoreAPI.DeleteKeyRequestObject) (kvstoreAPI.DeleteKeyResponseObject, error) {
	if !s.kvStore.IsMaster() {
		return kvstoreAPI.DeleteKeydefaultJSONResponse{
			Body: kvstoreAPI.ErrorResponse{
				Error: "operation not permitted on non-master node",
			},
			StatusCode: http.StatusUnauthorized,
		}, nil
	}

	key := request.Key
	if s.kvStore.Delete(key) {
		return kvstoreAPI.DeleteKey200JSONResponse{
			Key: key,
		}, nil
	}
	return kvstoreAPI.DeleteKey404JSONResponse{
		Error: "Key not found",
	}, nil
}

func (s *server) GetOperation(ctx context.Context, request kvstoreAPI.GetOperationRequestObject) (kvstoreAPI.GetOperationResponseObject, error) {
	op, exists := s.kvStore.GetOperation(request.OpId)
	if !exists {
		return kvstoreAPI.GetOperation404JSONResponse{}, nil
	}

	return kvstoreAPI.GetOperation200JSONResponse{
		Id:    op.Id,
		Type:  op.Type,
		Key:   op.Key,
		Value: op.Value,
	}, nil
}

func (s *server) SyncOperations(ctx context.Context, request kvstoreAPI.SyncOperationsRequestObject) (kvstoreAPI.SyncOperationsResponseObject, error) {
	ops := s.kvStore.GetOperationsAfter(request.LastOpId)
	return kvstoreAPI.SyncOperations200JSONResponse(ops), nil
}
