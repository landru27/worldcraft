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
	var lineglyphtags []string
	var match bool
	var matches []string

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
		if match, matches = regexpParse(linein, `^ *== +([a-z]+) +:((?: +[-A-Za-z]{1,4}:[-a-z0-9]+){1,9})`); match {
			var tagname string
			var eleminfo string
			var elemname string
			var elemdata string

			tagname = matches[1]
			eleminfo = matches[2]

			for {
				if match = regexpMatch(eleminfo, `^\s*$`); match { break }

				_, matches = regexpParse(eleminfo, `^ +([-A-Za-z]{1,4}):([-a-z0-9]+)`)
				elemname = matches[1]
				elemdata = matches[2]

				if elemname == `----` && elemdata == `--` {
					glyphTags[glyphTagIndx[tagname]].Indx++
				}

				if glyphs[glyphIndx[elemname]].Type == "item" {
					var indx int
					var nbtI NBT
					var nbtG NBT

					item := glyphs[glyphIndx[elemname]]

					if glyphs[glyphIndx[elemname]].Base != (NBT{}) {
						indx = glyphTagIndx[tagname]

						nbtP, _ := glyphs[glyphIndx[elemname]].Base.DeepCopy()

						nbtI = *nbtP
						nbtI.Data.([]NBT)[1].Data = byte(glyphTags[indx].Indx)

						nbtG = glyphTags[indx].Data
					} else {

						if _, okay := glyphTagIndx[tagname]; okay {
							indx = glyphTagIndx[tagname]

							nbtG = glyphTags[indx].Data
						} else {
							nbtG = NBT{TAG_List, TAG_Compound, "Items", 0, make([]NBT, 0)}

							gt := GlyphTag{tagname, 0, nbtG}
							glyphTags = append(glyphTags, gt)
							glyphTagIndx[tagname] = len(glyphTags) - 1
							indx = glyphTagIndx[tagname]
						}

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

				_, eleminfo = regexpReplace(eleminfo, `^ +[-A-Za-z]{1,4}:[-a-z0-9]+`, ``)
			}
			//fmt.Printf("... %v\n", glyphTags)

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

			dx++

			indx := glyphIndx[g]

			var databyte byte
			var nbtentity *NBT

			// glyphs that represent blocks
			if glyphs[indx].Type == "block" {

				if glyphs[indx].Name == "none" { continue }

				databyte = glyphs[indx].Data

				if glyphs[indx].Base != (NBT{}) {
					nbtentity, _ = glyphs[indx].Base.DeepCopy()

					if len(nbtentity.Data.([]NBT)) > 4 {
						if nbtentity.Data.([]NBT)[4].Name == "Items" {
							if gi < len(lineglyphtags) {
								lineglyphtag := lineglyphtags[gi]
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

				if glyphs[indx].Glyph == "E" {
					if gi >= len(lineglyphtags) {
						fmt.Printf("more glyphs requiring glyph-tags than glyph-tags listed [%s]\n", linein)
						os.Exit(7)
					}

					nbtentity = &glyphTags[glyphTagIndx[lineglyphtags[gi]]].Data
					gi++
				} else {
					nbtentity = buildEntity(glyphs[indx].Name)
				}

				world.EditEntity(bx, by, bz, nbtentity)

				continue
			}
		}
		dx = 0
		dz++

		lineglyphtags = nil
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
func buildEntity(top string) (rslt *NBT) {
	molecule := NBT{TAG_Compound, 0, "", 0, make([]NBT, 0)}

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

	var nbts []NBT
	var nbtc *NBT

	nbts = make([]NBT, 0)

	for {
		if len(stack) == 0 { break }

		next, stack = stack[len(stack)-1], stack[:len(stack)-1]
		//fmt.Printf("buildEntity : add Data and Info : next : %s\n", next)

		indx := entityAtomIndx[next]
		if entityAtoms[indx].Data.Type != TAG_End {
			//fmt.Printf("buildEntity : add Data : next : %s\n", next)

			for _, elem := range entityAtoms[indx].Data.Data.([]NBT) {
				nbtc, _ = elem.DeepCopy()
				nbts = append(nbts, *nbtc)

				molecule.Size++
			}
			molecule.Data = nbts
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
					molecule.Data.([]NBT)[0].Size = uint32(len(valu.(string)))
					molecule.Data.([]NBT)[0].Data = valu.(string)

				case "Health":
					molecule.Data.([]NBT)[14].Data = float32(valu.(float64))

				case "MaxHealth":
					molecule.Data.([]NBT)[28].Data.([]NBT)[0].Data.([]NBT)[0].Data = valu.(float64)

				case "MoveSpeed":
					molecule.Data.([]NBT)[28].Data.([]NBT)[1].Data.([]NBT)[0].Data = valu.(float64)

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

	// calculate a v4 UUID
	var uuid [16]byte
	_, err := rand.Read(uuid[:])
	panicOnErr(err)
	uuid[8] = (uuid[8] | 0x40) & 0x7F
	uuid[6] = (uuid[6] &  0xF) | (4 << 4)
	uuidmost := int64(binary.BigEndian.Uint64(uuid[0:8]))
	uuidlest := int64(binary.BigEndian.Uint64(uuid[8:16]))

	// modify the entity to have its own UUID
	molecule.Data.([]NBT)[1].Data = uuidmost
	molecule.Data.([]NBT)[2].Data = uuidlest

	rslt = &molecule
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
