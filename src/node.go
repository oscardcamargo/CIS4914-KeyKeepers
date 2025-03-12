package main


import (
	"github.com/asynkron/protoactor-go/actor"
	"fmt"
	"io"
	"crypto/sha1"
	"encoding/binary"
	"math/rand/v2"
	"slices"
)

// m-bit identifier space
const m int = 10

type NodeInfo struct {
	name string
	address string
	nodeID uint64
}

type Node struct {
	name string
	address string
	nodeID uint64
	nodePID *actor.PID
	nextFingerIndex int
	successor *NodeInfo
	predecessor *NodeInfo
	fingerTable []*NodeInfo

	awaitingJoin bool
	awaitingStabilize bool
	awaitingFixFingers bool
}

func (n* Node) Receive(context actor.Context) {
	//fmt.Printf("%T\n", context.Message())
	switch message := context.Message().(type) {
	case *Initialize:
		n.handleInitialize(message, context)
	case *StabilizeSelf:
		n.stabilize(context)
	case *FixFingers:
		n.fixFingers(context)
	case *RequestSuccessor:
		n.find_successor(message.GetNodeID(), context)
	case *RequestPredecessor:
		n.handleRequestPredecessor(context)
	case *Notify:
		n.notify(context)
	case *Response:
		n.handleResponse(context)
	case *InfoCommand:
		n.printInfo()
	case *FingersCommand:
		n.printFingers()
	}
}

func (n* Node) handleInitialize(parameters *Initialize, context actor.Context) {
	n.name = parameters.GetName()
	n.address = parameters.GetAddress()
	n.nodeID = consistent_hash(n.address)
	n.nodePID = context.Self()
	n.nextFingerIndex = 0
	//n.fingerTable = make([]*NodeInfo, m)
	n.awaitingJoin = false
	
	//check if first node and do the appropriate stuff
	if parameters.GetRemoteAddress() == "" && parameters.GetRemoteName() == "" {
		n.successor = &NodeInfo{name: n.name, address: n.address, nodeID: n.nodeID}
		n.fingerTable = slices.Repeat([]*NodeInfo{n.successor}, m)
		n.predecessor = n.successor
		fmt.Println("Ready.")
	} else {
		to_join := actor.NewPID(parameters.GetRemoteAddress(), parameters.GetRemoteName())
		n.join(to_join, context)
		// dont put anything past here
	}

	fmt.Println("ID: ", n.nodeID)

}

func (n *Node) handleRequestPredecessor(context actor.Context) {
	if n.predecessor == nil {
		context.Respond(&Response{
			Name: n.name,
			Address: n.address,
			NodeID: n.nodeID,
		})
		return
	}
	context.Respond(&Response{
		Name: n.predecessor.name,
		Address: n.predecessor.address,
		NodeID: n.predecessor.nodeID,
	})
}

func (n *Node) handleResponse(context actor.Context) {
	response := context.Message().(*Response)

	reponseNodeInfo := &NodeInfo{
		name: response.GetName(),
		address: response.GetAddress(),
		nodeID: response.GetNodeID(),
	}

	if n.awaitingJoin {
		//finish the join process with the newly acquired successor
		n.successor = reponseNodeInfo
		n.fingerTable = slices.Repeat([]*NodeInfo{{
			name: n.successor.name, 
			address: n.successor.address, 
			nodeID: n.successor.nodeID,
			}}, m)
			fmt.Println("Ready.")
		n.awaitingJoin = false 
	}

	if n.awaitingStabilize {
		//fmt.Println("successor.predecessor = ", response.GetName())
		if isBetween(response.GetNodeID(), n.nodeID, n.successor.nodeID) {
			n.successor = reponseNodeInfo
			n.fingerTable[0] = n.successor
			fmt.Printf("\t->Updated successor to <%s>\n", n.successor.name)
		}
		succPID := actor.NewPID(n.successor.address, n.successor.name) 
		context.Send(succPID, &Notify{
			Name: n.name,
			Address: n.address,
			NodeID: n.nodeID,
		})
		n.awaitingStabilize = false
	}

	if n.awaitingFixFingers {
		n.fingerTable[n.nextFingerIndex] = reponseNodeInfo
		//fmt.Printf("Fixed finger[%d] = <%s>\n", n.nextFingerIndex, response.GetName())
		n.awaitingFixFingers = false
	}
}

func (n* Node) join(toJoin *actor.PID, context actor.Context) {
	n.predecessor = nil
	n.awaitingJoin = true
	context.Request(toJoin, &RequestSuccessor{NodeID: n.nodeID})
}

func (n *Node) stabilize(context actor.Context) {
	succPID := actor.NewPID(n.successor.address, n.successor.name) 
	n.awaitingStabilize = true
	context.Request(succPID, &RequestPredecessor{})
}

func (n *Node) notify(context actor.Context) {
	message := context.Message().(*Notify)
	//fmt.Println("Message: ", message)
	if n.predecessor == nil || isBetween(message.GetNodeID(), n.predecessor.nodeID, n.nodeID) {
		n.predecessor = &NodeInfo{
			name: message.GetName(),
			address: message.GetAddress(),
			nodeID: message.GetNodeID(),
		}
		fmt.Printf("\t->Updated predecessor to <%s>\n", n.predecessor.name)
	}
}

//context here is about the original sender
func (n *Node) find_successor(id uint64, context actor.Context) {
	if(n.name == n.successor.name) {
		context.Respond(&Response{
			Name: n.successor.name, 
			Address: n.successor.address,
			NodeID: n.successor.nodeID,
		})
	} else if isBetween(id, n.nodeID, n.successor.nodeID + 1) {
		context.Respond(&Response{
			Name: n.successor.name, 
			Address: n.successor.address,
			NodeID: n.successor.nodeID,
		})
	} else {
		u := n.closest_preceeding_node(id)
		context.Forward(u)
	}
}

/*
=== PSEUDOCODE ===
n.fix_fingers():
	i = random index > 1 into finger[];
	finger[i].node = find_successor(finger[i].start);
==================
Notes:
* look in handleResponse() for rest of function
* chord paper uses indices [1, m] -- we use indices [0, m-1]
* finger[k] = first node succeeding id n + 2^k
*/
func (n *Node) fixFingers(context actor.Context) {
	n.nextFingerIndex = rand.IntN(m)
	start := (n.nodeID + (1 << n.nextFingerIndex)) % (1 << m)
	n.awaitingFixFingers = true
	context.Request(context.Self(), &RequestSuccessor{NodeID: start})
}

func (n *Node) closest_preceeding_node(id uint64) *actor.PID {
	for i := m-1; i >= 0; i-- {
		if n.fingerTable[i] == nil {
			break
		}
		if isBetween(n.fingerTable[i].nodeID, n.nodeID, id) {
			return actor.NewPID(n.fingerTable[i].address, n.fingerTable[i].name)
		}
	}
	return n.nodePID
}

func consistent_hash(str string) uint64 {
	hash := sha1.New()
	io.WriteString(hash, str)
	result := hash.Sum(nil)
	value := binary.BigEndian.Uint64(result)
	if m < 64 {
        // Mask off the upper bits to keep only m bits
        value = value & ((1 << uint(m)) - 1)
    }

	return value
}

//used for checking if id x is between (a, b)
func isBetween(x, a, b uint64) bool {
	//if a < b then check regularly:
	if (a < b) {
		return (a < x) && (x < b)
	} else {
		return a < x || x < b
	}
}

func (n *Node) printInfo() {
	fmt.Println("========== INFO ==========")
	fmt.Printf("Name: %s\nID: %d\nPID: %v\nSuccessor: %s (%d)\nPredecessor: %s (%d)\n", n.name, n.nodeID, n.nodePID, n.successor.name, n.successor.nodeID, n.predecessor.name, n.predecessor.nodeID)
	fmt.Println("==========================")
}

func (n *Node) printFingers() {
	fmt.Println("========= FINGERS =========")
	for i:= range n.fingerTable {
		fmt.Printf("[%d] = <%s> (%d)\n", i, n.fingerTable[i].name, n.fingerTable[i].nodeID)
	}
	fmt.Println("===========================")
}