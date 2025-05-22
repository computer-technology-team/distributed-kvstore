package kvstore

import (
	kvstoreAPI "github.com/computer-technology-team/distributed-kvstore/api/kvstore"
)

func (kv *KVStore) GetOperation(id int64) (*kvstoreAPI.Operation, bool) {
	kv.mu.RLock()
	defer kv.mu.RUnlock()

	for _, op := range kv.opLog {
		if op.Id == id {
			return &op, true
		}
	}
	return nil, false
}

func (kv *KVStore) GetOperationsAfter(id int64) []kvstoreAPI.Operation {
	kv.mu.RLock()
	defer kv.mu.RUnlock()

	var ops []kvstoreAPI.Operation
	for _, op := range kv.opLog {
		if op.Id > id {
			ops = append(ops, op)
		}
	}
	return ops
}
