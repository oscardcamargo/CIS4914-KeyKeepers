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

	// go run . <hostname> <port> <name> <remote_hostname> <remote_port> <remote_name>
	if len(os.Args) < 4 {
		fmt.Println("Bad command line arguments")
		return
	} else {
		hostname = os.Args[1]
		port, _ = strconv.Atoi(os.Args[2])
	}

	//Create the actor system on this network.
	system := actor.NewActorSystem()
	server := remote.NewRemote(system, remote.Configure(hostname, port))
	server.Start()

	//Spawn the first node
	props := actor.PropsFromProducer(func() actor.Actor { return &NodeActor{} })
	node_name := os.Args[3]
	node_pid, err := system.Root.SpawnNamed(props, node_name)
	if err != nil {
		fmt.Printf("[Actor spawn failed]: %v\n", err)
	}
	//fmt.Println(node_pid.GetAddress(), node_pid.GetId())
	system.Root.Send(node_pid, &Initialize{Address: node_pid.GetAddress(), Name: node_pid.GetId()})

	//This means an existing Chord node was provided
	// [node2] --(connect)--> [node1]
	if len(os.Args) == 7 {
		var remote_hostname = os.Args[4]
		if remote_hostname == "localhost" {
			remote_hostname = "127.0.0.1"
		}
		remote_address := fmt.Sprintf("%s:%s", remote_hostname, os.Args[5])
		remote_name := os.Args[6]

		//send node2 a join message, passing node1's information
		//node2 will call its join() function upon receiption of this message
		system.Root.Send(node_pid, &Join{Address: remote_address, Name: remote_name})

		select {}
	}
	system.Root.Send(node_pid, &FirstNode{})
	//used to keep the application running
	//TODO: make a more graceful way of keeping it up and shutting it down
	select {}
}
