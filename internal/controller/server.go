package controller

import (
	"context"
	"fmt"
	"log/slog"

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
		Id: id,
	}, nil
}

// GetState implements controller.StrictServerInterface.
func (s *server) GetState(ctx context.Context, request controller.GetStateRequestObject) (controller.GetStateResponseObject, error) {
	return controller.GetState200JSONResponse(s.controller.GetState()), nil
}

// UpdateNodeStateByNode implements database.StrictServerInterface
func (s *server) UpdateNodeStateByNode(ctx context.Context, request controller.UpdateNodeStateByNodeRequestObject) (controller.UpdateNodeStateByNodeResponseObject, error) {
	if request.Body == nil {
		return controller.UpdateNodeStateByNode400JSONResponse{
			Error: "request body is nil",
		}, nil
	}

	// Verify node exists
	if !s.controller.HasNode(request.NodeId) {
		return controller.UpdateNodeStateByNode404JSONResponse{
			Error: "node not found",
		}, nil
	}

	// Update node state
	err := s.controller.UpdateNodeState(request.NodeId, *request.Body)
	if err != nil {
		return controller.UpdateNodeStateByNode400JSONResponse{
			Error: fmt.Sprintf("failed to update node state: %v", err),
		}, nil
	}

	return controller.UpdateNodeStateByNode200Response{}, nil
}

func NewServer(controller *Controller) controller.StrictServerInterface {
	return &server{
		controller: controller,
	}
}
