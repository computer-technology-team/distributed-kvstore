package common

import (
	"errors"
	"hash/fnv"
	"sort"
)

func (s *State) GetPartition(key string) (*Partition, error) {
	if len(s.VirtualNodes) == 0 {
		return nil, errors.New("no virtual nodes available")
	}

	h := fnv.New64a()
	h.Write([]byte(key))
	keyHash := int64(h.Sum64())

	idx := s.findVirtualNode(keyHash)
	if idx == -1 {
		return nil, errors.New("no virtual node found")
	}

	partitionId := s.VirtualNodes[idx].PartitionId

	for _, partition := range s.Partitions {
		if partition.Id == partitionId {
			return &partition, nil
		}
	}

	return nil, errors.New("partition not found")
}

func (s *State) findVirtualNode(keyHash int64) int {
	idx := sort.Search(len(s.VirtualNodes), func(i int) bool {
		return s.VirtualNodes[i].Hash >= keyHash
	})

	if idx == len(s.VirtualNodes) {
		idx = 0
	}

	return idx
}
