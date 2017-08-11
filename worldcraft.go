package main

///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
//  import necessary external packages  ///////////////////////////////////////////////////////////////////////////////////////

import (
	"bufio"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"strconv"
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
var qtyBlockEntityEdits int
var qtyBlockEntityEditsSkipped int

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
	qtyBlockEntityEdits = 0
	qtyBlockEntityEditsSkipped = 0

	///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
	// define the available command-line arguments
	flagDebug := flag.Bool("debug", false, "a flag to enable verbose output, for bug diagnosis and to validate detailed functionality")
	flagJSOND := flag.Bool("json", false, "a flag to enable dumping the chunkdata to JSON")
	pathWorld := flag.String("world", "UNDEFINED", "a directory containing a collection of Minecraft region files")
	fileBPrnt := flag.String("blueprint", "UNDEFINED", "a file containing a blueprint of edits to make to the specified Minecraft world")
	anchorX := flag.Int("X", 0, "the westernmost  coordinate where the blueprint will be rendered in the gameworld")
	anchorY := flag.Int("Y", 0, "the lowest-layer coordinate where the blueprint will be rendered in the gameworld")
	anchorZ := flag.Int("Z", 0, "the northernmost coordinate where the blueprint will be rendered in the gameworld")
	flagXAirBlocks := flag.Bool("xairblocks", false, "a flag to treat 'air' blocks as 'X' glyphs, skipping over them")
	flagSkipEntities := flag.Bool("skipentities", false, "a flag to suppress the inclusion of entities shown on a blueprint")
	flagResetBlockEntities := flag.Bool("resetblockentities", false, "a flag to reset each affected chunk's blockentities prior to adding any from the blueprint")
	flag.Parse()

	// report to the user what values will be used
	fmt.Printf("output flags    : debug:%t  JSON:%t\n", *flagDebug, *flagJSOND)
	fmt.Printf("action flags    : XAirBlocks:%t  SkipEntities:%t  ResetBlockEntities:%t\n", *flagXAirBlocks, *flagSkipEntities, *flagResetBlockEntities)
	fmt.Printf("world directory : %s\n", *pathWorld)
	fmt.Printf("blueprint file  : %s\n", *fileBPrnt)
	fmt.Printf("build starts at : %d, %d, %d\n", *anchorX, *anchorY, *anchorZ)
	fmt.Printf("\n")

	// the world object is at the root of the Minecraft data, and so is our interface to that data
	world = MCWorld{FlagDebug: *flagDebug, FlagJSOND: *flagJSOND, FlagXAirBlocks: *flagXAirBlocks, FlagSkipEntities: *flagSkipEntities, FlagResetBlockEntities: *flagResetBlockEntities, PathWorld: *pathWorld}

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

	///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
	// read in the definitions of Atoms, so that the associated Minecraft objects can be composed as needed
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

	///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
	// workhorse variables
	var linein string
	var lineglyphtags []string
	var match bool
	var matches []string

	var ax, ay, az int
	var dx, dy, dz int
	var bx, by, bz int
	var mx, my, mz int

	// coordinates for where to start building
	ax = *anchorX
	ay = *anchorY
	az = *anchorZ

	// coordinates to track offsets as we traverse the blueprint
	dx = 0
	dy = 0
	dz = 0

	///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
	// read in the blueprint from stdin
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
		_, linein = regexpReplace(linein, `##.*$`, ``)

		// strip off leading and trailing whitespace
		_, linein = regexpReplace(linein, `^\s+`, ``)
		_, linein = regexpReplace(linein, `\s+$`, ``)

		// skip any blank lines
		if match = regexpMatch(linein, `^\s*$`); match { continue }

		// respond to the end-of-layer marker; increment Y-offset and reset Z-offset
		if match = regexpMatch(linein, `^ *--`); match {
			dy++
			dz = 0
			continue
		}

		// == defines a glyph-tag
		if match, matches = regexpParse(linein, `^ *== +([a-z]+) +:((?: +[-A-Za-z]{1,4}:[-_a-z0-9]+){1,9})`); match {
			var tagname string
			var eleminfo string
			var elemname string
			var elemdata string
			var indx int

			tagname = matches[1]
			eleminfo = matches[2]

			// if we have not seen this glyphtag before, define it as a glyphtag and in the glyphtag map
			if _, okay := glyphTagIndx[tagname]; !okay {
				gt := GlyphTag{tagname, 0, NBT{TAG_List, TAG_Compound, "Items", 0, make([]NBT, 0)}}
				glyphTags = append(glyphTags, gt)
				glyphTagIndx[tagname] = len(glyphTags) - 1

			}
			indx = glyphTagIndx[tagname]

			// digest the elements which make up the definition of this glyphtag
			for {
				// we are done when we've run out of elements
				if match = regexpMatch(eleminfo, `^\s*$`); match { break }

				_, matches = regexpParse(eleminfo, `^ +([-A-Za-z]{1,4}):([-_a-z0-9]+)`)
				elemname = matches[1]
				elemdata = matches[2]

				// ----:-- is a placeholder, mainly used for empty slots in an item inventory list
				if elemname == `----` && elemdata == `--` {
					glyphTags[indx].Indx++
				}

				if glyphs[glyphIndx[elemname]].Type == "item" {
					var nbtI NBT
					var nbtG NBT


					// if the glyph refers to an item with pre-defined NBT, use that and set its slot
					// to be the current spot in the inventory list; otherwise construct the item NBT
					// from the glyph definition
					//
					if glyphs[glyphIndx[elemname]].Base != (NBT{}) {
						nbtP, _ := glyphs[glyphIndx[elemname]].Base.DeepCopy()

						nbtI = *nbtP
						nbtI.Data.([]NBT)[1].Data = byte(glyphTags[indx].Indx)
					} else {
						item := glyphs[glyphIndx[elemname]]

						slot := glyphTags[indx].Indx
						idstr := "minecraft:" + item.Name
						lenstr := uint32(len(idstr))

						qty, _ := strconv.Atoi(elemdata)

						nbtA := NBT{TAG_String, 0, "id", lenstr, idstr}
						nbtB := NBT{TAG_Byte, 0, "Slot", 0, byte(slot)}
						nbtC := NBT{TAG_Byte, 0, "Count", 0, byte(qty)}
						nbtD := NBT{TAG_Short, 0, "Damage", 0, int16(item.Data)}

						nbtI = NBT{TAG_Compound, 0, "LISTELEM", 4, []NBT{nbtA, nbtB, nbtC, nbtD}}
					}

					// add the item to the glyphtag definition
					nbtG = glyphTags[indx].Data
					tmps := nbtG.Data.([]NBT)
					tmps = append(tmps, nbtI)
					nbtG.Data = tmps
					nbtG.Size++

					glyphTags[indx].Data = nbtG

					glyphTags[indx].Indx++
				}

				if glyphs[glyphIndx[elemname]].Type == "entity" {

					nbtentity := buildEntity(elemdata)

					glyphTags = append(glyphTags, GlyphTag{tagname, 0, *nbtentity})
					glyphTagIndx[tagname] = len(glyphTags) - 1
				}

				// consume the element we just processed
				_, eleminfo = regexpReplace(eleminfo, `^ +[-A-Za-z]{1,4}:[-_a-z0-9]+`, ``)
			}

			continue
		}

		// :: sets a glyph-tag for a glyph within the corresponding glyph line
		if match, matches = regexpParse(linein, ` *:: +(.+)$`); match {
			lineglyphtags = strings.Fields(matches[1])
			_, linein = regexpReplace(linein, ` *:: +.+$`, ``)
		}

		// glyph line : split the input line into its individual blueprint symbols
		gg := strings.Fields(linein)
		gi := 0
		for _, g := range gg {
			// track which world (block) coordinate we are dealing with, as we move
			// from symbol to symbol on the blueprint
			bx = ax + dx
			by = ay + dy
			bz = az + dz

			mx = bx
			my = by
			mz = bz

			dx++

			indx := glyphIndx[g]

			var databyte byte
			var nbtentity *NBT

			// glyphs that represent blocks
			if glyphs[indx].Type == "block" {

				// this leaves whatever block is already at this spot in the Minecraft world intact
				if glyphs[indx].Name == "null" { continue }

				databyte = glyphs[indx].Data

				// if the glyph refers to a block with pre-defined NBT, use that to also add a
				// BlockEntity to go with this Block
				//
				if glyphs[indx].Base != (NBT{}) {
					nbtentity, _ = glyphs[indx].Base.DeepCopy()

					// if the block's NBT has an inventory list, look for a glyphtag and use the
					// NBT from that glyphtag to fill out this block's blockentity's inventory
					//
					if len(nbtentity.Data.([]NBT)) > 4 {
						if nbtentity.Data.([]NBT)[4].Name == "Items" {
							if gi < len(lineglyphtags) {
								lineglyphtag := lineglyphtags[gi]

								// if the glyphtag also has a number suffix, use that number
								// to set the block's data; e.g., the direction a chest faces
								//
								if match, matches = regexpParse(lineglyphtag, `^([a-z]+):([0-9]+)$`); match {
									lineglyphtag = matches[1]
									i, _ := strconv.ParseUint(matches[2], 10, 8)
									databyte = byte(i)
								}

								nbtentity.Data.([]NBT)[4] = glyphTags[glyphTagIndx[lineglyphtag]].Data
								gi++
							}
						}
					}

					world.EditBlockEntity(bx, by, bz, nbtentity)
				}

				world.EditBlock(bx, by, bz, glyphs[indx].ID, databyte)

				continue
			}

			// glyphs that represent entities
			if glyphs[indx].Type == "entity" {

				// if the glyph is 'E' or 'I', there must be a corrsponding glyphtag; the expectation
				// is that this glyphtag refers to an entity (built from atoms), and we want to use
				// that for the NBT for this entity; otherwise, the glyph bears a name that can be used
				// to build an entity from atoms
				//
				if glyphs[indx].Glyph == "E" || glyphs[indx].Glyph == "I" {
					if gi >= len(lineglyphtags) {
						fmt.Printf("more glyphs requiring glyph-tags than glyph-tags listed [%s]\n", linein)
						os.Exit(7)
					}

					nbtentity, _ = glyphTags[glyphTagIndx[lineglyphtags[gi]]].Data.DeepCopy()
					assignEntityUUID(nbtentity)

					gi++
				} else {
					nbtentity = buildEntity(glyphs[indx].Name)
				}

				world.EditEntity(bx, by, bz, nbtentity)

				// also make this block an air block, otherwise, if the chunkdata already had a block
				// in this spot, it will remain; worse, it will potentially suffocate the new entity
				world.EditBlock(bx, by, bz, 0, 0)

				continue
			}
		}
		dx = 0
		dz++

		lineglyphtags = nil
	}

	err = scanner.Err()
	panicOnErr(err)

	for indxz := 0; indxz < mz; indxz++ {
		for indxx := 0; indxx < mx; indxx++ {
			world.FixHeightMaps(indxx, my, indxz)
		}
	}

	///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
	// save the net effect of all edits to new region file(s)
	world.SaveAllEdits()

	os.Exit(0)
}

///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
// data handling functions
//
func buildEntity(top string) (rslt *NBT) {
	var stack []string
	var next string
	var base string

	// the input parameter names an atom at the top of an atom hierarchy; descend that hierarchy to find the
	// constituent atoms that make up the entity we are building;  then run through that stack, appending
	// NBT elements as we go;  at each point, also look for specific properties to set in order to make a
	// specific entity out of a generic one

	// (clever, eh?)
	molecule := NBT{TAG_Compound, 0, "", 0, make([]NBT, 0)}

	stack = make([]string, 0)
	stack = append(stack, top)
	next = top
	for {
		indx := entityAtomIndx[next]
		base = entityAtoms[indx].Base

		if (base == "") { break }

		stack = append(stack, base)
		next = base
	}

	var nbts []NBT
	var nbtc *NBT

	nbts = make([]NBT, 0)

	for {
		if len(stack) == 0 { break }

		next, stack = stack[len(stack)-1], stack[:len(stack)-1]

		indx := entityAtomIndx[next]
		if entityAtoms[indx].Data.Type != TAG_End {

			for _, elem := range entityAtoms[indx].Data.Data.([]NBT) {
				nbtc, _ = elem.DeepCopy()
				nbts = append(nbts, *nbtc)

				molecule.Size++
			}
			molecule.Data = nbts
		}

		if entityAtoms[indx].Info != nil {

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
					molecule.Data.([]NBT)[0].Size = uint32(len(valu.(string)))
					molecule.Data.([]NBT)[0].Data = valu.(string)

				case "Health":
					molecule.Data.([]NBT)[14].Data = float32(valu.(float64))

				case "MaxHealth":
					molecule.Data.([]NBT)[20].Data.([]NBT)[0].Data.([]NBT)[0].Data = valu.(float64)

				case "MoveSpeed":
					molecule.Data.([]NBT)[20].Data.([]NBT)[1].Data.([]NBT)[0].Data = valu.(float64)

				case "SheepColor":
					molecule.Data.([]NBT)[32].Data = byte(valu.(float64))

				case "CatType":
					molecule.Data.([]NBT)[34].Data = int32(valu.(float64))

				case "CollarColor":
					molecule.Data.([]NBT)[34].Data = byte(valu.(float64))

				case "CustomName":
					customname := NBT{TAG_String, 0, "CustomName", uint32(len(valu.(string))), valu.(string)}
					tmps := molecule.Data.([]NBT)
					tmps = append(tmps, customname)
					molecule.Data = tmps

				default:
					fmt.Printf("buildEntity : unsupported AtomInfo.Attr : %s\n", attr)
					os.Exit(7)
				}
			}
		}
	}

	assignEntityUUID(&molecule)

	rslt = &molecule
	return
}

func assignEntityUUID(dst *NBT) {
	// calculate a v4 UUID
	var uuid [16]byte
	_, err := rand.Read(uuid[:])
	panicOnErr(err)
	uuid[8] = (uuid[8] | 0x40) & 0x7F
	uuid[6] = (uuid[6] &  0xF) | (4 << 4)
	uuidmost := int64(binary.BigEndian.Uint64(uuid[0:8]))
	uuidlest := int64(binary.BigEndian.Uint64(uuid[8:16]))

	// modify the entity to have its own UUID;  we know the array indexes, because we constructed the entity
	dst.Data.([]NBT)[1].Data = uuidmost
	dst.Data.([]NBT)[2].Data = uuidlest
}

///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
// utility functions

func panicOnErr(e error) {
	if e != nil {
		panic(e)
	}
}

// these regexp functions are intended to make the most common regular-expression use cases easier to write, read, and maintain
//
func regexpMatch(subject string, pattern string) (rtrn bool) {
	rtrn = false

	var re *regexp.Regexp

	re = regexp.MustCompile(pattern)
	if re.FindStringIndex(subject) != nil {
		rtrn = true
	}

	return
}

func regexpReplace(subject string, pattern string, replace string) (rtrn bool, rslt string) {
	rtrn = false
	rslt = subject

	var re *regexp.Regexp

	re = regexp.MustCompile(pattern)
	if re.FindStringIndex(subject) != nil {
		rtrn = true
		rslt = re.ReplaceAllLiteralString(subject, replace)
	}

	return
}

func regexpParse(subject string, pattern string) (rtrn bool, rslt []string) {
	rtrn = false
	rslt = nil

	var re *regexp.Regexp

	re = regexp.MustCompile(pattern)
	if re.FindStringIndex(subject) != nil {
		rtrn = true
		rslt = re.FindStringSubmatch(subject)
	}

	return
}
