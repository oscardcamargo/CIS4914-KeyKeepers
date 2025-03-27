package main

import (
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"io"
	"math/rand/v2"
	"slices"

	"github.com/asynkron/protoactor-go/actor"
	"os"
	"time"
)

// m-bit identifier space
const m int = 10

type NodeInfo struct {
	name    string
	address string
	nodeID  uint64
}

type Node struct {
	name            string
	address         string
	nodeID          uint64     //Chord identifier (a hash)
	nodePID         *actor.PID //Proto.Actor identifier
	nextFingerIndex int
	successor       *NodeInfo
	predecessor     *NodeInfo
	fingerTable     []*NodeInfo //list of actors at the predetermined locations on the ring

	awaitingJoin             bool
	awaitingStabilize        bool
	awaitingFixFingers       bool
	awaitingAcceptConnection bool
	awaitingChunkReceipt     bool

	//lists of active file transfers. Tracked by file name
	incomingFileTransfers map[string]*transfer
	outgoingFileTransfers map[string]*transfer
}

type transfer struct {
	fileName         string
	lineRange        []Range
	peerPID          *actor.PID
	localFile        *os.File // Can be read or write depending on transfer direction
	lastTransferTime time.Time
	retryCount       int // Only used by Outgoing transfers
}

func (n *Node) Receive(context actor.Context) {
	//fmt.Printf("%T\n", context.Message())
	switch message := context.Message().(type) {
	case *Initialize:
		n.handleInitialize(message, context)
	case *StabilizeSelf:
		n.stabilize(context)
	case *FixFingers:
		n.fixFingers(context)
	case *RequestSuccessor:
		n.find_successor(message, context)
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
	case *StartTransfer:
		n.startTransfer(message, context)
	case *FileChunk:
		n.handleFileChunk(message, context)
	}
}

func (n *Node) handleInitialize(parameters *Initialize, context actor.Context) {
	n.name = parameters.GetName()
	n.address = parameters.GetAddress()
	n.nodeID = consistent_hash(n.address)
	n.nodePID = context.Self()
	n.nextFingerIndex = 0
	//n.fingerTable = make([]*NodeInfo, m)
	n.awaitingJoin = false
	n.awaitingAcceptConnection = false
	n.awaitingChunkReceipt = false
	n.incomingFileTransfers = make(map[string]*transfer)
	n.outgoingFileTransfers = make(map[string]*transfer)

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
			Name:        n.name,
			Address:     n.address,
			NodeID:      n.nodeID,
			ResponseFor: ResponseFor_STABILIZE,
		})
		return
	}
	context.Respond(&Response{
		Name:        n.predecessor.name,
		Address:     n.predecessor.address,
		NodeID:      n.predecessor.nodeID,
		ResponseFor: ResponseFor_STABILIZE,
	})
}

func (n *Node) handleResponse(context actor.Context) {
	response := context.Message().(*Response)

	reponseNodeInfo := &NodeInfo{
		name:    response.GetName(),
		address: response.GetAddress(),
		nodeID:  response.GetNodeID(),
	}

	// TODO: Should these confirm if they came from the expected node?
	if n.awaitingJoin && response.ResponseFor == ResponseFor_JOIN {
		//finish the join process with the newly acquired successor
		n.successor = reponseNodeInfo
		n.fingerTable = slices.Repeat([]*NodeInfo{{
			name:    n.successor.name,
			address: n.successor.address,
			nodeID:  n.successor.nodeID,
		}}, m)
		fmt.Println("Ready.")
		n.awaitingJoin = false
	}

	if n.awaitingStabilize && response.ResponseFor == ResponseFor_STABILIZE {
		//fmt.Println("successor.predecessor = ", response.GetName())
		if isBetween(response.GetNodeID(), n.nodeID, n.successor.nodeID) {
			n.successor = reponseNodeInfo
			n.fingerTable[0] = n.successor
			fmt.Printf("\t->Updated successor to <%s>\n", n.successor.name)
		}
		succPID := actor.NewPID(n.successor.address, n.successor.name)
		context.Send(succPID, &Notify{
			Name:    n.name,
			Address: n.address,
			NodeID:  n.nodeID,
		})
		n.awaitingStabilize = false
	}

	if n.awaitingFixFingers && response.ResponseFor == ResponseFor_FIX_FINGERS {
		n.fingerTable[n.nextFingerIndex] = reponseNodeInfo
		//fmt.Printf("Fixed finger[%d] = <%s>\n", n.nextFingerIndex, response.GetName())
		n.awaitingFixFingers = false
	}

	// Handle the confirmation of a transfer start or a chunk confirmation.
	if n.awaitingAcceptConnection && response.ResponseFor == ResponseFor_START_TRANSFER ||
		(n.awaitingChunkReceipt && response.ResponseFor == ResponseFor_CHUNK_RECEIPT) {
		var instance *transfer
		exists := false
		for _, value := range n.outgoingFileTransfers {
			if value.peerPID.Id == response.Name {
				exists = true
				instance = value
				break
			}
		}

		if exists && (response.Status == Status_ERROR || response.Status == Status_DECLINE) {
			if response.Status == Status_DECLINE {
				fmt.Println("Peer declined database transfer.")
			} else if response.Status == Status_ERROR {
				fmt.Printf("Peer returned an error when starting database transfer: %v\n", response.Message)
			}

			if instance.retryCount < 3 {
				instance.retryCount++
				n.startDatabaseTransfer(instance.peerPID, context, instance.lineRange)
			}
		} else if response.Status == Status_OK {
			// Send next chunk
			n.sendChunk(instance, context)
		}
	}

	n.cleanUpTimedOutTransfers(n.incomingFileTransfers)
	n.cleanUpTimedOutTransfers(n.outgoingFileTransfers)
}

func (n *Node) join(toJoin *actor.PID, context actor.Context) {
	n.predecessor = nil
	n.awaitingJoin = true
	message := &RequestSuccessor{
		NodeID:      n.nodeID,
		ResponseFor: ResponseFor_JOIN}
	context.Request(toJoin, message)
}

func (n *Node) stabilize(context actor.Context) {
	if n.successor == nil {
		return
	}
	succPID := actor.NewPID(n.successor.address, n.successor.name)
	n.awaitingStabilize = true
	context.Request(succPID, &RequestPredecessor{})
}

func (n *Node) notify(context actor.Context) {
	message := context.Message().(*Notify)
	//fmt.Println("Message: ", message)
	if n.predecessor == nil || isBetween(message.GetNodeID(), n.predecessor.nodeID, n.nodeID) {
		n.predecessor = &NodeInfo{
			name:    message.GetName(),
			address: message.GetAddress(),
			nodeID:  message.GetNodeID(),
		}
		fmt.Printf("\t->Updated predecessor to <%s>\n", n.predecessor.name)
	}
}

// context here is about the original sender
func (n *Node) find_successor(message *RequestSuccessor, context actor.Context) {
	if n.name == n.successor.name {
		context.Respond(&Response{
			Name:        n.successor.name,
			Address:     n.successor.address,
			NodeID:      n.successor.nodeID,
			ResponseFor: message.ResponseFor,
		})
	} else if isBetween(message.NodeID, n.nodeID, n.successor.nodeID+1) {
		context.Respond(&Response{
			Name:        n.successor.name,
			Address:     n.successor.address,
			NodeID:      n.successor.nodeID,
			ResponseFor: message.ResponseFor,
		})
	} else {
		u := n.closest_preceeding_node(message.NodeID)
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
	n.nextFingerIndex = rand.IntN(m) //[0, m)
	start := (n.nodeID + (1 << n.nextFingerIndex)) % (1 << m)
	n.awaitingFixFingers = true
	message := &RequestSuccessor{
		NodeID:      start,
		ResponseFor: ResponseFor_FIX_FINGERS}
	context.Request(context.Self(), message)
}

func (n *Node) closest_preceeding_node(id uint64) *actor.PID {
	for i := m - 1; i >= 0; i-- {
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

// used for checking if id x is between (a, b)
func isBetween(x, a, b uint64) bool {
	//if a < b then check regularly:
	if a < b {
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
	fmt.Println("========= FINGERS ========")
	for i := range n.fingerTable {
		fmt.Printf("[%d] = <%s> (%d)\n", i, n.fingerTable[i].name, n.fingerTable[i].nodeID)
	}
	fmt.Println("==========================")
}

func (n *Node) startTransfer(message *StartTransfer, context actor.Context) {
	// TODO in future: Should this node confirm that the incoming database lines belong to this node?
	// TODO in future: Should this also include a hash of the file to confirm after?

	response := &Response{
		Name:        n.name,
		Address:     n.address,
		NodeID:      n.nodeID,
		ResponseFor: ResponseFor_START_TRANSFER,
	}

	var err error
	file, err := openFileWrite(message.GetFilename())
	if err != nil {
		fmt.Printf("[SYSTEM]: Error opening file %v: %v", message.GetFilename(), err)
		return
	}

	var newRangeSlice []Range
	for _, rng := range message.Ranges {
		newRangeSlice = append(newRangeSlice, Range{start: int(rng.Start), end: int(rng.End)})
	}

	newFileTransfer := transfer{
		fileName:         message.GetFilename(),
		lineRange:        newRangeSlice,
		peerPID:          actor.NewPID(context.Sender().Address, context.Sender().Id),
		localFile:        file,
		lastTransferTime: time.Now(),
	}
	n.incomingFileTransfers[message.GetFilename()] = &newFileTransfer

	response.Status = Status_OK
	context.Respond(response)

	fmt.Printf("[SYSTEM]: File Transfer request initialized by Address: %v ID:%v \n", context.Sender().Address, context.Sender().Id)
}

func (n *Node) handleFileChunk(message *FileChunk, context actor.Context) {
	instance, exists := n.incomingFileTransfers[message.GetFilename()]

	instance.lastTransferTime = time.Now()
	response := &Response{
		Name:        n.name,
		Address:     n.address,
		NodeID:      n.nodeID,
		ResponseFor: ResponseFor_START_TRANSFER,
	}

	if !exists {
		fmt.Printf("[SYSTEM]: Recieved a file chunk for a non-active file transfer\n")
		response.Status = Status_DECLINE
		context.Respond(response)
		return
	}
	if instance.peerPID.Id != context.Sender().Id {
		fmt.Printf("[SYSTEM]: File chunk for %v wasn't sent from expected sender. "+
			"Expected ID: %v, sender:%v, \n", instance.fileName, instance.peerPID, context.Sender().Id)
		response.Status = Status_DECLINE
		context.Respond(response)
		return
	}

	_, err := instance.localFile.Write(message.GetChunk())
	if err != nil {
		response.Status = Status_ERROR
		response.Message = err.Error()
		context.Respond(response)

		fmt.Printf("[SYSTEM]: Transfer canceled. Error writing chunk to local file: %v\n", err)
		err := n.incomingFileTransfers[message.GetFilename()].localFile.Close()
		if err != nil {
			fmt.Printf("[SYSTEM]: Error closing %v: %v\n", message.GetFilename(), err)
			return
		}

		err = os.Remove(message.GetFilename())
		if err != nil {
			fmt.Printf("[SYSTEM]: Error deleting file %v\n", err)
		}

		delete(n.incomingFileTransfers, message.GetFilename())

		return
	}

	if message.GetEndOf() {
		err := instance.localFile.Close()
		if err != nil {
			fmt.Printf("[SYSTEM]: Error closing %v: %v\n", message.GetFilename(), err)
			response.Status = Status_ERROR
			response.Message = err.Error()
			context.Respond(response)
			return
		}

		if !importDatabase(message.GetFilename(), n.incomingFileTransfers[message.GetFilename()].lineRange) {
			response.Status = Status_ERROR
			response.Message = "Failed to import file."
			context.Respond(response)
		} else {
			response.Status = Status_OK
			context.Respond(response)
		}

		delete(n.incomingFileTransfers, message.GetFilename())
		deleteDB(message.GetFilename())
		fmt.Printf("[SYSTEM] File Transfer finished successfully from Address: %v ID:%v\n", context.Sender().Address, context.Sender().Id)
		return
	}

	response.Status = Status_OK
	context.Respond(response)
}

func (n *Node) startDatabaseTransfer(peer *actor.PID, context actor.Context, rangeSlice []Range) {

	// Check to make sure the lines are in the database
	for _, rng := range rangeSlice {
		lineStart := rng.start
		lineEnd := rng.end
		if !rangeInDatabase(lineStart, lineEnd) {
			fmt.Printf("[SYSTEM]: Error: Initiated transfer for line(s) that aren't in the database\n")
			return
		}
	}

	exportName, err := exportDatabaseLines(rangeSlice)
	if err != nil {
		return
	}

	file, err := openFileRead(exportName)
	if err != nil {
		fmt.Println("[SYSTEM] Error opening file:", err)
		return
	}

	var protoRange []*ProtoRange
	for _, rng := range rangeSlice {
		protoRange = append(protoRange, &ProtoRange{Start: int32(rng.start), End: int32(rng.end)})
	}

	transferMessage := &StartTransfer{
		Filename: exportName,
		Ranges:   protoRange,
	}

	context.Request(peer, transferMessage)

	newFileTransfer := transfer{
		fileName:         exportName,
		lineRange:        rangeSlice,
		peerPID:          peer,
		localFile:        file,
		lastTransferTime: time.Now(),
	}

	n.outgoingFileTransfers[exportName] = &newFileTransfer
	n.awaitingAcceptConnection = true

	fmt.Printf("[SYSTEM] File Transfer initiated to ID: %v Address: %v\n", peer.Id, peer.Address)

}

func (n *Node) sendChunk(outTransfer *transfer, context actor.Context) {
	chunkSize := 1024 * 100 // 100kb
	buffer := make([]byte, chunkSize)
	var chunkMessage *FileChunk
	chunkMessage = &FileChunk{
		Filename: outTransfer.fileName,
	}

	size, err := outTransfer.localFile.Read(buffer)
	if err == io.EOF {
		// If there is nothing else to read, the transfer is complete. Delete the transfer.
		fmt.Printf("[SYSTEM]: File Transfer complete with ID: %v Address: %v\n",
			outTransfer.peerPID.Id, outTransfer.peerPID.Address)

		delete(n.outgoingFileTransfers, outTransfer.fileName)

		err := outTransfer.localFile.Close()
		if err != nil {
			return
		}
		deleteDB(outTransfer.fileName)

		return
	} else if err != nil {
		fmt.Printf("[SYSTEM]: Error reading chunk: %v\n", err)
		return
	}
	chunkMessage.Chunk = buffer[:size]
	if size < chunkSize {
		chunkMessage.EndOf = true

	}

	context.Request(outTransfer.peerPID, chunkMessage)
}

func (n *Node) cleanUpTimedOutTransfers(transfers map[string]*transfer) {
	for fileName, transfer := range transfers {
		if time.Since(transfer.lastTransferTime) > time.Minute {
			fmt.Printf("[SYSTEM]: Transfer of %v canceled. Timed out.\n", transfer.fileName)

			if err := transfer.localFile.Close(); err != nil {
				fmt.Printf("[SYSTEM]: Error closing %v: %v\n", transfer.fileName, err)
			}

			if err := os.Remove(fileName); err != nil {
				fmt.Printf("[SYSTEM]: Error deleting file %v\n", err)
			}

			delete(transfers, fileName)
		}
	}
}
