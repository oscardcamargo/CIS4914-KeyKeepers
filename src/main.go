package main

import (
	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/remote"
	"fmt"
	"os"
	"strconv"
	"time"
)


func main() {
  	// var testHash string = `4c3f9505b832a5a8bb22d5d339b1dfd4800d96d3ffec4a495fdc2274efa6601c`
	// var hashResult = checkHash(testHash)

	// fmt.Println(hashResult)
	var hostname string
	var port int
	// go run . <hostname> <port> <name> <remote_hostname> <remote_port> <remote_name>
	if len(os.Args) < 4 {
		fmt.Println("Bad command line arguments");
		return
	} else {
		hostname = os.Args[1]
		port, _ = strconv.Atoi(os.Args[2])
	}

	//Create the actor system on this network.
	system := actor.NewActorSystem()
	server := remote.NewRemote(system, remote.Configure(hostname, port))
	server.Start()

	//Spawn the node
	props := actor.PropsFromProducer(func() actor.Actor { return &Node{} })
	node_name := os.Args[3]
	node_pid, err := system.Root.SpawnNamed(props, node_name)
	if err != nil {
		fmt.Printf("[Actor spawn failed]: %v\n", err)
	}
	

	//These parameters will change if a bootstrap node was provided
	var remote_address string = ""
	var remote_name string = ""
	if len(os.Args) == 7 {

		remote_hostname := os.Args[4]
		if remote_hostname == "localhost" {
			remote_hostname = "127.0.0.1"
		}

		remote_address = fmt.Sprintf("%s:%s", remote_hostname, os.Args[5])
		remote_name = os.Args[6]
	}

	//Send the initialization message with the data required to set up the node properties
	system.Root.Send(node_pid, &Initialize{Name: node_pid.GetId(), Address: node_pid.GetAddress(), RemoteName: remote_name, RemoteAddress: remote_address})
	//time.Sleep(2*time.Second)

	// go routine to capture user input
	go func() {
		var command string
		for command != "quit" {
				_, err := fmt.Scanf("%s\n", &command)
			if err != nil {
				fmt.Println("Error:", err)
				continue
			}
	
			switch(command) {
			case "info":
				system.Root.Send(node_pid, &InfoCommand{})
			case "fingers":
				system.Root.Send(node_pid, &FingersCommand{})
			}
		}
	
		os.Exit(1)
	}()

	//for loop to periodically stabilize and fix fingers
	for {
		time.Sleep(2500*time.Millisecond)
		system.Root.Send(node_pid, &StabilizeSelf{})
		time.Sleep(2500*time.Millisecond)
		system.Root.Send(node_pid, &FixFingers{})
	}

	//TODO: make a graceful way of shutting down
	//Maybe a shutdown message sent to the node ?
}



