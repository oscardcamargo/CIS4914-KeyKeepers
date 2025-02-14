package main

import (
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"io"
	"time"

	"github.com/asynkron/protoactor-go/actor"
)

type NodeInfo struct {
	pid *actor.PID
	nodeID uint64
	address string
}

type NodeActor struct {
	nodeID		uint64 //Chord identifier (a hash)
	address 	string
	name		string
	data 		map[string]string //piece of the overall hashtable this node holds
	selfPID		*actor.PID //Proto.Actor identifier
	successor 	*NodeInfo //actor that would be queried by this node
	predecessor *NodeInfo //actor that would query this node
	//fingerTable []*actor.PID //list of actors at the predetermined locations on the ring
}

func (state *NodeActor) Receive(context actor.Context) {
	//fmt.Println("RECEIVED MESSAGE OF TYPE: ", context.Message())
	switch msg := context.Message().(type) {
		case *Message:
			fmt.Printf("\nGot a message: %v\n", msg.Text)
		case *Initialize:
			state.address = msg.GetAddress()
			state.name = msg.GetName()
			state.nodeID = consistent_hash(msg.GetAddress())
			state.data = make(map[string]string)
			state.selfPID = actor.NewPID(msg.GetAddress(), msg.GetName())
			state.successor = nil
			state.predecessor = nil
		case *Join:
			target_PID := actor.NewPID(msg.GetAddress(), msg.GetName())
			state.join(target_PID, context)
		case *FindSuccessor:
			//target_PID := actor.NewPID(msg.GetAddress(), msg.GetName())
			nodeInfo := state.find_successor(consistent_hash(msg.GetAddress()), context)

			context.Respond(&Message{Text: nodeInfo.address})
	}
}

func (state *NodeActor) join(node *actor.PID, context actor.Context) {
	//get
	fmt.Println("IN JOIN")
	state.predecessor = nil
	future := context.RequestFuture(node, &FindSuccessor{Address: state.address, Name: state.name}, 1*time.Second)
	result, err := future.Result()
	if err != nil {
		fmt.Printf("\n[Something went horribly wrong]: %v\n", err)
	}
	fmt.Println("some results: ", result)
}

//accepts 64-bit id hash to identify node
func (state *NodeActor) find_successor(id uint64, context actor.Context) *NodeInfo {
	//finding successor of provided id
	if (state.nodeID < id) && (id < state.successor.nodeID) {
		return state.successor
	} else {
		nodeInfo := state.closestPrecedingNode(id)
		future := context.RequestFuture(nodeInfo.pid, &FindSuccessor{Address: state.address, Name: state.name}, 1*time.Second)
		result, err := future.Result()
		if err != nil {
			fmt.Printf("\n[Something went horribly wrong]: %v\n", err)
		}
		fmt.Println("some results: ", result)
	}

	return nil
}

func (state *NodeActor) closestPrecedingNode(id uint64) *NodeInfo {
	return nil
}

func consistent_hash(str string) uint64 {
	hash := sha1.New()
	io.WriteString(hash, str)
	result := hash.Sum(nil)
	fmt.Printf("\nHASH RESULT:[%v]\n", binary.BigEndian.Uint64(result))
	return binary.BigEndian.Uint64(result)
}
