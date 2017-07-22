package main

///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
//  import necessary external packages  ///////////////////////////////////////////////////////////////////////////////////////

import (
	//"bytes"
	//"compress/zlib"
	//"encoding/binary"
	//"encoding/json"
	"flag"
	"fmt"
	//"io"
	//"io/ioutil"
	//"math"
	"os"
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
var glyphIndexs map[string]int
var glyphTagIndexs map[string]int

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

	panicOnErr(err)

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
