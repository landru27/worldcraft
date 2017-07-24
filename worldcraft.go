package main

///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
//  import necessary external packages  ///////////////////////////////////////////////////////////////////////////////////////

import (
	"bufio"
	//"bytes"
	//"compress/zlib"
	//"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	//"io"
	"io/ioutil"
	//"math"
	"os"
	"regexp"
	"strings"
	"time"
)

///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
// declare global variables
//
var timeExec time.Time

var flagDebug *bool
var flagJSOND *bool
var pathWorld *string
var fileBPrnt *string
var anchorX *int
var anchorY *int
var anchorZ *int

var world MCWorld

var glyphs []Glyph
var glyphTags []GlyphTag
var glyphIndx map[string]int
var glyphTagIndx map[string]int

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

	glyphs = make([]Glyph, 0)
	glyphTags = make([]GlyphTag, 0)
	glyphIndx = make(map[string]int, 0)
	glyphTagIndx = make(map[string]int, 0)

	qtyBlockEdits = 0
	qtyBlockEditsSkipped = 0
	qtyEntityEdits = 0
	qtyEntityEditsSkipped = 0
	qtyTileEntityEdits = 0
	qtyTileEntityEditsSkipped = 0

	///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
	// define the available command-line arguments
	flagDebug = flag.Bool("debug", false, "a flag to enable verbose output, for bug diagnosis and to validate detailed functionality")
	flagJSOND = flag.Bool("json", false, "a flag to enable dumping the chunkdata to JSON")
	pathWorld = flag.String("world", "UNDEFINED", "a directory containing a collection of Minecraft region files")
	fileBPrnt = flag.String("blueprint", "UNDEFINED", "a file containing a blueprint of edits to make to the specified Minecraft world")
	anchorX = flag.Int("X", 0, "the westernmost  coordinate where the blueprint will be rendered in the gameworld")
	anchorY = flag.Int("Y", 0, "the lowest-layer coordinate where the blueprint will be rendered in the gameworld")
	anchorZ = flag.Int("Z", 0, "the northernmost coordinate where the blueprint will be rendered in the gameworld")
	flag.Parse()

	// report to the user what values will be used
	fmt.Printf("output flags    : debug:%t  JSON:%t\n", *flagDebug, *flagJSOND)
	fmt.Printf("world directory : %s\n", *pathWorld)
	fmt.Printf("blueprint file  : %s\n", *fileBPrnt)
	fmt.Printf("build starts at : %d, %d, %d\n", *anchorX, *anchorY, *anchorZ)
	fmt.Printf("\n")

	world = MCWorld{FlagDebug: *flagDebug, FlagJSOND: *flagJSOND, PathWorld: *pathWorld}

	///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
	// read in the definitions of Glyphs, so that the associated blueprint symbols can be interpretted as the Minecraft
	// objects that they are intended to represent
	var bufJson []byte
	var mapJson map[string][]Glyph

	bufJson, err = ioutil.ReadFile("blueprint-glyphs.json")
	panicOnErr(err)

	err = json.Unmarshal(bufJson, &mapJson)
	panicOnErr(err)

	// assign the data to an array, and build some maps for refering into that array by symbol, and by name
	glyphs = mapJson["Glyphs"]
	for indx, elem := range glyphs {
		glyphIndx[elem.Glyph] = indx
		glyphIndx[elem.Name] = indx
	}

	//debug
	//fmt.Printf("glyphs array : %v\n", glyphs)
	//fmt.Printf("glyphIndx array : %v\n", glyphIndx)

	///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
	// read in the blueprint from stdin
	var linein string
	var re *regexp.Regexp

	var ax, ay, az int
	var dx, dy, dz int
	var bx, by, bz int

	// coordinates for where to start building
	ax = *anchorX
	ay = *anchorY
	az = *anchorZ

	// coordinates to track offsets as we traverse the blueprint
	dx = 0
	dy = 0
	dz = 0

	fh, err := os.Open(*fileBPrnt)
	if err != nil {
		fmt.Printf("unable to open blueprint file [%s] [%s]\n", *fileBPrnt, err)
		os.Exit(3)
	}
	defer fh.Close()

	scanner := bufio.NewScanner(fh)
	for scanner.Scan() {
		linein = scanner.Text()

		// strip off comments, marked by '##'
		re = regexp.MustCompile(`##.*$`)
		if re.FindStringIndex(linein) != nil {
			linein = re.ReplaceAllLiteralString(linein, ``)
		}

		// strip off leading and trailing whitespace
		re = regexp.MustCompile(`^\s+`)
		if re.FindStringIndex(linein) != nil {
			linein = re.ReplaceAllLiteralString(linein, ``)
		}
		re = regexp.MustCompile(`\s+$`)
		if re.FindStringIndex(linein) != nil {
			linein = re.ReplaceAllLiteralString(linein, ``)
		}

		// skip any blank lines
		re = regexp.MustCompile(`^\s*$`)
		if re.FindStringIndex(linein) != nil {
			continue
		}

		// respond to the end-of-layer marker; increment Y-offset and reset Z-offset
		re = regexp.MustCompile(`--`)
		if re.FindStringIndex(linein) != nil {
			dy++
			dz = 0
			continue
		}

		// == defines a glyph-tag

		// :: sets a glyph-tag for a glyph within the corresponding glyph line

		// glyph line : split the input line into its individual blueprint symbols
		gg := strings.Fields(linein)
		for _, g := range gg {
			// track which world (block) coordinate we are dealing with, as we move
			// from symbol to symbol on the blueprint
			bx = ax + dx
			by = ay + dy
			bz = az + dz

			dx++

			indx := glyphIndx[g]

			// glyphs that represent blocks
			if glyphs[indx].Type == "block" {
				world.EditBlock(bx, by, bz, glyphs[indx].ID, glyphs[indx].Data)

				continue
			}

			// glyphs that represent entities
			if glyphs[indx].Type == "entity" {
//				var indxEntity uint8
//				var entityInfo nbt.NBT
//				var entityCopy nbt.NBT
//
//				indxEntity = stockEntitiesIndx[blockValues[glyph].Symbol]
//				entityInfo = stockEntities[indxEntity].Base
//
//				// marshal and unmarshal the entity information as a way to make a deep copy;
//				// simple assignment assigns a pointer, and thus multiple identical entities
//				var bufJSON []byte
//				bufJSON, err = json.Marshal(entityInfo)
//				panicOnErr(err)
//				err = json.Unmarshal(bufJSON, &entityCopy)
//
//				var uuid [16]byte
//				_, err = rand.Read(uuid[:])
//				panicOnErr(err)
//				uuid[8] = (uuid[8] | 0x40) & 0x7F
//				uuid[6] = (uuid[6] &  0xF) | (4 << 4)
//				uuidmost := int64(binary.BigEndian.Uint64(uuid[0:8]))
//				uuidlest := int64(binary.BigEndian.Uint64(uuid[8:16]))
//
//				// modify the (copy of the) base entity to have its own UUID
//				entityCopy.Data.([]nbt.NBT)[1].Data = uuidmost
//				entityCopy.Data.([]nbt.NBT)[2].Data = uuidlest
//
//				// modify the (copy of the) base entity to place it as indicated on the blueprint
//				entityCopy.Data.([]nbt.NBT)[3].Data.([]nbt.NBT)[0].Data = bx
//				entityCopy.Data.([]nbt.NBT)[3].Data.([]nbt.NBT)[1].Data = by
//				entityCopy.Data.([]nbt.NBT)[3].Data.([]nbt.NBT)[2].Data = bz
//
//				EntityEdits = append(EntityEdits, wcedit{"entity", wcentity{bx, by, bz, entityCopy}})
//
//				continue
			}
		}
		dx = 0
		dz++
	}

	err = scanner.Err()
	panicOnErr(err)

	///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
	// save the net effect of all edits to new region file(s)
	world.SaveAllEdits()

	os.Exit(0)
}

///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
// utility functions
//
func panicOnErr(e error) {
	if e != nil {
		panic(e)
	}
}
