package controller

import (
	"context"
	"errors"
	"fmt"
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
	nodeClients         map[uuid.UUID]database.ClientWithResponsesInterface
	virtualNodeCount    int
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
		// Initialize partitions map for each node
		for i := range c.state.Nodes {
			if c.state.Nodes[i].Partitions == nil {
				c.state.Nodes[i].Partitions = map[string]common.PartitionRole{}
			}
			role := common.PartitionRole{
				IsMaster: i == 0,
				Status:   common.Healthy,
			}

			c.state.Nodes[i].Partitions[partitionID] = role
		}

		updatedNodeIds := make([]openapi_types.UUID, len(c.state.Nodes))
		for i, node := range c.state.Nodes {
			updatedNodeIds[i] = node.Id
		}

		c.state.Partitions[partitionID] = common.Partition{
			Id:           partitionID,
			MasterNodeId: c.state.Nodes[0].Id,
			NodeIds:      updatedNodeIds,
			Status:       common.Healthy,
		}

		err := c.generateVirtualNodesForPartition(partitionID, c.virtualNodeCount)
		if err != nil {
			slog.Error("failed to generate virtual nodes for partition", "error", err)
			return err
		}

		nodeStateUpdates := lo.Map(updatedNodeIds, func(nodeID openapi_types.UUID, _ int) database.NodeState {
			node, _ := lo.Find(c.state.Nodes, func(n common.Node) bool { return n.Id == nodeID })
			return database.NodeState{
				NodeID:     nodeID,
				Partitions: node.Partitions,
			}
		})

		go func() {
			c.dispatchNodeState(nodeStateUpdates)
			c.dispatchState()
		}()
		return nil
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
		// Initialize empty partitions map
		partitions := make(map[string]common.PartitionRole)

		c.state.Nodes = append(c.state.Nodes, common.Node{
			Address:    nodeAddress,
			Id:         nodeID,
			Partitions: partitions,
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

	// Initialize empty partitions map
	partitions := make(map[string]common.PartitionRole)

	registeredNode := common.Node{
		Address:    unregisteredNode.Address,
		Id:         unregisteredNode.Id,
		Partitions: partitions,
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
	c.lock.Lock()
	defer c.lock.Unlock()

	for i := range c.state.Nodes {
		c.checkNode(&c.state.Nodes[i])
	}
}

// updateNodePartitionsStatus updates the status of all partitions for a node
func (c *Controller) updateNodePartitionsStatus(node *common.Node, status common.Status) {
	if node.Partitions == nil {
		return
	}

	for partitionID := range node.Partitions {
		node.Partitions[partitionID] = common.PartitionRole{
			IsMaster: node.Partitions[partitionID].IsMaster,
			Status:   status,
		}
	}
}

func (c *Controller) checkNode(node *common.Node) {
	client, err := database.NewClientWithResponses("http://" + node.Address)
	if err != nil {
		// Update status for all partitions this node is responsible for
		c.updateNodePartitionsStatus(node, common.Unhealthy)
		slog.Error("could not initalize database client", "error", err,
			"node_address", node.Address)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), c.healthCheckTimeout)
	defer cancel()

	resp, err := client.GetClusterStateWithResponse(ctx)
	if err != nil {
		// Update status for all partitions this node is responsible for
		c.updateNodePartitionsStatus(node, common.Unhealthy)
		slog.Error("could not get state", "node_address", node.Address, "error", err)
		return
	}

	if resp.StatusCode() != 200 {
		// Update status for all partitions this node is responsible for
		c.updateNodePartitionsStatus(node, common.Unhealthy)
		slog.Error("state response non 200",
			"node_address", node.Address, "status_code", resp.StatusCode())
		return
	}

	// Update status for all partitions this node is responsible for
	c.updateNodePartitionsStatus(node, common.Healthy)
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
	client, err := database.NewClientWithResponses("http://" + address)
	if err != nil {
		slog.Error("could not create database client", "node_address", address)
		return uuid.Max, fmt.Errorf("could not create database client: %w", err)
	}

	c.nodeClients[id] = client

	// Create an empty partitions map for the unregistered node
	c.state.UnRegisteredNodes = append(c.state.UnRegisteredNodes,
		common.Node{
			Address: address,
			Id:      id,
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

func (c *Controller) dispatchNodeState(nodeStateUpdates []database.NodeState) {
	for _, node := range nodeStateUpdates {
		dbClient := c.nodeClients[node.NodeID]
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		defer cancel()
		resp, err := dbClient.UpdateNodeStateWithResponse(ctx, node.NodeID, database.UpdateNodeStateJSONRequestBody(node))
		if err != nil || resp.StatusCode() != 200 {
			slog.Error("could not update node status", "node_id", node.NodeID, "response_status_code", resp.StatusCode())
		}
	}
}

func (c *Controller) dispatchState() {
	ctx := context.Background()
	_, err := c.balancerClient.SetStateWithResponse(ctx, loadbalancer.SetStateJSONRequestBody(c.state))
	if err != nil {
		slog.Error("could not set state in load balancer", "error", err)
		return
	}
}

// SetReplicaNumber sets the replica number with proper locking
func (c *Controller) SetReplicaCount(replicaNum int) error {
	c.lock.Lock()
	defer c.lock.Unlock()
	if replicaNum >= len(c.state.Nodes) {
		return errors.New("replica count can not be equal or more than node count")

	}

	c.state.ReplicaCount = replicaNum
	return nil
}

func NewController(virtualNodeCount int, healthCheckInterval time.Duration, healthCheckTimeout time.Duration, balancerClient loadbalancer.ClientWithResponsesInterface) *Controller {
	return &Controller{
		balancerClient:      balancerClient,
		startTime:           time.Now(),
		healthCheckInterval: healthCheckInterval,
		healthCheckTimeout:  healthCheckTimeout,
		stopWorker:          make(chan int),
		nodeClients:         make(map[uuid.UUID]database.ClientWithResponsesInterface),
		virtualNodeCount:    virtualNodeCount,
	}
}
