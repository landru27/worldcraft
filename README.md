# worldcraft
Mindcraft world editor, written as an exercise in learning golang


### a brief history

**v0.4.0**  works correctly and has most of the target features, but is not very idiomatic Go, and not well-designed for a Go program.

**v1.3.0**  is hopefully a better design, even with sizeable chunks of logic from v0.4.0 retained.

This version also combines functionality that existed as separate utilities previously.  There are pros and cons to separating functionality into different programs, and for the overall approach I used before, separate utilities made more sense.  With the current design, combining the associated functions makes for smoother usage, without sacrificing utility.

These utilities are written for and have been tested with Minecraft v1.11.2


### usage summary

Usage of worldcraft:  
    &nbsp;&nbsp;&nbsp;&nbsp; -debug : a flag to enable verbose output, for bug diagnosis and to validate detailed functionality  
    &nbsp;&nbsp;&nbsp;&nbsp; -json  : a flag to enable dumping the chunkdata to JSON  
    &nbsp;&nbsp;&nbsp;&nbsp; -world : a directory containing a collection of Minecraft region files (default "UNDEFINED")  
    &nbsp;&nbsp;&nbsp;&nbsp; -blueprint : a file containing a blueprint of edits to make to the specified Minecraft world (default "UNDEFINED")  
    &nbsp;&nbsp;&nbsp;&nbsp; -X : the westernmost  coordinate where the blueprint will be rendered in the gameworld  
    &nbsp;&nbsp;&nbsp;&nbsp; -Y : the lowest-layer coordinate where the blueprint will be rendered in the gameworld  
    &nbsp;&nbsp;&nbsp;&nbsp; -Z : the northernmost coordinate where the blueprint will be rendered in the gameworld  


### example usage

The `worldcraft` program looks for the blueprint legend files `blueprint-glyphs.json` and `blueprint-entities.json` in the same directory as the `worldcraft` executable.  So, for example, if you perform a typical `go install ./...`, you will need to cp these two `.json` files to `~/go/bin`.

a typical edit; places a small keep with stocked chests and other furnishings into the world
```
./worldcraft -blueprint blueprints/adventure/blueprint.homestead -world [MINECRAFT_PATH]/saves/Hesperia/region -X 4 -Y 59 -Z 173
```

the same edit, repeated; thus, we skip re-adding the livestock since they already exist, and we reset things like the chest contents, to avoid region file data problems
```
./worldcraft -blueprint blueprints/adventure/blueprint.homestead -world [MINECRAFT_PATH]/saves/Hesperia/region -X 4 -Y 59 -Z 173 -xairblocks -skipentities -resetblockentities
```

a different kind of structure, further afield
```
./worldcraft -blueprint blueprints/adventure/blueprint.outpost -world [MINECRAFT_PATH]/saves/Hesperia/region -X 147 -Y 63 -Z 3834
```

an edit in the Nether; a protective structure around our portal
```
./worldcraft -blueprint blueprints/adventure/blueprint.netherbunker -world [MINECRAFT_PATH]/saves/Hesperia/DIM-1/region -X -2 -Y 73 -Z 6
```

