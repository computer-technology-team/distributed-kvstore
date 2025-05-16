package controller

import (
	"context"
	"log/slog"

	"github.com/computer-technology-team/distributed-kvstore/api/common"
	"github.com/computer-technology-team/distributed-kvstore/api/controller"
)

type server struct {
	controller *Controller
}

// PostNodesRegister implements controller.StrictServerInterface.
func (s *server) PostNodesRegister(ctx context.Context, request controller.PostNodesRegisterRequestObject) (controller.PostNodesRegisterResponseObject, error) {
	id, err := s.controller.RegisterNodeByAddress(request.Body.Address)
	if err != nil {
		slog.Error("could not register node", "error", err)
		return controller.PostNodesRegister409Response{}, nil
	}

	return controller.PostNodesRegister201JSONResponse{
		Id:     id,
		Status: common.Unregistered,
	}, nil
}

// GetState implements controller.StrictServerInterface.
func (s *server) GetState(ctx context.Context, request controller.GetStateRequestObject) (controller.GetStateResponseObject, error) {
	return controller.GetState200JSONResponse(s.controller.GetState()), nil
}

func NewServer(controller *Controller) controller.StrictServerInterface {
	return &server{
		controller: controller,
	}
}
