package controller

import (
	"context"
	"errors"
	"hash/fnv"
	"log/slog"
	"slices"
	"sync"
	"time"

	"github.com/computer-technology-team/distributed-kvstore/api/common"
	"github.com/computer-technology-team/distributed-kvstore/api/database"
	"github.com/computer-technology-team/distributed-kvstore/api/loadbalancer"
	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"
	"github.com/samber/lo"
)

var startWorkerOnce sync.Once

type Controller struct {
	balancerClient      loadbalancer.ClientWithResponsesInterface
	state               common.State
	lock                sync.RWMutex
	startTime           time.Time
	healthCheckInterval time.Duration
	healthCheckTimeout  time.Duration
	ticker              *time.Ticker
	stopWorker          chan int
}

func (c *Controller) AddPartition(partitionID string) error {
	c.lock.Lock()
	defer c.lock.Unlock()
	if len(c.state.Nodes) == 0 {
		return errors.New("no available nodes")
	}

	if _, exists := c.state.Partitions[partitionID]; exists {
		return errors.New("partition id exist")
	}

	if c.state.Partitions == nil {
		c.state.Partitions = make(map[string]common.Partition)
	}

	if len(c.state.Partitions) == 0 {
		// Update nodes with partition information
		for i := range c.state.Nodes {
			c.state.Nodes[i].PartitionID = &partitionID
			c.state.Nodes[i].IsMaster = lo.ToPtr(false)
			// Generate replica IDs for each node
			replicaID := openapi_types.UUID(uuid.New())
			c.state.Nodes[i].ReplicaID = &replicaID
		}

		// Set the first node as master
		c.state.Nodes[0].IsMaster = lo.ToPtr(true)

		// Extract replica IDs from nodes
		replicaIds := make([]openapi_types.UUID, len(c.state.Nodes))
		for i, node := range c.state.Nodes {
			replicaIds[i] = node.Id
		}

		c.state.Partitions[partitionID] = common.Partition{
			Id:              partitionID,
			MasterReplicaId: c.state.Nodes[0].Id,
		}
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

func (c *Controller) GetUptime() time.Duration {
	return time.Since(c.startTime)
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
func (c *Controller) StartWatcher() {
	startWorkerOnce.Do(c.startWorker)
}

func (c *Controller) startWorker() {
	c.ticker = time.NewTicker(5 * time.Second)
	for {
		select {
		case <-c.ticker.C:
			c.checkNodes()
		case <-c.stopWorker:
			return
		}
	}
}

func (c *Controller) checkNodes() {
	for _, node := range c.state.Nodes {
		c.checkNode(&node)
	}
}

func (c *Controller) checkNode(node *common.Node) {
	client, err := database.NewClientWithResponses("http://" + node.Address)
	if err != nil {
		node.Status = common.Unhealthy
		slog.Error("could not initalize database client", "error", err,
			"node_address", node.Address)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), c.healthCheckTimeout)
	defer cancel()

	resp, err := client.GetStateWithResponse(ctx)
	if err != nil {
		node.Status = common.Unhealthy
		slog.Error("could not get state", "node_address", node.Address, "error", err)
		return
	}

	if resp.StatusCode() != 200 {
		node.Status = common.Unhealthy
		slog.Error("state response non 200",
			"node_address", node.Address, "status_code", resp.StatusCode())
		return
	}

	node.Status = common.Healthy
}

func (c *Controller) StopWatcher() {
	c.ticker.Stop()
	close(c.stopWorker)
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
	_, exists := c.state.Partitions[partitionId]
	if !exists {
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

func NewController(healthCheckInterval time.Duration, healthCheckTimeout time.Duration, balancerClient loadbalancer.ClientWithResponsesInterface) *Controller {
	return &Controller{
		balancerClient:      balancerClient,
		startTime:           time.Now(),
		healthCheckInterval: healthCheckInterval,
		healthCheckTimeout:  healthCheckTimeout,
		stopWorker:          make(chan int),
	}
}
