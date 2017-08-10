package main

///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
//  import necessary external packages  ///////////////////////////////////////////////////////////////////////////////////////

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"os"
)

///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
//  declare our internal datatypes and their interfaces  //////////////////////////////////////////////////////////////////////

type MCWorld struct {
	FlagDebug bool
	FlagJSOND bool
	FlagXAirBlocks bool
	FlagSkipEntities bool
	FlagResetBlockEntities bool
	PathWorld string
	Regions   []MCRegion
}

func (w *MCWorld) EditBlock(x int, y int, z int, id uint16, data uint8) (err error) {

	// this flag causes 'air' blocks ('.' blueprint glyph) to be treated like 'null' blocks ('X' blueprint glyph);
	// this is useful when redo'ing a blueprint after fixing the blocks on the blueprint; assuming the blueprint
	// is reasonably complete to begin with, things that one has added to the gameworld will tend to be in the
	// 'empty' spaces -- i.e., where there is only air;  with this flag, redo'ing a blueprint will preserve those
	// sorts of in-game edits;  crops that have been planted will be preserved, too, if those planting areas do
	// not have planted-crops defined on the blueprint
	//
	// this is not a 100% solution of course; places where the ground has been trimmed away, structures that have
	// been tweaked, etc. will be reset according to the blueprint; but it should work well as a 96% solution
	//
	if w.FlagXAirBlocks {
		if id == 0 {
			qtyBlockEditsSkipped++
			return
		}
	}

	var fqsn string

	rgn, err := w.LoadRegion(x, y, z)
	panicOnErr(err)
	rx := rgn.RX
	rz := rgn.RZ

	// calculate the in-region chunk coordinates and chunkdata index
	cx := int(math.Floor(float64(x) / 16.0))
	cy := int(                   y  / 16   )
	cz := int(math.Floor(float64(z) / 16.0))
	indxChunk := ((cz - (rz * 32)) * 32) + (cx - (rx * 32))

	// calculate the in-chunk block coordinates and blockdata index
	ix := x - (cx * 16)
	iy := y       % 16
	iz := z - (cz * 16)
	indxBlock := (iy * 256) + (iz * 16) + ix

	// empty Sections of a chunk are not stored in the region file, but we might want to build into them anyway;
	// thus, if a Section is not in the current data, we first add it as a Section filled with air; we also
	// add any empty sections between this one and the first existing one below this one;  in theory, Minecraft
	// supports missing Sections inbetween existing Sections, but Minecraft itself seems to define them anyway
	// when the occassion arises
	//
	for indx := 0; indx <= cy; indx++ {
		fqsn = fmt.Sprintf("Sections/%d/Y", indx)
		if rgn.Chunks[indxChunk].ChunkDataRefs[fqsn] == nil {
			sectiondataA := NBT{TAG_Byte, 0, "Y", 0, byte(indx)}
			sectiondataB := NBT{TAG_Byte_Array, 0, "Blocks", 4096, make([]byte, 4096)}
			sectiondataC := NBT{TAG_Byte_Array, 0, "Data", 2048, make([]byte, 2048)}
			sectiondataD := NBT{TAG_Byte_Array, 0, "SkyLight", 2048, make([]byte, 2048)}
			sectiondataE := NBT{TAG_Byte_Array, 0, "BlockLight", 2048, make([]byte, 2048)}
			sectiondata := []NBT{sectiondataA, sectiondataB, sectiondataC, sectiondataD, sectiondataE}
			section := NBT{TAG_Compound, 0, "LISTELEM", 5, sectiondata}

			fqsn = fmt.Sprintf("Sections")
			dataSections := rgn.Chunks[indxChunk].ChunkDataRefs[fqsn]
			dataSections.Size++
			dataSections.Data = append(dataSections.Data.([]NBT), section)

			fqsn = fmt.Sprintf("Sections/%d/Y", indx)
			rgn.Chunks[indxChunk].ChunkDataRefs[fqsn] = &dataSections.Data.([]NBT)[indx].Data.([]NBT)[0]

			fqsn = fmt.Sprintf("Sections/%d/Blocks", indx)
			rgn.Chunks[indxChunk].ChunkDataRefs[fqsn] = &dataSections.Data.([]NBT)[indx].Data.([]NBT)[1]

			fqsn = fmt.Sprintf("Sections/%d/Data", indx)
			rgn.Chunks[indxChunk].ChunkDataRefs[fqsn] = &dataSections.Data.([]NBT)[indx].Data.([]NBT)[2]
		}
	}

	// fetch references to the data structures we need to update; return early if they do not exist
	fqsn = fmt.Sprintf("Sections/%d/Blocks", cy)
	dataBlocks := rgn.Chunks[indxChunk].ChunkDataRefs[fqsn]

	fqsn = fmt.Sprintf("Sections/%d/Data", cy)
	dataBlockData := rgn.Chunks[indxChunk].ChunkDataRefs[fqsn]

	if dataBlocks == nil {
		qtyBlockEditsSkipped++
		return
	}

	if dataBlockData == nil {
		qtyBlockEditsSkipped++
		return
	}

	// Minecraft block IDs historically have been less than 256, but the chunkdata format actually
	// supports 12-bit values, with the second bit being stored in one of the nybbles of an additional
	// array that is half the size of the regular block ID array ...  this is compactness at the price
	// of simplicity; we keep it simple by dealing with just the legacy 8-bit portion and being content
	// with blueprints that only make use of the legacy blocks
	//
	// but, we keep the commented-out line for valuAddtnl below, as future-looking placeholder
	//
	valuBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(valuBytes, id)
//	valuAddtnl := valuBytes[0]
	valuBlocks := valuBytes[1]
	dataBlocks.Data.([]byte)[indxBlock] = valuBlocks

	// more compactness at the price of simplicity :  Minecraft stores data that characterizes some
	// blocks in another array, again as one nybble per block; a full byte-array would be both simpler
	// and a bit more future proof, but iiwii
	//
	indxBlockData := int(indxBlock / 2)
	currDataValue := dataBlockData.Data.([]byte)[indxBlockData]
	var keepNybble, valuNybble byte
	if (indxBlock % 2) == 0 {
		keepNybble = currDataValue & 0xF0
		valuNybble = data
	} else {
		keepNybble = currDataValue & 0x0F
		valuNybble = data << 4
	}
	dataBlockData.Data.([]byte)[indxBlockData] = keepNybble + valuNybble

	// the HeightMap figures heavily into light-level calculations; we update it as we build, but we
	// skip 'air' blocks, so that light can shine down to the highest solid block
	//
	dataHeightMap := rgn.Chunks[indxChunk].ChunkDataRefs["HeightMap"]
	hx := x - (cx * 16)
	hz := z - (cz * 16)
	indxHeightMap := (hz * 16) + hx

	hy := dataHeightMap.Data.([]int32)[indxHeightMap]
	if id != 0 {
		if int32(y) > hy {
			dataHeightMap.Data.([]int32)[indxHeightMap] = int32(y)
		}
	}

	// set this to zero, to instruct Minecraft to recalculate lighting for this chunk
	//
	dataLightPopulated := rgn.Chunks[indxChunk].ChunkDataRefs["LightPopulated"]
	dataLightPopulated.Data = byte(0)

	qtyBlockEdits++

	return
}

func (w *MCWorld) EditEntity(x int, y int, z int, nbtentity *NBT) (err error) {

	// this flag causes all entity edits to be skipped; this is useful when redo'ing a blueprint after
	// fixing the blocks on the blueprint; blocks always replace themselves, but entities are always
	// new, so skipping entities can prevent ending up with too much livestock roaming around, superimposed
	// armorstands, and duplicate named entities such as dogs and cats, which are intended (by naming them)
	// to be unique
	//
	// blocks always replace themselves because the data structures holding blocks are of fixed size and
	// have positional implications, whereas entities are stored in an open-ended list of elements and have
	// their position encoded as explicit properties of those elements
	//
	if w.FlagSkipEntities {
		qtyEntityEditsSkipped++
		return
	}

	rgn, err := w.LoadRegion(x, y, z)
	panicOnErr(err)
	rx := rgn.RX
	rz := rgn.RZ

	// calculate the in-region chunk coordinates and chunkdata index
	cx := int(math.Floor(float64(x) / 16.0))
	cz := int(math.Floor(float64(z) / 16.0))
	indxChunk := ((cz - (rz * 32)) * 32) + (cx - (rx * 32))

	// fetch references to the data structures we need to update; return early if they do not exist
	dataEntities := rgn.Chunks[indxChunk].ChunkDataRefs["Entities"]

	if dataEntities == nil {
		qtyEntityEditsSkipped++
		return
	}

	// modify the entity to give it a position in the Minecraft world
	nbtentity.Data.([]NBT)[3].Data.([]NBT)[0].Data = float64(x)
	nbtentity.Data.([]NBT)[3].Data.([]NBT)[1].Data = float64(y)
	nbtentity.Data.([]NBT)[3].Data.([]NBT)[2].Data = float64(z)

	// ensure that it is marked as a LISTELEM
	nbtentity.Name = "LISTELEM"

	//debug
	//fmt.Printf("EditEntity : %v\n", nbtentity)

	// add the entity to the collection of entities in this chunk's chunkdata
	//
	// the first line setting the List value to TAG_Compound is really only necessary for the
	// 1st entity, since before that (when the 'Entities' list is empty), it is TAG_End, but
	// just setting it each time is less work (fewer opcodes) than testing the current value,
	// even though it seems pointless to us pesky humans in our concrete, analog existence
	//
	dataEntities.List = TAG_Compound
	dataEntities.Size++
	dataEntities.Data = append(dataEntities.Data.([]NBT), *nbtentity)

	qtyEntityEdits++

	return
}

func (w *MCWorld) EditBlockEntity(x int, y int, z int, nbtentity *NBT) (err error) {

	rgn, err := w.LoadRegion(x, y, z)
	panicOnErr(err)
	rx := rgn.RX
	rz := rgn.RZ

	// calculate the in-region chunk coordinates and chunkdata index
	cx := int(math.Floor(float64(x) / 16.0))
	cz := int(math.Floor(float64(z) / 16.0))
	indxChunk := ((cz - (rz * 32)) * 32) + (cx - (rx * 32))

	dataBlockEntities := rgn.Chunks[indxChunk].ChunkDataRefs["TileEntities"]

	if dataBlockEntities == nil {
		qtyBlockEntityEditsSkipped++
		return
	}

	// this flag resets the blockentities in the current chunk; this is useful when redo'ing a blueprint
	// after fixing the blocks on the blueprint; blocks always replace themselves, but blockentities are
	// always new, and this evidently causes a serious problem for Minecraft, as it typically crashes when
	// loading a region with duplicate blockentities
	//
	// we use a property on the chunk itself to avoid resetting the blockentities more than once (which
	// would of course lead to a blockentities list one item in length)
	//
	// presumably, the in-game-memory representation of, say, a chest's inventory cannot cope with more
	// than one item stack assigned to the same inventory slot, or something like that
	//
	// blocks always replace themselves because the data structures holding blocks are of fixed size and
	// have positional implications, whereas blockentities are stored in an open-ended list of elements
	// and have their position encoded as explicit properties of those elements
	//
	if w.FlagResetBlockEntities {
		if rgn.Chunks[indxChunk].ResetBENeeded {
			dataBlockEntities.Size = 0
			dataBlockEntities.Data = make([]NBT, 0)

			rgn.Chunks[indxChunk].ResetBENeeded = false
		}
	}

	// modify the blockentity to give it a position in the Minecraft world
	nbtentity.Data.([]NBT)[1].Data = int32(x)
	nbtentity.Data.([]NBT)[2].Data = int32(y)
	nbtentity.Data.([]NBT)[3].Data = int32(z)

	// ensure that it is marked as a LISTELEM
	nbtentity.Name = "LISTELEM"

	//debug
	//fmt.Printf("EditBlockEntity : %v\n", nbtentity)

	// add the blockentity to the collection of blockentities in this chunk's chunkdata
	//
	// the first line setting the List value to TAG_Compound is really only necessary for the
	// 1st entity, since before that (when the 'TileEntities' list is empty), it is TAG_End, but
	// just setting it each time is less work (fewer opcodes) than testing the current value,
	// even though it seems pointless to us pesky humans in our concrete, analog existence
	//
	dataBlockEntities.List = TAG_Compound
	dataBlockEntities.Size++
	dataBlockEntities.Data = append(dataBlockEntities.Data.([]NBT), *nbtentity)

	qtyBlockEntityEdits++

	return
}

func (w *MCWorld) LoadRegion(x int, y int, z int) (rgn *MCRegion, err error) {
	rgn = nil
	err = nil

	// blocks are identified in terms of world-coordinates, so we need to first translate from world-coordinates
	// to region-coordinates; if we've already loaded the indicated region, use it; if not, load it

	// calculate the region-x, region-z where the input world-x, world-z resides
	rx := int(math.Floor(float64(x) / 512.0))
	rz := int(math.Floor(float64(z) / 512.0))

	// check to see if the indicated region is already loaded
	for _, elem := range w.Regions {
		if (elem.RX == rx) && (elem.RZ == rz) {
			rgn = &elem
			return
		}
	}

	// the indicated region is not yet loaded, so load it into memory;  region files are stored in the 'world'
	// directory, whose path has been previously set, and are identified by their x, z coordinates;  construct
	// the fqfn and read the file in
	filename := fmt.Sprintf("%s/r.%d.%d.mca", w.PathWorld, rx, rz)
	fmt.Printf("LoadRegion filename is  : %s\n", filename)
	bufFile, err := ioutil.ReadFile(filename)
	if err != nil {
		fmt.Printf("unable to open region file [%s] [%s]\n", filename, err)
		os.Exit(3)
	}
	panicOnErr(err)

	// instantiate a new region object
	newrgn := MCRegion{RX: rx, RZ: rz}
	newrgn.Chunks = make([]MCChunk, 0)

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
		newchnk := MCChunk{IX: ix, IZ: iz, CX: cx, CZ: cz, ResetBENeeded: w.FlagResetBlockEntities}

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
						panic(fmt.Errorf("not ZLib compression!  chunk %d, %d\n", ix, iz))
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

					// a non-empty debug string is the signal to ReadNBTData to produce verbose output
					strDebug := ""
					if w.FlagDebug {
						strDebug = fmt.Sprintf("chunk %d, %d", ix, iz)
					}

					// parse the data out of Minecraft's NBT format into data structures we interact with
					var rdrTemp *bytes.Reader
					rdrTemp = bytes.NewReader(bufTemp.Bytes())
					newchnk.ChunkData, err = ReadNBTData(rdrTemp, TAG_NULL, strDebug)
					newchnk.BuildDataRefs()
				}
			}
		}

		// add the new chunk (whether we also populated it with data or not) into this region's list of chunks
		newrgn.Chunks = append(newrgn.Chunks, newchnk)
	}

	// add this region to the global list of regions
	w.Regions = append(w.Regions, newrgn)

	// find the newly-added region in our global array of regions, so that we can return a pointer to that instance
	// of the region, instead of the temporary object instantiated inside this function
	for _, elem := range w.Regions {
		if (elem.RX == rx) && (elem.RZ == rz) {
			rgn = &elem
			return
		}
	}

	// if we made it here, something went wrong, and we have no data to return
	return nil, nil
}

func (w *MCWorld) SaveAllEdits() (err error) {
	for _, elem := range w.Regions {
		w.SaveRegion(elem.RX, elem.RZ)
	}

	return
}

func (w *MCWorld) SaveRegion(rx, rz int) (err error) {
	var filename string
	var rgn MCRegion

	// open a new file for holding the edited region; we include a timestamp in the
	// filename to avoid overwriting the original file and any edited files already made
	//filename = fmt.Sprintf("r.%d.%d.mca.%d", rx, rz, timeExec.Unix())

        // open the original region file to overwrite it with our edits
	filename = fmt.Sprintf("%s/r.%d.%d.mca", w.PathWorld, rx, rz)
	fmt.Printf("SaveRegion filename is  : %s\n", filename)

	fh, err := os.Create(filename)
	panicOnErr(err)
	defer fh.Close()

	// regions might be loaded in any order; find the region we want by scanning the array of regions for an x, z match
	for _, elem := range w.Regions {
		if (elem.RX == rx) && (elem.RZ == rz) {
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
		if rgn.Chunks[indx].IX != indx % 32 {
			panic(fmt.Errorf("unexpected IX coordinate; region %d, %d;  indx %d;  chunk %d, %d\n", rx, rz, indx, rgn.Chunks[indx].IX, rgn.Chunks[indx].IZ))
		}
		if rgn.Chunks[indx].IZ != int(indx / 32) {
			panic(fmt.Errorf("unexpected IZ coordinate; region %d, %d;  indx %d;  chunk %d, %d\n", rx, rz, indx, rgn.Chunks[indx].IX, rgn.Chunks[indx].IZ))
		}

		// optionally output the chunkdata to JSON, for various sorts of external analysis
		if w.FlagJSOND == true {
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

type MCRegion struct {
	RX                 int
	RZ                 int
	ChunkDataLocations [1024]MCChunkdatalocation
	ChunkTimestamps    [1024]int32
	Chunks             []MCChunk
}

// MCChunkdatalocation
//
// a chunkdata descriptor indicates where within the region file the chuck data is found;  the offset is the (0-indexed)
// index of the first 4KB block holding the chunk data; the count is the number of 4KB blocks used for this chunk;  the offset
// does not ignore the two header blocks, so the lowest offset for chunk data will be "2", i.e. starting at byte 8192 in the
// region file (i.e., the 8,193rd byte, the start of the 3rd 4KB block)
//
type MCChunkdatalocation struct {
	Offset [3]byte
	Count  uint8
}

// these interface methods are principally for dealing with the odd choice to store the offset as a 24-bit number  (this
// choice is made even more odd by the fact that the 'count', taking up the 4th byte of what could be a conventional 32-bit
// number, is completely redundant, since the first piece of chunkdata is the length of the data)
//
func (cdl *MCChunkdatalocation) getOffsetValue() (rtrn int) {
	rtrn = (int(cdl.Offset[2]) << 0) | (int(cdl.Offset[1]) << 8) | (int(cdl.Offset[0]) << 16)

	return
}

func (cdl *MCChunkdatalocation) setOffset(value int) {
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

// MCChunk
//
type MCChunk struct {
	IX              int
	IZ              int
	CX              int
	CZ              int
	Length          uint32
	CompressionType byte
	ChunkData       NBT
	ChunkDataRefs   map[string]*NBT
	ResetBENeeded   bool
}

// this builds a map of data objects for this chunk's chunkdata;  the chunkdata is in an unordered hierarchy, making it
// cumbersome to go looking for a given data object every time, so we build up a mapping using a pathed name as the key
//
func (c *MCChunk) BuildDataRefs() {
	if c.ChunkDataRefs != nil {
		fmt.Printf("BuildDataRefs called again for the same chunk [%d, %d]\n", c.CX, c.CZ)
		os.Exit(5)
	}
	c.ChunkDataRefs = make(map[string]*NBT, 0)

	refLevl := c.ChunkData.Data.([]NBT)[0]

	c.ChunkDataRefs["Level"] = &refLevl

	for indxA, elemLevl := range refLevl.Data.([]NBT) {
		c.ChunkDataRefs[elemLevl.Name] = &refLevl.Data.([]NBT)[indxA]

		if elemLevl.Name == "Sections" {
			for indxB, arrySect := range elemLevl.Data.([]NBT) {
				for indxC, elemSect := range arrySect.Data.([]NBT) {
					fqsn := fmt.Sprintf("Sections/%d/%s", indxB, elemSect.Name)
					c.ChunkDataRefs[fqsn] = &refLevl.Data.([]NBT)[indxA].Data.([]NBT)[indxB].Data.([]NBT)[indxC]
				}
			}
		}
	}
}

// blueprint datatypes
//
// a glyph is a symbol on a blueprint, either a 1-character symbol denoting a block to be placed, or a 4-character abbreviated
// name denoting an item to be added to an inventory list, or the 4-character 'NTTY' denoting an entity to be further specified
// by a glyphtag
//
// a glyphtag is used to give a 1-character glyph more definition, by tying it to a set of other glyphs; current common cases:
//     -  define the items inside of a chest, then designate which chest on the blueprint goes with which defined chest
//        -  the same thing could be done for a furnace or a brewing stand, too
//     -  select a given variety of a given entity, then designate which entity on the blueprint goes with which variation
//        -  sheep of different colors, dogs and cats with specific names, and specific armor stands
//
// an atom is used to establish a hierarchy of entity composition, chiefly so that specific entities can be defined with a
// minimum of information; this avoids the pitfalls of denormalized data, and is just plain easier to do and maintain
//
// an atominfo is used to define specific elements of an entity built from atoms that should be modified to make the generic
// entity into a specifc entity; e.g., mob-specific values/settings of attributes/properties held in common among all entities
//
type Glyph struct {
	Glyph string `json:"glyph"`
	Type  string `json:"type"`
	Name  string `json:"name"`
	ID    uint16 `json:"id"`
	Data  uint8  `json:"data"`
	Base  NBT    `json:"base"`
}

type GlyphTag struct {
	Tag  string `json:"tag"`
	Indx uint8  `json:"indx"`
	Data NBT    `json:"data"`
}

type Atom struct {
	Name string     `json:"name"`
	Base string     `json:"base"`
	Data NBT        `json:"data"`
	Info []AtomInfo `json:"info"`
}

type AtomInfo struct {
	Attr string      `json:"attr"`
	Valu interface{} `json:"valu"`
}
