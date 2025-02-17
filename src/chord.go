// Chord protocol implementation based on the research paper
// Chord: A Scalable Peer-to-peer Lookup Protocol for Internet Applications

package main

import (
	"github.com/asynkron/protoactor-go/actor"
)

// Represents a node from a Chord ring
type chordNode struct {
	fingerTable []*actor.PID
	successor   *actor.PID
	predecessor *actor.PID
}

// NewChordNode default constructor
func NewChordNode() actor.Actor {
	return &chordNode{
		fingerTable: make([]*actor.PID, 0),
		successor:   nil,
		predecessor: nil,
	}
}

func (state *chordNode) findSuccessor() {
	
}

func (state *chordNode) closestPrecedingNode() {

}

func (state *chordNode) create() {

}

func (state *chordNode) join() {

}

func (state *chordNode) stabilize() {

}

func (state *chordNode) notify() {

}

func (state *chordNode) fixFingers() {

}

func (state *chordNode) checkPredecessor() {

}

// Receive handles message passing
func (state *chordNode) Receive(context actor.Context) {
	switch context.Message().(type) {
	case actor.Started:
		state.create()
	}
}