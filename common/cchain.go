package common

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"sync"
)

type CChain struct {
	ChainID      *Hash
	Name         [][]byte
	//Blocks       []*CBlock
	NextBlock *CBlock
	NextBlockID  uint32
	BlockMutex   sync.Mutex	
}

type CBlock struct {
	//Marshalized
	Header    *CBlockHeader
	CBEntries []CBEntry //Interface
	
	//Not Marshalized
	CBHash   *Hash
	MerkleRoot *Hash	
	Chain    *CChain
	IsSealed bool
}

type CBInfo struct {
	CBHash     *Hash
	FBHash     *Hash
	FBBlockNum uint64
	ChainID    *Hash
}

func CreateCBlock(chain *CChain, prev *CBlock, cap uint) (b *CBlock, err error) {
	if prev == nil && chain.NextBlockID != 0 {
		return nil, errors.New("Previous block cannot be nil")
	} else if prev != nil && chain.NextBlockID == 0 {
		return nil, errors.New("Origin block cannot have a parent block")
	}

	b = new(CBlock)

	b.Header = new (CBlockHeader)
	b.Header.ChainID = chain.ChainID
	
	if prev == nil {
		b.Header.PrevKeyMR = NewHash()
		b.Header.PrevHash = NewHash()	
		b.Header.BodyHash = NewHash()	
	} else {
		if prev.MerkleRoot == nil {
			prev.BuildMerkleRoot()
		}
		b.Header.PrevKeyMR = prev.MerkleRoot	
			
		if prev.CBHash == nil {
			prev.BuildCBHash()
		}
		b.Header.PrevHash = prev.CBHash		
	}

	b.Header.DBHeight = chain.NextBlockID
	b.Header.SegmentsMR = NewHash()
	b.Header.BalanceMR = NewHash()
	b.Chain = chain
	b.CBEntries = make([]CBEntry, 0, cap)
	b.IsSealed = false

	return b, err
}

func (block *CBlock) BuildMerkleRoot() (err error) {
	
	// Create the Entry Block Key Merkle Root from the hash of Header and the Body Merkle Root	
	hashes := make([]*Hash, 0, 2)	
	binaryEBHeader, _ := block.Header.MarshalBinary()
	hashes = append(hashes, Sha(binaryEBHeader))
	hashes = append(hashes, block.Header.BodyHash)	
	merkle := BuildMerkleTreeStore(hashes)
	block.MerkleRoot = merkle[len(merkle)-1] // MerkleRoot is not marshalized in Entry Block

	return
}

func (block *CBlock) BuildCBHash() (err error) {
	
	binaryEB, _ := block.MarshalBinary()
	block.CBHash = Sha(binaryEB) 

	return
}

func (block *CBlock) BuildCBBodyHash() (bodyHash *Hash, err error) {
	var buf bytes.Buffer
	for i := 0; i < len(block.CBEntries); i++ {
		data, _ := block.CBEntries[i].MarshalBinary()
		buf.Write(data)
	}		
	bodyHash = Sha(buf.Bytes()) 

	return bodyHash, nil
}

func (b *CBlock) AddCBEntry(e CBEntry) (err error) {
	b.CBEntries = append(b.CBEntries, e)
	return
}

func (b *CBlock) MarshalBinary() (data []byte, err error) {
	var buf bytes.Buffer

	data, _ = b.Header.MarshalBinary()
	buf.Write(data)

	count := uint64(len(b.CBEntries))
	binary.Write(&buf, binary.BigEndian, count)
	for i := uint64(0); i < count; i++ {
		data, _ := b.CBEntries[i].MarshalBinary()
		buf.Write(data)
	}
	return buf.Bytes(), err
}

func (b *CBlock) MarshalledSize() uint64 {
	var size uint64 = 0

	size += b.Header.MarshalledSize()
	size += 8 // len(Entries) uint64

	for _, entry := range b.CBEntries {
		size += entry.MarshalledSize()
	}

	fmt.Println("cblock.MarshalledSize=", size)

	return size
}

func (b *CBlock) UnmarshalBinary(data []byte) (err error) {
	h := new(CBlockHeader)
	h.UnmarshalBinary(data)
	b.Header = h

	data = data[h.MarshalledSize():]

	count, data := binary.BigEndian.Uint64(data[0:8]), data[8:]

	b.CBEntries = make([]CBEntry, count)
	for i := uint64(0); i < count; i++ {
		if data[0] == TYPE_BUY {
			b.CBEntries[i] = new(BuyCBEntry)
		} else if data[0] == TYPE_PAY_CHAIN {
			b.CBEntries[i] = new(PayChainCBEntry)
		} else if data[0] == TYPE_PAY_ENTRY {
			b.CBEntries[i] = new(PayEntryCBEntry)
		}
		err = b.CBEntries[i].UnmarshalBinary(data)
		if err != nil {
			return
		}
		data = data[b.CBEntries[i].MarshalledSize():]
	}

	return nil
}

func (b *CBInfo) MarshalBinary() (data []byte, err error) {
	var buf bytes.Buffer

	data, _ = b.CBHash.MarshalBinary()
	buf.Write(data)

	data, _ = b.FBHash.MarshalBinary()
	buf.Write(data)

	binary.Write(&buf, binary.BigEndian, b.FBBlockNum)

	data, _ = b.ChainID.MarshalBinary()
	buf.Write(data)

	return buf.Bytes(), err
}

func (b *CBInfo) MarshalledSize() uint64 {
	var size uint64 = 0
	size += 33 //b.EBHash
	size += 33 //b.FBHash
	size += 8  //b.FBBlockNum
	size += 33 //b.ChainID

	return size
}

func (b *CBInfo) UnmarshalBinary(data []byte) (err error) {
	b.CBHash = new(Hash)
	b.CBHash.UnmarshalBinary(data[:33])

	data = data[33:]
	b.FBHash = new(Hash)
	b.FBHash.UnmarshalBinary(data[:33])

	data = data[33:]
	b.FBBlockNum = binary.BigEndian.Uint64(data[0:8])

	data = data[8:]
	b.ChainID = new(Hash)
	b.ChainID.UnmarshalBinary(data[:33])

	return nil
}

func (b *CChain) MarshalBinary() (data []byte, err error) {
	var buf bytes.Buffer

	data, _ = b.ChainID.MarshalBinary()
	buf.Write(data)

	count := len(b.Name)
	binary.Write(&buf, binary.BigEndian, uint64(count))

	for _, bytes := range b.Name {
		count = len(bytes)
		binary.Write(&buf, binary.BigEndian, uint64(count))
		buf.Write(bytes)
	}

	return buf.Bytes(), err
}

func (b *CChain) MarshalledSize() uint64 {
	var size uint64 = 0
	size += 33 //b.ChainID
	size += 8  //Name length
	for _, bytes := range b.Name {
		size += 8
		size += uint64(len(bytes))
	}
	return size
}

func (b *CChain) UnmarshalBinary(data []byte) (err error) {
	b.ChainID = new(Hash)
	b.ChainID.UnmarshalBinary(data[:33])

	data = data[33:]
	count := binary.BigEndian.Uint64(data[0:8])
	data = data[8:]

	b.Name = make([][]byte, count, count)

	for i := uint64(0); i < count; i++ {
		length := binary.BigEndian.Uint64(data[0:8])
		data = data[8:]
		b.Name[i] = data[:length]
		data = data[length:]
	}

	return nil
}

//Entry Credit Block Header
type CBlockHeader struct {
	ChainID		  *Hash	
//	BlockID       		uint64
//	PrevBlockHash 		*Hash
//	EntryCount    		uint32
//	CreditsPerFactoid 	uint32 
	BodyHash	  *Hash
	PrevKeyMR	  *Hash
	PrevHash      *Hash	
	DBHeight       uint32
	SegmentsMR	  *Hash	
	BalanceMR	  *Hash		

	BodySize	   uint64	
}

func (b *CBlockHeader) MarshalBinary() (data []byte, err error) {
	var buf bytes.Buffer
	
	data, err = b.ChainID.MarshalBinary()
	if err != nil {
		return nil, err
	}
	buf.Write(data)	

	data, err = b.BodyHash.MarshalBinary()
	if err != nil {
		return nil, err
	}
	buf.Write(data)	

	data, err = b.PrevKeyMR.MarshalBinary()
	if err != nil {
		return nil, err
	}
	buf.Write(data)	
	
	data, err = b.PrevHash.MarshalBinary()
	if err != nil {
		return nil, err
	}
	buf.Write(data)		

	binary.Write(&buf, binary.BigEndian, b.DBHeight)
	
	data, err = b.SegmentsMR.MarshalBinary()
	if err != nil {
		return nil, err
	}
	buf.Write(data)		
	
	data, err = b.BalanceMR.MarshalBinary()
	if err != nil {
		return nil, err
	}
	buf.Write(data)		

	binary.Write(&buf, binary.BigEndian, b.BodySize)	
	
	
	return buf.Bytes(), err
}

func (b *CBlockHeader) MarshalledSize() uint64 {
	var size uint64 = 0

	size += b.ChainID.MarshalledSize()
	size += b.BodyHash.MarshalledSize()
	size += b.PrevKeyMR.MarshalledSize()
	size += b.PrevHash.MarshalledSize()
	size += 4 // DB Height
	size += b.SegmentsMR.MarshalledSize()
	size += b.BalanceMR.MarshalledSize()	
	size += 8 // Body Size
	
	return size
}

func (b *CBlockHeader) UnmarshalBinary(data []byte) (err error) {
	
	b.ChainID = new(Hash)
	b.ChainID.UnmarshalBinary(data)
	data = data[b.ChainID.MarshalledSize():]

	b.BodyHash = new(Hash)
	b.BodyHash.UnmarshalBinary(data)
	data = data[b.BodyHash.MarshalledSize():]

	b.PrevKeyMR = new(Hash)
	b.PrevKeyMR.UnmarshalBinary(data)
	data = data[b.PrevKeyMR.MarshalledSize():]

	b.PrevHash = new(Hash)
	b.PrevHash.UnmarshalBinary(data)
	data = data[b.PrevHash.MarshalledSize():]
	
	b.DBHeight, data = binary.BigEndian.Uint32(data[0:4]), data[4:]	

	b.SegmentsMR = new(Hash)
	b.SegmentsMR.UnmarshalBinary(data)
	data = data[b.SegmentsMR.MarshalledSize():]

	b.BalanceMR = new(Hash)
	b.BalanceMR.UnmarshalBinary(data)
	data = data[b.BalanceMR.MarshalledSize():]

	b.BodySize, data = binary.BigEndian.Uint64(data[0:8]), data[8:]		
		
	return nil
}

//---------------------------------------------------------------
// Three types of entries (transactions) for Entry Credit Block
//---------------------------------------------------------------
const (
 	TYPE_SERVER_INDEX uint8 = iota
	TYPE_MINUTE_NUMBER 
	TYPE_PAY_CHAIN
	TYPE_PAY_ENTRY	
	TYPE_BUY	
)

type CBEntry interface {
	Type() byte
	PublicKey() *Hash
	Credits() int32
	MarshalBinary() ([]byte, error)
	MarshalledSize() uint64
	UnmarshalBinary(data []byte) (err error)
}

type BuyCBEntry struct {
	entryType    byte
	publicKey    *Hash
	credits      int32
	CBEntry      //interface
	FactomTxHash *Hash
}

type PayEntryCBEntry struct {
	entryType byte
	publicKey *Hash
	credits   int32
	CBEntry   //interface
	EntryHash *Hash
	TimeStamp int64
	Sig       []byte	
}

type PayChainCBEntry struct {
	entryType        byte
	publicKey        *Hash
	credits          int32
	CBEntry          //interface
	EntryHash        *Hash
	ChainIDHash      *Hash
	EntryChainIDHash *Hash //Hash(EntryHash+ChainIDHash)
	Sig       		 []byte	
}

type ECBalance struct {
	PublicKey *Hash
	Credits   int32
}

func NewPayEntryCBEntry(pubKey *Hash, entryHash *Hash, credits int32,
	timeStamp int64, sig []byte) *PayEntryCBEntry {
	e := &PayEntryCBEntry{}
	e.publicKey = pubKey
	e.entryType = TYPE_PAY_ENTRY
	e.credits = credits
	e.EntryHash = entryHash
	e.TimeStamp = timeStamp
	e.Sig = sig

	return e
}

func NewPayChainCBEntry(pubKey *Hash, entryHash *Hash, credits int32,
	chainIDHash *Hash, entryChainIDHash *Hash, sig []byte) *PayChainCBEntry {
	e := &PayChainCBEntry{}
	e.publicKey = pubKey
	e.entryType = TYPE_PAY_CHAIN
	e.credits = credits
	e.EntryHash = entryHash
	e.ChainIDHash = chainIDHash
	e.EntryChainIDHash = entryChainIDHash
	e.Sig = sig

	return e
}

func NewBuyCBEntry(pubKey *Hash, factoidTxHash *Hash,
	credits int32) *BuyCBEntry {
	e := &BuyCBEntry{}
	e.publicKey = pubKey
	e.entryType = TYPE_BUY
	e.FactomTxHash = factoidTxHash
	e.credits = credits

	return e
}

func (e *BuyCBEntry) Type() byte {
	return e.entryType
}

func (e *BuyCBEntry) PublicKey() *Hash {
	return e.publicKey
}

func (e *BuyCBEntry) Credits() int32 {
	return e.credits
}

func (e *BuyCBEntry) MarshalBinary() (data []byte, err error) {
	var buf bytes.Buffer

	buf.Write([]byte{e.entryType})

	data, err = e.publicKey.MarshalBinary()
	if err != nil {
		return
	}
	buf.Write(data)
	binary.Write(&buf, binary.BigEndian, e.Credits())

	data, err = e.FactomTxHash.MarshalBinary()
	if err != nil {
		return
	}
	buf.Write(data)

	return buf.Bytes(), nil
}

func (e *BuyCBEntry) MarshalledSize() uint64 {
	var size uint64 = 0
	size += 1                               // Type (byte)
	size += e.publicKey.MarshalledSize()    // PublicKey
	size += 4                               // Credits (int32)
	size += e.FactomTxHash.MarshalledSize() // Factoid Trans Hash

	return size
}

func (e *BuyCBEntry) UnmarshalBinary(data []byte) (err error) {
	e.entryType, data = data[0], data[1:]
	e.publicKey = new(Hash)

	e.publicKey.UnmarshalBinary(data)
	data = data[e.publicKey.MarshalledSize():]

	buf, data := bytes.NewBuffer(data[:4]), data[4:]
	binary.Read(buf, binary.BigEndian, &e.credits)

	e.FactomTxHash = new(Hash)
	e.FactomTxHash.UnmarshalBinary(data)

	return nil
}

func (e *PayEntryCBEntry) Type() byte {
	return e.entryType
}

func (e *PayEntryCBEntry) PublicKey() *Hash {
	return e.publicKey
}

func (e *PayEntryCBEntry) Credits() int32 {
	return e.credits
}

func (e *PayEntryCBEntry) MarshalBinary() (data []byte, err error) {
	var buf bytes.Buffer

	buf.Write([]byte{e.entryType})

	data, err = e.publicKey.MarshalBinary()
	if err != nil {
		return
	}
	buf.Write(data)

	binary.Write(&buf, binary.BigEndian, e.credits)

	data, err = e.EntryHash.MarshalBinary()
	if err != nil {
		return
	}
	buf.Write(data)

	binary.Write(&buf, binary.BigEndian, e.TimeStamp) 
	
	count := len(e.Sig)
	binary.Write(&buf, binary.BigEndian, uint32(count))
	_, err = buf.Write(e.Sig)
	if err != nil {
		return nil, err
	}	
	

	return buf.Bytes(), nil
}

func (e *PayEntryCBEntry) MarshalledSize() uint64 {
	var size uint64 = 0
	size += 1                            // Type (byte)
	size += e.publicKey.MarshalledSize() // PublicKey
	size += 4                            // Credits (int32)
	size += e.EntryHash.MarshalledSize() // Entry Hash
	size += 8                            //	TimeStamp int64
	size += 4                            // len(e.Sig)	
	size += uint64(len(e.Sig))			 // sig

	return size
}

func (e *PayEntryCBEntry) UnmarshalBinary(data []byte) (err error) {
	e.entryType, data = data[0], data[1:]

	e.publicKey = new(Hash)
	e.publicKey.UnmarshalBinary(data)
	data = data[e.publicKey.MarshalledSize():]

	buf, data := bytes.NewBuffer(data[:4]), data[4:]
	binary.Read(buf, binary.BigEndian, &e.credits)

	e.EntryHash = new(Hash)
	e.EntryHash.UnmarshalBinary(data)
	data = data[e.EntryHash.MarshalledSize():]

	buf = bytes.NewBuffer(data[:8])
	binary.Read(buf, binary.BigEndian, &e.TimeStamp)
	data = data[8:]
	
	length := binary.BigEndian.Uint32(data[0:4])
	data = data[4:]
	e.Sig = data[:length]	

	return nil
}

func (e *PayChainCBEntry) Type() byte {
	return e.entryType
}

func (e *PayChainCBEntry) PublicKey() *Hash {
	return e.publicKey
}

func (e *PayChainCBEntry) Credits() int32 {
	return e.credits
}

func (e *PayChainCBEntry) MarshalBinary() (data []byte, err error) {
	var buf bytes.Buffer

	buf.Write([]byte{e.entryType})

	data, err = e.publicKey.MarshalBinary()
	if err != nil {
		return
	}
	buf.Write(data)

	binary.Write(&buf, binary.BigEndian, e.credits)

	data, err = e.EntryHash.MarshalBinary()
	if err != nil {
		return
	}
	buf.Write(data)

	data, err = e.ChainIDHash.MarshalBinary()
	if err != nil {
		return
	}
	buf.Write(data)

	data, err = e.EntryChainIDHash.MarshalBinary()
	if err != nil {
		return
	}
	buf.Write(data)
	
	count := len(e.Sig)
	binary.Write(&buf, binary.BigEndian, uint32(count))
	
	_, err = buf.Write(e.Sig)
	if err != nil {
		return nil, err
	}		

	return buf.Bytes(), nil
}

func (e *PayChainCBEntry) MarshalledSize() uint64 {
	var size uint64 = 0
	size += 1                                   // Type (byte)
	size += e.publicKey.MarshalledSize()        // PublicKey
	size += 4                                   // Credits (int32)
	size += e.EntryHash.MarshalledSize()        // Entry Hash
	size += e.ChainIDHash.MarshalledSize()      // ChainID Hash
	size += e.EntryChainIDHash.MarshalledSize() // EntryChainID Hash
	size += 4                            		// len(e.Sig)	
	size += uint64(len(e.Sig))			 		// sig	

	return size
}

func (e *PayChainCBEntry) UnmarshalBinary(data []byte) (err error) {
	e.entryType, data = data[0], data[1:]

	e.publicKey = new(Hash)
	e.publicKey.UnmarshalBinary(data)
	data = data[e.publicKey.MarshalledSize():]

	buf, data := bytes.NewBuffer(data[:4]), data[4:]
	binary.Read(buf, binary.BigEndian, &e.credits)

	e.EntryHash = new(Hash)
	e.EntryHash.UnmarshalBinary(data)
	data = data[e.EntryHash.MarshalledSize():]

	e.ChainIDHash = new(Hash)
	e.ChainIDHash.UnmarshalBinary(data)
	data = data[e.ChainIDHash.MarshalledSize():]

	e.EntryChainIDHash = new(Hash)
	e.EntryChainIDHash.UnmarshalBinary(data)
	data = data[e.EntryChainIDHash.MarshalledSize():]
	
	length := binary.BigEndian.Uint32(data[0:4])
	data = data[4:]
	e.Sig = data[:length]		

	return nil
}