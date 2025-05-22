package kvstore

import (
	"net/http"
	"sync"
	"time"

	kvstoreAPI "github.com/computer-technology-team/distributed-kvstore/api/kvstore"
)

type KVStore struct {
	mu             sync.RWMutex
	store          map[string]string
	IsMaster       bool
	opLog          []kvstoreAPI.Operation
	nextOpID       int64
	masterAddr     string
	SyncInterval   time.Duration
	lastSyncedOpID int64
	httpClient     *http.Client
}

func NewKVStore() *KVStore {
	return &KVStore{
		store:    make(map[string]string),
		opLog:    make([]kvstoreAPI.Operation, 0),
		nextOpID: 1,
		IsMaster: false,
	}
}

func (kv *KVStore) Get(key string) (string, bool) {
	kv.mu.RLock()
	defer kv.mu.RUnlock()
	value, exists := kv.store[key]
	return value, exists
}

func (kv *KVStore) Exists(key string) bool {
	kv.mu.RLock()
	defer kv.mu.RUnlock()
	_, exists := kv.store[key]
	return exists
}

func (kv *KVStore) Set(key, value string) {
	kv.mu.Lock()
	defer kv.mu.Unlock()

	op := kvstoreAPI.Operation{
		Id:    kv.nextOpID,
		Type:  kvstoreAPI.Set,
		Key:   key,
		Value: &value,
	}

	kv.store[key] = value
	kv.opLog = append(kv.opLog, op)
	kv.nextOpID++
}

func (kv *KVStore) Delete(key string) bool {
	kv.mu.Lock()
	defer kv.mu.Unlock()

	if _, exists := kv.store[key]; exists {
		op := kvstoreAPI.Operation{
			Id:   kv.nextOpID,
			Type: kvstoreAPI.Set,
			Key:  key,
		}

		delete(kv.store, key)
		kv.opLog = append(kv.opLog, op)
		kv.nextOpID++

		return true
	}
	return false
}
