package controller

import (
	"context"
	"errors"
	"hash/fnv"
	"log/slog"
	"slices"
	"sync"

	"github.com/computer-technology-team/distributed-kvstore/api/common"
	"github.com/computer-technology-team/distributed-kvstore/api/loadbalancer"
	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"
	"github.com/samber/lo"
)

type Controller struct {
	balancerClient loadbalancer.ClientWithResponsesInterface
	state          common.State
	lock           sync.RWMutex
}

func (c *Controller) AddPartition(partitionID string) error {
	c.lock.Lock()
	defer c.lock.Unlock()
	if len(c.state.Nodes) == 0 {
		return errors.New("no available nodes")
	}

	for _, p := range c.state.Partitions {
		if p.Id == partitionID {
			return errors.New("partition id exist")
		}
	}

	if len(c.state.Partitions) == 0 {
		c.state.Partitions = append(c.state.Partitions, common.Partition{
			Id:              partitionID,
			MasterReplicaId: c.state.Nodes[0].Id,
			Replicas: lo.Map(c.state.Nodes, func(n common.Node, i int) common.Replica {
				return common.Replica{
					Address: n.Address,
					Id:      n.Id,
					Status:  n.Status,
				}
			}),
		})
		go c.dispatchState()
		return c.generateVirtualNodesForPartition(partitionID, 3*len(c.state.Nodes))
	}

	return errors.New("unimplemented")
}

func (c *Controller) RemoveNode(nodeID string) error {
	panic("unimplemented")
}

func (c *Controller) AddNode(nodeID uuid.UUID, nodeAddress string) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	if len(c.state.Partitions) == 0 {
		c.state.Nodes = append(c.state.Nodes, common.Node{
			Address: nodeAddress,
			Id:      nodeID,
			Status:  common.Uninitialized,
		})
		return nil
	}

	return errors.New("unimplemented")
}

func (c *Controller) RemovePartition(partitionID string) error {
	panic("unimplemented")
}

func (c *Controller) GetState() common.State {
	c.lock.RLock()
	defer c.lock.RUnlock()

	return c.state
}

func (c *Controller) RegisterNode(nodeID string) (uuid.UUID, error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	unregisteredNode, idx, found := lo.FindIndexOf(c.state.UnRegisteredNodes, func(n common.Node) bool {
		return n.Id.String() == nodeID
	})

	if !found {
		return uuid.Max, errors.New("unregistered node not found")
	}

	_, found = lo.Find(c.state.Nodes, func(n common.Node) bool {
		return n.Address == unregisteredNode.Address
	})
	if found {
		return uuid.Max, errors.New("registered node with this address already exists")
	}

	registeredNode := common.Node{
		Address:     unregisteredNode.Address,
		Id:          unregisteredNode.Id,
		PartitionID: nil,
		Status:      common.Healthy,
	}

	c.state.Nodes = append(c.state.Nodes, registeredNode)

	c.state.UnRegisteredNodes = slices.Delete(c.state.UnRegisteredNodes, idx, idx+1)

	return uuid.UUID(registeredNode.Id), nil
}

// RegisterNodeByAddress registers a new node by its address
func (c *Controller) RegisterNodeByAddress(address string) (uuid.UUID, error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	_, found := lo.Find(c.state.Nodes, func(n common.Node) bool {
		return n.Address == address
	})
	if found {
		return uuid.Max, errors.New("node already exists")
	}

	id := uuid.New()

	c.state.UnRegisteredNodes = append(c.state.UnRegisteredNodes,
		common.Node{
			Address:     address,
			Id:          openapi_types.UUID(id),
			PartitionID: nil,
			Status:      common.Unregistered,
		})

	return id, nil
}

func (c *Controller) generateVirtualNodesForPartition(partitionId string, count int) error {
	partitionExists := false
	for _, p := range c.state.Partitions {
		if p.Id == partitionId {
			partitionExists = true
			break
		}
	}

	if !partitionExists {
		return errors.New("partition does not exist")
	}

	for range count {
		vnodeId := uuid.New()

		h := fnv.New64a()
		h.Write([]byte(vnodeId.String()))
		hash := int64(h.Sum64())

		c.state.VirtualNodes = append(c.state.VirtualNodes, common.VirtualNode{
			Id:          openapi_types.UUID(vnodeId),
			Hash:        hash,
			PartitionId: partitionId,
		})

		slices.SortFunc(c.state.VirtualNodes, func(a common.VirtualNode, b common.VirtualNode) int {
			return int(a.Hash - b.Hash)
		})
	}

	return nil
}

func (c *Controller) dispatchState() {
	ctx := context.Background()
	_, err := c.balancerClient.SetStateWithResponse(ctx, loadbalancer.SetStateJSONRequestBody(c.state))
	if err != nil {
		slog.Error("could not set state in load balancer", "error", err)
		return
	}
}

func NewController(balancerClient loadbalancer.ClientWithResponsesInterface) *Controller {
	return &Controller{
		balancerClient: balancerClient,
	}
}
