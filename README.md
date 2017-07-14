# worldcraft
Mindcraft world editor, written as an exercise in learning golang


### the utilities

**worldcraft**  reads a set of edits from a file, applies them to a Minecraft world, and generates edited region files
**blueprint2edits**  reads a 'blueprint' of block edits, and generates an 'edits' input file for use with **worldcraft**

These utilities are written for and have been tested with Minecraft v1.11.2


### example usage
#### basic usage
To test the fidelity of `worldcraft`, you can transform an original Minecraft-generated region file with a NOP 'edits' file \[let's call the result 'A'\], then transform the result of that again \[to get 'B'\].  Files 'A' and 'B' should match, byte-for-byte.

The reason for the first transformation is that Minecraft will generate chunks in a non-linear order, mainly (I think) because it's generating them as your avatar 'sees' them, which I assume includes ones you don't actually see yet but are near (such as the ones behind you when you first spawn, the ones off to the sides as you wander about, etc).  `worldcraft`, on the other hand, writes the chunks in their index-order.  So, even though it's not changing any content, it's reordering it.  But, 'A' and 'B' will be the same, because they are both written by `worldcraft`.

In order for this to work, you need to use a NOP 'edits' file.  The included file `just-load-and-save-r0.0.json` will work fine for a Superflat world.  You can make one similar that references an air block close to the ground.  An empty 'edits' file won't work, because `worldcraft` loads region files only as needed; so a file with no edits at all won't load any region files.  Also, if you fire up Minecraft -- even to take a quick peek -- the file will change quite a bit, because entities will move around, food might grow, fire might burn things, etc.

The commands for this would be something like the following (after you have initiated the world in Minecraft):
```bash
cp -ip /path/to/your/minecraft/saves/bravenewworld/region/r.0.0.mca  ./r.0.0.mca--bravenewworld--orig

mkdir testworldA
mkdir testworldB
cp -ip r.0.0.mca--bravenewworld--orig testworldA/

./worldcraft -edits just-load-and-save-r0.0.json -world ./testworldA
cp -ip  r.0.0.mca.1500003174  ./testworldB/r.0.0.mca

./worldcraft -edits just-load-and-save-r0.0.json -world ./testworldB

md5sum  r.0.0.mca.1500003174  r.0.0.mca.1500003193
```

The exact filename of the files produced by `worldcraft` will differ; they will have the execution-time timestamp on the filename.  This prevents overwriting any original region files, and it makes it easy to collect a series of edited files.


#### more complete usage
```bash
cp -ip /path/to/your/minecraft/saves/bravenewworld/region/r.0.0.mca  ./r.0.0.mca--bravenewworld--orig

# select a location for the structure; you specify the lower, northern, western corner; default is 0, 0, 0
# the following puts the tower in the middle of region 0, 0 and flush with the ground in a Superflat world
cat blueprint.small-tower | ./blueprint2edits -X 250 -Y 3 -Z 250 > a-small-tower.json
./worldcraft -edits a-small-tower.json -world /path/to/your/minecraft/saves/bravenewworld/region
cp -ip r.0.0.mca.1500004112 /path/to/your/minecraft/saves/bravenewworld/region
```
