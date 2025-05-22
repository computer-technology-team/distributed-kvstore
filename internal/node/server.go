package node

import (
	"context"

	kvstoreAPI "github.com/computer-technology-team/distributed-kvstore/api/kvstore"
	internalKVStore "github.com/computer-technology-team/distributed-kvstore/internal/kvstore"
	"github.com/google/uuid"
	"github.com/oapi-codegen/runtime/types"
)

type server struct {
	kvStore *internalKVStore.NodeStore
	id      uuid.UUID
}

func NewServer(id types.UUID) kvstoreAPI.StrictServerInterface {
	// Convert the types.UUID to a uuid.UUID
	nodeID := uuid.UUID{}
	copy(nodeID[:], id[:])
	return &server{
		kvStore: internalKVStore.NewKVStore(),
		id:      nodeID,
	}
}

func (s *server) PingServer(ctx context.Context, request kvstoreAPI.PingServerRequestObject) (kvstoreAPI.PingServerResponseObject, error) {
	return kvstoreAPI.PingServer200JSONResponse{
		Ping: "pong",
	}, nil
}

func (s *server) GetValue(ctx context.Context, request kvstoreAPI.GetValueRequestObject) (kvstoreAPI.GetValueResponseObject, error) {
	key := request.Key
	
	// Find a replica that has this key
	partitions := s.kvStore.GetAllPartitions()
	for partitionID := range partitions {
		// Get the value directly from the partition
		if value, exists, err := s.kvStore.Get(partitionID, key); err == nil && exists {
			return kvstoreAPI.GetValue200JSONResponse{
				Value: value,
				Key:   key,
			}, nil
		}
	}

	return kvstoreAPI.GetValue404JSONResponse{
		Error: "Key not found",
	}, nil
}

func (s *server) SetValue(ctx context.Context, request kvstoreAPI.SetValueRequestObject) (kvstoreAPI.SetValueResponseObject, error) {
	key := request.Key
	value := request.Body.Value

	// Find a master replica to write to
	partitions := s.kvStore.GetAllPartitions()
	for partitionID, isMaster := range partitions {
		if !isMaster {
			continue // Skip non-master partitions
		}
		
		// Set the value directly in the partition
		if err := s.kvStore.Set(partitionID, key, value); err == nil {
			return kvstoreAPI.SetValue200JSONResponse{
				Key:   key,
				Value: value,
			}, nil
		}
	}

	return kvstoreAPI.SetValue400JSONResponse{
		Error: "No master replica available for write operation",
	}, nil
}

func (s *server) DeleteKey(ctx context.Context, request kvstoreAPI.DeleteKeyRequestObject) (kvstoreAPI.DeleteKeyResponseObject, error) {
	key := request.Key
	
	// Find a master replica to delete from
	partitions := s.kvStore.GetAllPartitions()
	for partitionID, isMaster := range partitions {
		if !isMaster {
			continue // Skip non-master partitions
		}
		
		// Delete the key directly from the partition
		if deleted, err := s.kvStore.Delete(partitionID, key); err == nil && deleted {
			return kvstoreAPI.DeleteKey200JSONResponse{
				Key: key,
			}, nil
		}
	}

	return kvstoreAPI.DeleteKey404JSONResponse{
		Error: "Key not found or no master replica available",
	}, nil
}
