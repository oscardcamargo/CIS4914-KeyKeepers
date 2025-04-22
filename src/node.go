package main

import (
	"crypto/sha1"
	"database/sql"

	//"encoding/binary"
	"fmt"
	"github.com/asynkron/protoactor-go/actor"
	"io"
	"math/big"
	"math/rand/v2"
	"os"
	"slices"
	"time"
)

// m-bit identifier space
const m int = 160

type NodeInfo struct {
	name    string
	address string
	nodeID  *big.Int
}

type Node struct {
	name            string
	address         string
	nodeID          *big.Int   //Chord identifier (a hash)
	nodePID         *actor.PID //Proto.Actor identifier
	nextFingerIndex int
	successor       *NodeInfo
	predecessor     *NodeInfo
	fingerTable     []*NodeInfo //list of actors at the predetermined locations on the ring

	awaitingJoin                bool
	awaitingStabilize           bool
	awaitingFixFingers          bool
	awaitingAcceptConnection    bool
	awaitingChunkReceipt        bool
	awaitingPredPredForTransfer bool
	ongoingTransfer             bool
	newPredecessor              bool

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
	case *EndTransfer:
		n.handleEndTransfer()
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
	n.ongoingTransfer = false
	n.newPredecessor = false
	n.incomingFileTransfers = make(map[string]*transfer)
	n.outgoingFileTransfers = make(map[string]*transfer)

	//check if first node and do the appropriate stuff
	if parameters.GetRemoteAddress() == "" && parameters.GetRemoteName() == "" {
		n.successor = &NodeInfo{name: n.name, address: n.address, nodeID: new(big.Int).Set(n.nodeID)}
		n.fingerTable = make([]*NodeInfo, m)
		for i := range n.fingerTable {
			n.fingerTable[i] = n.successor
		}
		n.predecessor = &NodeInfo{name: n.name, address: n.address, nodeID: new(big.Int).Set(n.nodeID)}
		//dbInit()
		fmt.Println("Ready.")
	} else {
		to_join := actor.NewPID(parameters.GetRemoteAddress(), parameters.GetRemoteName())
		n.join(to_join, context)
		// dont put anything past here
	}

	fmt.Println("ID: ", n.nodeID.Text(16))

}

func (n *Node) handleRequestPredecessor(context actor.Context) {
	if n.predecessor == nil {
		context.Respond(&Response{
			Name:        n.name,
			Address:     n.address,
			NodeID:      n.nodeID.Text(16),
			ResponseFor: ResponseFor_STABILIZE,
		})
		return
	}
	context.Respond(&Response{
		Name:        n.predecessor.name,
		Address:     n.predecessor.address,
		NodeID:      n.predecessor.nodeID.Text(16),
		ResponseFor: ResponseFor_STABILIZE,
	})
}

func (n *Node) handleResponse(context actor.Context) {
	response := context.Message().(*Response)
	num := new(big.Int)
	num.SetString(response.GetNodeID(), 16)

	responseNodeInfo := &NodeInfo{
		name:    response.GetName(),
		address: response.GetAddress(),
		nodeID:  num,
	}

	// TODO: Should these confirm if they came from the expected node?
	if n.awaitingJoin && response.ResponseFor == ResponseFor_JOIN {
		//finish the join process with the newly acquired successor
		n.successor = responseNodeInfo
		n.fingerTable = slices.Repeat([]*NodeInfo{{
			name:    n.successor.name,
			address: n.successor.address,
			nodeID:  n.successor.nodeID, //MIGHT CAUSE ISSUES
		}}, m)
		fmt.Println("Ready.")
		//fmt.Println("successor: ", n.successor.name)
		context.Send(actor.NewPID(n.successor.address, n.successor.name), &Notify{
			Name:    n.name,
			Address: n.address,
			NodeID:  n.nodeID.Text(16),
		})
		n.awaitingJoin = false
	}

	if n.awaitingStabilize && response.ResponseFor == ResponseFor_STABILIZE {
		var succPID *actor.PID
		if response.NodeID != "<nil>" && isBetween(responseNodeInfo.nodeID, n.nodeID, n.successor.nodeID) {
			n.successor = responseNodeInfo
			n.fingerTable[0] = n.successor
			succPID = actor.NewPID(n.successor.address, n.successor.name)
			context.Send(succPID, &Notify{
				Name:    n.name,
				Address: n.address,
				NodeID:  n.nodeID.Text(16),
			})
			fmt.Printf("[STABILIZE]: Updated successor to <%s>\n", n.successor.name)
			fmt.Printf("My range: %s - %s\n", n.predecessor.nodeID.Text(16), n.nodeID.Text(16))
		}

		// If predecessor exists, predecessor isn't itself, and there isn't an ongoing transfer
		if n.predecessor != nil && n.predecessor.nodeID.Cmp(n.nodeID) != 0 && !n.ongoingTransfer {
			succPID = actor.NewPID(n.successor.address, n.successor.name)
			var minHashSQL, maxHashSQL sql.NullString
			err := db.QueryRow("SELECT MIN(sha1_hash) FROM " + TABLE_NAME).Scan(&minHashSQL)
			checkError(err)
			err = db.QueryRow("SELECT MAX(sha1_hash) FROM " + TABLE_NAME).Scan(&maxHashSQL)
			checkError(err)
			if !minHashSQL.Valid || !maxHashSQL.Valid {
				n.awaitingStabilize = false
				return
			}
			minHash := minHashSQL.String
			maxHash := maxHashSQL.String

			/*
				The following logic will reference these concepts
				N: this node's ID
				P: this node's predecessor's ID
				S: this node's successor's ID
			*/
			//fmt.Printf("This node ID (N): %v\n", n.nodeID.Text(16))
			//fmt.Printf("Pred node ID (P): %v\n", n.predecessor)
			//fmt.Printf("Succ node ID (S): %v\n", n.successor)
			N := big.NewInt(0).Add(n.nodeID, big.NewInt(1)).Text(16) // this is really N + 1
			P := n.predecessor.nodeID.Text(16)
			S := n.successor.nodeID.Text(16)
			var batch []Range
			if n.successor.nodeID.Cmp(n.predecessor.nodeID) == 0 {
				// N < S
				if n.nodeID.Cmp(n.successor.nodeID) == -1 {
					// send (N, S] to successor
					rows, _ := getRowsInHashRange(N, S)
					if n.checkRowsPresent(rows) {
						checkError(err)
						batch = batchIDs(rows)
						n.startDatabaseTransfer(succPID, context, batch)
					}
				} else if n.nodeID.Cmp(n.successor.nodeID) == 1 {
					// S < N
					// send (N, max] to successor
					rows, _ := getRowsInHashRange(N, maxHash)
					if n.checkRowsPresent(rows) {
						checkError(err)
						batch = batchIDs(rows)
						n.startDatabaseTransfer(succPID, context, batch)
					}
					// send [min, S] to S
					rows, _ = getRowsInHashRange(minHash, S)
					if n.checkRowsPresent(rows) {
						checkError(err)
						batch = batchIDs(rows)
						n.startDatabaseTransfer(succPID, context, batch)
					}
				}
			} else {
				predPID := actor.NewPID(n.predecessor.address, n.predecessor.name)
				// case: min < P < N < S < max
				if n.nodeID.Cmp(n.predecessor.nodeID) == 1 && n.nodeID.Cmp(n.successor.nodeID) == -1 {
					// send (N, S] to succ
					rows, err := getRowsInHashRange(N, S)
					if n.checkRowsPresent(rows) {
						checkError(err)
						batch = batchIDs(rows)
						n.startDatabaseTransfer(succPID, context, batch)
					}
					//send [min, P] to pred
					rows, err = getRowsInHashRange(minHash, P)
					if n.checkRowsPresent(rows) {
						checkError(err)
						batch = batchIDs(rows)
						n.startDatabaseTransfer(predPID, context, batch)
					}

					// send (S, max] to pred
					rows, err = getRowsInHashRange(S, maxHash)
					if n.checkRowsPresent(rows) {
						checkError(err)
						batch = batchIDs(rows)
						n.startDatabaseTransfer(predPID, context, batch)
					}
				} else if n.nodeID.Cmp(n.predecessor.nodeID) == 1 && n.predecessor.nodeID.Cmp(n.successor.nodeID) == 1 {
					// case: min < S < P < N < max
					// send (N, max] to succ
					rows, err := getRowsInHashRange(N, maxHash)
					if n.checkRowsPresent(rows) {
						checkError(err)
						batch = batchIDs(rows)
						n.startDatabaseTransfer(succPID, context, batch)
					}
					// send [min, S] to succ
					rows, err = getRowsInHashRange(minHash, S)
					if n.checkRowsPresent(rows) {
						checkError(err)
						batch = batchIDs(rows)
						n.startDatabaseTransfer(succPID, context, batch)
					}
					// send (S, P] to pred
					rows, err = getRowsInHashRange(S, P)
					if n.checkRowsPresent(rows) {
						checkError(err)
						batch = batchIDs(rows)
						n.startDatabaseTransfer(predPID, context, batch)
					}
				} else if n.nodeID.Cmp(n.successor.nodeID) == -1 && n.successor.nodeID.Cmp(n.predecessor.nodeID) == -1 {
					// case: min < N < S < P < max
					// send (N, S] to succ
					rows, err := getRowsInHashRange(N, S)
					if n.checkRowsPresent(rows) {
						checkError(err)
						batch = batchIDs(rows)
						n.startDatabaseTransfer(succPID, context, batch)
					}
					// send (S, P] to pred
					rows, err = getRowsInHashRange(S, P)
					if n.checkRowsPresent(rows) {
						checkError(err)
						batch = batchIDs(rows)
						n.startDatabaseTransfer(predPID, context, batch)
					}
				}
			}

			// TODO: This seems kind of important to have uncommented.
			//deleteHashes(BigRange{start: big.NewInt(0).Add(n.nodeID, big.NewInt(1)).Text(16), end: n.successor.nodeID.Text(16)})
		}
		n.awaitingStabilize = false
	} /*else if n.awaitingPredPredForTransfer {
		rows, err := getRowsInHashRange(responseNodeInfo.nodeID.Text(16), n.predecessor.nodeID.Text(16))
		if err != nil {
			panic(err)
		}
		batch := batchIDs(rows)
		predPID := actor.NewPID(n.predecessor.address, n.predecessor.name)
		n.startDatabaseTransfer(predPID, context, batch)
		fmt.Println("Transferred DB LINES TO PRED!!!!!!")
		n.awaitingPredPredForTransfer = false
	}*/

	if n.awaitingFixFingers && response.ResponseFor == ResponseFor_FIX_FINGERS {
		n.fingerTable[n.nextFingerIndex] = responseNodeInfo

		//fmt.Printf("[FIX_FINGERS]: Fixed finger[%d]\n", n.nextFingerIndex)
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
			switch response.Status {
			case Status_ERROR:
				fmt.Printf("Peer returned an error when starting database transfer: %v\n", response.Message)
			case Status_DECLINE:
				fmt.Println("Peer declined database transfer.")
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
		NodeID:      n.nodeID.Text(16),
		ResponseFor: ResponseFor_JOIN}
	context.Request(toJoin, message)
}

func (n *Node) stabilize(context actor.Context) {
	if n.successor == nil {
		return
	}
	//fmt.Println("[Stabilize]: requesting predecessor of ", n.successor.name)
	succPID := actor.NewPID(n.successor.address, n.successor.name)
	n.awaitingStabilize = true
	context.Request(succPID, &RequestPredecessor{})
}

func (n *Node) notify(context actor.Context) {
	message := context.Message().(*Notify)
	//fmt.Println("Message: ", message)
	id := new(big.Int)
	id.SetString(message.GetNodeID(), 16)
	if n.predecessor == nil || n.predecessor.nodeID.Cmp(n.nodeID) == 0 || isBetween(id, n.predecessor.nodeID, n.nodeID) {
		n.predecessor = &NodeInfo{
			name:    message.GetName(),
			address: message.GetAddress(),
			nodeID:  id,
		}
		fmt.Printf("[NOTIFY]: Updated predecessor to <%s>\n", n.predecessor.name)
		n.newPredecessor = true
	}
}

// context here is about the original sender
func (n *Node) find_successor(message *RequestSuccessor, context actor.Context) {
	if n.successor == nil { // If there is no successor none of the following functions can work.
		return
	}

	id := new(big.Int)
	id.SetString(message.GetNodeID(), 16)

	succBound := new(big.Int).Add(n.successor.nodeID, big.NewInt(1))

	if n.name == n.successor.name {
		context.Respond(&Response{
			Name:        n.successor.name,
			Address:     n.successor.address,
			NodeID:      n.successor.nodeID.Text(16),
			ResponseFor: message.ResponseFor,
		})
	} else if isBetween(id, n.nodeID, succBound) {
		context.Respond(&Response{
			Name:        n.successor.name,
			Address:     n.successor.address,
			NodeID:      n.successor.nodeID.Text(16),
			ResponseFor: message.ResponseFor,
		})
	} else {
		u := n.closest_preceeding_node(id)
		context.Forward(u)
	}
}

func (n *Node) fixFingers(context actor.Context) {
	n.nextFingerIndex = rand.IntN(m) //[0, m)

	shift := new(big.Int).Lsh(big.NewInt(1), uint(n.nextFingerIndex))

	modulo := new(big.Int).Lsh(big.NewInt(1), uint(m))

	start := new(big.Int).Add(n.nodeID, shift)
	start.Mod(start, modulo)

	//start := (n.nodeID + (1 << n.nextFingerIndex)) % (1 << m)
	n.awaitingFixFingers = true
	message := &RequestSuccessor{
		NodeID:      start.Text(16),
		ResponseFor: ResponseFor_FIX_FINGERS}
	context.Request(context.Self(), message)
}

func (n *Node) closest_preceeding_node(id *big.Int) *actor.PID {
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

func consistent_hash(str string) *big.Int {
	hash := sha1.New()
	_, err := io.WriteString(hash, str)
	if err != nil {
		fmt.Println("[SYSTEM]: Error hashing address:", err)
	}
	result := hash.Sum(nil)

	// value := binary.BigEndian.Uint64(result)
	// if m < 64 {
	// 	// Mask off the upper bits to keep only m bits
	// 	value = value & ((1 << uint(m)) - 1)
	// }

	return new(big.Int).SetBytes(result)
}

// used for checking if id x is between (a, b)
func isBetween(x, a, b *big.Int) bool {
	//if a < b then check regularly:
	if b.Cmp(a) == 1 {
		return x.Cmp(a) == 1 && b.Cmp(x) == 1
	}
	// wrap around
	return x.Cmp(a) == 1 || x.Cmp(b) == -1
}

func (n *Node) printInfo() {

	fmt.Println("========== INFO ==========")
	fmt.Printf("Name: %s\nID: %s\nPID: %v\nSuccessor: %s (%s)\nPredecessor: %s (%s)\n",
		n.name, n.nodeID.Text(16), n.nodePID, n.successor.name, n.successor.nodeID.Text(16), n.predecessor.name, n.predecessor.nodeID.Text(16))
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
		NodeID:      n.nodeID.Text(16),
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
	fmt.Println("[DEBUG]: The transfer message PID is: ", newFileTransfer.peerPID)
	fmt.Printf("[SYSTEM]: File Transfer request initialized by Address: %v ID:%v \n", context.Sender().Address, context.Sender().Id)
}

func (n *Node) handleFileChunk(message *FileChunk, context actor.Context) {
	instance, exists := n.incomingFileTransfers[message.GetFilename()]

	instance.lastTransferTime = time.Now()
	response := &Response{
		Name:        n.name,
		Address:     n.address,
		NodeID:      n.nodeID.Text(16),
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
		//deleteDB(message.GetFilename())
		fmt.Printf("[SYSTEM] File Transfer finished successfully from Address: %v ID:%v\n", context.Sender().Address, context.Sender().Id)
		fmt.Println("Sending EndTransfer to: ", instance.peerPID)
		context.Send(instance.peerPID, &EndTransfer{})
		if len(n.incomingFileTransfers) == 0 {
			n.ongoingTransfer = false
		}
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
	chunkMessage := &FileChunk{
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

func (n *Node) handleEndTransfer() {
	fmt.Println("handleEndTransfer in da house!!!")
	keepRange := BigRange{start: n.predecessor.nodeID.Text(16), end: n.nodeID.Text(16)}
	deleteOtherHashes(keepRange)
	n.ongoingTransfer = false
}

func checkError(e error) {
	if e != nil {
		panic(e)
	}
}

func (n *Node) checkRowsPresent(rows []int) bool {
	if len(rows) != 0 {
		n.ongoingTransfer = true
		return true
	}
	return false
}
