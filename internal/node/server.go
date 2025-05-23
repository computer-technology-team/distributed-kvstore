package node

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/computer-technology-team/distributed-kvstore/api/common"
	"github.com/computer-technology-team/distributed-kvstore/api/database"
	internalKVStore "github.com/computer-technology-team/distributed-kvstore/internal/kvstore"
	"github.com/google/uuid"
	"github.com/oapi-codegen/nullable"
	"github.com/oapi-codegen/runtime/types"
)

type server struct {
	nodeStore *internalKVStore.NodeStore
	id        uuid.UUID
}

func NewServer(id types.UUID) database.StrictServerInterface {
	return &server{
		nodeStore: internalKVStore.NewNodeStore(id),
		id:        id,
	}
}

// Database API implementation
func (s *server) GetClusterState(ctx context.Context, request database.GetClusterStateRequestObject) (database.GetClusterStateResponseObject, error) {
	slog.Info("GetClusterState called")

	state := s.nodeStore.GetState()

	return database.GetClusterState200JSONResponse(state), nil
}

// UpdateNodeState implements database.StrictServerInterface.
func (s *server) UpdateNodeState(ctx context.Context, request database.UpdateNodeStateRequestObject) (database.UpdateNodeStateResponseObject, error) {
	slog.Info("UpdateNodeState called", "request", request.Body)

	if request.Body == nil {
		slog.Error("request body is nil")
		return database.UpdateNodeState400JSONResponse{Error: "request body is nil"}, nil
	}

	err := s.nodeStore.SetState(*request.Body)
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

	slog.Info("GetValueFromPartition called", "partitionID", partitionID, "key", key)

	// Get the value directly from the specified partition
	if value, exists, err := s.nodeStore.Get(partitionID, key); err == nil && exists {
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

	slog.Info("SetValueInPartition called", "partitionID", partitionID, "key", key)

	if request.Body == nil {
		slog.Error("Missing request body", "partitionID", partitionID, "key", key)
		return database.SetValueInPartition400JSONResponse{
			Error: "Missing request body",
		}, nil
	}

	value := request.Body.Value
	slog.Info("SetValueInPartition details", "partitionID", partitionID, "key", key, "value", value)

	// Set the value directly in the specified partition
	if err := s.nodeStore.Set(partitionID, key, value); err != nil {
		slog.Error("Failed to set value", "partitionID", partitionID, "key", key, "error", err)
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

	slog.Info("DeleteKeyFromPartition called", "partitionID", partitionID, "key", key)

	deleted, err := s.nodeStore.Delete(partitionID, key)
	if err != nil {
		slog.Error("Failed to delete key", "partitionID", partitionID, "key", key, "error", err)
		return database.DeleteKeyFromPartition500JSONResponse{
			Error: err.Error(),
		}, nil
	}

	if !deleted {
		slog.Info("Key not found for deletion", "partitionID", partitionID, "key", key)
		return database.DeleteKeyFromPartition404JSONResponse{
			Error: "Key not found in partition",
		}, nil
	}

	return database.DeleteKeyFromPartition200JSONResponse{
		Key: key,
	}, nil
}

// GetOperation implements the replication endpoint to get a specific operation by ID
func (s *server) GetOperation(ctx context.Context, request database.GetOperationRequestObject) (database.GetOperationResponseObject, error) {
	partitionId := request.PartitionID
	operationId := request.OperationID

	slog.Info("GetOperation called", "partitionID", partitionId, "operationID", operationId)

	operation, err := s.nodeStore.GetOperation(partitionId, operationId)
	if err != nil {
		slog.Error("failed to get operation",
			"error", err,
			"partition_id", partitionId,
			"operation_id", operationId)

		if err.Error() == "operation not found" || err.Error() == "operation is out of bound" {
			return database.GetOperation404JSONResponse{
				Error: fmt.Sprintf("Operation not found: %v", err),
			}, nil
		}

		if err.Error() == "partition not found" {
			return database.GetOperation404JSONResponse{
				Error: fmt.Sprintf("Partition not found: %s", partitionId),
			}, nil
		}

		if err.Error() == "partition is not a stable master" {
			return database.GetOperation404JSONResponse{
				Error: "Partition is not a stable master",
			}, nil
		}

		return database.GetOperation404JSONResponse{
			Error: fmt.Sprintf("Internal server error: %v", err),
		}, nil
	}

	return database.GetOperation200JSONResponse(*operation), nil
}

// GetOperationsAfter implements the replication endpoint to get operations after a specific ID
func (s *server) GetOperationsAfter(ctx context.Context, request database.GetOperationsAfterRequestObject) (database.GetOperationsAfterResponseObject, error) {
	partitionId := request.PartitionID
	lastOperationId := request.LastOperationID

	slog.Info("GetOperationsAfter called", "partitionID", partitionId, "lastOperationID", lastOperationId)

	operations, err := s.nodeStore.GetOperations(partitionId, lastOperationId)
	if err != nil {
		slog.Error("failed to get operations for checkpoint",
			"error", err,
			"partition_id", partitionId,
			"last_operation_id", lastOperationId)

		if err.Error() == "partition not found" {
			return database.GetOperationsAfter200JSONResponse([]common.Operation{}), nil
		}

		if err.Error() == "partition is not a stable master" {
			return database.GetOperationsAfter200JSONResponse([]common.Operation{}), nil
		}

		return database.GetOperationsAfter200JSONResponse([]common.Operation{}), nil
	}

	if len(operations) == 0 {
		slog.Info("no operations found for checkpoint",
			"partition_id", partitionId,
			"last_operation_id", lastOperationId)

		return database.GetOperationsAfter200JSONResponse([]common.Operation{}), nil
	}

	slog.Info("returning operations for checkpoint",
		"partition_id", partitionId,
		"last_operation_id", lastOperationId,
		"operations_count", len(operations))

	return database.GetOperationsAfter200JSONResponse(operations), nil
}

// ApplyOperation implements the endpoint for applying operations to a replica
func (s *server) ApplyOperation(ctx context.Context, request database.ApplyOperationRequestObject) (database.ApplyOperationResponseObject, error) {
	if request.Body == nil {
		return database.ApplyOperation400JSONResponse{
			Error: "Missing operation in request body",
		}, nil
	}

	partitionID := request.PartitionID
	operation := *request.Body

	err := s.nodeStore.ApplyOperation(partitionID, operation)
	if err != nil {
		return database.ApplyOperation400JSONResponse{
			Error: err.Error(),
		}, nil
	}

	return database.ApplyOperation200Response{}, nil
}
