package node

import (
	"context"
	"log/slog"
	"time"

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
	if !s.kvStore.IsMaster {
		// return unauthorized
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
	if !s.kvStore.IsMaster {
		// return unauthorized
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
	if !s.kvStore.IsMaster {
		// return unauthorized
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

func (s *server) startBackgroundSync() {
	ctx, cancel := context.WithCancel(context.Background())
	s.cancelSync = cancel

	go func() {
		ticker := time.NewTicker(s.kvStore.SyncInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				s.syncWithMaster()
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (s *server) Stop() {
	if s.cancelSync != nil {
		s.cancelSync()
	}
}

func (s *server) syncWithMaster() {
	lastOpID := s.kvStore.GetLastSyncedOpID()

	// Get operations from master
	client, err := kvstoreAPI.NewClientWithResponses(s.kvStore.MasterAddr)
	if err != nil {
		slog.Info("Failed to create client for master: %v", err)
		return
	}

	resp, err := client.SyncOperationsWithResponse(context.Background(), lastOpID)
	if err != nil {
		slog.Info("Failed to sync operations: %v", err)
		return
	}

	if resp.JSON200 != nil {
		s.applyOperations(*resp.JSON200)
	}
}

func (s *server) applyOperations(ops []kvstoreAPI.Operation) {
	s.kvStore.mu.Lock()
	defer s.kvStore.mu.Unlock()

	for _, op := range ops {
		switch op.Type {
		case "set":
			s.kvStore.store[op.Key] = op.Value
		case "delete":
			delete(s.kvStore.store, op.Key)
		}

		// Update operation log
		s.kvStore.opLog = append(s.kvStore.opLog, Operation{
			ID:    op.Id,
			Type:  OperationType(op.Type),
			Key:   op.Key,
			Value: op.Value,
		})

		s.kvStore.lastSyncedOpID = op.Id
	}
}
