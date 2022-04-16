package main

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"os"
	"strconv"
	"strings"
	"sync"

	bencode "github.com/jackpal/bencode-go"
)

// Torrent stores all data about a torrent generated from a magnet link
type Torrent struct {
	magLink  string
	name     string
	infoHash []byte

	trackers []Tracker
	peers    []Peer // all peers collected by the tracker, not necessarily connected
	maxPeers int

	// Metadata-specific
	metadataSize   int // in bytes, given by first extended handshake
	metadataRaw    []byte
	metadataPieces []byte // array of [1/0, 1/0,...] denoting whether we have the piece or not
	metadata       Metadata
	metadataMx     sync.Mutex // to ensure that that we only trigger "building" the metadata once

	pieces         []Piece
	obtainedBlocks []byte // similar to 'metadata pieces', allows for quick bitwise checking which pieces we have, if the ith bit is set to 1 we have that block
	numPiecesMx    sync.Mutex
	numPieces      int

	isDownloaded bool
	downloadedMx sync.Mutex

	logFile *os.File

	connHandler *ConnectionHandler
	progressBar Bar

	pieceCH chan BlockData

	logger *log.Logger
}

// TODO: not to be confused with Block
// BlockData is is the type that is sent through a pieceCH when a peer sends a block
type BlockData struct {
	piece  int
	offset int
	data   []byte
}

// for simplicity, only magnet links will be supportd for no
func newTorrent(magnetLink string, maxPeers int) *Torrent {
	var torrent Torrent
	torrent.logFile, _ = os.Create("debug.log")
	torrent.magLink = magnetLink
	torrent.maxPeers = maxPeers
	torrent.parseMagnetLink()
	torrent.connHandler = torrent.newConnHandler()
	torrent.isDownloaded = false
	torrent.logger = log.New(torrent.logFile, "[Torrent Info]: ", log.Ltime)
	torrent.pieceCH = make(chan BlockData)

	return &torrent
}

// TODO: overhaul on link parsing, this was a bit of a hack
// only supporting udp links
func (torrent *Torrent) parseMagnetLink() {
	data := strings.Split(torrent.magLink, "&")
	for i := 0; i < len(data); i++ {
		switch data[i][:2] {
		case "dn":
			torrent.name = strings.Replace(data[i][3:], "%20", " ", -1)
		case "tr":
			trackerLink := data[i][3:] // cut off the tr=
			trackerLen := len(trackerLink)
			index := 0

			for index < trackerLen {
				if strings.Compare(string(trackerLink[index]), "%") == 0 {
					token, err := hex.DecodeString(string(trackerLink[index+1 : index+3]))
					if err != nil {
						panic(err)
					}
					trackerLink = string(trackerLink[0:index]) + string(token) + string(trackerLink[index+3:])
					trackerLen -= 2
				}
				index++
			}
			if trackerLink[0:3] == "udp" {
				if strings.Contains(trackerLink, "announce") {
					trackerLink = trackerLink[:len(trackerLink)-len("/announce")]
				}
				newTracker := newTracker(trackerLink[6:])
				torrent.trackers = append(torrent.trackers, *newTracker)
			}
		default:
			hash, err := hex.DecodeString(data[i][strings.LastIndex(data[i], ":")+1:])
			if err != nil {
				panic(err)
			}
			torrent.infoHash = hash
		}
	}
}

func (torrent *Torrent) String() {
	fmt.Println("Name: " + torrent.name)
	fmt.Println("Magnet: " + torrent.magLink)
	fmt.Println("Trackers:")
	for i := 0; i < len(torrent.trackers); i++ {
		fmt.Println(" -- " + torrent.trackers[i].link)
	}
	fmt.Println("Known peers:")
	if len(torrent.peers) == 0 {
		fmt.Println(" -- None")
	} else {
		for i := 0; i < len(torrent.peers); i++ {
			fmt.Println(" -- " + torrent.peers[i].ip)
		}
	}
	if torrent.metadata.Length != 0 {
		fmt.Println("Metadata info (" + strconv.Itoa(torrent.metadataSize) + " bytes with " + strconv.Itoa(torrent.numMetadataPieces()) + " pieces)")
		fmt.Println("-------------")
		fmt.Println(torrent.metadata.String())
	}
}

// send 2x announce requests to all trackers, the first to find out how many peers they have,
// the second to request that many, so that we have a large pool to pull from
func (torrent *Torrent) findPeers() {
	var wg sync.WaitGroup

	fmt.Println("Contacting trackers...")
	// TODO: fix bad trackers?
	for i := 0; i < len(torrent.trackers); i++ {
		wg.Add(1)
		go func(wg *sync.WaitGroup, tracker Tracker) {
			defer wg.Done()

			err := tracker.connect()
			if err != nil {
				return
			}

			err = tracker.setConnectionID()
			if err != nil {
				return
			}

			seeders, err := tracker.announce(torrent, 0)
			if err != nil {
				return
			}

			_, err = tracker.announce(torrent, seeders)
			if err != nil {
				return
			}
			err = tracker.disconnect()
			if err != nil {
				panic(err)
			}
		}(&wg, torrent.trackers[i])
	}
	wg.Wait()

	torrent.removeDuplicatePeers()
	fmt.Printf("%d peers in swarm\n", len(torrent.peers))
}

// remove all instances of repeating peer ip addresses from torrent.peers
func (torrent *Torrent) removeDuplicatePeers() {
	seen := map[string]bool{}
	trimmed := []Peer{}

	for i := range torrent.peers {
		if !seen[torrent.peers[i].ip] {
			seen[torrent.peers[i].ip] = true
			trimmed = append(trimmed, torrent.peers[i])
		}
	}
	torrent.peers = trimmed
}

// assumes the filename is "metadata.torrent",whichof course will not be valid in the future if there are multiple torrents
func (torrent *Torrent) parseMetadataFile() error {
	data, err := ioutil.ReadFile("metadata.torrent")
	if err != nil {
		return err
	}

	var result = Metadata{"", "", 0, "", 0, nil}
	reader := bytes.NewReader(data)
	err = bencode.Unmarshal(reader, &result)
	if err != nil {
		return err
	}

	torrent.metadata = result

	if len(result.Files) >= 1 {
		for i := 0; i < len(result.Files); i++ {
			torrent.metadata.Length += result.Files[i].Length
		}
	}

	torrent.name = torrent.metadata.Name

	// create empty pieces slice
	torrent.pieces = make([]Piece, int(math.Ceil(float64(torrent.metadata.Length)/float64(torrent.metadata.PieceLen))))
	for i := 0; i < len(torrent.pieces)-1; i++ {
		torrent.pieces[i].blocks = make([]Block, torrent.metadata.PieceLen/(BlockLen))
	}
	torrent.pieces[len(torrent.pieces)-1].blocks = make([]Block, int(math.Ceil(float64(torrent.metadata.Length-(torrent.metadata.PieceLen*(len(torrent.pieces)-1)))/float64(BlockLen))))
	torrent.obtainedBlocks = make([]byte, int(math.Ceil(float64(torrent.getNumBlocks())/float64(8))))

	go torrent.blockHandler()

	torrent.progressBar.newOption(0, int64(torrent.getNumBlocks()))
	return nil
}

// "main" function of a torrent
func (torrent *Torrent) startDownload() {
	// get num_want peers and store in masterlist of peers
	torrent.findPeers()

	// eventually this will be backgrounded but ok to just connect for now

	torrent.connHandler.run()

	torrent.String()
}

func (torrent *Torrent) blockHandler() {
	torrent.logger.Println("Started the blockHandler")
	for {
		ch := <-torrent.pieceCH
		torrent.logger.Println("AIIII")
		if torrent.hasBlock(ch.piece, ch.offset) {
			continue
		}

		torrent.logger.Println("Block received")
		// Set this data
		torrent.pieces[ch.piece].blocks[ch.offset/BlockLen].data = ch.data

		// Mark this block as 'have'
		blockIndex := (ch.piece*torrent.getNumBlocksInPiece() + (ch.offset / BlockLen))
		setByte(&torrent.obtainedBlocks, blockIndex)

		torrent.numPieces++

		// Update progress bar
		torrent.progressBar.play(int64(torrent.numPieces))
	}
}

func (torrent *Torrent) hasBlock(pieceIndex int, offset int) bool {
	if torrent.obtainedBlocks == nil {
		return false
	}
	blockIndex := (pieceIndex*torrent.getNumBlocksInPiece() + (offset / BlockLen))
	return byteIsSet(torrent.obtainedBlocks, blockIndex)
}

func (torrent *Torrent) getNumBlocks() int {
	return int(math.Ceil(float64(torrent.metadata.Length) / float64(BlockLen)))
}

func (torrent *Torrent) getNumBlocksInPiece() int {
	return torrent.metadata.PieceLen / BlockLen
}

func (torrent *Torrent) checkDownloadStatus() {
	torrent.downloadedMx.Lock()
	if torrent.hasAllData() && !torrent.isDownloaded {
		torrent.isDownloaded = true
		fmt.Println(torrent.obtainedBlocks)
		torrent.buildFile()
	}
	torrent.downloadedMx.Unlock()
}

func (torrent *Torrent) hasAllData() bool {
	for i := 0; i < len(torrent.obtainedBlocks); i++ {
		if !byteIsSet(torrent.obtainedBlocks, i) {
			return false
		}
	}
	return true
}

func (torrent *Torrent) buildFile() {
	torrent.progressBar.finish()
	if len(torrent.metadata.Files) > 1 {
		// Create new directory
		path := "downloads/" + torrent.name + "/"
		err := os.MkdirAll("downloads/"+torrent.name, 0770)
		if err != nil {
			fmt.Println(err)
		}

		var bytesWritten int
		for i := 0; i < len(torrent.metadata.Files); i++ {
			torrent.createFile(bytesWritten, torrent.metadata.Files[i].Length, path, torrent.metadata.Files[i].Path[0])
			bytesWritten += torrent.metadata.Files[i].Length
		}
	} else {
		// Single files
		file, _ := os.Create("downloads/" + torrent.metadata.Name)
		for i := 0; i < len(torrent.pieces); i++ {
			for j := 0; j < len(torrent.pieces[i].blocks); j++ {
				_, _ = file.Write(torrent.pieces[i].blocks[j].data)
			}
		}
	}
}

// Writes one file to given directory (util function for build_file)
func (torrent *Torrent) createFile(offset int, fileSize int, path string, name string) {
	file, _ := os.Create(path + name)

	// write data spilling into front block, ie block 1 here [  xx] [xxxx] [xx  ]

	startPiece := offset / BlockLen / torrent.getNumBlocksInPiece()
	startBlock := offset / BlockLen % torrent.getNumBlocksInPiece()

	bytesWritten, _ := file.Write(torrent.pieces[startPiece].blocks[startBlock].data[offset%BlockLen:])

	// finish this piece for easy iteration -- this assumes that there is > BLOCK_LEN * torrent.get_num_blocks_in_piece() data left to write
	for i := startBlock + 1; i < len(torrent.pieces[startPiece].blocks); i++ {
		b, err := file.Write(torrent.pieces[startPiece].blocks[i].data)
		if err != nil {
			panic(err)
		}
		bytesWritten += b
	}
	startPiece++

	blocksToWrite := (fileSize - bytesWritten) / BlockLen

	for i := startPiece; i < startPiece+(blocksToWrite/torrent.getNumBlocksInPiece()); i++ {
		for j := 0; j < torrent.getNumBlocksInPiece(); j++ {
			if i == startPiece+(blocksToWrite/torrent.getNumBlocksInPiece())-1 && j == (blocksToWrite%torrent.getNumBlocksInPiece()) {
				break
			}
			b, _ := file.Write(torrent.pieces[i].blocks[j].data)
			bytesWritten += b
		}

	}

	// write data spilling into back block
	lastPiece := (fileSize + offset) / BlockLen / torrent.getNumBlocksInPiece()
	lastBlock := (fileSize + offset) / BlockLen % torrent.getNumBlocksInPiece()
	_, _ = file.Write(torrent.pieces[lastPiece].blocks[lastBlock].data[:fileSize%BlockLen])

	for i := 0; i < len(torrent.pieces); i++ {
		for j := 0; j < len(torrent.pieces[i].blocks); j++ {
			_, _ = file.Write(torrent.pieces[i].blocks[j].data)
		}
	}
}
