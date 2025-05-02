package kvstore

import (
	"context"
	"github.com/computer-technology-team/distributed-kvstore/api/kvstore"
)

type server struct{}

func NewServer() kvstore.StrictServerInterface {
	return &server{}
}

func (s *server) GetPing(ctx context.Context, request kvstore.GetPingRequestObject) (kvstore.GetPingResponseObject, error) {
	return kvstore.GetPing200JSONResponse{
		Ping: "Pong",
	}, nil
}
