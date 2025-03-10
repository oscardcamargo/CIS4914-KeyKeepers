package main

import (
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"io"
	"github.com/asynkron/protoactor-go/actor"
)

type NodeInfo struct {
	pid *actor.PID
	nodeID uint64
	address string
	name string
}

type NodeActor struct {
	nodeID		uint64 //Chord identifier (a hash)
	address 	string
	name		string
	data 		map[string]string //piece of the overall hashtable this node holds
	selfPID		*actor.PID //Proto.Actor identifier
	successor 	*NodeInfo //actor that would be queried by this node
	predecessor *NodeInfo //actor that would query this node
	fingerTable []*NodeInfo //list of actors at the predetermined locations on the ring

	//state and flag variables for keeping track of requests and responses
	findSuccessorResult *NodeInfo
	currentPredecessorInfo *NodeInfo
	awaitingJoinResponse bool
	awaitingSuccessorResponse bool
	awaitingPredecessorResponse bool
}

func (state *NodeActor) Receive(context actor.Context) {
	fmt.Printf("[SYSTEM]: Received a message: [%T]\n", context.Message())
	switch msg := context.Message().(type) {
		case *Initialize:
			state.handleInitialize(msg, context)
		case *StabilizeSelf:
			state.stabilize(context)
		case *DelayedStabilize:
		case *Notify:
			state.handleNotify(msg)
		case *RequestSuccessor:
			state.handleRequestSuccessor(msg.GetAddress(), context)
		case *RequestPredecessor:
			state.handleRequestPredecessor(context)
		case *NodeInfoMsg:
			state.handleResponse(msg, context)
	}
}

//maybe use goroutines here?
func (state *NodeActor) handleResponse(msg *NodeInfoMsg, context actor.Context) {
	if(state.awaitingJoinResponse) {
		successor_info := msg
		state.successor = &NodeInfo{
			pid: actor.NewPID(successor_info.GetAddress(), successor_info.GetName()),
			nodeID: successor_info.GetNodeID(),
			address: successor_info.GetAddress(),
			name: successor_info.GetName(),
		}
		
		//set the first index of finger table to successor
		state.fingerTable[1] = state.successor
		fmt.Printf("\t->[join]: Succesfully joined the ring. Successor has been set to [%v]\n", state.successor.name)
		//fmt.Printf("[SYSTEM]: Your PID is <%v>. Your successor's is <%v>", state.selfPID, state.successor.pid)
		state.awaitingJoinResponse = false
	}

	if(state.awaitingSuccessorResponse) {
		//update the findSuccessorResult
		state.findSuccessorResult = &NodeInfo{
			pid: actor.NewPID(msg.GetAddress(), msg.GetName()),
			nodeID: msg.GetNodeID(),
			address: msg.GetAddress(),
			name: msg.GetName(),
		}

		state.awaitingSuccessorResponse = false
	}

	//continue stabilize
	if(state.awaitingPredecessorResponse) {
		state.currentPredecessorInfo = &NodeInfo{
			pid: actor.NewPID(msg.GetAddress(), msg.GetName()),
			nodeID: msg.GetNodeID(),
			address: msg.GetAddress(),
			name: msg.GetName(),
		}
			//check if that node is between you and your successor.
			//if it is, we just found a closer successor, so update it.
			if isBetween(state.currentPredecessorInfo.nodeID, state.nodeID, state.successor.nodeID) {
				state.successor = state.currentPredecessorInfo
				fmt.Printf("\t->[stabilize]: updated your successor to: %v\n", state.successor.name)
			}
			//fmt.Println("Sending notify message to ", state.successor.name)
			context.Send(state.successor.pid, &Notify{Address: state.address, Name: state.name})
		state.awaitingPredecessorResponse = false
	}
}

func (state *NodeActor) handleRequestPredecessor(context actor.Context) {
	//this node was prompted for its predecessor by another node. It MUST respond back!
	if state.predecessor == nil {
		fmt.Println("Node was requested for its predecessor, but its NIL. Likely the first node in network -- responding with self!")
		context.Respond(&NodeInfoMsg{
			NodeID: state.nodeID,
			Address: state.address,
			Name: state.name,
		})
	} else {
		context.Respond(&NodeInfoMsg{
			NodeID: state.predecessor.nodeID,
			Address: state.predecessor.address,
			Name: state.predecessor.name,
		})
	}
}

func (state *NodeActor) handleRequestSuccessor(address string, context actor.Context) {
	//this node was prompted for the successor of the node specified by the msg address. It MUST respond back!
	//TODO: this may be changed so that only the nodeID is required in the message, instead of address AND name.
	nodeInfo := state.find_successor(consistent_hash(address), context)
	context.Respond(&NodeInfoMsg{
		NodeID: nodeInfo.nodeID, 
		Address: nodeInfo.address, 
		Name: nodeInfo.name,
	})
	//fmt.Println("Responded to FindSuccessor back with node ", nodeInfo.address)
}

func (state *NodeActor) handleInitialize(msg *Initialize, context actor.Context) {
	//Initialize basic actor properties.
	state.initialize(msg.GetAddress(), msg.GetName())
	fmt.Printf("[SYSTEM]: Your ID is <%v>.\n", state.nodeID)
	if msg.GetRemoteAddress() != "" && msg.GetRemoteName() != "" {
		//send join request to the provided remote node
		join_node := actor.NewPID(msg.GetRemoteAddress(), msg.GetRemoteName())
		state.join(join_node, context)
		return
	}
	//You can only get get here if you weren't provded a remote node.
	//Therefore you're the first node, so set some initial values here.
	state.successor = &NodeInfo{
		pid: actor.NewPID(state.address, state.name),
		nodeID: state.nodeID,
		address: state.address,
		name: state.name,
	}
	state.fingerTable[1] = state.successor
	fmt.Println("[SYSTEM]: You are the first node!")
}

func (state *NodeActor) handleNotify(msg *Notify) {
	target_PID := actor.NewPID(msg.GetAddress(), msg.GetName())
	state.notify(&NodeInfo{
		pid: target_PID,
		nodeID: consistent_hash(msg.GetAddress()),
		address: msg.GetAddress(),
		name: msg.GetName(),
	})
}

func (state *NodeActor) initialize(address string, name string) {
	state.address = address
	state.name = name
	state.nodeID = consistent_hash(address)
	state.data = make(map[string]string)
	state.selfPID = actor.NewPID(address, name)
	state.successor = nil
	state.predecessor = nil
	state.fingerTable = make([]*NodeInfo, 64)
	state.fingerTable[0] = nil
}

// node attempting to join the ring calls this
//GO
func (state *NodeActor) join(node *actor.PID, context actor.Context) {
	fmt.Println("[join]: This node is attempting to join node: ", node)
	state.predecessor = nil

	//call find successor on the node your joining, passing yourself in
	state.awaitingJoinResponse = true
	context.Request(node, &RequestSuccessor{Address: state.address, Name: state.name})
}

//GO
func (state *NodeActor) stabilize(context actor.Context) {
	//we need to get the predecessor of this node's successor
	//must be done with message passing

	//We need to make it a NodeInfoMsg here because that is what is returned by the RequestPredecessor

	// Special case: If your successor is yourself, then this is the only node.
	if state.successor.pid.String() == state.selfPID.String() {
		if state.predecessor != nil && state.predecessor.pid.String() != state.selfPID.String() {
		state.successor = state.predecessor
        fmt.Printf("\t->[stabilize]: updated successor to: %v\n", state.successor.name)
		}
		if state.predecessor == nil {
			context.Send(state.successor.pid, &Notify{Address: state.address, Name: state.name})
		}
	} else {
		//NOT the only node in the ring
		state.awaitingPredecessorResponse = true
		context.Request(state.successor.pid, &RequestPredecessor{})

		//i hate this
	}
}

//GO
//accepts 64-bit id hash to identify node
func (state *NodeActor) find_successor(id uint64, context actor.Context) *NodeInfo {
	//finding successor of provided id
	fmt.Printf("[find_successor]: started to find successor of %v\n", id)

	// if your successor is yourself, youre the only one in the ring
	//dealing with the first node in the ring, just return its successor
	if state.successor.name == state.name {
		fmt.Printf("\t->[find_sucessor]: Looks like only one node in ring, so returning successor\n")
		return state.successor
	}

	if isBetween(id, state.nodeID, state.successor.nodeID) {
		fmt.Printf("\t->[find_successor]: ID is between node and its successor, returning")
		return state.successor
	} else {
		//get the closest preceding node and ask it to find the successor of id
		node_info := state.closest_preceding_node(id)
		fmt.Printf("\t->[find_successor]: Found the closest preceding node for [%v]: %v\n", id, node_info.name)

		state.awaitingSuccessorResponse = true;
		context.Request(node_info.pid, &RequestSuccessor{Address: state.address, Name: state.name})
	}
	return state.findSuccessorResult
}

func (state *NodeActor) closest_preceding_node(id uint64) *NodeInfo {
	fmt.Println("\t->[closest_prec_node]: Started to look for closest preceding node of id ", id)

	for i := 63; i > 0; i-- {
		node := state.fingerTable[i]

		if node == nil {
			continue
		}

		if isBetween(node.nodeID, state.nodeID, id) {
			fmt.Println("\t->[closest_prec_node]: Found a preceding node! Its: ", node.nodeID)
			return node
		}
	}
	node_info := new(NodeInfo)
	node_info = &NodeInfo{
		nodeID: state.nodeID,
		address: state.address,
		name: state.name,
	}
	fmt.Printf("Could not find result in finger table. Returning self instead: [%v]\n", node_info)
	return node_info
}

//parameter node thinks its THIS node's predecessor
func (state *NodeActor) notify(node_info *NodeInfo) {
	if state.predecessor == nil || isBetween(node_info.nodeID, state.predecessor.nodeID, state.nodeID) {
		state.predecessor = node_info
		fmt.Printf("\t->[notify]: Updated predecessor to: %v\n", state.predecessor.name)
	}
}

func consistent_hash(str string) uint64 {
	//assuming m = 64 for this implementation of Chord
	hash := sha1.New()
	io.WriteString(hash, str)
	result := hash.Sum(nil)
	return binary.BigEndian.Uint64(result)
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