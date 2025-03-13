package node

import (
	"KeyKeeper/message"
	"fmt"
	"github.com/asynkron/protoactor-go/actor"
	"strconv"
	"testing"
	"time"
)

func TestJoinTenNodes(t *testing.T) {
	// Initializes the actor system
	system := actor.NewActorSystem()
	context := system.Root

	// stores the process id of all the nodes in the network
	pids := make([]*actor.PID, 10)

	// creates 10 nodes
	for i := 0; i < 10; i++ {
		props := actor.PropsFromProducer(func() actor.Actor { return &Node{} })

		pid, err := system.Root.SpawnNamed(props, strconv.Itoa(i))
		if err != nil {
			fmt.Printf("[Actor spawn failed]: %v\n", err)
		}

		pids[i] = pid

		// Join using the pid instead of the IP address
		if i == 0 {
			context.Send(pid, &message.Initialize{Name: pid.GetId(), Address: "nonhost"})
		} else {
			context.Send(pids[i-1], &message.Initialize{Name: pids[i-1].GetId(), Address: "nonhost", RemoteName: pid.GetId(), RemoteAddress: "nonhost"})
		}
		time.Sleep(2500 * time.Millisecond)
	}

	for i := 0; i < 10; i++ {
		time.Sleep(2500 * time.Millisecond)
		system.Root.Send(pids[i], &message.StabilizeSelf{})
		time.Sleep(2500 * time.Millisecond)
		context.Send(pids[i], &message.FixFingers{})
		time.Sleep(2500 * time.Millisecond)
		context.Send(pids[i], &message.InfoCommand{})
		time.Sleep(2500 * time.Millisecond)
		context.Send(pids[i], &message.FingersCommand{})
	}

}
