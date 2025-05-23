package kvstore

func (kv *KVStore) IsMaster() bool {
	return kv.isMaster
}

func (kv *KVStore) SetMaster(master bool) {
	kv.isMaster = master
}

func (kv *KVStore) GetLastSyncOpID() int64 {
	return kv.lastSyncedOpID
}

