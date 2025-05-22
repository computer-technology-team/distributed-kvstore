package node

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/computer-technology-team/distributed-kvstore/api/database"
	internalKVStore "github.com/computer-technology-team/distributed-kvstore/internal/kvstore"
	"github.com/google/uuid"
	"github.com/oapi-codegen/nullable"
	"github.com/oapi-codegen/runtime/types"
)

type server struct {
	kvStore    *internalKVStore.NodeStore
	id         uuid.UUID
	cancelSync context.CancelFunc
}

func NewServer(id types.UUID) database.StrictServerInterface {
	return &server{
		kvStore: internalKVStore.NewNodeStore(),
		id:      id,
	}
}

// Database API implementation
func (s *server) GetClusterState(ctx context.Context, request database.GetClusterStateRequestObject) (database.GetClusterStateResponseObject, error) {

	return database.GetClusterState200JSONResponse{
		Partitions: s.kvStore.GetPartitionRoles(),
		NodeID:     s.id,
	}, nil
}

// UpdateNodeState implements database.StrictServerInterface.
func (s *server) UpdateNodeState(ctx context.Context, request database.UpdateNodeStateRequestObject) (database.UpdateNodeStateResponseObject, error) {
	if request.NodeID != s.id {
		slog.Error("node ids don't match")
		return database.UpdateNodeState500JSONResponse{Error: "node id does not match"}, nil
	}

	err := s.kvStore.SetState(*request.Body)
	if err != nil {
		slog.Error("failed to set state", "error", err)
		return database.UpdateNodeState500JSONResponse{Error: fmt.Sprintf("failed to set state: %v", err)}, nil
	}

	return database.UpdateNodeState200Response{}, nil
}

// KVStore API with partition ID implementation
func (s *server) GetValueFromPartition(ctx context.Context, request database.GetValueFromPartitionRequestObject) (database.GetValueFromPartitionResponseObject, error) {
	partitionID := request.PartitionID
	key := request.Key

	// Get the value directly from the specified partition
	if value, exists, err := s.kvStore.Get(partitionID, key); err == nil && exists {
		return database.GetValueFromPartition200JSONResponse{
			Value: nullable.NewNullableWithValue(value),
			Key:   key,
		}, nil
	}

	return database.GetValueFromPartition404JSONResponse{
		Error: "Key not found in partition",
	}, nil
}

func (s *server) SetValueInPartition(ctx context.Context, request database.SetValueInPartitionRequestObject) (database.SetValueInPartitionResponseObject, error) {
	partitionID := request.PartitionID
	key := request.Key

	if request.Body == nil {
		return database.SetValueInPartition400JSONResponse{
			Error: "Missing request body",
		}, nil
	}

	value := request.Body.Value

	// Set the value directly in the specified partition
	if err := s.kvStore.Set(partitionID, key, value); err != nil {
		return database.SetValueInPartition400JSONResponse{
			Error: err.Error(),
		}, nil
	}

	return database.SetValueInPartition200JSONResponse{
		Key:   key,
		Value: value,
	}, nil
}

func (s *server) DeleteKeyFromPartition(ctx context.Context, request database.DeleteKeyFromPartitionRequestObject) (database.DeleteKeyFromPartitionResponseObject, error) {
	partitionID := request.PartitionID
	key := request.Key

	deleted, err := s.kvStore.Delete(partitionID, key)
	if err != nil {
		return database.DeleteKeyFromPartition500JSONResponse{
			Error: err.Error(),
		}, nil
	}

	if !deleted {
		return database.DeleteKeyFromPartition404JSONResponse{
			Error: "Key not found in partition",
		}, nil
	}

	return database.DeleteKeyFromPartition200JSONResponse{
		Key: key,
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
