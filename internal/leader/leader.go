package leader

import (
	"sync"
)

type KVStore struct {
	mu    sync.RWMutex
	store map[string]string
}

func NewKVStore() *KVStore {
	return &KVStore{
		store: make(map[string]string),
	}
}

// Create or Update a key-value pair
func (kv *KVStore) Set(key, value string) {
	kv.mu.Lock()
	defer kv.mu.Unlock()
	kv.store[key] = value
}

// Read a value by key
func (kv *KVStore) Get(key string) (string, bool) {
	kv.mu.RLock()
	defer kv.mu.RUnlock()
	value, exists := kv.store[key]
	return value, exists
}

// Delete a key-value pair
func (kv *KVStore) Delete(key string) bool {
	kv.mu.Lock()
	defer kv.mu.Unlock()
	if _, exists := kv.store[key]; exists {
		delete(kv.store, key)
		return true
	}
	return false
}

// Check if a key exists
func (kv *KVStore) Exists(key string) bool {
	kv.mu.RLock()
	defer kv.mu.RUnlock()
	_, exists := kv.store[key]
	return exists
}
