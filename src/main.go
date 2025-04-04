package main

import (
	"fmt"
	"net"
	"os"
	"regexp"
	"strconv"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/remote"
)

// Returns the IP of the specified network link.
// connectionNumber is which connection in the list to get. If 1, get the ip of the first network. If 2, gets the second, etc.
// Docker external is often 1, Docker internal is often 3
func getConnectionIP(connectionNumber int) string {
	interfaces, err := net.Interfaces()
	if err != nil {
		fmt.Println("Error:", err)
		return "localhost"
	}

	counter := 1
	for _, iface := range interfaces {
		addrs, err := iface.Addrs()
		if err != nil {
			fmt.Println("Error:", err)
			continue
		}

		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if ok && !ipNet.IP.IsLoopback() && ipNet.IP.To4() != nil {
				if counter != connectionNumber {
					counter = counter + 1
					continue
				}

				fmt.Println("Using listening IP:", ipNet.IP.String())
				return ipNet.IP.String()
			}
		}
	}

	fmt.Println("Could not find connection number, using localhost.")
	return "localhost"
}

func main() {
	// var testHash string = `4c3f9505b832a5a8bb22d5d339b1dfd4800d96d3ffec4a495fdc2274efa6601c`
	// var hashResult = checkHash(testHash)

	// fmt.Println(hashResult)
	var hostname string
	var port int
	// go run . <hostname> <port> <name> <remote_hostname> <remote_port> <remote_name>
	if len(os.Args) < 4 {
		fmt.Println("Bad command line arguments")
		return
	}

	hostname = os.Args[1]
	// Check for special hostname "connection#" to choose the #th network interface the computer has.
	connectionRegex := regexp.MustCompile(`^connection(\d+)$`)
	connectionMatch := connectionRegex.FindStringSubmatch(hostname)
	if connectionMatch != nil {
		connectionNumber, err := strconv.Atoi(connectionMatch[1])
		if err != nil {
			fmt.Println("Bad connection number")
			return
		}
		hostname = getConnectionIP(connectionNumber)
	}

	port, _ = strconv.Atoi(os.Args[2])

	fmt.Println("Arguments:")
	fmt.Println("This server's IP: ", os.Args[1])
	fmt.Println("This node's port: ", os.Args[2])
	fmt.Println("This node's name: ", os.Args[3])

	if len(os.Args) == 7 {
		fmt.Println("Target hostname: ", os.Args[4])
		fmt.Println("Target port: ", os.Args[5])
		fmt.Println("Target Name: ", os.Args[6])
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
	var remote_address = ""
	var remote_name = ""
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

	//used to keep the application running
	//TODO: make a more graceful way of keeping it up and shutting it down
	go func() {
		var command string
		for command != "quit" {
			_, err := fmt.Scanf("%s\n", &command)
			if err != nil {
				fmt.Println("Error:", err)
				continue
			}

			switch command {
			case "info":
				system.Root.Send(node_pid, &InfoCommand{})
			case "fingers":
				system.Root.Send(node_pid, &FingersCommand{})
			}
		}

		os.Exit(1)
	}()

	for {
		time.Sleep(2500 * time.Millisecond)
		system.Root.Send(node_pid, &StabilizeSelf{})
		time.Sleep(2500 * time.Millisecond)
		system.Root.Send(node_pid, &FixFingers{})
	}
}
