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
	"os"
	"time"
)

///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
//  declare our internal datatypes and their interfaces  //////////////////////////////////////////////////////////////////////

// NBT datatypes
//
// NBT, Named Binary Tag, is the hierarchical data structure that Minecraft uses to store most game data
//
// TAG_End items are stored as the single byte indicating the type as TAG_End; other TAG types are followed by a name-length
// and a name, although the name can be empty (so, the length can be zero); _Array types are followed by the number of
// elements in the array and that number of elements of the appropriate size; TAG_List is followed by a TAG type byte, the
// number of List elements, and then that number of unnamed TAG items of that type (so, without repeating the type and with
// no name-length and no name string); TAG_Compound is followed by fully-formed TAG items (including nested TAG_List and
// TAG_Compound items) until ended by a TAG_End item
//
// TAG types other than TAG_Compound are of the size dictated by either datatype-size or encoded collection (Array, List) size,
// and so are followed immediately by the next item; TAG_Compound items are ended by a TAG_End item, because the size varies
// by the variability of the items that might follow it
//
// a TAG_List can be empty, in which case the type is TAG_End and the size is zero, with no further bytes for the List item

type NBTTAG_Type byte

const (
	TAG_End        NBTTAG_Type = iota //  0 : size: 0                 no payload, no name
	TAG_Byte                          //  1 : size: 1                 signed  8-bit integer
	TAG_Short                         //  2 : size: 2                 signed 16-bit integer
	TAG_Int                           //  3 : size: 4                 signed 32-bit integer
	TAG_Long                          //  4 : size: 8                 signed 64-bit integer
	TAG_Float                         //  5 : size: 4                 IEEE 754-2008 32-bit floating point number
	TAG_Double                        //  6 : size: 8                 IEEE 754-2008 64-bit floating point number
	TAG_Byte_Array                    //  7 : size: 4 + 1*elem        size TAG_Int, then payload [size]byte
	TAG_String                        //  8 : size: 2 + 4*elem        length TAG_Short, then payload (utf-8) string (of length length)
	TAG_List                          //  9 : size: 1 + 4 + len*elem  tagID TAG_Byte, length TAG_Int, then payload [length]tagID
	TAG_Compound                      // 10 : size: varies            { tagID TAG_Byte, name TAG_String, payload tagID }... TAG_End
	TAG_Int_Array                     // 11 : size: 4 + 4*elem        size TAG_Int, then payload [size]TAG_Int
	TAG_Long_Array                    // 12 : size: 4 + 8*elem        size TAG_Int, then payload [size]TAG_Long
	TAG_NULL                          // 13 : local extension of the NBT spec, for indicating 'not yet known', or 'read data to determine', etc.
)

var NBTTAG_Name = map[NBTTAG_Type]string{
	TAG_End:        "TAG_End",
	TAG_Byte:       "TAG_Byte",
	TAG_Short:      "TAG_Short",
	TAG_Int:        "TAG_Int",
	TAG_Long:       "TAG_Long",
	TAG_Float:      "TAG_Float",
	TAG_Double:     "TAG_Double",
	TAG_Byte_Array: "TAG_Byte_Array",
	TAG_String:     "TAG_String",
	TAG_List:       "TAG_List",
	TAG_Compound:   "TAG_Compound",
	TAG_Int_Array:  "TAG_Int_Array",
	TAG_Long_Array: "TAG_Long_Array",
	TAG_NULL:       "TAG_NULL",
}

func (tag NBTTAG_Type) String() string {
	name := "Unknown"

	switch tag {
	case TAG_End:
		name = "TAG_End"
	case TAG_Byte:
		name = "TAG_Byte"
	case TAG_Short:
		name = "TAG_Short"
	case TAG_Int:
		name = "TAG_Int"
	case TAG_Long:
		name = "TAG_Long"
	case TAG_Float:
		name = "TAG_Float"
	case TAG_Double:
		name = "TAG_Double"
	case TAG_Byte_Array:
		name = "TAG_Byte_Array"
	case TAG_String:
		name = "TAG_String"
	case TAG_List:
		name = "TAG_List"
	case TAG_Compound:
		name = "TAG_Compound"
	case TAG_Int_Array:
		name = "TAG_Int_Array"
	case TAG_Long_Array:
		name = "TAG_Long_Array"
	case TAG_NULL:
		name = "TAG_NULL"
	}

	return fmt.Sprintf("%s (0x%02x)", name, byte(tag))
}

// because NBT elements are of various datatypes, the Data property is an interface{}; this also supports the hierarchical
// nature of NBT, because it allows the Data property to itself be an NBT element
//
type wcnbt struct {
	Type NBTTAG_Type
	List NBTTAG_Type
	Name string
	Size uint32
	Data interface{}
}

// Worldcrft datatypes
//
// these model Minecraft data, with a focus on region files, since Worldcraft is a utility for editing region files

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
	X                  int
	Z                  int
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
	CX              int
	CZ              int
	Length          uint32
	CompressionType byte
	ChunkData       wcnbt
}

// these datatypes model editing instructions; they are also sketches for now, likely to be fleshed out more later as
// work on the actual editing capability proceeds
//
type wcblock struct {
	Y    int
	X    int
	Z    int
	ID   int
	Data int
}

type wcentity struct {
	Y          int
	X          int
	Z          int
	ID         int
	attributes map[string]interface{}
}

type wctileentity struct {
	Y          int
	X          int
	Z          int
	ID         int
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

var BlockEdits []wcblock
var EntityEdits []wcentity
var TileEntityEdits []wctileentity
var Regions []wcregion

///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
// main execution point
//
func main() {
	var err error

	///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
	// define global variables
	timeExec = time.Now()

	BlockEdits = make([]wcblock, 0, 1024)
	EntityEdits = make([]wcentity, 0, 1024)
	TileEntityEdits = make([]wctileentity, 0, 1024)
	Regions = make([]wcregion, 0, 4)

	///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
	// define the available command-line arguments
	pathWorld = flag.String("world", "UNDEFINED", "a directory containing a collection of Minecraft region files")
	fileEdits = flag.String("edits", "UNDEFINED", "a file containing a set of edits to make to the specified Minecraft world")
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

	///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
	// apply edits gatherd from input
	for _, elem := range BlockEdits {
		EditBlock(&elem)
	}

	for _, elem := range EntityEdits {
		EditEntity(&elem)
	}

	for _, elem := range TileEntityEdits {
		EditTileEntity(&elem)
	}

	///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
	// save the net effect of all edits to new region file(s)
	for _, elem := range Regions {
		SaveRegion(elem.X, elem.Z)
	}

	os.Exit(0)
}

///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
// data operation functions
//
func EditBlock(blk *wcblock) {
	var editRegion *wcregion

	editRegion = PickRegion(blk.X, blk.Z)
	fmt.Printf("EditBlock : will use region %d, %d\n", editRegion.X, editRegion.Z)

	editRegion.applyBlockEdit(blk)
}

func EditEntity(ent *wcentity) {
	var editRegion *wcregion

	editRegion = PickRegion(ent.X, ent.Z)
	fmt.Printf("EditEntity : will use region %d, %d\n", editRegion.X, editRegion.Z)

	editRegion.applyEntityEdit(ent)
}

func EditTileEntity(tnt *wctileentity) {
	var editRegion *wcregion

	editRegion = PickRegion(tnt.X, tnt.Z)
	fmt.Printf("EditTileEntity : will use region %d, %d\n", editRegion.X, editRegion.Z)

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
		if (elem.X == rx) && (elem.Z == rz) {
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
	newrgn := wcregion{X: rx, Z: rz}

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
		cx := indx % 32
		cz := int(indx / 32)

		// instantiate a new chunk object; we do this even if there will be no data to read, so that we stay in
		// alignment with the serial chunk index when we later scan through chunks to write out to file
		newchnk := wcchunk{CX: cx, CZ: cz}

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
						panic(fmt.Errorf("\n\n\nnot ZLib compression!  chunk %d, %d\n\n\n", cx, cz))
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
					newchnk.ChunkData, err = ReadNBTData(rdrTemp, TAG_NULL, "")

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
					// newchnk.ChunkData, err = ReadNBTData(rdrTemp, TAG_NULL, (fmt.Sprintf("chunk %d, %d", cx, cz)))
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
		if (elem.X == rx) && (elem.Z == rz) {
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
		if (elem.X == rx) && (elem.Z == rz) {
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
		if (rgn.Chunks[indx].CX != indx % 32) {
			panic(fmt.Errorf("\n\n\nunexpected CX coordinate; region %d, %d;  indx %d;  chunk %d, %d\n\n\n", rx, rz, indx, rgn.Chunks[indx].CX, rgn.Chunks[indx].CZ))
		}
		if (rgn.Chunks[indx].CZ != int(indx / 32)) {
			panic(fmt.Errorf("\n\n\nunexpected CY coordinate; region %d, %d;  indx %d;  chunk %d, %d\n\n\n", rx, rz, indx, rgn.Chunks[indx].CX, rgn.Chunks[indx].CZ))
		}

		// instantiate a buffer for the compressed NBT data -- what we want to actually write to file
		var bufZ bytes.Buffer

		if rgn.ChunkDataLocations[indx].getOffsetValue() != 0 {
			if rgn.ChunkDataLocations[indx].Count != 0 {
				if rgn.ChunkTimestamps[indx] != 0 {

					err = WriteNBTData(&bufChunkData, &rgn.Chunks[indx].ChunkData)
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
			fmt.Printf("zero-length zlib buffer for chunk %d\n", indx)
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
// NBT functions
//
func ReadNBTData(r *bytes.Reader, t NBTTAG_Type, debug string) (rtrnwcnbt wcnbt, err error) {
	var tb byte
	var tt NBTTAG_Type

	// 't' is essentially a sentinal value for reading / parsing TAG_List data; if we don't already know what type of
	// NBT item we are reading, start by reading the type from the input data; if we do know (if it's been passed in as
	// part of the function call), it means we aren't going to find it in the data (chiefly (only?), because we are
	// reading the elements of a TAG_List item

	if t == TAG_NULL {
		tb, err = r.ReadByte()
		panicOnErr(err)
		tt = NBTTAG_Type(tb)
	} else {
		tt = t
	}

	rtrnwcnbt = wcnbt{Type: tt}

	// if the NBT type is TAG_End, there is no further data to read, not even a name, nor even a name-length telling us
	// there is no name; TAG_End items are just the type indicator itself, which is perfect for how they are used
	if tt == TAG_End {
		return rtrnwcnbt, nil
	}

	// NBT items other than TAG_End have the type indicator followed by a name-length and a name; however, the length
	// is permitted to be zero, in which case of course only the bytes encoding a zero-length are there, which simply
	// means the name is an empty string
	//
	// the use of the input parameter 't' as a sentinal value for TAG_List elements is used here, too, since TAG_List
	// elements are nameless, which is differnet from haveing a name of "" : there isn't even a name-length indicator
	var strlen int16
	var name string
	if t == TAG_NULL {
		err = binary.Read(r, binary.BigEndian, &strlen)
		panicOnErr(err)
		if strlen > 0 {
			data := make([]byte, strlen)
			_, err = io.ReadFull(r, data)
			panicOnErr(err)
			name = string(data)
		} else {
			name = ""
		}
	} else {
		// since an emtpy string is a valid name, we use this as a sentinal value when writing NBT items back out
		// to indicated TAG_List elements, for which we must skip both the name and the name-length; yes, there is
		// potential collision with NBT items named "LISTELEM", but Minecraft does not currently do that anywhere
		name = "LISTELEM"
	}

	rtrnwcnbt.Name = name

	// see previous code comments near the main-loop call to ReadNBTData for the purpose of the 'debug' input parameter
	if debug != "" {
		debug = debug + fmt.Sprintf("; type %s; name %s", tt, name)
		fmt.Printf("%s\n", debug)
	}

	var b byte
	switch tt {
	case TAG_Byte:
		b, err = r.ReadByte()
		panicOnErr(err)

		rtrnwcnbt.Data = b

	case TAG_Short:
		var datashort int16
		err = binary.Read(r, binary.BigEndian, &datashort)
		panicOnErr(err)

		rtrnwcnbt.Data = datashort

	case TAG_Int:
		var dataint int32
		err = binary.Read(r, binary.BigEndian, &dataint)
		panicOnErr(err)

		rtrnwcnbt.Data = dataint

	case TAG_Long:
		var datalong int64
		err = binary.Read(r, binary.BigEndian, &datalong)
		panicOnErr(err)

		rtrnwcnbt.Data = datalong

	case TAG_Float:
		var datafloat float32
		err = binary.Read(r, binary.BigEndian, &datafloat)
		panicOnErr(err)

		rtrnwcnbt.Data = datafloat

	case TAG_Double:
		var datadouble float64
		err = binary.Read(r, binary.BigEndian, &datadouble)
		panicOnErr(err)

		rtrnwcnbt.Data = datadouble

	case TAG_String:
		var strlen int16
		err = binary.Read(r, binary.BigEndian, &strlen)
		panicOnErr(err)
		rtrnwcnbt.Size = uint32(strlen)

		data := make([]byte, strlen)
		_, err = io.ReadFull(r, data)
		panicOnErr(err)

		rtrnwcnbt.Data = string(data)

	case TAG_Byte_Array:
		var sizeint uint32
		err = binary.Read(r, binary.BigEndian, &sizeint)
		panicOnErr(err)
		rtrnwcnbt.Size = sizeint

		arraybyte := make([]byte, sizeint)
		err = binary.Read(r, binary.BigEndian, &arraybyte)
		panicOnErr(err)

		rtrnwcnbt.Data = arraybyte

	case TAG_Int_Array:
		var sizeint uint32
		err = binary.Read(r, binary.BigEndian, &sizeint)
		panicOnErr(err)
		rtrnwcnbt.Size = sizeint

		arrayint := make([]int32, sizeint)
		err = binary.Read(r, binary.BigEndian, &arrayint)
		panicOnErr(err)

		rtrnwcnbt.Data = arrayint

	case TAG_Long_Array:
		var sizeint uint32
		err = binary.Read(r, binary.BigEndian, &sizeint)
		panicOnErr(err)
		rtrnwcnbt.Size = sizeint

		arraylong := make([]int64, sizeint)
		err = binary.Read(r, binary.BigEndian, &arraylong)
		panicOnErr(err)

		rtrnwcnbt.Data = arraylong

	case TAG_List:
		// TAG_List NBT items include in their payload a byte indicating the NBT type of the elements of the
		// forthcoming List; this is one reason the List elements do not also bear the usual TAG_Type byte
		var id byte
		id, err = r.ReadByte()
		panicOnErr(err)
		rtrnwcnbt.List = NBTTAG_Type(id)

		var sizeint uint32
		err = binary.Read(r, binary.BigEndian, &sizeint)
		panicOnErr(err)
		rtrnwcnbt.Size = sizeint

		// the Data of a TAG_List NBT item is an array of NBT items
		listnbt := make([]wcnbt, sizeint)

		// we use a recursive call to this function to read in the List elements; along with TAG_Compound, this
		// manifests the hierarchical nature of the NBT encoding scheme;  for these List elements, though, we send
		// in the TAG_Type of the List elements; see code comments at the top of this function for more detail why
		for indx := 0; indx < int(sizeint); indx++ {
			listnbt[indx], err = ReadNBTData(r, NBTTAG_Type(id), debug)
			panicOnErr(err)
		}

		// the Data of a TAG_List NBT item is an array of NBT items
		rtrnwcnbt.Data = listnbt

	case TAG_Compound:
		// the Data of a TAG_Compound NBT item is a collection of fully-formed NBT items
		rtrnwcnbt.Data = make(map[string]wcnbt)
		rtrnwcnbt.Size = 0

		var nbt wcnbt
		for {
			// we use a recursive call to this function to read in the Compound elements; along with TAG_List,
			// this manifests the hierarchical nature of the NBT encoding scheme;  unlike TAG_List, each
			// TAG_Compound element is a fully-formed NBT item, so we call ReadNBTData() in the normal manner
			nbt, err = ReadNBTData(r, TAG_NULL, debug)
			panicOnErr(err)

			// TAG_Compound has no other way to indicate the end of the collection, other than TAG_End
			if nbt.Type == TAG_End {
				break
			}

			// we track and store the size of this TAG_Compound item, for potential future usefulness; this is
			// not written back out when we write the NBT data
			rtrnwcnbt.Size++

			// the Data of a TAG_Compound NBT item is a collection of fully-formed NBT items
			refA := rtrnwcnbt.Data.(map[string]wcnbt)
			refA[nbt.Name] = nbt
		}

	default:
		panic(fmt.Errorf("\n\n\nReadNBTData : TAG type unkown! [%d]\n\n\n", tt))
	}

	return rtrnwcnbt, err
}

func WriteNBTData(buf *bytes.Buffer, srcwcnbt *wcnbt) (err error) {
	// if we reach this point with an NBTTAG bearing our internal NULL-type TAG or nil data, something went wrong
	// somewhere, so we abend
	if srcwcnbt.Type == TAG_NULL {
		panic(fmt.Errorf("\n\n\nattempted to write a TAG type NULL!\n\n\n"))
	}

	if srcwcnbt.Data == nil {
		panic(fmt.Errorf("\n\n\nattempted to write a TAG name of nil!\n\n\n"))
	}

	// if the Name of this NBTTAG is "LISTELEM", then it is an element of a TAG_List, and we store only the payload; the
	// type of the list elements has already been stored at the start of the TAG_List, and each element is nameless, not
	// even having the 0-byte normally used to indicate a 0-length name
	//
	// otherwise, it is a named TAG, so before storing the payload, we store the TAG type, the length of the name and the
	// name itself; although the name might be zero-length
	if srcwcnbt.Name != "LISTELEM" {
		err = binary.Write(buf, binary.BigEndian, byte(srcwcnbt.Type))
		panicOnErr(err)

		// TAG_End never has a name, nor a name length, so we are done after just storing the type
		if srcwcnbt.Type == TAG_End {
			return nil
		}

		strlen := len(srcwcnbt.Name)
		err = binary.Write(buf, binary.BigEndian, int16(strlen))
		panicOnErr(err)

		if strlen > 0 {
			_, err = buf.WriteString(srcwcnbt.Name)
			panicOnErr(err)
		}
	}

	switch srcwcnbt.Type {
	case TAG_Byte:
		err = binary.Write(buf, binary.BigEndian, srcwcnbt.Data.(byte))
		panicOnErr(err)

	case TAG_Short:
		err = binary.Write(buf, binary.BigEndian, srcwcnbt.Data.(int16))
		panicOnErr(err)

	case TAG_Int:
		err = binary.Write(buf, binary.BigEndian, srcwcnbt.Data.(int32))
		panicOnErr(err)

	case TAG_Long:
		err = binary.Write(buf, binary.BigEndian, srcwcnbt.Data.(int64))
		panicOnErr(err)

	case TAG_Float:
		err = binary.Write(buf, binary.BigEndian, srcwcnbt.Data.(float32))
		panicOnErr(err)

	case TAG_Double:
		err = binary.Write(buf, binary.BigEndian, srcwcnbt.Data.(float64))
		panicOnErr(err)

	case TAG_String:
		strlen := len(srcwcnbt.Data.(string))
		err = binary.Write(buf, binary.BigEndian, int16(strlen))
		panicOnErr(err)

		if strlen > 0 {
			_, err = buf.WriteString(srcwcnbt.Data.(string))
			panicOnErr(err)
		}

	case TAG_Byte_Array:
		err = binary.Write(buf, binary.BigEndian, srcwcnbt.Size)
		panicOnErr(err)

		err = binary.Write(buf, binary.BigEndian, srcwcnbt.Data.([]byte))
		panicOnErr(err)

	case TAG_Int_Array:
		err = binary.Write(buf, binary.BigEndian, srcwcnbt.Size)
		panicOnErr(err)

		err = binary.Write(buf, binary.BigEndian, srcwcnbt.Data.([]int32))
		panicOnErr(err)

	case TAG_Long_Array:
		err = binary.Write(buf, binary.BigEndian, srcwcnbt.Size)
		panicOnErr(err)

		err = binary.Write(buf, binary.BigEndian, srcwcnbt.Data.([]int64))
		panicOnErr(err)

	case TAG_List:
		id := srcwcnbt.List
		err = binary.Write(buf, binary.BigEndian, byte(id))
		panicOnErr(err)

		err = binary.Write(buf, binary.BigEndian, srcwcnbt.Size)
		panicOnErr(err)

		arrlen := len(srcwcnbt.Data.([]wcnbt))
		for indx := 0; indx < int(arrlen); indx++ {
			tmpwcnbt := srcwcnbt.Data.([]wcnbt)[indx]
			err = WriteNBTData(buf, &tmpwcnbt)
			panicOnErr(err)
		}

	case TAG_Compound:
		for _, v := range srcwcnbt.Data.(map[string]wcnbt) {
			err = WriteNBTData(buf, &v)
			panicOnErr(err)
		}
		// we used the TAG_End at the end of a collection of TAG_Compound elements to break out of the reading loop;
		// so, we have not stored it; so, we write out a TAG_End NBT item after writing out all the Compound elements
		err = binary.Write(buf, binary.BigEndian, byte(TAG_End))
		panicOnErr(err)

	default:
		panic(fmt.Errorf("\n\n\nWriteNBTData : TAG type unkown! [%d]\n\n\n", srcwcnbt.Type))
	}

	return err
}

///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
// utility functions
//
func panicOnErr(e error) {
	if e != nil {
		panic(e)
	}
}
