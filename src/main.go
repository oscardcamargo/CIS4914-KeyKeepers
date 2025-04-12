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
		fmt.Println("[Main] Error getting network interfaces:", err)
		return "localhost"
	}

	counter := 1
	for _, iface := range interfaces {
		addrs, err := iface.Addrs()
		if err != nil {
			fmt.Println("[Main] Error getting interface addresses:", err)
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

// Returns the IP a connection based on a specific subnet
// Ex: "10.20.0.0/24" will return the IP address of a networking interface from the 10.20.0.x range.
func getSubnetIP(subnet string) string {
	_, ipnet, err := net.ParseCIDR(subnet)
	if err != nil {
		fmt.Printf("Invalid subnet: %v\n", err)
		return ""
	}

	interfaces, err := net.Interfaces()
	if err != nil {
		fmt.Println("[MAIN] Error getting network interfaces:", err)
		return "localhost"
	}

	for _, iface := range interfaces {
		/*
			// This is to skip down or loopback interfaces, but maybe we want those?
			// Maybe network interfaces come back up and loopback can connect to local nodes.
				if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
					continue
				}
		*/

		addrs, err := iface.Addrs()
		if err != nil {
			fmt.Println("[MAIN] Error getting interface addresses:", err)
			continue
		}

		for _, addr := range addrs {
			var ip net.IP

			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			// Only consider IPv4 addresses
			if ip == nil || ip.To4() == nil {
				continue
			}

			if ipnet.Contains(ip) {
				fmt.Println("Using listening IP: ", ip.String())
				return ip.String()
			}
		}
	}

	fmt.Printf("Could not find subnet %v, using localhost.", subnet)
	return "localhost"
}

func main() {
	// var testHash string = `163c47c4e45116fc184c41428ec0cbbd60d2bdacb8df759d6f744f252d6c1305`
	// hashResult, err := checkHash(testHash)

	// fmt.Println(hashResult)
	var hostname string
	var port int
	// go run . <hostname> <port> <name> <remote_hostname> <remote_port> <remote_name>
	if len(os.Args) < 4 {
		fmt.Println("Bad command line arguments")
		return
	}

	hostname = os.Args[1]
	// Check for special hostnames.
	// "connection#" to choose the #th network interface the computer has
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
	//or xxx.xxx.xxx.xxx/xx to choose an interface with the specific subnet
	subnetRegex := regexp.MustCompile(`/\d{1,2}$`)
	subnetMatch := subnetRegex.FindStringSubmatch(hostname)
	if subnetMatch != nil {
		hostname = getSubnetIP(hostname)
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
		fmt.Printf("[MAIN] Actor spawn failed: %v\n", err)
	}

	//These parameters will change if a bootstrap node was provided
	var remote_address = ""
	var remote_name = ""
	if len(os.Args) == 7 {

		remote_hostname := os.Args[4]
		if remote_hostname == "localhost" {
			remote_hostname = "127.0.0.1"
		}

		// Attempt to resolve hostname if the address is not an IP
		_ = net.ParseIP(remote_hostname)
		if net.ParseIP(remote_hostname) == nil {
			ips, err := net.LookupIP(remote_hostname)
			if err != nil {
				fmt.Println("[MAIN] Error resolving remote host:", err)
			}
			remote_hostname = ips[0].String()
			fmt.Printf("The IP address of %v resolved to %v\n", os.Args[6], remote_address)
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
		var interactable = true
		// Non-interactable environments (Such as docker containers) will error out if attempting to scanf
		_, err := fmt.Scanf("%s\n", &command)
		if err != nil && err.Error() == "EOF" {
			interactable = false
		}
		for command != "quit" {
			if interactable {
				_, err := fmt.Scanf("%s\n", &command)
				if err != nil {
					fmt.Println("[MAIN] Error scanning for commands:", err)
					continue
				}

				switch command {
				case "info":
					system.Root.Send(node_pid, &InfoCommand{})
				case "fingers":
					system.Root.Send(node_pid, &FingersCommand{})
				}
			}
			time.Sleep(200 * time.Millisecond)
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
