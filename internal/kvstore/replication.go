package kvstore

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/computer-technology-team/distributed-kvstore/api/common"
	"github.com/computer-technology-team/distributed-kvstore/api/controller"
)

func (ns *NodeStore) sendOperationToReplicas(partitionID string, op common.Operation) {
	// Get replicas for this partition from state
	ns.mu.RLock()
	node, found := extractNodeFromState(ns.state, ns.id)
	if !found {
		ns.mu.RUnlock()
		return
	}

	role, exists := node.Partitions[partitionID]
	if !exists || !role.IsMaster {
		ns.mu.RUnlock()
		return
	}

	// Find all replica nodes for this partition
	replicaNodes := make([]common.Node, 0)
	for _, n := range ns.state.Nodes {
		if r, ok := n.Partitions[partitionID]; ok && !r.IsMaster {
			replicaNodes = append(replicaNodes, n)
		}
	}
	ns.mu.RUnlock()

	// Send operation to all replicas
	for _, replica := range replicaNodes {
		url := "http://" + replica.Address + "/api/v1/partitions/" + partitionID + "/operations"
		payload, err := json.Marshal(op)
		if err != nil {
			fmt.Printf("failed to marshal operation: %v\n", err)
			continue
		}
		resp, err := http.Post(url, "application/json", bytes.NewReader(payload))
		if err != nil {
			fmt.Printf("failed to send operation to replica %s: %v\n", replica.Address, err)
			continue
		}
		resp.Body.Close()
	}
}

// ApplyOperation applies an operation to a non-master partition
func (ns *NodeStore) ApplyOperation(partitionID string, op common.Operation) error {
	ns.mu.RLock()
	store, exists := ns.stores[partitionID]
	if !exists {
		ns.mu.RUnlock()
		return fmt.Errorf("partition %s not found", partitionID)
	}
	ns.mu.RUnlock()

	store.mu.Lock()
	defer store.mu.Unlock()

	// Only allow operations on non-master partitions
	if store.isMaster {
		return fmt.Errorf("partition %s is the master, cannot apply operation", partitionID)
	}

	// Check for missing operations (gap > 1)
	if op.ID > store.nextOpID && (op.ID-store.nextOpID) > 1 {
		ns.setSyncingStatus(partitionID, true)
		// Request missing operations from master asynchronously
		go ns.syncReplicaPartitionWithMaster(partitionID)
		return fmt.Errorf("partition %s is syncing: missing operations", partitionID)
	}

	// Apply the operation using the extracted method
	if err := store.applyOperation(op); err != nil {
		return err
	}

	return nil
}

func (ns *NodeStore) syncReplicaPartitionWithMaster(partitionID string) {
	ns.mu.RLock()
	node, found := extractNodeFromState(ns.state, ns.id)
	if !found {
		ns.mu.RUnlock()
		fmt.Printf("failed to sync partition %s: node not found in state\n", partitionID)
		return
	}

	role, exists := node.Partitions[partitionID]
	if !exists || role.IsMaster {
		ns.mu.RUnlock()
		fmt.Printf("failed to sync partition %s: not a replica\n", partitionID)
		return
	}

	// Find the master node for this partition
	var masterNode *common.Node
	for _, n := range ns.state.Nodes {
		if r, ok := n.Partitions[partitionID]; ok && r.IsMaster {
			masterNode = &n
			break
		}
	}
	ns.mu.RUnlock()

	if masterNode == nil {
		fmt.Printf("failed to sync partition %s: master node not found\n", partitionID)
		return
	}

	// Request missing operations from the master
	url := "http://" + masterNode.Address + "/api/v1/partitions/" + partitionID + "/operations/sync"
	resp, err := http.Get(url)
	if err != nil {
		fmt.Printf("failed to sync partition %s from master: %v\n", partitionID, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("failed to sync partition %s: master returned status %d\n", partitionID, resp.StatusCode)
		return
	}

	var operations []common.Operation
	if err := json.NewDecoder(resp.Body).Decode(&operations); err != nil {
		fmt.Printf("failed to decode operations for partition %s: %v\n", partitionID, err)
		return
	}

	// Apply the operations to the local store
	ns.mu.RLock()
	store, exists := ns.stores[partitionID]
	ns.mu.RUnlock()
	if !exists {
		fmt.Printf("failed to sync partition %s: store not found\n", partitionID)
		return
	}

	store.mu.Lock()
	defer store.mu.Unlock()

	for _, op := range operations {
		// Apply each operation using the extracted method
		if err := store.applyOperation(op); err != nil {
			fmt.Printf("failed to apply operation %d: %v\n", op.ID, err)
			continue
		}
	}

	// Mark syncing as complete
	ns.setSyncingStatus(partitionID, false)
	fmt.Printf("successfully synced partition %s with master\n", partitionID)
}

// setSyncingStatus updates the syncing status both locally and in the cluster state
func (ns *NodeStore) setSyncingStatus(partitionID string, isSyncing bool) {
	ns.mu.Lock()
	defer ns.mu.Unlock()

	// Update local store
	if store, exists := ns.stores[partitionID]; exists {
		store.mu.Lock()
		store.isSyncing = isSyncing
		store.mu.Unlock()
	}

	// Update cluster state by finding and modifying the node directly
	var updatedNode *common.Node
	for i := range ns.state.Nodes {
		if ns.state.Nodes[i].Id == ns.id {
			if role, exists := ns.state.Nodes[i].Partitions[partitionID]; exists {
				role.IsSyncing = isSyncing
				ns.state.Nodes[i].Partitions[partitionID] = role
				updatedNode = &ns.state.Nodes[i]
			}
			break
		}
	}

	// Notify controller of the state change using generated API
	if updatedNode != nil {
		go func() {
			client, err := controller.NewClient(ns.controllerAddress)
			if err != nil {
				fmt.Printf("failed to create controller client: %v\n", err)
				return
			}
			if err := client.UpdateNodeState(ns.id, updatedNode); err != nil {
				fmt.Printf("failed to update controller state: %v\n", err)
			}
		}()
	}
}
