package main

///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
//  import necessary external packages  ///////////////////////////////////////////////////////////////////////////////////////

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"nbt"
	"os"
	"time"
)

///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
//  declare our internal datatypes and their interfaces  //////////////////////////////////////////////////////////////////////

// Worldcrft datatypes
//
// these model Minecraft data, with a focus on region files, since Worldcraft is a utility for editing region files
//

// a region is located in the Minecraft world on a horizontal x (west-to-east), z (north-to-south) grid; the x, z coordinates
// are stored in the filename instead of in the file; we store them in our wcregion datatype, to maintain the association
// after we are done dealing with the input file
//
// a region contains 1024 chunks in a 32x32 grid
//
// a region file has (1) a 4KB block for 1024 4-byte chunkdata descriptors, (2) a 4KB block for 1024 32-bit timestamps,
// and (3) a set of 4KB blocks for each defined chunk;  chunks that have not been created/defined do not have any chunk-data
// in the file;  chunkdata is stored compressed, and padded out to the next 4KB boundary
//
// because we know the number of chunkdata descriptor elements and timestamp elements, we declare fixed-size arrays for
// them; because there will be an unknown number (zero or more) of chunks actually defined, we declare an unsized array
// for them
//
type wcregion struct {
	RX                 int
	RZ                 int
	ChunkDataLocations [1024]wcchunkdatalocation
	ChunkTimestamps    [1024]int32
	Chunks             []wcchunk
}

// wcregion interface methods
//
// this early version of this utility does not actually perform any edits; we are stil in the final stages of validating
// the fidelity of NBT reading and writing
//
func (wcr *wcregion) applyBlockEdit(blk *wcblock) {
	rx := int(math.Floor(float64(blk.X) / 512.0))
	rz := int(math.Floor(float64(blk.Z) / 512.0))
	//fmt.Printf("rx, rz : %d, %d\n", rx, rz)

	cx := int(math.Floor(float64(blk.X) /  16.0))
	cy := int(                   blk.Y  /  16   )
	cz := int(math.Floor(float64(blk.Z) /  16.0))
	//fmt.Printf("cx, cy, cz : %d, %d, %d\n", cx, cy, cz)

	datapathBlocks := fmt.Sprintf("/rx%d/rz%d/cx%d/cz%d/Level/Sections/%d/Blocks", rx, rz, cx, cz, cy)
	dataBlocks := nbt.DataPaths[datapathBlocks]

	datapathBlockData := fmt.Sprintf("/rx%d/rz%d/cx%d/cz%d/Level/Sections/%d/Data", rx, rz, cx, cz, cy)
	dataBlockData := nbt.DataPaths[datapathBlockData]

	if dataBlocks == nil {
		qtyBlockEditsSkipped++
		return
	}

	if dataBlockData == nil {
		qtyBlockEditsSkipped++
		return
	}

	ix := blk.X - (cx * 16)
	iy := blk.Y       % 16
	iz := blk.Z - (cz * 16)
	indxBlocks := (iy * 256) + (iz * 16) + ix
	//fmt.Printf("ix, iy, iz, indxBlocks : %d, %d, %d, %d\n", ix, iy, iz, indxBlocks)

	valuBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(valuBytes, blk.ID)
//	valuAddtnl := valuBytes[0]
	valuBlocks := valuBytes[1]

	dataBlocks.Data.([]byte)[indxBlocks] = valuBlocks

	indxBlockData := int(indxBlocks / 2)
	currDataValue := dataBlockData.Data.([]byte)[indxBlockData]
	var keepNybble, valuNybble byte
	if (indxBlocks % 2) == 0 {
		keepNybble = currDataValue & 0xF0
		valuNybble = blk.Data
	} else {
		keepNybble = currDataValue & 0x0F
		valuNybble = blk.Data << 4
	}
	dataBlockData.Data.([]byte)[indxBlockData] = keepNybble + valuNybble
	//fmt.Printf("cx, cy, cz, ix, iy, iz;    ID;    DataA, DataB = Data  :  %d, %d, %d, %d, %d, %d;    %d;    %d, %d = %d\n", cx, cy, cz, ix, iy, iz, valuBlocks, keepNybble, valuNybble, (keepNybble + valuNybble))

	qtyBlockEdits++
}

func (wcr *wcregion) applyEntityEdit(blk *wcentity) {
}

func (wcr *wcregion) applyTileEntityEdit(blk *wctileentity) {
}

// a chunkdata descriptor indicates where within the region file the chuck data is found;  the offset is the (0-indexed)
// index of the first 4KB block holding the chunk data; the count is the number of 4KB blocks used for this chunk;  the offset
// does not ignore the two header blocks, so the lowest offset for chunk data will be "2", i.e. starting at byte 8192 in the
// region file (i.e., the 8,193rd byte, the start of the 3rd 4KB block)
//
type wcchunkdatalocation struct {
	Offset [3]byte
	Count  uint8
}

// wcchunkdatalocation interface methods
//
// these are principally for dealing with the odd choice to store the offset as a 24-bit number  (this choice is made
// even more odd by the fact that the 'count', taking up the 4th byte of what could be a conventional 32-bit number,
// is completely redundant, since the first piece of chunkdata is the length of the data)
//
func (wccdl *wcchunkdatalocation) getOffsetValue() (rtrn int) {
	rtrn = (int(wccdl.Offset[2]) << 0) | (int(wccdl.Offset[1]) << 8) | (int(wccdl.Offset[0]) << 16)

	return
}

func (wccdl *wcchunkdatalocation) setOffset(value int) {
	var err error

	var bufval bytes.Buffer
	var arrval []byte

	err = binary.Write(&bufval, binary.BigEndian, uint32(value))
	panicOnErr(err)
	arrval = bufval.Bytes()

	wccdl.Offset[0] = arrval[1]
	wccdl.Offset[1] = arrval[2]
	wccdl.Offset[2] = arrval[3]
}

// chunks are stored in serial order in the two 4KB header blocks, as a simple list of 1,024 chunks;  the x, z coordinates
// of the chunk within the region can be derived (x = index % 32, z = floor(index / 32)); we store the x, z coordinates for
// convenience and to support debug output
//
type wcchunk struct {
	IX              int
	IZ              int
	CX              int
	CZ              int
	Length          uint32
	CompressionType byte
	ChunkData       nbt.NBT
}

// these datatypes model editing instructions; they are also sketches for now, likely to be fleshed out more later as
// work on the actual editing capability proceeds
//
type wcblock struct {
	X    int
	Y    int
	Z    int
	ID   uint16
	Data byte
}

type wcentity struct {
	X          int
	Y          int
	Z          int
	ID         uint16
	attributes map[string]interface{}
}

type wctileentity struct {
	X          int
	Y          int
	Z          int
	ID         uint16
	attributes map[string]interface{}
}

type wcedit struct {
	Type string
	Info map[string]interface{}
}

///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
// declare global variables
//
var timeExec time.Time

var pathWorld *string
var fileEdits *string
var flagJSOND *bool

var BlockEdits []wcblock
var EntityEdits []wcentity
var TileEntityEdits []wctileentity

var Regions []wcregion
//var DataPaths map[string]*NBT

var qtyBlockEdits int
var qtyBlockEditsSkipped int
var qtyEntityEdits int
var qtyEntityEditsSkipped int
var qtyTileEntityEdits int
var qtyTileEntityEditsSkipped int

///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
// main execution point
//
func main() {
	var err error

	///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
	// define global variables
	timeExec = time.Now()

	BlockEdits = make([]wcblock, 0, 0)
	EntityEdits = make([]wcentity, 0, 0)
	TileEntityEdits = make([]wctileentity, 0, 0)
	Regions = make([]wcregion, 0, 0)
	nbt.DataPaths = make(map[string]*nbt.NBT, 0)

	qtyBlockEdits = 0
	qtyBlockEditsSkipped = 0
	qtyEntityEdits = 0
	qtyEntityEditsSkipped = 0
	qtyTileEntityEdits = 0
	qtyTileEntityEditsSkipped = 0

	///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
	// define the available command-line arguments
	pathWorld = flag.String("world", "UNDEFINED", "a directory containing a collection of Minecraft region files")
	fileEdits = flag.String("edits", "UNDEFINED", "a file containing a set of edits to make to the specified Minecraft world")
	flagJSOND = flag.Bool("json", false, "a flag to enable dumping the chunkdata to JSON")
	flag.Parse()

	// report to the user what values will be used
	fmt.Printf("world directory : %s\n", *pathWorld)
	fmt.Printf("edits file      : %s\n", *fileEdits)
	fmt.Printf("\n")

	///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
	// read in the edits file, and parse as generic JSON

	var bufEdits []byte
	var jsonEdits interface{}

	bufEdits, err = ioutil.ReadFile(*fileEdits)
	panicOnErr(err)

	err = json.Unmarshal(bufEdits, &jsonEdits)
	panicOnErr(err)

	///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
	// scan through the JSON looking for recognizable objects;
	// assign such objects to the appropriate datatype

	m := jsonEdits.(map[string]interface{})
	for k, v := range m {

		// make sure we are looking at an array
		_, ok := v.([]interface{})
		if ok {
			for _, elem := range v.([]interface{}) {
				// marshal the JSON back into data bytes so we can
				// unmarshal it back into one of our internal types
				var bufEdit []byte
				var wce wcedit

				bufEdit, err = json.Marshal(elem)
				panicOnErr(err)
				err = json.Unmarshal(bufEdit, &wce)
				panicOnErr(err)

				// handle each known datatype, and handle any unknowns
				switch wce.Type {
				case `block`:
					var wcb wcblock

					bufEdit, err = json.Marshal(wce.Info)
					panicOnErr(err)
					err = json.Unmarshal(bufEdit, &wcb)
					panicOnErr(err)

					BlockEdits = append(BlockEdits, wcb)

				case `entity`:
					var wcn wcentity

					bufEdit, err = json.Marshal(wce.Info)
					panicOnErr(err)
					err = json.Unmarshal(bufEdit, &wcn)
					panicOnErr(err)

					EntityEdits = append(EntityEdits, wcn)

				case `tileentity`:
					var wct wctileentity

					bufEdit, err = json.Marshal(wce.Info)
					panicOnErr(err)
					err = json.Unmarshal(bufEdit, &wct)
					panicOnErr(err)

					TileEntityEdits = append(TileEntityEdits, wct)

				default:
					fmt.Println("unknown edit type : ", k)
				}
			}
		} else {
			fmt.Println("edits file entry is not a map of strings : ", k)
		}
	}
	fmt.Printf("\n")

	///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
	// apply edits gatherd from input

	for _, elem := range BlockEdits {
		EditBlock(&elem)
	}
	fmt.Printf("blocks edits applied and skipped :  %5d,  %5d\n", qtyBlockEdits, qtyBlockEditsSkipped)

	for _, elem := range EntityEdits {
		EditEntity(&elem)
	}
	fmt.Printf("blocks edits applied and skipped :  %5d,  %5d\n", qtyEntityEdits, qtyEntityEditsSkipped)

	for _, elem := range TileEntityEdits {
		EditTileEntity(&elem)
	}
	fmt.Printf("blocks edits applied and skipped :  %5d,  %5d\n", qtyTileEntityEdits, qtyTileEntityEditsSkipped)
	fmt.Printf("\n")

	///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
	// save the net effect of all edits to new region file(s)

	for _, elem := range Regions {
		SaveRegion(elem.RX, elem.RZ)
	}
	fmt.Printf("\n")

	os.Exit(0)
}

///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
// data operation functions
//
func EditBlock(blk *wcblock) {
	var editRegion *wcregion

	editRegion = PickRegion(blk.X, blk.Z)
	editRegion.applyBlockEdit(blk)
}

func EditEntity(ent *wcentity) {
	var editRegion *wcregion

	editRegion = PickRegion(ent.X, ent.Z)
	editRegion.applyEntityEdit(ent)
}

func EditTileEntity(tnt *wctileentity) {
	var editRegion *wcregion

	editRegion = PickRegion(tnt.X, tnt.Z)
	editRegion.applyTileEntityEdit(tnt)
}

///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
// region file handling functions
//
func PickRegion(wx, wz int) (rgn *wcregion) {
	var err error

	// blocks are identified in terms of world-coordinates, so we need to first translate from world-coordinates to
	// region-coordinates; if we've already loaded the indicated region, use it; if not, load it

	// calculate the region-x, region-z where the input world-x, world-z resides
	rx := int(math.Floor(float64(wx) / 512.0))
	rz := int(math.Floor(float64(wz) / 512.0))

	// check to see if the indicated region is already loaded
	rgn = nil
	for _, elem := range Regions {
		if (elem.RX == rx) && (elem.RZ == rz) {
			rgn = &elem
			break
		}
	}

	// if the indicated region is not yet loaded, load it into memory
	if rgn == nil {
		rgn, err = LoadRegion(rx, rz)
		panicOnErr(err)
	}

	// return a pointer to the indicated region
	return
}

func LoadRegion(rx, rz int) (rgn *wcregion, e error) {
	var err error
	var filename string

	// region files are stored in the 'world' directory, whose path has been previously set, and are identified by their
	// x, z coordinates

	// construct the fqfn, establish an area in memory for holding the file contents, and read the file in
	filename = fmt.Sprintf("%s/r.%d.%d.mca", *pathWorld, rx, rz)
	fmt.Printf("LoadRegion filename is  : %s\n", filename)
	var bufFile []byte
	bufFile, err = ioutil.ReadFile(filename)
	panicOnErr(err)

	// instantiate a new region object
	newrgn := wcregion{RX: rx, RZ: rz}

	// slice the filedata to read the header blocks, and store into our 1024-element arrays for holding this information
	rChunkDataLocations := bytes.NewReader(bufFile[0:4096])
	err = binary.Read(rChunkDataLocations, binary.BigEndian, &newrgn.ChunkDataLocations)
	panicOnErr(err)

	rChunkTimestamps := bytes.NewReader(bufFile[4096:8192])
	err = binary.Read(rChunkTimestamps, binary.BigEndian, &newrgn.ChunkTimestamps)
	panicOnErr(err)

	// iterate over the 1024 possible chuncks in this region, looking for chunkdata to read
	for indx := 0; indx < 1024; indx++ {
		// calculate the in-region chunk coordinates from the serial chunk index
		ix := indx % 32
		iz := int(indx / 32)
		// and the world-coordinates
		cx := ix + (rx * 32)
		cz := iz + (rz * 32)

		// instantiate a new chunk object; we do this even if there will be no data to read, so that we stay in
		// alignment with the serial chunk index when we later scan through chunks to write out to file
		newchnk := wcchunk{IX: ix, IZ: iz, CX: cx, CZ: cz}

		// the Minecraft specs don't seem to indicate this, but we deduce that a chunk is only a defined chunk if
		// it has a non-zero data offset, data-block count, and timestamp

		if newrgn.ChunkDataLocations[indx].getOffsetValue() != 0 {
			if newrgn.ChunkDataLocations[indx].Count != 0 {
				if newrgn.ChunkTimestamps[indx] != 0 {

					// calculate filedata locations based on information from the header blocks
					offset := newrgn.ChunkDataLocations[indx].getOffsetValue()
					count := newrgn.ChunkDataLocations[indx].Count

					offsetbeg := uint32(offset) * 4096
					offsetend := offsetbeg + (uint32(count) * 4096)
					rChunkInfo := bytes.NewReader(bufFile[offsetbeg:offsetend])

					// read in 5 bytes (a 4-byte uint32 and a single byte) in order to determine the length
					// and compression scheme of the chunkdata
					var length uint32
					var cmpres byte
					err = binary.Read(rChunkInfo, binary.BigEndian, &length)
					panicOnErr(err)
					err = binary.Read(rChunkInfo, binary.BigEndian, &cmpres)
					panicOnErr(err)

					// the Minecraft region file spec supports both GZip and ZLib compression of chunk
					// data, but in practice only ZLib is used; if that ever changes, we can update this
					// utility; until then, it's premature optimization
					//
					if cmpres != 2 {
						panic(fmt.Errorf("\n\n\nnot ZLib compression!  chunk %d, %d\n\n\n", ix, iz))
					}

					newchnk.Length = length
					newchnk.CompressionType = cmpres

					// reslice for the actual worlddata of this chunk
					databeg := offsetbeg + 5
					dataend := offsetbeg + 5 + length
					rData := bytes.NewReader(bufFile[databeg:dataend])

					// uncompress the data so that we can work with it
					rChunkInfoZLib, err := zlib.NewReader(rData)
					panicOnErr(err)
					var bufTemp bytes.Buffer
					io.Copy(&bufTemp, rChunkInfoZLib)

					// parse the data out of Minecraft's NBT format into data structures we interact with
					var rdrTemp *bytes.Reader
					rdrTemp = bytes.NewReader(bufTemp.Bytes())
					//newchnk.ChunkData, err = nbt.ReadNBTData(rdrTemp, TAG_NULL, "")

					// this next line can be used in place of the above ReadNBTData() line, as a way to
					// produce debug output; by producing this output and by processing both an orginal
					// region file and then the output file that it produces (with some NOOP edit), we
					// can validate the fidelity of this utility's ablity to read and write NBT;  in
					// most cases, the output needs to be sort'd to be comparable, because we use a map[]
					// to store TAG_Compound subitems; including the chunk identifier in the output keeps
					// each chunck's data together in the sort, for a valid comparison
					//
					// tests show that the only differences are in the exact length of per-chunk data,
					// likely due to subtle but inconsequential differences in the ZLib compression
					// library implementation
					//
					newchnk.ChunkData, err = nbt.ReadNBTData(rdrTemp, nbt.TAG_NULL, (fmt.Sprintf("chunk %d, %d", ix, iz)))

					// build up a list of paths, for later use addressing edits to particular data structures
					stemdatapath := fmt.Sprintf("/rx%d/rz%d/cx%d/cz%d", rx, rz, cx, cz)
					err = nbt.BuildDataPaths(&newchnk.ChunkData, stemdatapath)
					panicOnErr(err)
				}
			}
		}

		// add the new chunk (whether we also populated it with data or not) into this region's list of chunks
		newrgn.Chunks = append(newrgn.Chunks, newchnk)
	}

	// add this region to the global list of regions
	Regions = append(Regions, newrgn)

	// find the newly-added region in our global array of regions, so that we can return a pointer to that instance
	// of the region, instead of the temporary object instantiated inside this function
	for _, elem := range Regions {
		if (elem.RX == rx) && (elem.RZ == rz) {
			rgn = &elem
			break
		}
	}

	return
}

func SaveRegion(rx, rz int) (e error) {
	var err error
	var filename string

	// open a new file for holding the edited region; we include a timestamp in the
	// filename to avoid overwriting the original file and any edited files already made
	filename = fmt.Sprintf("r.%d.%d.mca.%d", rx, rz, timeExec.Unix())    // add '%s/' and '*pathWorld' to create the new file next to the original
	fmt.Printf("SaveRegion filename is  : %s\n", filename)

	fh, err := os.Create(filename)
	panicOnErr(err)
	defer fh.Close()

	// regions might be loaded in any order; find the region we want by scanning the array of regions for an x, z match
	var rgn wcregion
	for _, elem := range Regions {
		if (elem.RX == rx) && (elem.RZ == rz) {
			fmt.Printf("SaveRegion found region for : %d, %d\n", rx, rz)
			rgn = elem
			break
		}
	}

	// begin writing chunkdata at the 3rd 4KB block, in order to skip over the two header blocks
	totaloffset := 2

	// gather up chunkdata into a set of buffers; we do this first, because some header information (the number of 4KB
	// blocks the data occupies) depends on preparing the data, and of course the data might have changed radically
	// as a result of edits applied
	var bufChunkDataSet []bytes.Buffer
	for indx := 0; indx < 1024; indx++ {
		var bufChunkData bytes.Buffer

		// sanity check to make sure we are dealing with the correct chunk
		if (rgn.Chunks[indx].IX != indx % 32) {
			panic(fmt.Errorf("\n\n\nunexpected IX coordinate; region %d, %d;  indx %d;  chunk %d, %d\n\n\n", rx, rz, indx, rgn.Chunks[indx].IX, rgn.Chunks[indx].IZ))
		}
		if (rgn.Chunks[indx].IZ != int(indx / 32)) {
			panic(fmt.Errorf("\n\n\nunexpected IZ coordinate; region %d, %d;  indx %d;  chunk %d, %d\n\n\n", rx, rz, indx, rgn.Chunks[indx].IX, rgn.Chunks[indx].IZ))
		}

		// optionally output the chunkdata to JSON, for various sorts of external analysis
		if *flagJSOND == true {
			var bufJSON []byte
			bufJSON, err = json.MarshalIndent(&rgn.Chunks[indx].ChunkData, "", "  ")
			panicOnErr(err)
			os.Stdout.Write(bufJSON)
		}

		// instantiate a buffer for the compressed NBT data -- what we want to actually write to file
		var bufZ bytes.Buffer

		if rgn.ChunkDataLocations[indx].getOffsetValue() != 0 {
			if rgn.ChunkDataLocations[indx].Count != 0 {
				if rgn.ChunkTimestamps[indx] != 0 {

					err = nbt.WriteNBTData(&bufChunkData, &rgn.Chunks[indx].ChunkData)
					panicOnErr(err)

					wz := zlib.NewWriter(&bufZ)
					wz.Write(bufChunkData.Bytes())
					wz.Close()

					// the additional byte in 'leninfo' is to account for the compression-type byte
					lendata := len(bufZ.Bytes())
					leninfo := lendata + 1
					// the length and compression type that we store are within the 1st 4KB block,
					// so we take them into account when calculating how many 4KB blocks we need
					lenin4k := int((leninfo + 4) / 4096) + 1

					rgn.ChunkDataLocations[indx].setOffset(totaloffset)
					rgn.ChunkDataLocations[indx].Count = uint8(lenin4k)

					rgn.Chunks[indx].Length = uint32(leninfo)
					rgn.Chunks[indx].CompressionType = 2

					// store the next block(s) of chunkdata after this block / these blocks
					totaloffset += lenin4k
				}
			}
		}

		// retain the result, even if we skipped writing out any NBT data, to stay in alignment with the 1024-element
		// loops during the rest of this function
		bufChunkDataSet = append(bufChunkDataSet, bufZ)
	}

	// write out the chuckdata location-in-file informtion
	for indx := 0; indx < 1024; indx++ {
		err = binary.Write(fh, binary.BigEndian, rgn.ChunkDataLocations[indx].Offset)
		panicOnErr(err)
		err = binary.Write(fh, binary.BigEndian, rgn.ChunkDataLocations[indx].Count)
		panicOnErr(err)
	}

	// write out the chunk timestamp information
	for indx := 0; indx < 1024; indx++ {
		err = binary.Write(fh, binary.BigEndian, rgn.ChunkTimestamps[indx])
		panicOnErr(err)
	}

	// write out the chunkdata, gathered up above; pad the chunkdata to the next 4KB boundary, because everything about
	// region files is in terms of 4KB blocks of filedata
	for indx := 0; indx < 1024; indx++ {
		// skip over chunks that are not defined by data
		if len(bufChunkDataSet[indx].Bytes()) == 0 {
			continue
		}

		err = binary.Write(fh, binary.BigEndian, rgn.Chunks[indx].Length)
		panicOnErr(err)
		err = binary.Write(fh, binary.BigEndian, rgn.Chunks[indx].CompressionType)
		panicOnErr(err)

		lenzpad := (int(rgn.ChunkDataLocations[indx].Count) * 4096) - int(rgn.Chunks[indx].Length + 4)
		for pad := 0; pad < lenzpad; pad++ {
			bufChunkDataSet[indx].WriteByte(0)
		}
		err = binary.Write(fh, binary.BigEndian, bufChunkDataSet[indx].Bytes())
		panicOnErr(err)
	}

	fh.Sync()

	return
}

///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
// utility functions
//
func panicOnErr(e error) {
	if e != nil {
		panic(e)
	}
}
