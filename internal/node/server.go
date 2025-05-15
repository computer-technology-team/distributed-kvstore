package node

import (
	"context"

	kvstoreAPI "github.com/computer-technology-team/distributed-kvstore/api/kvstore"
	"github.com/computer-technology-team/distributed-kvstore/internal/kvstore"
)

type server struct {
	kvStore *kvstore.KVStore
}

func NewServer() kvstoreAPI.StrictServerInterface {
	return &server{
		kvStore: kvstore.NewKVStore(),
	}
}

func (s *server) PingServer(ctx context.Context, request kvstoreAPI.PingServerRequestObject) (kvstoreAPI.PingServerResponseObject, error) {
	return kvstoreAPI.PingServer200JSONResponse{
		Ping: "pong",
	}, nil
}

func (s *server) GetValue(ctx context.Context, request kvstoreAPI.GetValueRequestObject) (kvstoreAPI.GetValueResponseObject, error) {
	key := request.Key
	if value, exists := s.kvStore.Get(key); exists {
		return kvstoreAPI.GetValue200JSONResponse{
			Value: value,
			Key:   key,
		}, nil
	}

	return kvstoreAPI.GetValue404JSONResponse{
		Error: "Key not found",
	}, nil
}

func (s *server) SetValue(ctx context.Context, request kvstoreAPI.SetValueRequestObject) (kvstoreAPI.SetValueResponseObject, error) {
	key := request.Key
	value := request.Body.Value

	s.kvStore.Set(key, value)

	return kvstoreAPI.SetValue200JSONResponse{
		Key:   key,
		Value: value,
	}, nil
}

func (s *server) DeleteKey(ctx context.Context, request kvstoreAPI.DeleteKeyRequestObject) (kvstoreAPI.DeleteKeyResponseObject, error) {
	key := request.Key
	if s.kvStore.Delete(key) {
		return kvstoreAPI.DeleteKey200JSONResponse{
			Key: key,
		}, nil
	}
	return kvstoreAPI.DeleteKey404JSONResponse{
		Error: "Key not found",
	}, nil
}
