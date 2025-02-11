package main

import (
	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/remote"
	"fmt"
	"os"
	"strconv"
)

func main() {
	var hostname string
	var port int

	// go run node.go main.go <hostname> <port> <name> <remote_hostname> <remote_port> <remote_name>
	if len(os.Args) < 4 {
		fmt.Println("Bad command line arguments")
		return
	} else {
		hostname = os.Args[1]
		port, _ = strconv.Atoi(os.Args[2])
	}

	//Create the actor system for this one node and set up the remote networking
	system := actor.NewActorSystem()
	network := remote.NewRemote(system, remote.Configure(hostname, port))
	network.Start()

	props := actor.PropsFromProducer(func() actor.Actor { return makeNode(hostname, port)} )
	node_name := os.Args[3]
	node_pid, err := system.Root.SpawnNamed(props, node_name)
	if err != nil {
		fmt.Printf("[Actor spawn failed]: %v\n", err)
	}

	system.Root.Send(node_pid, &Message{Text: "Welcome to the actor network!"})

	fmt.Println(node_pid)

	//this means you started the process with the intent of joining an existing node
	if len(os.Args) == 7 {
		remote_address := fmt.Sprintf("%s:%s", os.Args[4], os.Args[5])
		remote_name := os.Args[6]

		fmt.Printf("Looks like you want to connect to [%v] a.k.a [%v].\n", remote_address, remote_name)
		remote_PID := actor.NewPID(remote_address, remote_name)

		txt := fmt.Sprintf("Hello from node [%v] a.k.a [%v]", remote_address, remote_name)
		system.Root.Send(remote_PID, &Message{Text: txt})
	}

	//used to keep the application running
	//TODO: make a more graceful way of keeping it up and shutting it down
	select {}
}
