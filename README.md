# worldcraft
Mindcraft world editor, written as an exercise in learning golang


### example usage
At this stage, all you can really do is cause worldcraft to load a set of Minecraft 'region' files and re-write them as new 'region' files.  At this stage, what this accomplishes is to validate the reading, parsing, and writing of NBT data, which is not too complicated, but not trivial, either.  Also, since this is an exercise in golang, part of the point is to accomplish this elgantly and idomatically, in order to learn to think in golang.  Basic command-line usage:
```
./worldcraft -edits just-load-and-save-r0.0.json -world /path/to/Minecraft/world/region-directory
```
