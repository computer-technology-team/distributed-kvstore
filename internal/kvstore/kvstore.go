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

// KVStore represents a single key-value store for a partition
type KVStore struct {
	mu    sync.RWMutex
	store map[string]string // Regular map for key-value pairs
}

// StoreStatus represents the status of a partition store
type StoreStatus struct {
	IsMaster  bool // Whether this node is the master for this partition
	IsSyncing bool // Whether this partition is currently syncing
}

// NodeStore manages multiple KVStores for different partitions
type NodeStore struct {
	mu          sync.RWMutex
	stores      map[string]*KVStore       // Map of partitionID to KVStore
	storeStatus map[string]StoreStatus    // Map of partitionID to store status
	lastUpdated atomic.Pointer[time.Time] // Last updated timestamp
}

// NewNodeStore creates a new NodeStore instance
func NewNodeStore() *NodeStore {
	t := time.Now()
	ns := &NodeStore{
		stores:      make(map[string]*KVStore),
		storeStatus: make(map[string]StoreStatus),
	}
	ns.lastUpdated.Store(&t)
	return ns
}

// newKVStoreInstance creates a new KVStore instance
func newKVStoreInstance() *KVStore {
	return &KVStore{
		store: make(map[string]string),
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
		ns.stores[partitionID] = newKVStoreInstance()
		ns.storeStatus[partitionID] = StoreStatus{
			IsMaster:  state.Partitions[partitionID].IsMaster,
			IsSyncing: false, // Default to not syncing for new partitions
		}
	}

	// Remove partitions that are no longer assigned to this node
	for _, partitionID := range toBeRemoved {
		delete(ns.stores, partitionID)
		delete(ns.storeStatus, partitionID)
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

	status := ns.storeStatus[partitionID]
	ns.mu.RUnlock()

	// Only allow writes to master partitions
	if !status.IsMaster {
		return fmt.Errorf("partition %s is not the master", partitionID)
	}

	// Acquire write lock for this specific KVStore
	store.mu.Lock()
	defer store.mu.Unlock()

	// Set the value in the store
	store.store[key] = value
	return nil
}

// Get retrieves a value by key from the specified partition
func (ns *NodeStore) Get(partitionID string, key string) (string, bool, error) {
	ns.mu.RLock()
	store, exists := ns.stores[partitionID]
	ns.mu.RUnlock()

	if !exists {
		return "", false, fmt.Errorf("partition %s not found", partitionID)
	}

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

	status := ns.storeStatus[partitionID]
	ns.mu.RUnlock()

	// Only allow writes to master partitions
	if !status.IsMaster {
		return false, fmt.Errorf("partition %s is not the master", partitionID)
	}

	// Acquire write lock for this specific KVStore
	store.mu.Lock()
	defer store.mu.Unlock()

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
	for partitionID, status := range ns.storeStatus {
		partitions[partitionID] = common.PartitionRole{
			IsMaster:  status.IsMaster,
			IsSyncing: status.IsSyncing,
			Status:    common.Healthy,
		}
	}

	return partitions
}
