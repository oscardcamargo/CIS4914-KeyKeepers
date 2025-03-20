package main

import (
	"fmt"
	"github.com/asynkron/protoactor-go/actor"
	"strconv"
	"testing"
	"time"
)

func TestJoinFourNodes(t *testing.T) {
	// Initializes the actor system
	system := actor.NewActorSystem()
	context := system.Root

	// stores the process id of all the nodes in the network
	pids := make([]*actor.PID, 4)

	// creates 4 nodes
	for i := 0; i < 4; i++ {
		props := actor.PropsFromProducer(func() actor.Actor { return &Node{} })

		pid, err := system.Root.SpawnNamed(props, strconv.Itoa(i))
		if err != nil {
			fmt.Printf("[Actor spawn failed]: %v\n", err)
		}

		pids[i] = pid

		// Join using the pid instead of the IP address
		if i == 0 {
			context.Send(pid, &Initialize{Name: pid.GetId(), Address: "nonhost"})
		} else {
			context.Send(pids[i], &Initialize{Name: pids[i].GetId(), Address: "nonhost", RemoteName: pids[0].GetId(), RemoteAddress: "nonhost"})
		}
		time.Sleep(2 * time.Second)
	}

	for i := 0; i < 8; i++ {
		time.Sleep(2 * time.Second)
		system.Root.Send(pids[i%4], &StabilizeSelf{})
		time.Sleep(2 * time.Second)
		system.Root.Send(pids[i%4], &FixFingers{})
		time.Sleep(2 * time.Second)
		system.Root.Send(pids[i%4], &InfoCommand{})
		time.Sleep(2 * time.Second)
		system.Root.Send(pids[i%4], &FingersCommand{})
	}

}
