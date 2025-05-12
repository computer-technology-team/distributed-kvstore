package cmd

import (
	"fmt"

	"github.com/computer-technology-team/distributed-kvstore/internal/leader"
	"github.com/spf13/cobra"
)

// NewKVStoreCmd creates a new KVStore command
func NewKVStoreCmd() *cobra.Command {
	kvStore := leader.NewKVStore()

	cmd := &cobra.Command{
		Use:   "kvstore",
		Short: "Interact with the KVStore",
		Long:  "This command allows you to interact with the KVStore. You can set, get, delete, and check the existence of keys.",
	}

	var setCmd = &cobra.Command{
		Use:   "set [key] [value]",
		Short: "Set a key-value pair",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			key, value := args[0], args[1]
			kvStore.Set(key, value)
			fmt.Printf("Key '%s' set to value '%s'\n", key, value)
		},
	}

	var getCmd = &cobra.Command{
		Use:   "get [key]",
		Short: "Get the value of a key",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			key := args[0]
			if value, exists := kvStore.Get(key); exists {
				fmt.Printf("Value for key '%s': '%s'\n", key, value)
			} else {
				fmt.Printf("Key '%s' does not exist\n", key)
			}
		},
	}

	var deleteCmd = &cobra.Command{
		Use:   "delete [key]",
		Short: "Delete a key-value pair",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			key := args[0]
			if kvStore.Delete(key) {
				fmt.Printf("Key '%s' deleted\n", key)
			} else {
				fmt.Printf("Key '%s' does not exist\n", key)
			}
		},
	}

	var existsCmd = &cobra.Command{
		Use:   "exists [key]",
		Short: "Check if a key exists",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			key := args[0]
			if kvStore.Exists(key) {
				fmt.Printf("Key '%s' exists\n", key)
			} else {
				fmt.Printf("Key '%s' does not exist\n", key)
			}
		},
	}

	var blockCmd = &cobra.Command{
		Use:   "block",
		Short: "Block the program to prevent it from closing immediately",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("Program is now blocked. Press Ctrl+C to exit.")
			select {} // Block forever
		},
	}

	cmd.AddCommand(setCmd, getCmd, deleteCmd, existsCmd, blockCmd)

	return cmd
}
