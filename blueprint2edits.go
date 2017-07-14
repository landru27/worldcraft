package main

///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
//  import necessary external packages  ///////////////////////////////////////////////////////////////////////////////////////

import (
	"bufio"
	"encoding/json"
	"flag"
	"os"
	"regexp"
	"strings"
	"time"
)

///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
//  declare our internal datatypes and their interfaces  //////////////////////////////////////////////////////////////////////

// Worldcrft datatypes
//

// these datatypes model editing instructions
//
type blueprintblock struct {
	Symbol string
	ID     uint16
	Data   byte
}

type wcblock struct {
	X    int
	Y    int
	Z    int
	ID   uint16
	Data byte
}

type wcedit struct {
	Type string
	Info wcblock
}

///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
// declare global variables
//
var timeExec time.Time

var BlockEdits []wcedit

var blockValues map[string]blueprintblock

///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
// main execution point
//
func main() {
	var err error

	///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
	// define global variables
	timeExec = time.Now()
	BlockEdits = make([]wcedit, 0, 0)

	defineGlobalVariables()

	///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
	// define the available command-line arguments
	anchorX := flag.Int("X", 0, "the westernmost  coordinate where the blueprint will be rendered in the gameworld")
	anchorY := flag.Int("Y", 0, "the lowest-layer coordinate where the blueprint will be rendered in the gameworld")
	anchorZ := flag.Int("Z", 0, "the northernmost coordinate where the blueprint will be rendered in the gameworld")
	flag.Parse()

	///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
	// read in the blueprint from stdin
	var linein string
	var re *regexp.Regexp

	var ax, ay, az int
	var dx, dy, dz int
	var bx, by, bz int

	ax = *anchorX
	ay = *anchorY
	az = *anchorZ

	dx = 0
	dy = 0
	dz = 0

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		linein = scanner.Text()

		// strip off comments, marked by '##', and any preceeding whitespace characters
		re = regexp.MustCompile(`\s*##.*$`)
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

		// split the input line into its individual blueprint symbols
		blocks := strings.Fields(linein)
		for _, block := range blocks {
			bx = ax + dx
			by = ay + dy
			bz = az + dz

			dx++

			// skip placeholder markers
			if blockValues[block].ID == 8193 {
				continue
			}

			BlockEdits = append(BlockEdits, wcedit{"block", wcblock{bx, by, bz, blockValues[block].ID, blockValues[block].Data}})
		}
		dx = 0
		dz++
	}

	err = scanner.Err()
	panicOnErr(err)

	///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
	// write out the resulting edits to stdout

	allBlockEdits := make(map[string][]wcedit)
	allBlockEdits["edits"] = BlockEdits

	var bufJSON []byte
	bufJSON, err = json.Marshal(allBlockEdits)
	os.Stdout.Write(bufJSON)

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

func defineGlobalVariables() {
	blockValues = map[string]blueprintblock{
		`.`:  blueprintblock{`.`,    0,  0 },    // air
		`#`:  blueprintblock{`#`,   98,  0 },    // stone bricks
		`=`:  blueprintblock{`=`,  139,  0 },    // stone fence
		`*`:  blueprintblock{`*`,    3,  0 },    // dirt
		`+`:  blueprintblock{`+`,   60,  0 },    // tilled dirt
		`~`:  blueprintblock{`~`,    9,  0 },    // water
		`"`:  blueprintblock{`"`,   12,  0 },    // sand
		`%`:  blueprintblock{`%`,   13,  0 },    // gravel
		`&`:  blueprintblock{`&`,   82,  0 },    // clay

		`1`:  blueprintblock{`1`,  109,  2 },    // stone brick steps, with steps facing (approachable from) north
		`2`:  blueprintblock{`2`,  109,  1 },    // stone brick steps, with steps facing (approachable from) east
		`3`:  blueprintblock{`3`,  109,  3 },    // stone brick steps, with steps facing (approachable from) south
		`4`:  blueprintblock{`4`,  109,  0 },    // stone brick steps, with steps facing (approachable from) west

		`5`:  blueprintblock{`5`,   67,  2 },    // cobblestone steps, with steps facing (approachable from) north
		`6`:  blueprintblock{`6`,   67,  1 },    // cobblestone steps, with steps facing (approachable from) east
		`7`:  blueprintblock{`7`,   67,  3 },    // cobblestone steps, with steps facing (approachable from) south
		`8`:  blueprintblock{`8`,   67,  0 },    // cobblestone steps, with steps facing (approachable from) west

		`t`:  blueprintblock{`t`,   50,  3 },    // torch attached on block to the north
		`u`:  blueprintblock{`u`,   50,  2 },    // torch attached on block to the east
		`v`:  blueprintblock{`v`,   50,  4 },    // torch attached on block to the south
		`w`:  blueprintblock{`w`,   50,  1 },    // torch attached on block to the west
		`x`:  blueprintblock{`x`,   50,  5 },    // torch attached on block below

		`S`:  blueprintblock{`S`,    1,  0 },    // stone
		`A`:  blueprintblock{`A`,    1,  5 },    // andesite
		`a`:  blueprintblock{`a`,    1,  6 },    // polished andesite
		`R`:  blueprintblock{`R`,    1,  3 },    // diorite
		`r`:  blueprintblock{`r`,    1,  4 },    // polished diorite
		`G`:  blueprintblock{`G`,    1,  1 },    // granite
		`g`:  blueprintblock{`g`,    1,  2 },    // polished granite
		`O`:  blueprintblock{`O`,   49,  0 },    // obsidian

		`T`:  blueprintblock{`T`,   58,  0 },    // crafting table
		`F`:  blueprintblock{`F`,    0,  0 },    // furnace                  --  needs a tile entity for full definition
		`V`:  blueprintblock{`V`,  145,  0 },    // anvil
		`L`:  blueprintblock{`L`,   47,  0 },    // bookcase
		`P`:  blueprintblock{`P`,    0,  0 },    // potion brewing stand     --  needs a tile entity for full definition
		`W`:  blueprintblock{`W`,  118,  3 },    // cauldron for water for potions

		`Y`:  blueprintblock{`Y`,   89,  0 },    // glowstone

		`D`:  blueprintblock{`D`,    0,  0 },    // spruce door              --  two-block object
		`B`:  blueprintblock{`B`,    0,  0 },    // bed                      --  two-block object
		`b`:  blueprintblock{`b`, 8193,  0 },    // head of bed placeholder
		`C`:  blueprintblock{`C`,    0,  0 },    // chest                    --  needs a tile entity for full definition
		`K`:  blueprintblock{`K`,    0,  0 },    // enchanting table         --  needs a tile entity for full definition
	}
}
