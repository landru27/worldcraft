package main

///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
//  import necessary external packages  ///////////////////////////////////////////////////////////////////////////////////////

import (
	"bytes"
	"encoding/binary"
	//"encoding/json"
	//"fmt"
	//"reflect"
	//"io"
	//"regexp"
)

///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
//  declare our internal datatypes and their interfaces  //////////////////////////////////////////////////////////////////////

type MCWorld struct {
	PathWorld string
	Regions   []MCRegion
	Chunks    []MCChunk
}

type MCRegion struct {
	RX                 int
	RZ                 int
	ChunkDataLocations [1024]MCchunkdatalocation
	ChunkTimestamps    [1024]int32
}

// a chunkdata descriptor indicates where within the region file the chuck data is found;  the offset is the (0-indexed)
// index of the first 4KB block holding the chunk data; the count is the number of 4KB blocks used for this chunk;  the offset
// does not ignore the two header blocks, so the lowest offset for chunk data will be "2", i.e. starting at byte 8192 in the
// region file (i.e., the 8,193rd byte, the start of the 3rd 4KB block)
//
type MCchunkdatalocation struct {
	Offset [3]byte
	Count  uint8
}

// MCchunkdatalocation interface methods
//
// these are principally for dealing with the odd choice to store the offset as a 24-bit number  (this choice is made
// even more odd by the fact that the 'count', taking up the 4th byte of what could be a conventional 32-bit number,
// is completely redundant, since the first piece of chunkdata is the length of the data)
//
func (cdl *MCchunkdatalocation) getOffsetValue() (rtrn int) {
	rtrn = (int(cdl.Offset[2]) << 0) | (int(cdl.Offset[1]) << 8) | (int(cdl.Offset[0]) << 16)

	return
}

func (cdl *MCchunkdatalocation) setOffset(value int) {
	var err error

	var bufval bytes.Buffer
	var arrval []byte

	err = binary.Write(&bufval, binary.BigEndian, uint32(value))
	panicOnErr(err)
	arrval = bufval.Bytes()

	cdl.Offset[0] = arrval[1]
	cdl.Offset[1] = arrval[2]
	cdl.Offset[2] = arrval[3]
}

type MCChunk struct {
}

type Glyph struct {
	Glyph string
	Name  string
	Type  string
	ID    uint32
	Data  uint8
	Base  NBT
}

type GlyphTag struct {
	Glyph Glyph
	Tag   string
	Items []Item
}

type Item struct {
	ID    string
	Slot  uint8
	Count uint8
	Data  uint8
	Tags  NBT
}
