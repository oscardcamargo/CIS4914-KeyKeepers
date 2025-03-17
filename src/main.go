package main

import (
	"fmt"
	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/remote"
	"net"
	"os"
	"regexp"
	"strconv"
	"time"
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
	var hostname string
	var port int
	// go run . <hostname, IP, or connection#> <port> <name> <remote_hostname> <remote_port> <remote_name>
	if len(os.Args) < 4 {
		fmt.Println("Bad command line arguments")
		return
	} else {
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

		fmt.Println("Arguments:")
		fmt.Println("This server's IP: ", os.Args[1])
		fmt.Println("This node's port: ", os.Args[2])
		fmt.Println("This node's name: ", os.Args[3])

		if len(os.Args) == 7 {
			fmt.Println("Target hostname: ", os.Args[4])
			fmt.Println("Target port: ", os.Args[5])
			fmt.Println("Target Name: ", os.Args[6])
		}
		port, _ = strconv.Atoi(os.Args[2])
	}

	//Create the actor system on this network.
	system := actor.NewActorSystem()

	fmt.Println("Hostname: ", hostname)
	fmt.Println("Port: ", port)
	address := hostname + ":" + strconv.Itoa(port)
	// TODO: remote.WithAdvertisedHost is optional. Determine what this affects as this might mess with binding to all interfaces option (0.0.0.0)
	remoteConfig := remote.Configure(hostname, port, remote.WithAdvertisedHost(address))
	//remoteConfig := remote.Configure(hostname, port)
	server := remote.NewRemote(system, remoteConfig)
	server.Start()

	//Spawn the node
	props := actor.PropsFromProducer(func() actor.Actor { return &NodeActor{} })
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
	system.Root.Send(node_pid, &Initialize{Address: node_pid.GetAddress(), Name: node_pid.GetId(), RemoteAddress: remote_address, RemoteName: remote_name})
	//time.Sleep(2*time.Second)

	//used to keep the application running
	//TODO: make a more graceful way of keeping it up and shutting it down
	for {
		time.Sleep(5 * time.Second)
		system.Root.Send(node_pid, &StabilizeSelf{})
	}
}
