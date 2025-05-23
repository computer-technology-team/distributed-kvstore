package kvstore

import (
	"errors"
	"log/slog"
	"sort"

	"github.com/computer-technology-team/distributed-kvstore/api/common"
)

var ErrOperationIsOutOfBound = errors.New("operation is out of bound")
var ErrOperationNotFound = errors.New("operation not found")

func (kv *KVStore) GetOperation(id int64) (*common.Operation, error) {
	kv.mu.RLock()
	defer kv.mu.RUnlock()

	if id >= kv.nextOpID {
		return nil, ErrOperationIsOutOfBound
	}

	for _, op := range kv.opLog {
		if op.ID == id {
			return &op, nil
		}
	}

	return nil, ErrOperationNotFound
}

func (kv *KVStore) GetOperationsAfter(id int64) []common.Operation {
	kv.mu.RLock()
	defer kv.mu.RUnlock()

	idx, found := sort.Find(len(kv.opLog), func(i int) int {
		return int(id - kv.opLog[i].ID)
	})
	if !found {
		slog.Warn("no operation found after", "after_id", id)
		return nil
	}

	var ops []common.Operation
	for i := idx; i < len(kv.opLog); i++ {
		op := kv.opLog[i]
		if op.ID > id {
			ops = append(ops, op)
		}
	}

	return ops
}
