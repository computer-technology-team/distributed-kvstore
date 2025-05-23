package controller

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"log/slog"
	"math"
	"slices"
	"sort"
	"sync"
	"time"

	"github.com/computer-technology-team/distributed-kvstore/api/common"
	"github.com/computer-technology-team/distributed-kvstore/api/database"
	"github.com/computer-technology-team/distributed-kvstore/api/loadbalancer"
	"github.com/google/uuid"
	"github.com/mohae/deepcopy"
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

func (c *Controller) SetPartitionCount(partitionCount int) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	// Validate inputs
	if partitionCount <= 0 {
		return errors.New("partition count must be greater than 0")
	}

	if len(c.state.Nodes) == 0 {
		return errors.New("no available nodes")
	}

	currentPartitionCount := len(c.state.Partitions)

	// No change needed
	if currentPartitionCount == partitionCount {
		return nil
	}

	// Initialize partitions map if needed
	if c.state.Partitions == nil {
		c.state.Partitions = make(map[string]common.Partition)
	}

	// Initialize migration ranges if needed
	if c.state.MigrationRanges == nil {
		c.state.MigrationRanges = &[]common.MigrationRange{}
	}

	// Set resharding flag - only set to true if we're not creating the first partition
	c.state.IsResharding = currentPartitionCount != 0

	// Create a deep copy of the state for later use
	stateCopy := deepcopy.Copy(c.state).(common.State)
	nodeIDSet := make(map[openapi_types.UUID]struct{})

	if currentPartitionCount < partitionCount {
		// Need to add partitions
		for i := currentPartitionCount; i < partitionCount; i++ {
			partitionID := uuid.NewString()

			// Determine which nodes will host the partition
			var nodeIDs []openapi_types.UUID
			var partitionNodes []common.Node

			if len(c.state.Partitions) == 0 {
				// First partition: assign to all nodes
				nodeIDs, partitionNodes = c.getAllNodesForPartition()
			} else {
				// Subsequent partitions: distribute based on load
				nodeIDs, partitionNodes = c.selectNodesForPartition()
			}

			// Create the partition and assign it to nodes
			c.createPartition(partitionID, nodeIDs[0], nodeIDs)
			c.assignPartitionToNodes(partitionID, partitionNodes)

			// Generate virtual nodes for the partition
			if err := c.generateVirtualNodesForPartition(partitionID, c.virtualNodeCount); err != nil {
				slog.Error("failed to generate virtual nodes for partition", "error", err)
				return err
			}

			// Add node IDs to the set to ensure uniqueness
			for _, nodeID := range nodeIDs {
				nodeIDSet[nodeID] = struct{}{}
			}

			// Create migration ranges for the new partition if this isn't the first partition
			if c.state.IsResharding {
				c.createMigrationRangesForNewPartition(partitionID)
			}
		}
	} else if currentPartitionCount > partitionCount {
		// Need to remove partitions
		// Determine which partitions to remove
		partitionsToRemove := c.selectPartitionsToRemove(currentPartitionCount - partitionCount)

		// For each partition to remove
		for _, partitionID := range partitionsToRemove {
			// Get the nodes that host this partition
			partition := c.state.Partitions[partitionID]

			// Add node IDs to the set for state updates
			for _, nodeID := range partition.NodeIds {
				nodeIDSet[nodeID] = struct{}{}
			}

			// Create migration ranges for redistributing data from this partition
			c.createMigrationRangesForRemovedPartition(partitionID)

			// Remove virtual nodes for this partition
			c.removeVirtualNodesForPartition(partitionID)

			// Remove the partition from nodes
			c.removePartitionFromNodes(partitionID)

			// Delete the partition from state
			delete(c.state.Partitions, partitionID)
		}
	}

	// Mark partitions as migrating if we're resharding
	if c.state.IsResharding {
		for partitionID := range c.state.Partitions {
			partition := c.state.Partitions[partitionID]
			isMigrating := true
			partition.IsMigrating = &isMigrating
			c.state.Partitions[partitionID] = partition
		}
	}

	// Convert nodeIDSet to a slice for state updates
	var allNodeIDs []openapi_types.UUID
	for nodeID := range nodeIDSet {
		allNodeIDs = append(allNodeIDs, nodeID)
	}

	// Create node state updates
	nodeStateUpdates := lo.Map(lo.Uniq(allNodeIDs), func(nodeID openapi_types.UUID, _ int) lo.Tuple2[openapi_types.UUID, database.NodeState] {
		return lo.T2(nodeID, stateCopy)
	})

	// Dispatch state updates in a goroutine
	go func() {
		c.dispatchNodeState(nodeStateUpdates)
		c.dispatchState()
	}()

	return nil
}

// getAllNodesForPartition returns all nodes for the first partition
func (c *Controller) getAllNodesForPartition() ([]openapi_types.UUID, []common.Node) {
	nodeIDs := make([]openapi_types.UUID, len(c.state.Nodes))
	for i, node := range c.state.Nodes {
		nodeIDs[i] = node.Id
	}
	return nodeIDs, c.state.Nodes
}

// selectNodesForPartition selects nodes for a new partition based on load balancing
func (c *Controller) selectNodesForPartition() ([]openapi_types.UUID, []common.Node) {
	currentPartitionCount := len(c.state.Partitions)
	currentNodeCount := len(c.state.Nodes)
	maxNodePartitions := int(math.Round(float64(currentPartitionCount+1) / float64(currentNodeCount)))

	candidateNodes := lo.Filter(c.state.Nodes, func(n common.Node, _ int) bool {
		return len(n.Partitions) < maxNodePartitions
	})

	partitionNodes := lo.Samples(candidateNodes, c.state.ReplicaCount+1)
	partitionNodesIDs := lo.Map(partitionNodes, func(n common.Node, _ int) openapi_types.UUID {
		return n.Id
	})

	return partitionNodesIDs, partitionNodes
}

// createPartition creates a new partition in the state
func (c *Controller) createPartition(partitionID string, masterNodeID openapi_types.UUID, nodeIDs []openapi_types.UUID) {
	c.state.Partitions[partitionID] = common.Partition{
		Id:           partitionID,
		MasterNodeId: masterNodeID,
		NodeIds:      nodeIDs,
	}
}

// assignPartitionToNodes assigns a partition to the selected nodes
func (c *Controller) assignPartitionToNodes(partitionID string, nodes []common.Node) {
	for i := range nodes {
		// Initialize partitions map if needed
		if nodes[i].Partitions == nil {
			nodes[i].Partitions = map[string]common.PartitionRole{}
		}

		// Create partition role
		role := common.PartitionRole{
			IsMaster:  i == 0,
			IsSyncing: len(c.state.Partitions) > 1, // Only set syncing for non-first partitions
		}

		// Assign role to nodes[i]
		nodes[i].Partitions[partitionID] = role
	}
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
	c.ticker = time.NewTicker(c.healthCheckInterval)
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

	node.Status = status

	for partitionID := range node.Partitions {
		node.Partitions[partitionID] = common.PartitionRole{
			IsMaster: node.Partitions[partitionID].IsMaster,
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

func (c *Controller) dispatchNodeState(nodeStateUpdates []lo.Tuple2[openapi_types.UUID, database.NodeState]) {
	for _, update := range nodeStateUpdates {
		nodeID, state := update.Unpack()
		dbClient := c.nodeClients[nodeID]
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		defer cancel()
		resp, err := dbClient.UpdateNodeStateWithResponse(ctx, nodeID, database.UpdateNodeStateJSONRequestBody(state))
		if err != nil || resp.StatusCode() != 200 {
			slog.Error("could not update node status", "node_id", nodeID, "response_status_code", resp.StatusCode())
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

// selectPartitionsToRemove selects partitions to remove based on load balancing
func (c *Controller) selectPartitionsToRemove(count int) []string {
	return lo.Samples(lo.Keys(c.state.Partitions), count)
}

// removeVirtualNodesForPartition removes all virtual nodes for a given partition
func (c *Controller) removeVirtualNodesForPartition(partitionID string) {
	// Filter out virtual nodes for the given partition
	c.state.VirtualNodes = lo.Filter(c.state.VirtualNodes, func(vn common.VirtualNode, _ int) bool {
		return vn.PartitionId != partitionID
	})
}

// removePartitionFromNodes removes a partition from all nodes that host it
func (c *Controller) removePartitionFromNodes(partitionID string) {
	// Get the partition
	partition, exists := c.state.Partitions[partitionID]
	if !exists {
		return
	}

	// Remove the partition from each node
	for _, nodeID := range partition.NodeIds {
		for i := range c.state.Nodes {
			if c.state.Nodes[i].Id == nodeID {
				delete(c.state.Nodes[i].Partitions, partitionID)
				break
			}
		}
	}
}

// createMigrationRangesForNewPartition creates migration ranges when a new partition is added
func (c *Controller) createMigrationRangesForNewPartition(newPartitionID string) {
	// Sort virtual nodes by hash to ensure consistent ranges
	slices.SortFunc(c.state.VirtualNodes, func(a common.VirtualNode, b common.VirtualNode) int {
		return int(a.Hash - b.Hash)
	})

	// Find all virtual nodes for the new partition
	newVNodes := lo.Filter(c.state.VirtualNodes, func(vn common.VirtualNode, _ int) bool {
		return vn.PartitionId == newPartitionID
	})

	// For each virtual node in the new partition, create a migration range
	for _, newVNode := range newVNodes {
		// Find the previous virtual node in the ring
		prevIdx := c.findPreviousVirtualNodeIndex(newVNode.Hash)
		if prevIdx == -1 {
			continue // Skip if no previous node found (shouldn't happen in a properly initialized ring)
		}

		prevVNode := c.state.VirtualNodes[prevIdx]

		// Skip if the previous node is also from the new partition
		if prevVNode.PartitionId == newPartitionID {
			continue
		}

		// Create a migration range from the previous partition to the new one
		migrationRange := common.MigrationRange{
			Id:                openapi_types.UUID(uuid.New()),
			RangeStart:        prevVNode.Hash,
			RangeEnd:          newVNode.Hash,
			SourcePartitionId: prevVNode.PartitionId,
			TargetPartitionId: newPartitionID,
			Status:            common.NotStarted,
		}

		// Add the migration range to the state
		*c.state.MigrationRanges = append(*c.state.MigrationRanges, migrationRange)
	}
}

// createMigrationRangesForRemovedPartition creates migration ranges when a partition is removed
func (c *Controller) createMigrationRangesForRemovedPartition(removedPartitionID string) {
	// Get all virtual nodes for the removed partition
	removedVNodes := lo.Filter(c.state.VirtualNodes, func(vn common.VirtualNode, _ int) bool {
		return vn.PartitionId == removedPartitionID
	})

	// Sort virtual nodes by hash
	slices.SortFunc(c.state.VirtualNodes, func(a common.VirtualNode, b common.VirtualNode) int {
		return int(a.Hash - b.Hash)
	})

	// For each virtual node in the removed partition, create a migration range
	for _, removedVNode := range removedVNodes {
		// Find the next virtual node in the ring that's not from the removed partition
		nextVNode := c.findNextNonRemovedVirtualNode(removedVNode.Hash, removedPartitionID)
		if nextVNode == nil {
			continue // Skip if no suitable next node found
		}

		// Create a migration range from the removed partition to the next one
		migrationRange := common.MigrationRange{
			Id:                openapi_types.UUID(uuid.New()),
			RangeStart:        removedVNode.Hash,
			RangeEnd:          nextVNode.Hash,
			SourcePartitionId: removedPartitionID,
			TargetPartitionId: nextVNode.PartitionId,
			Status:            common.NotStarted,
		}

		// Add the migration range to the state
		*c.state.MigrationRanges = append(*c.state.MigrationRanges, migrationRange)
	}
}

// findPreviousVirtualNodeIndex finds the index of the virtual node that comes before the given hash
func (c *Controller) findPreviousVirtualNodeIndex(hash int64) int {
	if len(c.state.VirtualNodes) <= 1 {
		return -1
	}

	// Find the index of the first node with hash >= the given hash
	idx := sort.Search(len(c.state.VirtualNodes), func(i int) bool {
		return c.state.VirtualNodes[i].Hash >= hash
	})

	// If we found the exact node or we're at the beginning, the previous node is the last one
	if idx == 0 || (idx < len(c.state.VirtualNodes) && c.state.VirtualNodes[idx].Hash == hash) {
		return len(c.state.VirtualNodes) - 1
	}

	// Otherwise, the previous node is the one before the found index
	return idx - 1
}

// findNextNonRemovedVirtualNode finds the next virtual node that's not in the removed partition
func (c *Controller) findNextNonRemovedVirtualNode(hash int64, removedPartitionID string) *common.VirtualNode {
	if len(c.state.VirtualNodes) <= 1 {
		return nil
	}

	// Find the index of the first node with hash > the given hash
	idx := sort.Search(len(c.state.VirtualNodes), func(i int) bool {
		return c.state.VirtualNodes[i].Hash > hash
	})

	// If we're at the end, wrap around to the beginning
	if idx == len(c.state.VirtualNodes) {
		idx = 0
	}

	// Find the first node that's not in the removed partition
	startIdx := idx
	for {
		if c.state.VirtualNodes[idx].PartitionId != removedPartitionID {
			return &c.state.VirtualNodes[idx]
		}

		idx = (idx + 1) % len(c.state.VirtualNodes)
		if idx == startIdx {
			// We've gone all the way around and found no suitable node
			return nil
		}
	}
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
