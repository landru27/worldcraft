package main

///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
//  import necessary external packages  ///////////////////////////////////////////////////////////////////////////////////////

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"flag"
	"io/ioutil"
	"nbt"
	"os"
	"crypto/rand"
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

type blueprintentity struct {
	Symbol string
	Base   nbt.NBT
}

type wcblock struct {
	X    int
	Y    int
	Z    int
	ID   uint16
	Data byte
}

type wcentity struct {
	X    int
	Y    int
	Z    int
	Attr nbt.NBT
}

type wctileentity struct {
	X    int
	Y    int
	Z    int
	ID   uint16
	Attr nbt.NBT
}

type wcedit struct {
	Type string
	Info interface{}
}

///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
// declare global variables
//
var timeExec time.Time

var BlockEdits []wcedit
var EntityEdits []wcedit

var blockValues map[string]blueprintblock
var stockEntities []blueprintentity

///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
// main execution point
//
func main() {
	var err error

	///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
	// define global variables
	timeExec = time.Now()
	BlockEdits = make([]wcedit, 0, 0)
	EntityEdits = make([]wcedit, 0, 0)

	defineGlobalVariables()

	///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
	// define the available command-line arguments
	anchorX := flag.Int("X", 0, "the westernmost  coordinate where the blueprint will be rendered in the gameworld")
	anchorY := flag.Int("Y", 0, "the lowest-layer coordinate where the blueprint will be rendered in the gameworld")
	anchorZ := flag.Int("Z", 0, "the northernmost coordinate where the blueprint will be rendered in the gameworld")
	flag.Parse()

	///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
	// read in the stock entities datafile
	var bufEntities []byte
	var jsonEntities map[string][]blueprintentity

	bufEntities, err = ioutil.ReadFile("blueprint.entities.json")
	panicOnErr(err)

	err = json.Unmarshal(bufEntities, &jsonEntities)
	panicOnErr(err)

	stockEntities = jsonEntities["Entities"]
	stockEntitiesIndx := make(map[string]uint8, 0)
	for indx, elem := range stockEntities {
		stockEntitiesIndx[elem.Symbol] = uint8(indx)
	}

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
		glyphs := strings.Fields(linein)
		for _, glyph := range glyphs {
			// track which world (block) coordinate we are dealing with, as we move
			// from symbol to symbol on the blueprint
			bx = ax + dx
			by = ay + dy
			bz = az + dz

			dx++

			// blocks in Minecraft can be modeled with very little data, but other objects
			// require much more data; we use block IDs outside of the valid range for
			// Minecraft blocks (0 - 4095) in order to mesh [a] the use of the blueprint to
			// indicate positioning with [b] the need to supply additional data

			// direct use of block IDs to place ordinary blocks
			if blockValues[glyph].ID < 4096 {
				BlockEdits = append(BlockEdits, wcedit{"block", wcblock{bx, by, bz, blockValues[glyph].ID, blockValues[glyph].Data}})
			}

			// skip placeholder markers
			if blockValues[glyph].ID == 8193 {
				continue
			}

			// read an entity from the (array built from the) file of entity definitions
			if blockValues[glyph].ID == 8201 {
				var indxEntity uint8
				var entityInfo nbt.NBT
				var entityCopy nbt.NBT

				indxEntity = stockEntitiesIndx[blockValues[glyph].Symbol]
				entityInfo = stockEntities[indxEntity].Base

				// marshal and unmarshal the entity information as a way to make a deep copy;
				// simple assignment assigns a pointer, and thus multiple identical entities
				var bufJSON []byte
				bufJSON, err = json.Marshal(entityInfo)
				panicOnErr(err)
				err = json.Unmarshal(bufJSON, &entityCopy)

				var uuid [16]byte
				_, err = rand.Read(uuid[:])
				panicOnErr(err)
				uuid[8] = (uuid[8] | 0x40) & 0x7F
				uuid[6] = (uuid[6] &  0xF) | (4 << 4)
				uuidmost := int64(binary.BigEndian.Uint64(uuid[0:8]))
				uuidlest := int64(binary.BigEndian.Uint64(uuid[8:16]))

				// modify the (copy of the) base entity to have its own UUID
				entityCopy.Data.([]nbt.NBT)[1].Data = uuidmost
				entityCopy.Data.([]nbt.NBT)[2].Data = uuidlest

				// modify the (copy of the) base entity to place it as indicated on the blueprint
				entityCopy.Data.([]nbt.NBT)[3].Data.([]nbt.NBT)[0].Data = bx
				entityCopy.Data.([]nbt.NBT)[3].Data.([]nbt.NBT)[1].Data = by
				entityCopy.Data.([]nbt.NBT)[3].Data.([]nbt.NBT)[2].Data = bz

				EntityEdits = append(EntityEdits, wcedit{"entity", wcentity{bx, by, bz, entityCopy}})

				continue
			}
		}
		dx = 0
		dz++
	}

	err = scanner.Err()
	panicOnErr(err)

	///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
	// write out the resulting edits to stdout

	allEdits := make(map[string][]wcedit)

	allEdits["edits"] = BlockEdits
	allEdits["edits"] = append(allEdits["edits"], EntityEdits...)

	var bufJSON []byte
	bufJSON, err = json.MarshalIndent(allEdits, "", "  ")
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
		`-`:  blueprintblock{`-`,  188,  0 },    // spruce fence
		`*`:  blueprintblock{`*`,    3,  0 },    // dirt
		`+`:  blueprintblock{`+`,   60,  0 },    // tilled dirt
		`~`:  blueprintblock{`~`,    9,  0 },    // water
		`"`:  blueprintblock{`"`,   12,  0 },    // sand
		`%`:  blueprintblock{`%`,   13,  0 },    // gravel
		`&`:  blueprintblock{`&`,   82,  0 },    // clay

		`,`:  blueprintblock{`/`,   59,  0 },    // wheat, just planted
		`/`:  blueprintblock{`/`,   59,  5 },    // wheat, ripe with seeds

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

		`D`:  blueprintblock{`D`,  193,  3 },    // spruce door, bottom      --  two-block object
		`d`:  blueprintblock{`d`,  193,  8 },    // spruce door, top         --  two-block object
		`B`:  blueprintblock{`B`,    0,  0 },    // bed                      --  two-block object
		`b`:  blueprintblock{`b`, 8193,  0 },    // head of bed placeholder
		`C`:  blueprintblock{`C`,    0,  0 },    // chest                    --  needs a tile entity for full definition
		`K`:  blueprintblock{`K`,    0,  0 },    // enchanting table         --  needs a tile entity for full definition

		`s`:  blueprintblock{`s`, 8201,  0 },    // sheep
		`c`:  blueprintblock{`c`, 8201,  0 },    // cow
		`k`:  blueprintblock{`k`, 8201,  0 },    // chicken
		`p`:  blueprintblock{`p`, 8201,  0 },    // pig
	}
}
