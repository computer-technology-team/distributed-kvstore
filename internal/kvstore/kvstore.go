package kvstore

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/computer-technology-team/distributed-kvstore/api/common"
	"github.com/computer-technology-team/distributed-kvstore/api/database"
	"github.com/samber/lo"
)

// KVStore represents a single key-value store for a partition with its status
type KVStore struct {
	mu        sync.RWMutex
	store     map[string]string // Regular map for key-value pairs
	isMaster  bool              // Whether this node is the master for this partition
	isSyncing bool              // Whether this partition is currently syncing
}

// NodeStore manages multiple KVStores for different partitions
type NodeStore struct {
	mu          sync.RWMutex
	stores      map[string]*KVStore       // Map of partitionID to KVStore
	lastUpdated atomic.Pointer[time.Time] // Last updated timestamp
}

// NewNodeStore creates a new NodeStore instance
func NewNodeStore() *NodeStore {
	t := time.Now()
	ns := &NodeStore{
		stores: make(map[string]*KVStore),
	}
	ns.lastUpdated.Store(&t)
	return ns
}

// newKVStoreInstance creates a new KVStore instance
func newKVStoreInstance() *KVStore {
	return &KVStore{
		store:     make(map[string]string),
		isMaster:  false,
		isSyncing: false,
	}
}

func (ns *NodeStore) SetState(state database.NodeState) error {
	ns.mu.Lock()
	defer ns.mu.Unlock()

	toBeAdded, toBeRemoved := lo.Difference(lo.Keys(state.Partitions), lo.Keys(ns.stores))

	// Update the timestamp
	t := time.Now()
	ns.lastUpdated.Store(&t)

	// Add new partitions
	for _, partitionID := range toBeAdded {
		store := newKVStoreInstance()
		store.isMaster = state.Partitions[partitionID].IsMaster
		ns.stores[partitionID] = store
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
	return true, nil
}

func (ns *NodeStore) GetPartitionRoles() map[string]common.PartitionRole {
	ns.mu.RLock()
	defer ns.mu.RUnlock()

	partitions := make(map[string]common.PartitionRole)
	for partitionID, store := range ns.stores {
		store.mu.RLock()
		partitions[partitionID] = common.PartitionRole{
			IsMaster:  store.isMaster,
			IsSyncing: store.isSyncing,
			Status:    common.Healthy,
		}
		store.mu.RUnlock()
	}

	return partitions
}
