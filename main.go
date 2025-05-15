package main

import (
	"fmt"

	"github.com/computer-technology-team/distributed-kvstore/cmd"
)

func main() {
	fmt.Println("Initializing root command")
	cmd := cmd.NewRootCmd()
	cmd.Execute()
}
