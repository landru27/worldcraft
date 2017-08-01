package main

///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
//  import necessary external packages  ///////////////////////////////////////////////////////////////////////////////////////

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"crypto/rand"
	"regexp"
	"strings"
	"time"
)

///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
// declare global variables
//
var timeExec time.Time

var world MCWorld

var glyphs []Glyph
var glyphTags []GlyphTag
var glyphIndx map[string]int
var glyphTagIndx map[string]int

var entityAtoms []Atom
var entityAtomIndx map[string]int

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

	entityAtoms = make([]Atom, 0)
	entityAtomIndx = make(map[string]int, 0)

	qtyBlockEdits = 0
	qtyBlockEditsSkipped = 0
	qtyEntityEdits = 0
	qtyEntityEditsSkipped = 0
	qtyTileEntityEdits = 0
	qtyTileEntityEditsSkipped = 0

	///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
	// define the available command-line arguments
	flagDebug := flag.Bool("debug", false, "a flag to enable verbose output, for bug diagnosis and to validate detailed functionality")
	flagJSOND := flag.Bool("json", false, "a flag to enable dumping the chunkdata to JSON")
	pathWorld := flag.String("world", "UNDEFINED", "a directory containing a collection of Minecraft region files")
	fileBPrnt := flag.String("blueprint", "UNDEFINED", "a file containing a blueprint of edits to make to the specified Minecraft world")
	anchorX := flag.Int("X", 0, "the westernmost  coordinate where the blueprint will be rendered in the gameworld")
	anchorY := flag.Int("Y", 0, "the lowest-layer coordinate where the blueprint will be rendered in the gameworld")
	anchorZ := flag.Int("Z", 0, "the northernmost coordinate where the blueprint will be rendered in the gameworld")
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
	// read in the definitions of Atoms, so that the associated blueprint symbols can be interpretted as the Minecraft
	// objects that they are intended to represent
	var bufJsonA []byte
	var mapJsonA map[string][]Atom

	bufJsonA, err = ioutil.ReadFile("blueprint-entities.json")
	panicOnErr(err)

	err = json.Unmarshal(bufJsonA, &mapJsonA)
	panicOnErr(err)

	// assign the data to an array, and build some maps for refering into that array by name
	entityAtoms = mapJsonA["EntityAtoms"]
	for indx, elem := range entityAtoms {
		entityAtomIndx[elem.Name] = indx
	}

	//debug
	//fmt.Printf("entityAtoms array : %v\n", entityAtoms)
	//fmt.Printf("entityAtomIndx array : %v\n", entityAtomIndx)


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
				entitymolecule := NBT{TAG_Compound, 0, "", 0, make([]NBT, 0)}
				buildEntity(bx, by, bz, "name-from-blueprint", &entitymolecule)

//				world.EditEntity(entitymolecule)

				continue
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
// data handling functions
//
func buildEntity(x int, y int, z int, top string, rslt *NBT) {
	var stack []string
	var next string
	var base string

	stack = make([]string, 0)
	stack = append(stack, top)

	next = top
	for {
		indx := entityAtomIndx[next]
		base = entityAtoms[indx].Base
		//fmt.Printf("buildEntity : trace : next, base : %s, %s\n", next, base)

		if (base == "") { break }

		stack = append(stack, base)
		next = base
	}

	for {
		if len(stack) == 0 { break }

		next, stack = stack[len(stack)-1], stack[:len(stack)-1]
		//fmt.Printf("buildEntity : add Data and Info : next : %s\n", next)

		indx := entityAtomIndx[next]
		if entityAtoms[indx].Data.Type != TAG_End {
			//fmt.Printf("buildEntity : add Data : next : %s\n", next)

			nbt := entityAtoms[indx].Data
			arr := nbt.Data.([]NBT)
			for _, elem := range arr {
				tmparr := rslt.Data.([]NBT)
				tmparr = append(tmparr, elem)
				rslt.Data = tmparr

				rslt.Size++
			}
		}

		if entityAtoms[indx].Info != nil {
			//fmt.Printf("buildEntity : add Info : next : %s\n", next)

			for _, atom := range entityAtoms[indx].Info {
				attr := atom.Attr
				valu := atom.Valu

				// the definition of a particular entity in Minecraft is all over the place;
				// such data is even stored at various levels in the hierarchy, under at
				// least three tagging schemes;  so we use local keywords to translate to
				// key places where we might want to tweak the definition for particular
				// entities being generated;  the mapping of those keywords happens here;
				//
				// we can rely on the array index values, because we constructed the entity
				// data ourselves; after being in-game and saved back to file by Minecraft,
				// there is no guarantee about the order of any entity data; thus, entity
				// -editing- functionality would need to search for the places to make the
				// edits; but here we are -generating- entities
				//
				switch attr {
				case "MCName":
					rslt.Data.([]NBT)[0].Size = uint32(len(valu.(string)))
					rslt.Data.([]NBT)[0].Data = valu.(string)

				case "MaxHealth":
					rslt.Data.([]NBT)[28].Data.([]NBT)[0].Data.([]NBT)[0].Data = valu.(float64)

				case "MoveSpeed":
					rslt.Data.([]NBT)[28].Data.([]NBT)[1].Data.([]NBT)[0].Data = valu.(float64)

				case "SheepColor":
					rslt.Data.([]NBT)[32].Data = byte(valu.(float64))

				case "CatType":
					rslt.Data.([]NBT)[34].Data = int32(valu.(float64))

				case "CollarColor":
					rslt.Data.([]NBT)[34].Data = byte(valu.(float64))

				case "CustomName":
					customname := NBT{TAG_String, 0, "CustomName", uint32(len(valu.(string))), valu.(string)}
					tmparr := rslt.Data.([]NBT)
					tmparr = append(tmparr, customname)
					rslt.Data = tmparr

				default:
					fmt.Printf("buildEntity : unsupported AtomInfo.Attr : %s\n", attr)
					os.Exit(7)
				}
			}
		}
	}

	// calculate a v4 UUID
	var uuid [16]byte
	_, err := rand.Read(uuid[:])
	panicOnErr(err)
	uuid[8] = (uuid[8] | 0x40) & 0x7F
	uuid[6] = (uuid[6] &  0xF) | (4 << 4)
	uuidmost := int64(binary.BigEndian.Uint64(uuid[0:8]))
	uuidlest := int64(binary.BigEndian.Uint64(uuid[8:16]))

	// modify the entity to have its own UUID
	rslt.Data.([]NBT)[1].Data = uuidmost
	rslt.Data.([]NBT)[2].Data = uuidlest

	// modify the entity to place it as indicated on the blueprint
	rslt.Data.([]NBT)[3].Data.([]NBT)[0].Data = x
	rslt.Data.([]NBT)[3].Data.([]NBT)[1].Data = y
	rslt.Data.([]NBT)[3].Data.([]NBT)[2].Data = z
}


///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
// utility functions
//
func panicOnErr(e error) {
	if e != nil {
		panic(e)
	}
}
