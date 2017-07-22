# worldcraft
Mindcraft world editor, written as an exercise in learning golang


### a brief history

**v0.4.0**  works correctly and has most of the target features, but is not very idiomatic Go, and not well-designed for a Go program.

The **current** version, still under development, is in some ways a fresh start, in order to pursue what hopefully is a better design.  However, I am using several sizable chunks of logic from v0.4.0.  In particular, the NBT code is the same, although under v0.4.0 it was an import and currently it is a separate file within the `worldcraft` 'main' package.

This version also combines functionality that existed as separate utilities previously.  There are pros and cons to separating functionality into different programs, and for the overall approach I used before, separate utilities made more sense.  With the current design, combining the associated functions makes for smoother usage, without sacrificing utility.

These utilities are written for and have been tested with Minecraft v1.11.2


### usage summary

Usage of worldcraft:  
    &nbsp;&nbsp;&nbsp;&nbsp; -X : the westernmost  coordinate where the blueprint will be rendered in the gameworld  
    &nbsp;&nbsp;&nbsp;&nbsp; -Y : the lowest-layer coordinate where the blueprint will be rendered in the gameworld  
    &nbsp;&nbsp;&nbsp;&nbsp; -Z : the northernmost coordinate where the blueprint will be rendered in the gameworld  
    &nbsp;&nbsp;&nbsp;&nbsp; -blueprint : a file containing a blueprint of edits to make to the specified Minecraft world (default "UNDEFINED")  
    &nbsp;&nbsp;&nbsp;&nbsp; -debug : a flag to enable verbose output, for bug diagnosis and to validate detailed functionality  
    &nbsp;&nbsp;&nbsp;&nbsp; -json  : a flag to enable dumping the chunkdata to JSON  
    &nbsp;&nbsp;&nbsp;&nbsp; -world : a directory containing a collection of Minecraft region files (default "UNDEFINED")


### example usage

_coming soon_

