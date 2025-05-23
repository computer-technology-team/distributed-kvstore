package kvstore

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/computer-technology-team/distributed-kvstore/api/common"
	"github.com/google/uuid"
	"github.com/oapi-codegen/nullable"
	"github.com/samber/lo"
)

// NodeStore manages multiple KVStores for different partitions
type NodeStore struct {
	mu          sync.RWMutex
	stores      map[string]*KVStore       // Map of partitionID to KVStore
	lastUpdated atomic.Pointer[time.Time] // Last updated timestamp
	state       common.State
	id          uuid.UUID
}

// NewNodeStore creates a new NodeStore instance
func NewNodeStore(id uuid.UUID) *NodeStore {
	t := time.Now()
	ns := &NodeStore{
		stores: make(map[string]*KVStore),
	}

	ns.lastUpdated.Store(&t)
	return ns
}

func (ns *NodeStore) SetState(state common.State) error {
	ns.mu.Lock()
	defer ns.mu.Unlock()

	ns.state = state

	node, nodeFound := extractNodeFromState(state, ns.id)
	if !nodeFound {
		return errors.New("node not found in state")
	}

	partitionRoles := node.Partitions

	toBeAdded, toBeRemoved := lo.Difference(lo.Keys(partitionRoles), lo.Keys(ns.stores))

	// Update the timestamp
	t := time.Now()
	ns.lastUpdated.Store(&t)

	// Add new partitions
	for _, partitionID := range toBeAdded {
		store := newKVStoreInstance()
		store.isMaster = partitionRoles[partitionID].IsMaster
		store.isSyncing = partitionRoles[partitionID].IsSyncing
		ns.stores[partitionID] = store
	}

	// Update existing partitions
	for partitionID, store := range ns.stores {
		if role, exists := partitionRoles[partitionID]; exists {
			store.mu.Lock()
			store.isMaster = role.IsMaster
			store.isSyncing = role.IsSyncing
			store.mu.Unlock()
		}
	}

	// Remove partitions that are no longer assigned to this node
	for _, partitionID := range toBeRemoved {
		delete(ns.stores, partitionID)
	}

	return nil
}

// Set sets a key-value pair in the specified partition
func (ns *NodeStore) Set(partitionID string, key, value string) error {
	ns.mu.RLock()
	store, exists := ns.stores[partitionID]
	if !exists {
		ns.mu.RUnlock()
		return fmt.Errorf("partition %s not found", partitionID)
	}
	ns.mu.RUnlock()

	// Acquire write lock for this specific KVStore
	store.mu.Lock()
	defer store.mu.Unlock()

	// Only allow writes to master partitions
	if !store.isMaster {
		return fmt.Errorf("partition %s is not the master", partitionID)
	}

	// Set the value in the store
	store.store[key] = value

	// Create and append the operation
	op := common.Operation{
		ID:    store.nextOpID,
		Key:   key,
		Type:  common.Set,
		Value: nullable.NewNullableWithValue(value),
	}
	store.opLog = append(store.opLog, op)
	store.nextOpID += 1

	// Send operation to replicas asynchronously
	go ns.sendOperationToReplicas(partitionID, op)

	return nil
}

// Get retrieves a value by key from the specified partition
func (ns *NodeStore) Get(partitionID string, key string) (string, bool, error) {
	ns.mu.RLock()
	store, exists := ns.stores[partitionID]
	if !exists {
		ns.mu.RUnlock()
		return "", false, fmt.Errorf("partition %s not found", partitionID)
	}
	ns.mu.RUnlock()

	// Acquire read lock for this specific KVStore
	store.mu.RLock()
	defer store.mu.RUnlock()

	// Get the value from the store
	value, exists := store.store[key]
	if !exists {
		return "", false, nil
	}

	return value, true, nil
}

// Delete removes a key-value pair from the specified partition
func (ns *NodeStore) Delete(partitionID string, key string) (bool, error) {
	ns.mu.RLock()
	store, exists := ns.stores[partitionID]
	if !exists {
		ns.mu.RUnlock()
		return false, fmt.Errorf("partition %s not found", partitionID)
	}
	ns.mu.RUnlock()

	// Acquire write lock for this specific KVStore
	store.mu.Lock()
	defer store.mu.Unlock()

	// Only allow writes to master partitions
	if !store.isMaster {
		return false, fmt.Errorf("partition %s is not the master", partitionID)
	}

	// Check if the key exists before deleting
	_, exists = store.store[key]
	if !exists {
		return false, nil
	}

	// Delete the key
	delete(store.store, key)

	store.opLog = append(store.opLog, common.Operation{
		ID:    store.nextOpID,
		Key:   key,
		Type:  common.Delete,
		Value: nullable.NewNullNullable[string](),
	})
	store.nextOpID += 1

	return true, nil
}

func (ns *NodeStore) GetState() common.State {
	ns.mu.RLock()
	defer ns.mu.RUnlock()

	return ns.state
}

func (ns *NodeStore) GetOperation(partitionID string, operationID int64) (*common.Operation, error) {
	ns.mu.RLock()
	defer ns.mu.RUnlock()
	partitionStore, found := ns.stores[partitionID]
	if !found {
		return nil, errors.New("partition not found")
	}

	partitionStore.mu.RLock()
	defer partitionStore.mu.RUnlock()

	if !partitionStore.isMaster || partitionStore.isSyncing {
		return nil, errors.New("partition is not a stable master")
	}

	return partitionStore.GetOperation(operationID)
}

func (ns *NodeStore) GetOperations(partitionID string, fromOperationID int64) ([]common.Operation, error) {
	ns.mu.RLock()
	defer ns.mu.RUnlock()

	partitionStore, found := ns.stores[partitionID]
	if !found {
		return nil, errors.New("partition not found")
	}

	partitionStore.mu.RLock()
	defer partitionStore.mu.RUnlock()

	if !partitionStore.isMaster || partitionStore.isSyncing {
		return nil, errors.New("partition is not a stable master")
	}

	operations := partitionStore.GetOperationsAfter(fromOperationID)
	return operations, nil
}

func extractNodeFromState(state common.State, nodeID uuid.UUID) (common.Node, bool) {
	return lo.Find(state.Nodes, func(item common.Node) bool {
		return item.Id == nodeID
	})
}
