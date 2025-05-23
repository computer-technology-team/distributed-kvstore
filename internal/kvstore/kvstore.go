package kvstore

import (
	"sync"

	"github.com/computer-technology-team/distributed-kvstore/api/common"
)

// KVStore represents a single key-value store for a partition with its status
type KVStore struct {
	mu        sync.RWMutex
	store     map[string]string // Regular map for key-value pairs
	isMaster  bool              // Whether this node is the master for this partition
	isSyncing bool              // Whether this partition is currently syncing
	opLog     []common.Operation
	nextOpID  int64
}


// newKVStoreInstance creates a new KVStore instance
func newKVStoreInstance() *KVStore {
	return &KVStore{
		store:     make(map[string]string),
		isMaster:  false,
		isSyncing: false,
	}
}