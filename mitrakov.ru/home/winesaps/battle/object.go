package battle

import "fmt"
import "sync"
import "math/rand"
import "container/list"
import . "mitrakov.ru/home/winesaps/utils" // nolint

// IObject is a base interface for ALL objects, like Apples, Pears, Actors, Wolves, Blocks, Umbrellas and so on
type IObject interface {
    getID() byte
    getNum() byte   // according to ServerAPI each object MUST return its number: positive for Movables and 0 for others
    getCell() *Cell
}

// Solid is an interface for solid objects like Block, Dais or BeamChunk
type Solid interface {
    IObject
    solidInfo() string
}

// Movable is an interface for all objects that could disappear (like Things) or change the location (like Actors)
type Movable interface {
    IObject
    setCell(cell *Cell)
}

// Block (ID=1) is a solid impassable object that can be used both for bottom and for casual object
type Block struct /*implements Solid*/ {
    cell *Cell
}

// newBlock creates a new instance of Block within a given cell
func newBlock(cell *Cell) Solid {
    Assert(cell)
    return &Block{cell}
}

// getID returns the ID of the object
func (*Block) getID() byte { return 0x01 }

// getNum returns a unique incremental number of the Movable object on the battlefield (or 0 for non-Movable objects)
func (*Block) getNum() byte { return 0 }

// getCell returns the cell in which the object is placed
func (block *Block) getCell() *Cell {
    return block.cell
}

// solidInfo is a "crutch" to make Solid interface differ from the others; please LMK if you know better solution!
func (*Block) solidInfo() string {
    return fmt.Sprintf("{Block}")
}

// Dais (ID=2) is bottom-only object that is impassable (by default) without a Stair (in other words, Dais is a Block,
// but twice higher)
type Dais struct /*implements Solid*/ {
    cell *Cell
}

// newDais creates a new instance of Dais within a given cell
func newDais(cell *Cell) Solid {
    Assert(cell)
    return &Dais{cell}
}

// getID returns the ID of the object
func (*Dais) getID() byte { return 0x02 }

// getNum returns a unique incremental number of the Movable object on the battlefield (or 0 for non-Movable objects)
func (*Dais) getNum() byte { return 0 }

// getCell returns the cell in which the object is placed
func (dais *Dais) getCell() *Cell {
    return dais.cell
}

// solidInfo is a "crutch" to make Solid interface differ from the others; please LMK if you know better solution!
func (*Dais) solidInfo() string {
    return fmt.Sprintf("{Dais}")
}

// Water (ID=3) is a non-solid object that can be used both for bottom and for casual object. By default, wolves can
// pass the Water bottom, whilst actors - can't
type Water struct {
    cell *Cell
}

// newWater creates a new instance of Water within a given cell
func newWater(cell *Cell) IObject {
    Assert(cell)
    return &Water{cell}
}

// getID returns the ID of the object
func (*Water) getID() byte { return 0x03 }

// getNum returns a unique incremental number of the Movable object on the battlefield (or 0 for non-Movable objects)
func (*Water) getNum() byte { return 0 }

// getCell returns the cell in which the object is placed
func (water *Water) getCell() *Cell {
    return water.cell
}

// Animate is a common interface for animated objects, like actors or wolves
type Animate interface {
    Movable
    animateInfo() string
}

// Actor is a "manifestation" of a user limited to a battlefield: it can move, eat food, take things and so on
type Actor interface {
    Animate
    getCharacter() byte
    setCharacter(character byte)
    isDirectedToRight() bool
    setDirectionRight(toRight bool)
    addStep()
    setEffect(id effectT, steps byte, callback func(effectT))
    hasEffect(id effectT) bool
    addSwagga(s Swagga)
    hasSwagga(s Swagga) bool
    getSwaggas() list.List // List[Swagga]
}

// actorEffectT is a supplemental data structure to keep info about Effect
type actorEffectT struct {
    steps byte
    callback func(effectT)
}

// Actor1 (ID=4) is an Actor for aggressors
type Actor1 struct /*implements Actor*/ {
    sync.RWMutex
    num       byte
    character byte
    dirRight  bool
    effects   map[effectT]*actorEffectT
    cell      *Cell
    swaggas   list.List // List[Swagga]
}

// newActor1 creates a new instance of Actor1 with a given sequential number within a given cell
func newActor1(num byte, cell *Cell) Actor {
    Assert(cell)
    return &Actor1{num: num, dirRight: true, effects: make(map[effectT]*actorEffectT), cell: cell}
}

// getID returns the ID of the object
func (*Actor1) getID() byte { return 0x04 }

// getNum returns a unique incremental number of the Movable object on the battlefield (or 0 for non-Movable objects)
func (actor *Actor1) getNum() byte {
    return actor.num
}

// getCell returns the cell in which the object is placed
func (actor *Actor1) getCell() *Cell {
    actor.RLock()
    defer actor.RUnlock()
    return actor.cell
}

// setCell assigns a new cell for the object
func (actor *Actor1) setCell(cell *Cell) {
    actor.Lock()
    actor.cell = cell
    actor.Unlock()
}

// getCharacter returns a character of the Actor (Rabbit, Squirrel and so on)
func (actor *Actor1) getCharacter() byte {
    return actor.character
}

// setCharacter assigns a new character for the object (Rabbit, Squirrel and so on)
func (actor *Actor1) setCharacter(character byte) {
    actor.character = character
}

// isDirectedToRight checks if an Actor is faced right
func (actor *Actor1) isDirectedToRight() bool {
    return actor.dirRight
}

// setDirectionRight sets current direction of the Actor (TRUE - to right, FALSE = to left)
func (actor *Actor1) setDirectionRight(toRight bool) {
    actor.dirRight = toRight
}

// addStep increases internal steps counter of all the Actor's effects and calls corresponding callbacks if needed
func (actor *Actor1) addStep() {
    actor.RLock()
    for k, v := range actor.effects {
        actor.RUnlock()
        if (v.steps > 0) {
            v.steps--
            if v.steps == 0 && v.callback != nil {
                v.callback(k)
            }
        }
        actor.RLock()
    }
    actor.RUnlock()
}

// setEffect adds a new effect with a given ID on the Actor that remains up to "steps" steps.
// After effect is finished, callback will be called (callback may be NULL)
func (actor *Actor1) setEffect(id effectT, steps byte, callback func(effectT)) {
    actor.Lock()
    actor.effects[id] = &actorEffectT{steps, callback}
    actor.Unlock()
}

// hasEffect checks if the Actor has a [still active] effect with a given ID
func (actor *Actor1) hasEffect(id effectT) bool {
    actor.RLock()
    defer actor.RUnlock()
    if v, ok := actor.effects[id]; ok {
        return v.steps > 0
    }
    return false
}

// addSwagga appends a new Swagga to the Actor
func (actor *Actor1) addSwagga(s Swagga) {
    actor.swaggas.PushBack(s)
}

// hasSwagga checks if the Actor has a given Swagga
func (actor *Actor1) hasSwagga(s Swagga) bool {
    for e := actor.swaggas.Front(); e != nil; e = e.Next() {
        if v, ok := e.Value.(Swagga); ok {
            if v == s {
                return true
            }
        }
    }
    return false
}

// getSwaggas returns a list of all Swaggas of the Actor
func (actor *Actor1) getSwaggas() list.List {
    return actor.swaggas
}

// animateInfo is a "crutch" to make Animate interface differ from the others; please LMK if you know better solution!
func (*Actor1) animateInfo() string {
    return fmt.Sprintf("{Actor1}")
}

// Actor2 (ID=5) is an Actor for defenders
type Actor2 struct /*implements Actor*/ {
    sync.RWMutex
    num       byte
    character byte
    dirRight  bool
    effects   map[effectT]*actorEffectT
    cell      *Cell
    swaggas   list.List // List[Swagga]
}

// newActor2 creates a new instance of Actor2 with a given sequential number within a given cell
func newActor2(num byte, cell *Cell) Actor {
    Assert(cell)
    return &Actor2{num: num, dirRight: false, effects: make(map[effectT]*actorEffectT), cell: cell}
}

// getID returns the ID of the object
func (*Actor2) getID() byte { return 0x05 }

// getNum returns a unique incremental number of the Movable object on the battlefield (or 0 for non-Movable objects)
func (actor *Actor2) getNum() byte {
    return actor.num
}

// getCell returns the cell in which the object is placed
func (actor *Actor2) getCell() *Cell {
    actor.RLock()
    defer actor.RUnlock()
    return actor.cell
}

// setCell assigns a new cell for the object
func (actor *Actor2) setCell(cell *Cell) {
    actor.Lock()
    actor.cell = cell
    actor.Unlock()
}

// getCharacter returns a character of the Actor (Rabbit, Squirrel and so on)
func (actor *Actor2) getCharacter() byte {
    return actor.character
}

// setCharacter assigns a new character for the object (Rabbit, Squirrel and so on)
func (actor *Actor2) setCharacter(character byte) {
    actor.character = character
}

// isDirectedToRight checks if an Actor is faced right
func (actor *Actor2) isDirectedToRight() bool {
    return actor.dirRight
}

// setDirectionRight sets current direction of the Actor (TRUE - to right, FALSE = to left)
func (actor *Actor2) setDirectionRight(toRight bool) {
    actor.dirRight = toRight
}

// addStep increases internal steps counter of all the Actor's effects and calls corresponding callbacks if needed
func (actor *Actor2) addStep() {
    actor.RLock()
    for k, v := range actor.effects {
        actor.RUnlock()
        if (v.steps > 0) {
            v.steps--
            if v.steps == 0 && v.callback != nil {
                v.callback(k)
            }
        }
        actor.RLock()
    }
    actor.RUnlock()
}

// setEffect adds a new effect with a given ID on the Actor that remains up to "steps" steps.
// After effect is finished, callback will be called (callback may be NULL)
func (actor *Actor2) setEffect(id effectT, steps byte, callback func(effectT)) {
    actor.Lock()
    actor.effects[id] = &actorEffectT{steps, callback}
    actor.Unlock()
}

// hasEffect checks if the Actor has a [still active] effect with a given ID
func (actor *Actor2) hasEffect(id effectT) bool {
    actor.RLock()
    defer actor.RUnlock()
    if v, ok := actor.effects[id]; ok {
        return v.steps > 0
    }
    return false
}

// addSwagga appends a new Swagga to the Actor
func (actor *Actor2) addSwagga(s Swagga) {
    actor.swaggas.PushBack(s)
}

// hasSwagga checks if the Actor has a given Swagga
func (actor *Actor2) hasSwagga(s Swagga) bool {
    for e := actor.swaggas.Front(); e != nil; e = e.Next() {
        if v, ok := e.Value.(Swagga); ok {
            if v == s {
                return true
            }
        }
    }
    return false
}

// getSwaggas returns a list of all Swaggas of the Actor
func (actor *Actor2) getSwaggas() list.List {
    return actor.swaggas
}

// animateInfo is a "crutch" to make Animate interface differ from the others; please LMK if you know better solution!
func (*Actor2) animateInfo() string {
    return fmt.Sprintf("{Actor2}")
}

// Wolf (ID=6) is an animated object that "chases" the actors; there may be several wolves on the battle field
type Wolf struct /*implements Animate*/ {
    sync.RWMutex
    num            byte
    justUsedLadder bool
    cell           *Cell
    curDir         int
}

// newWolf creates a new instance of Wolf with a given sequential number within a given cell
func newWolf(num byte, cell *Cell) Animate {
    Assert(cell)
    wolf := &Wolf{num: num, cell: cell, curDir: 1}
    wolf.setRandomDir()
    return wolf
}

// getID returns the ID of the object
func (*Wolf) getID() byte { return 0x06 }

// getNum returns a unique incremental number of the Movable object on the battlefield (or 0 for non-Movable objects)
func (wolf *Wolf) getNum() byte {
    return wolf.num
}

// getCell returns the cell in which the object is placed
func (wolf *Wolf) getCell() *Cell {
    wolf.RLock()
    defer wolf.RUnlock()
    return wolf.cell
}

// setCell assigns a new cell for the object
func (wolf *Wolf) setCell(cell *Cell) {
    wolf.Lock()
    wolf.cell = cell
    wolf.Unlock()
}

// setRandomDir randomly assigns the direction of the Wolf (right or left)
func (wolf *Wolf) setRandomDir() {
    if rand.Intn(2) == 0 {
        wolf.curDir = -1
    } else {
        wolf.curDir = 1
    }
}

// animateInfo is a "crutch" to make Animate interface differ from the others; please LMK if you know better solution!
func (*Wolf) animateInfo() string {
    return fmt.Sprintf("{Wolf}")
}

// Entry is an "invisible" start point of the Actors where they respawn at the beggining of the battle or after "dying"
type Entry interface {
    IObject
    entryInfo() string
}

// Entry1 (ID=7) is an entry point for Actor1
type Entry1 struct {
    cell *Cell
}

// newEntry1 creates a new instance of Entry1 within a given cell
func newEntry1(cell *Cell) Entry {
    Assert(cell)
    return &Entry1{cell}
}

// getID returns the ID of the object
func (*Entry1) getID() byte { return 0x07 }

// getNum returns a unique incremental number of the Movable object on the battlefield (or 0 for non-Movable objects)
func (*Entry1) getNum() byte { return 0 }

// getCell returns the cell in which the object is placed
func (entry1 *Entry1) getCell() *Cell {
    return entry1.cell
}

// entryInfo is a "crutch" to make Entry interface differ from the others; please LMK if you know better solution!
func (*Entry1) entryInfo() string {
    return fmt.Sprintf("{Entry1}")
}

// Entry2 (ID=8) is an entry point for Actor2
type Entry2 struct {
    cell *Cell
}

// newEntry2 creates a new instance of Entry2 within a given cell
func newEntry2(cell *Cell) Entry {
    Assert(cell)
    return &Entry2{cell}
}

// getID returns the ID of the object
func (*Entry2) getID() byte { return 0x08 }

// getNum returns a unique incremental number of the Movable object on the battlefield (or 0 for non-Movable objects)
func (*Entry2) getNum() byte { return 0 }

// getCell returns the cell in which the object is placed
func (entry2 *Entry2) getCell() *Cell {
    return entry2.cell
}

// entryInfo is a "crutch" to make Entry interface differ from the others; please LMK if you know better solution!
func (*Entry2) entryInfo() string {
    return fmt.Sprintf("{Entry2}")
}

// Ladder is an interface for ladders: objects that Animated objects can use to move up and down
type Ladder interface {
    IObject
    ladderInfo() string
}

// LadderTop (ID=9) is an object that Animated objects can use to move down
type LadderTop struct {
    cell *Cell
}

// newLadderTop creates a new instance of LadderTop within a given cell
func newLadderTop(cell *Cell) Ladder {
    Assert(cell)
    return &LadderTop{cell}
}

// getID returns the ID of the object
func (*LadderTop) getID() byte { return 0x09 }

// getNum returns a unique incremental number of the Movable object on the battlefield (or 0 for non-Movable objects)
func (*LadderTop) getNum() byte { return 0 }

// getCell returns the cell in which the object is placed
func (ladderTop *LadderTop) getCell() *Cell {
    return ladderTop.cell
}

// ladderInfo is a "crutch" to make Ladder interface differ from the others; please LMK if you know better solution!
func (*LadderTop) ladderInfo() string {
    return fmt.Sprintf("{LadderTop}")
}

// LadderBottom (ID=10) is an object that Animated objects can use to move up
type LadderBottom struct {
    cell *Cell
}

// newLadderBottom creates a new instance of LadderBottom within a given cell
func newLadderBottom(cell *Cell) Ladder {
    Assert(cell)
    return &LadderBottom{cell}
}

// getID returns the ID of the object
func (*LadderBottom) getID() byte { return 0x0A }

// getNum returns a unique incremental number of the Movable object on the battlefield (or 0 for non-Movable objects)
func (*LadderBottom) getNum() byte { return 0 }

// getCell returns the cell in which the object is placed
func (ladderBottom *LadderBottom) getCell() *Cell {
    return ladderBottom.cell
}

// ladderInfo is a "crutch" to make Ladder interface differ from the others; please LMK if you know better solution!
func (*LadderBottom) ladderInfo() string {
    return fmt.Sprintf("{LadderBottom}")
}

// Raisable is a interface for objects that Actors can use to mount a Dais
type Raisable interface {
    IObject
    raiseInfo() string
}

// Stair (ID=11) is an artificial object that Actors can use to mount a Dais
type Stair struct {
    cell *Cell
}

// newStair creates a new instance of Stair within a given cell
func newStair(cell *Cell) Raisable {
    Assert(cell)
    return &Stair{cell}
}

// getID returns the ID of the object
func (*Stair) getID() byte { return 0x0B }

// getNum returns a unique incremental number of the Movable object on the battlefield (or 0 for non-Movable objects)
func (*Stair) getNum() byte { return 0 }

// getCell returns the cell in which the object is placed
func (stair *Stair) getCell() *Cell {
    return stair.cell
}

// raiseInfo is a "crutch" to make Raisable interface differ from the others; please LMK if you know better solution!
func (*Stair) raiseInfo() string {
    return fmt.Sprintf("{Stair}")
}

// RopeLine (ID=12) is an object that Animated objects can use to raise up, like with LadderBottom
type RopeLine struct {
    cell *Cell
}

// newRopeLine creates a new instance of RopeLine within a given cell
func newRopeLine(cell *Cell) IObject {
    Assert(cell)
    return &RopeLine{cell}
}

// getID returns the ID of the object
func (*RopeLine) getID() byte { return 0x0C }

// getNum returns a unique incremental number of the Movable object on the battlefield (or 0 for non-Movable objects)
func (*RopeLine) getNum() byte { return 0 }

// getCell returns the cell in which the object is placed
func (ropeLine *RopeLine) getCell() *Cell {
    return ropeLine.cell
}

// Waterfall (ID=13) is an object that may hurt Actors (by default), and can be passed by with an Umbrella
type Waterfall struct {
    cell *Cell
}

// newWaterfall creates a new instance of Waterfall within a given cell
func newWaterfall(cell *Cell) IObject {
    Assert(cell)
    return &Waterfall{cell}
}

// getID returns the ID of the object
func (*Waterfall) getID() byte { return 0x0D }

// getNum returns a unique incremental number of the Movable object on the battlefield (or 0 for non-Movable objects)
func (*Waterfall) getNum() byte { return 0 }

// getCell returns the cell in which the object is placed
func (waterfall *Waterfall) getCell() *Cell {
    return waterfall.cell
}

// WaterfallSafe (ID=14) is the same as Waterfall, but doesn't hurt Actors. Used primarily for education purposes
// deprecated since 1.3.10, but kept for backward campatibility
type WaterfallSafe struct {
    cell *Cell
}

// newWaterfallSafe creates a new instance of WaterfallSafe within a given cell
func newWaterfallSafe(cell *Cell) IObject {
    Assert(cell)
    return &WaterfallSafe{cell}
}

// getCell returns the cell in which the object is placed
func (waterfall *WaterfallSafe) getCell() *Cell {
    return waterfall.cell
}

// getID returns the ID of the object
func (*WaterfallSafe) getID() byte { return 0x0E }

// getNum returns a unique incremental number of the Movable object on the battlefield (or 0 for non-Movable objects)
func (*WaterfallSafe) getNum() byte { return 0 }

// BeamChunk (ID=15) is a basic single component of bridges (a bridge basically consists of 3 chunks)
type BeamChunk struct /*implements Solid, Movable*/ {
    num  byte
    cell *Cell
}

// newBeamChunk creates a new instance of BeamChunk with a given sequential number within a given cell
func newBeamChunk(num byte, cell *Cell) Movable {
    Assert(cell)
    return &BeamChunk{num, cell}
}

// getID returns the ID of the object
func (*BeamChunk) getID() byte { return 0x0F }

// getNum returns a unique incremental number of the Movable object on the battlefield (or 0 for non-Movable objects)
func (chunk *BeamChunk) getNum() byte {
    return chunk.num
}

// getCell returns the cell in which the object is placed
func (chunk *BeamChunk) getCell() *Cell {
    return chunk.cell
}

// setCell assigns a new cell for the object
func (chunk *BeamChunk) setCell(cell *Cell) {
    chunk.cell = cell
}

// solidInfo is a "crutch" to make Solid interface differ from the others; please LMK if you know better solution!
func (*BeamChunk) solidInfo() string {
    return fmt.Sprintf("{BeamChunk}")
}

// Food is an interface for all food items: apples, pears, carrots, nuts, etc. These objects are consumed by Actors
type Food interface {
    Movable
    foodInfo() string
}

// Apple (ID=16) is common food: all characters can safely consume these objects
type Apple struct {
    num  byte
    cell *Cell
}

// newApple creates a new instance of Apple with a given sequential number within a given cell
func newApple(num byte, cell *Cell) Food {
    Assert(cell)
    return &Apple{num, cell}
}

// getID returns the ID of the object
func (*Apple) getID() byte { return 0x10 }

// getNum returns a unique incremental number of the Movable object on the battlefield (or 0 for non-Movable objects)
func (apple *Apple) getNum() byte {
    return apple.num
}

// getCell returns the cell in which the object is placed
func (apple *Apple) getCell() *Cell {
    return apple.cell
}

// setCell assigns a new cell for the object
func (apple *Apple) setCell(cell *Cell) {
    apple.cell = cell
}

// foodInfo is a "crutch" to make Food interface differ from the others; please LMK if you know better solution!
func (*Apple) foodInfo() string {
    return fmt.Sprintf("{Apple}")
}

// Pear (ID=17) is common food: all characters can safely consume these objects
type Pear struct {
    num  byte
    cell *Cell
}

// newPear creates a new instance of Pear with a given sequential number within a given cell
func newPear(num byte, cell *Cell) Food {
    Assert(cell)
    return &Pear{num, cell}
}

// getID returns the ID of the object
func (*Pear) getID() byte { return 0x11 }

// getNum returns a unique incremental number of the Movable object on the battlefield (or 0 for non-Movable objects)
func (pear *Pear) getNum() byte {
    return pear.num
}

// getCell returns the cell in which the object is placed
func (pear *Pear) getCell() *Cell {
    return pear.cell
}

// setCell assigns a new cell for the object
func (pear *Pear) setCell(cell *Cell) {
    pear.cell = cell
}

// foodInfo is a "crutch" to make Food interface differ from the others; please LMK if you know better solution!
func (*Pear) foodInfo() string {
    return fmt.Sprintf("{Pear}")
}

// Meat (ID=18) is food for Cats only; for other characters it's poison
type Meat struct {
    num  byte
    cell *Cell
}

// newMeat creates a new instance of Meat with a given sequential number within a given cell
func newMeat(num byte, cell *Cell) Food {
    Assert(cell)
    return &Meat{num, cell}
}

// getID returns the ID of the object
func (*Meat) getID() byte { return 0x12 }

// getNum returns a unique incremental number of the Movable object on the battlefield (or 0 for non-Movable objects)
func (meat *Meat) getNum() byte {
    return meat.num
}

// getCell returns the cell in which the object is placed
func (meat *Meat) getCell() *Cell {
    return meat.cell
}

// setCell assigns a new cell for the object
func (meat *Meat) setCell(cell *Cell) {
    meat.cell = cell
}

// foodInfo is a "crutch" to make Food interface differ from the others; please LMK if you know better solution!
func (*Meat) foodInfo() string {
    return fmt.Sprintf("{Meat}")
}

// Carrot (ID=19) is food for Rabbits only; for other characters it's poison
type Carrot struct {
    num  byte
    cell *Cell
}

// newCarrot creates a new instance of Carrot with a given sequential number within a given cell
func newCarrot(num byte, cell *Cell) Food {
    Assert(cell)
    return &Carrot{num, cell}
}

// getID returns the ID of the object
func (*Carrot) getID() byte { return 0x13 }

// getNum returns a unique incremental number of the Movable object on the battlefield (or 0 for non-Movable objects)
func (carrot *Carrot) getNum() byte {
    return carrot.num
}

// getCell returns the cell in which the object is placed
func (carrot *Carrot) getCell() *Cell {
    return carrot.cell
}

// setCell assigns a new cell for the object
func (carrot *Carrot) setCell(cell *Cell) {
    carrot.cell = cell
}

// foodInfo is a "crutch" to make Food interface differ from the others; please LMK if you know better solution!
func (*Carrot) foodInfo() string {
    return fmt.Sprintf("{Carrot}")
}

// Mushroom (ID=20) is food for Hedgehogs only; for other characters it's poison
type Mushroom struct {
    num  byte
    cell *Cell
}

// newMushroom creates a new instance of Mushroom with a given sequential number within a given cell
func newMushroom(num byte, cell *Cell) Food {
    Assert(cell)
    return &Mushroom{num, cell}
}

// getID returns the ID of the object
func (*Mushroom) getID() byte { return 0x14 }

// getNum returns a unique incremental number of the Movable object on the battlefield (or 0 for non-Movable objects)
func (mushroom *Mushroom) getNum() byte {
    return mushroom.num
}

// getCell returns the cell in which the object is placed
func (mushroom *Mushroom) getCell() *Cell {
    return mushroom.cell
}

// setCell assigns a new cell for the object
func (mushroom *Mushroom) setCell(cell *Cell) {
    mushroom.cell = cell
}

// foodInfo is a "crutch" to make Food interface differ from the others; please LMK if you know better solution!
func (*Mushroom) foodInfo() string {
    return fmt.Sprintf("{Mushroom}")
}

// Nut (ID=21) is food for Squirrels only; for other characters it's poison
type Nut struct {
    num  byte
    cell *Cell
}

// newNut creates a new instance of Nut with a given sequential number within a given cell
func newNut(num byte, cell *Cell) Food {
    Assert(cell)
    return &Nut{num, cell}
}

// getID returns the ID of the object
func (*Nut) getID() byte { return 0x15 }

// getNum returns a unique incremental number of the Movable object on the battlefield (or 0 for non-Movable objects)
func (nut *Nut) getNum() byte {
    return nut.num
}

// getCell returns the cell in which the object is placed
func (nut *Nut) getCell() *Cell {
    return nut.cell
}

// setCell assigns a new cell for the object
func (nut *Nut) setCell(cell *Cell) {
    nut.cell = cell
}

// foodInfo is a "crutch" to make Food interface differ from the others; please LMK if you know better solution!
func (*Nut) foodInfo() string {
    return fmt.Sprintf("{Nut}")
}

// FavouriteFood is an interface to designate favourite food (this is the virtual food that is replaced on round startup
// with actual food, e.g. with a Nut for Squirrels, a Carrot for Rabbits and so forth)
type FavouriteFood interface {
    Food
    favouriteFoodInfo() string
}

// FoodActor1 (ID=22) is a FavouriteFood implementation for Actor1
type FoodActor1 struct /*implements FavouriteFood*/ {
    num  byte
    cell *Cell
}

// newFoodActor1 creates a new instance of FoodActor1 with a given sequential number within a given cell
func newFoodActor1(num byte, cell *Cell) FavouriteFood {
    Assert(cell)
    return &FoodActor1{num, cell}
}

// getID returns the ID of the object
func (*FoodActor1) getID() byte { return 0x16 }

// getNum returns a unique incremental number of the Movable object on the battlefield (or 0 for non-Movable objects)
func (food *FoodActor1) getNum() byte {
    return food.num
}

// getCell returns the cell in which the object is placed
func (food *FoodActor1) getCell() *Cell {
    return food.cell
}

// setCell assigns a new cell for the object
func (food *FoodActor1) setCell(cell *Cell) {
    food.cell = cell
}

// foodInfo is a "crutch" to make Food interface differ from the others; please LMK if you know better solution!
func (*FoodActor1) foodInfo() string {
    return fmt.Sprintf("{FoodActor1}")
}

// favouriteFoodInfo is a "crutch" to make FavouriteFood interface differ from the others
func (*FoodActor1) favouriteFoodInfo() string {
    return fmt.Sprintf("{FoodActor1}")
}

// FoodActor2 (ID=23) is a FavouriteFood implementation for Actor2
type FoodActor2 struct /*implements FavouriteFood*/ {
    num  byte
    cell *Cell
}

// newFoodActor2 creates a new instance of FoodActor2 with a given sequential number within a given cell
func newFoodActor2(num byte, cell *Cell) FavouriteFood {
    Assert(cell)
    return &FoodActor2{num, cell}
}

// getID returns the ID of the object
func (*FoodActor2) getID() byte { return 0x17 }

// getNum returns a unique incremental number of the Movable object on the battlefield (or 0 for non-Movable objects)
func (food *FoodActor2) getNum() byte {
    return food.num
}

// getCell returns the cell in which the object is placed
func (food *FoodActor2) getCell() *Cell {
    return food.cell
}

// setCell assigns a new cell for the object
func (food *FoodActor2) setCell(cell *Cell) {
    food.cell = cell
}

// foodInfo is a "crutch" to make Food interface differ from the others; please LMK if you know better solution!
func (*FoodActor2) foodInfo() string {
    return fmt.Sprintf("{FoodActor2}")
}

// favouriteFoodInfo is a "crutch" to make FavouriteFood interface differ from the others
func (*FoodActor2) favouriteFoodInfo() string {
    return fmt.Sprintf("{FoodActor2}")
}

// Thing is an interface for items, that can be picked up by Actors and then converted into actual handy objects on the
// battlefield, e.g. an UmbrellaThing can be emplaced with an Umbrella to keep an Actor safe from Waterfalls.
// The action of "emplacement" will consume a Thing
type Thing interface {
    Movable
    emplace(num byte, cell *Cell) Emplaced
}

// Emplaced is an interface for "actual" objects that are established from the Things by demand of an Actor.
// E.g. when an Actor consumes a MineThing, it is emplaced with a Mine (which means: the Actor buried a Mine)
// The action of "emplacement" will consume a Thing
type Emplaced interface {
    Movable
    emplacedInfo() string
}

// UmbrellaThing (ID=32) is a Thing that can be emplaced with an Umbrella (ID=40)
type UmbrellaThing struct /*implements Thing*/ {
    num  byte
    cell *Cell
}

// newUmbrellaThing creates a new instance of UmbrellaThing with a given sequential number within a given cell
func newUmbrellaThing(num byte, cell *Cell) Thing {
    // for Things cell may be nil
    return &UmbrellaThing{num, cell}
}

// getID returns the ID of the object
func (*UmbrellaThing) getID() byte { return 0x20 }

// getNum returns a unique incremental number of the Movable object on the battlefield (or 0 for non-Movable objects)
func (umbrella *UmbrellaThing) getNum() byte {
    return umbrella.num
}

// getCell returns the cell in which the object is placed
func (umbrella *UmbrellaThing) getCell() *Cell {
    return umbrella.cell
}

// setCell assigns a new cell for the object
func (umbrella *UmbrellaThing) setCell(cell *Cell) {
    umbrella.cell = cell
}

// emplace establishes (well, creates) a new instance of Emplaced object with a given sequence number in a given cell
func (*UmbrellaThing) emplace(num byte, cell *Cell) Emplaced {
    return newUmbrella(num, cell)
}

// MineThing (ID=33) is a Thing that can be emplaced with a Mine (ID=41)
type MineThing struct /*implements Thing*/ {
    num  byte
    cell *Cell
}

// newMineThing creates a new instance of MineThing with a given sequential number within a given cell
func newMineThing(num byte, cell *Cell) Thing {
    // for Things cell may be nil
    return &MineThing{num, cell}
}

// getID returns the ID of the object
func (*MineThing) getID() byte { return 0x21 }

// getNum returns a unique incremental number of the Movable object on the battlefield (or 0 for non-Movable objects)
func (mine *MineThing) getNum() byte {
    return mine.num
}

// getCell returns the cell in which the object is placed
func (mine *MineThing) getCell() *Cell {
    return mine.cell
}

// setCell assigns a new cell for the object
func (mine *MineThing) setCell(cell *Cell) {
    mine.cell = cell
}

// emplace establishes (well, creates) a new instance of Emplaced object with a given sequence number in a given cell
func (*MineThing) emplace(num byte, cell *Cell) Emplaced {
    return newMine(num, cell)
}

// BeamThing (ID=34) is a Thing that can be emplaced with a Beam (ID=42).
type BeamThing struct /*implements Thing*/ {
    num  byte
    cell *Cell
}

// newBeamThing creates a new instance of BeamThing with a given sequential number within a given cell
func newBeamThing(num byte, cell *Cell) Thing {
    // for Things cell may be nil
    return &BeamThing{num, cell}
}

// getID returns the ID of the object
func (*BeamThing) getID() byte { return 0x22 }

// getNum returns a unique incremental number of the Movable object on the battlefield (or 0 for non-Movable objects)
func (beam *BeamThing) getNum() byte {
    return beam.num
}

// getCell returns the cell in which the object is placed
func (beam *BeamThing) getCell() *Cell {
    return beam.cell
}

// setCell assigns a new cell for the object
func (beam *BeamThing) setCell(cell *Cell) {
    beam.cell = cell
}

// emplace establishes (well, creates) a new instance of Emplaced object with a given sequence number in a given cell
func (*BeamThing) emplace(num byte, cell *Cell) Emplaced {
    return newBeam(num, cell)
}

// AntidoteThing (ID=35) is a Thing that can be emplaced with a bottle of Antidote (ID=43)
type AntidoteThing struct /*implements Thing*/ {
    num  byte
    cell *Cell
}

// newAntidoteThing creates a new instance of AntidoteThing with a given sequential number within a given cell
func newAntidoteThing(num byte, cell *Cell) Thing {
    // for Things cell may be nil
    return &AntidoteThing{num, cell}
}

// getID returns the ID of the object
func (*AntidoteThing) getID() byte { return 0x23 }

// getNum returns a unique incremental number of the Movable object on the battlefield (or 0 for non-Movable objects)
func (antidote *AntidoteThing) getNum() byte {
    return antidote.num
}

// getCell returns the cell in which the object is placed
func (antidote *AntidoteThing) getCell() *Cell {
    return antidote.cell
}

// setCell assigns a new cell for the object
func (antidote *AntidoteThing) setCell(cell *Cell) {
    antidote.cell = cell
}

// emplace establishes (well, creates) a new instance of Emplaced object with a given sequence number in a given cell
func (*AntidoteThing) emplace(num byte, cell *Cell) Emplaced {
    return newAntidote(num, cell)
}

// FlashbangThing (ID=36) is a Thing that can be emplaced with a FlashBang (ID=44)
type FlashbangThing struct /*implements Thing*/ {
    num  byte
    cell *Cell
}

// newFlashbangThing creates a new instance of FlashbangThing with a given sequential number within a given cell
func newFlashbangThing(num byte, cell *Cell) Thing {
    // for Things cell may be nil
    return &FlashbangThing{num, cell}
}

// getID returns the ID of the object
func (*FlashbangThing) getID() byte { return 0x24 }

// getNum returns a unique incremental number of the Movable object on the battlefield (or 0 for non-Movable objects)
func (bang *FlashbangThing) getNum() byte {
    return bang.num
}

// getCell returns the cell in which the object is placed
func (bang *FlashbangThing) getCell() *Cell {
    return bang.cell
}

// setCell assigns a new cell for the object
func (bang *FlashbangThing) setCell(cell *Cell) {
    bang.cell = cell
}

// emplace establishes (well, creates) a new instance of Emplaced object with a given sequence number in a given cell
func (*FlashbangThing) emplace(num byte, cell *Cell) Emplaced {
    return newFlashbang(num, cell)
}

// TeleportThing (ID=37) is a Thing that can be emplaced with a Teleport (ID=45)
type TeleportThing struct /*implements Thing*/ {
    num  byte
    cell *Cell
}

// newTeleportThing creates a new instance of TeleportThing with a given sequential number within a given cell
func newTeleportThing(num byte, cell *Cell) Thing {
    // for Things cell may be nil
    return &TeleportThing{num, cell}
}

// getID returns the ID of the object
func (*TeleportThing) getID() byte { return 0x25 }

// getNum returns a unique incremental number of the Movable object on the battlefield (or 0 for non-Movable objects)
func (teleport *TeleportThing) getNum() byte {
    return teleport.num
}

// getCell returns the cell in which the object is placed
func (teleport *TeleportThing) getCell() *Cell {
    return teleport.cell
}

// setCell assigns a new cell for the object
func (teleport *TeleportThing) setCell(cell *Cell) {
    teleport.cell = cell
}

// emplace establishes (well, creates) a new instance of Emplaced object with a given sequence number in a given cell
func (*TeleportThing) emplace(num byte, cell *Cell) Emplaced {
    return newTeleport(num, cell)
}

// DetectorThing (ID=38) is a Thing that can be emplaced with a mine Detector (ID=46)
type DetectorThing struct /*implements Thing*/ {
    num  byte
    cell *Cell
}

// newDetectorThing creates a new instance of DetectorThing with a given sequential number within a given cell
func newDetectorThing(num byte, cell *Cell) Thing {
    // for Things cell may be nil
    return &DetectorThing{num, cell}
}

// getID returns the ID of the object
func (*DetectorThing) getID() byte { return 0x26 }

// getNum returns a unique incremental number of the Movable object on the battlefield (or 0 for non-Movable objects)
func (detector *DetectorThing) getNum() byte {
    return detector.num
}

// getCell returns the cell in which the object is placed
func (detector *DetectorThing) getCell() *Cell {
    return detector.cell
}

// setCell assigns a new cell for the object
func (detector *DetectorThing) setCell(cell *Cell) {
    detector.cell = cell
}

// emplace establishes (well, creates) a new instance of Emplaced object with a given sequence number in a given cell
func (*DetectorThing) emplace(num byte, cell *Cell) Emplaced {
    return newDetector(num, cell)
}

// BoxThing (ID=33) is a Thing that can be emplaced with a Box (ID=47)
type BoxThing struct /*implements Thing*/ {
    num  byte
    cell *Cell
}

// newBoxThing creates a new instance of BoxThing with a given sequential number within a given cell
func newBoxThing(num byte, cell *Cell) Thing {
    // for Things cell may be nil
    return &BoxThing{num, cell}
}

// getID returns the ID of the object
func (*BoxThing) getID() byte { return 0x27 }

// getNum returns a unique incremental number of the Movable object on the battlefield (or 0 for non-Movable objects)
func (box *BoxThing) getNum() byte {
    return box.num
}

// getCell returns the cell in which the object is placed
func (box *BoxThing) getCell() *Cell {
    return box.cell
}

// setCell assigns a new cell for the object
func (box *BoxThing) setCell(cell *Cell) {
    box.cell = cell
}

// emplace establishes (well, creates) a new instance of Emplaced object with a given sequence number in a given cell
func (*BoxThing) emplace(num byte, cell *Cell) Emplaced {
    return newBox(num, cell)
}

// Umbrella (ID=40) is a tool to protect Actors from Waterfalls.
// Once established, it will stay up to the end of the round
type Umbrella struct {
    num  byte
    cell *Cell
}

// newUmbrella creates a new instance of Umbrella with a given sequential number within a given cell
func newUmbrella(num byte, cell *Cell) Emplaced {
    Assert(cell)
    return &Umbrella{num, cell}
}

// getID returns the ID of the object
func (*Umbrella) getID() byte { return 0x28 }

// getNum returns a unique incremental number of the Movable object on the battlefield (or 0 for non-Movable objects)
func (umbrella *Umbrella) getNum() byte {
    return umbrella.num
}

// getCell returns the cell in which the object is placed
func (umbrella *Umbrella) getCell() *Cell {
    return umbrella.cell
}

// setCell assigns a new cell for the object
func (umbrella *Umbrella) setCell(cell *Cell) {
    umbrella.cell = cell
}

// emplacedInfo is a "crutch" to make Emplaced interface differ from the others; please LMK if you know better solution!
func (umbrella *Umbrella) emplacedInfo() string {
    return fmt.Sprintf("{Umbrella(%d)}", umbrella.num)
}

// Mine (ID=41) is a object that explodes (by default) when Actors step on it. This causes lost of 1 live for Actors.
// Note that taking this object by Actors will consume it
type Mine struct {
    num  byte
    cell *Cell
}

// newMine creates a new instance of Mine with a given sequential number within a given cell
func newMine(num byte, cell *Cell) Emplaced {
    Assert(cell)
    return &Mine{num, cell}
}

// getID returns the ID of the object
func (*Mine) getID() byte { return 0x29 }

// getNum returns a unique incremental number of the Movable object on the battlefield (or 0 for non-Movable objects)
func (mine *Mine) getNum() byte {
    return mine.num
}

// getCell returns the cell in which the object is placed
func (mine *Mine) getCell() *Cell {
    return mine.cell
}

// setCell assigns a new cell for the object
func (mine *Mine) setCell(cell *Cell) {
    mine.cell = cell
}

// emplacedInfo is a "crutch" to make Emplaced interface differ from the others; please LMK if you know better solution!
func (mine *Mine) emplacedInfo() string {
    return fmt.Sprintf("{Mine(%d)}", mine.num)
}

// Beam (ID=42) is a tool that can be used to build a bridge 3 cells long.
// Note that taking this object by Actors will consume it, and new 3 chunks will appear instead.
// IMPORTANT TERMINOLOGY:
// If a Beam is just established (in vertical position) => it takes only 1 cell, so we call it a "Beam" (ID=42); but
// when the Beam is converted into a bridge (in horisontal position) => it will take 3 cells, so the original Beam will
// be destroyed, and 3 new chunks will be created, so we call them "BeamChunks" (ID=15)
type Beam struct {
    num  byte
    cell *Cell
}

// newBeam creates a new instance of Beam with a given sequential number within a given cell
func newBeam(num byte, cell *Cell) Emplaced {
    Assert(cell)
    return &Beam{num, cell}
}

// getID returns the ID of the object
func (*Beam) getID() byte { return 0x2A }

// getNum returns a unique incremental number of the Movable object on the battlefield (or 0 for non-Movable objects)
func (beam *Beam) getNum() byte {
    return beam.num
}

// getCell returns the cell in which the object is placed
func (beam *Beam) getCell() *Cell {
    return beam.cell
}

// setCell assigns a new cell for the object
func (beam *Beam) setCell(cell *Cell) {
    beam.cell = cell
}

// emplacedInfo is a "crutch" to make Emplaced interface differ from the others; please LMK if you know better solution!
func (beam *Beam) emplacedInfo() string {
    return fmt.Sprintf("{Beam(%d)}", beam.num)
}

// Antidote (ID=43) is an object that provides the persistence to poison food for 10 steps (by default).
// Note that taking this object by Actors will consume it
type Antidote struct {
    num  byte
    cell *Cell
}

// newAntidote creates a new instance of Antidote with a given sequential number within a given cell
func newAntidote(num byte, cell *Cell) Emplaced {
    Assert(cell)
    return &Antidote{num, cell}
}

// getID returns the ID of the object
func (*Antidote) getID() byte { return 0x2B }

// getNum returns a unique incremental number of the Movable object on the battlefield (or 0 for non-Movable objects)
func (antidote *Antidote) getNum() byte {
    return antidote.num
}

// getCell returns the cell in which the object is placed
func (antidote *Antidote) getCell() *Cell {
    return antidote.cell
}

// setCell assigns a new cell for the object
func (antidote *Antidote) setCell(cell *Cell) {
    antidote.cell = cell
}

// emplacedInfo is a "crutch" to make Emplaced interface differ from the others; please LMK if you know better solution!
func (antidote *Antidote) emplacedInfo() string {
    return fmt.Sprintf("{Antidote(%d)}", antidote.num)
}

// Flashbang (ID=44) is an object that can flash blind the enemy for several seconds.
// Note that taking this object by Actors will consume it
type Flashbang struct {
    num  byte
    cell *Cell
}

// newFlashbang creates a new instance of Flashbang with a given sequential number within a given cell
func newFlashbang(num byte, cell *Cell) Emplaced {
    Assert(cell)
    return &Flashbang{num, cell}
}

// getID returns the ID of the object
func (*Flashbang) getID() byte { return 0x2C }

// getNum returns a unique incremental number of the Movable object on the battlefield (or 0 for non-Movable objects)
func (bang *Flashbang) getNum() byte {
    return bang.num
}

// getCell returns the cell in which the object is placed
func (bang *Flashbang) getCell() *Cell {
    return bang.cell
}

// setCell assigns a new cell for the object
func (bang *Flashbang) setCell(cell *Cell) {
    bang.cell = cell
}

// emplacedInfo is a "crutch" to make Emplaced interface differ from the others; please LMK if you know better solution!
func (bang *Flashbang) emplacedInfo() string {
    return fmt.Sprintf("{Flashbang(%d)}", bang.num)
}

// Teleport (ID=45) is a special tool that relocates Actors to the mirrored (by default) cell on the battlefield.
// Note that taking this object by Actors will consume it
type Teleport struct {
    num  byte
    cell *Cell
}
// newTeleport creates a new instance of Teleport with a given sequential number within a given cell
func newTeleport(num byte, cell *Cell) Emplaced {
    Assert(cell)
    return &Teleport{num, cell}
}

// getID returns the ID of the object
func (*Teleport) getID() byte { return 0x2D }

// getNum returns a unique incremental number of the Movable object on the battlefield (or 0 for non-Movable objects)
func (teleport *Teleport) getNum() byte {
    return teleport.num
}

// getCell returns the cell in which the object is placed
func (teleport *Teleport) getCell() *Cell {
    return teleport.cell
}

// setCell assigns a new cell for the object
func (teleport *Teleport) setCell(cell *Cell) {
    teleport.cell = cell
}

// emplacedInfo is a "crutch" to make Emplaced interface differ from the others; please LMK if you know better solution!
func (teleport *Teleport) emplacedInfo() string {
    return fmt.Sprintf("{Teleport(%d)}", teleport.num)
}

// Detector (ID=46) is a handy tool that deactivates mines N steps ahead (by default N=8).
// Note that taking this object by Actors will consume it
type Detector struct {
    num  byte
    cell *Cell
}

// newDetector creates a new instance of Detector with a given sequential number within a given cell
func newDetector(num byte, cell *Cell) Emplaced {
    Assert(cell)
    return &Detector{num, cell}
}

// getID returns the ID of the object
func (*Detector) getID() byte { return 0x2E }

// getNum returns a unique incremental number of the Movable object on the battlefield (or 0 for non-Movable objects)
func (detector *Detector) getNum() byte {
    return detector.num
}

// getCell returns the cell in which the object is placed
func (detector *Detector) getCell() *Cell {
    return detector.cell
}

// setCell assigns a new cell for the object
func (detector *Detector) setCell(cell *Cell) {
    detector.cell = cell
}

// emplacedInfo is a "crutch" to make Emplaced interface differ from the others; please LMK if you know better solution!
func (detector *Detector) emplacedInfo() string {
    return fmt.Sprintf("{Detector(%d)}", detector.num)
}

// Box (ID=47) is a tool that helps Actors to raise a Dais (by default Actors aren't able to raise the Dais)
// Once established, it will stay up to the end of the round
type Box struct /*implements Emplaced, Raisable*/ {
    num  byte
    cell *Cell
}

// newBox creates a new instance of Box with a given sequential number within a given cell
func newBox(num byte, cell *Cell) Emplaced {
    Assert(cell)
    return &Box{num, cell}
}

// getID returns the ID of the object
func (*Box) getID() byte { return 0x2F }

// getNum returns a unique incremental number of the Movable object on the battlefield (or 0 for non-Movable objects)
func (box *Box) getNum() byte {
    return box.num
}

// getCell returns the cell in which the object is placed
func (box *Box) getCell() *Cell {
    return box.cell
}

// setCell assigns a new cell for the object
func (*Box) setCell(*Cell) {}

// emplacedInfo is a "crutch" to make Emplaced interface differ from the others; please LMK if you know better solution!
func (box *Box) emplacedInfo() string {
    return fmt.Sprintf("{Box(%d)}", box.num)
}

// raiseInfo is a "crutch" to make Raisable interface differ from the others; please LMK if you know better solution!
func (box *Box) raiseInfo() string {
    return fmt.Sprintf("{Box(%d)}", box.num)
}

// Decoration is an interface for bells and whistles on the battlefield; they WILL NOT affect the proccess of battle
type Decoration interface {
    IObject
    decorationInfo() string
}

// DecorationStatic (ID=48) is usual static decoration.
// It is drawn only by a Client, and won't affect the battle mechanics
type DecorationStatic struct /*implements Decoration*/ {
    cell *Cell
}

// newDecorationStatic creates a new instance of DecorationStatic within a given cell
func newDecorationStatic(cell *Cell) Decoration {
    Assert(cell)
    return &DecorationStatic{cell}
}

// getID returns the ID of the object
func (*DecorationStatic) getID() byte { return 0x30 }

// getNum returns a unique incremental number of the Movable object on the battlefield (or 0 for non-Movable objects)
func (*DecorationStatic) getNum() byte { return 0 }

// getCell returns the cell in which the object is placed
func (decoration *DecorationStatic) getCell() *Cell {
    return decoration.cell
}

// decorationInfo is a "crutch" to make Decoration interface differ from the others; LMK if you know better solution!
func (*DecorationStatic) decorationInfo() string {
    return fmt.Sprintf("{DecorationStatic}")
}

// DecorationDynamic (ID=49) is animated decoration.
// It is drawn only by a Client, and won't affect the battle mechanics
type DecorationDynamic struct /*implements Decoration*/ {
    cell *Cell
}

// newDecorationDynamic creates a new instance of DecorationDynamic within a given cell
func newDecorationDynamic(cell *Cell) Decoration {
    Assert(cell)
    return &DecorationDynamic{cell}
}

// getID returns the ID of the object
func (*DecorationDynamic) getID() byte { return 0x31 }

// getNum returns a unique incremental number of the Movable object on the battlefield (or 0 for non-Movable objects)
func (*DecorationDynamic) getNum() byte { return 0 }

// getCell returns the cell in which the object is placed
func (decoration *DecorationDynamic) getCell() *Cell {
    return decoration.cell
}

// decorationInfo is a "crutch" to make Decoration interface differ from the others; LMK if you know better solution!
func (*DecorationDynamic) decorationInfo() string {
    return fmt.Sprintf("{DecorationDynamic}")
}

// DecorationWarning (ID=50) is decoration like a sign that warns about some danger ahead.
// It is drawn only by a Client, and won't affect the battle mechanics
type DecorationWarning struct /*implements Decoration*/ {
    cell *Cell
}

// newDecorationWarning creates a new instance of DecorationWarning within a given cell
func newDecorationWarning(cell *Cell) Decoration {
    Assert(cell)
    return &DecorationWarning{cell}
}

// getID returns the ID of the object
func (*DecorationWarning) getID() byte { return 0x32 }

// getNum returns a unique incremental number of the Movable object on the battlefield (or 0 for non-Movable objects)
func (*DecorationWarning) getNum() byte { return 0 }

// getCell returns the cell in which the object is placed
func (decoration *DecorationWarning) getCell() *Cell {
    return decoration.cell
}

// decorationInfo is a "crutch" to make Decoration interface differ from the others; LMK if you know better solution!
func (*DecorationWarning) decorationInfo() string {
    return fmt.Sprintf("{DecorationWarning}")
}

// DecorationDanger (ID=51) is the same as DecorationWarning, but warns about buried Mines and looks highly aggressive.
// It is drawn only by a Client, and won't affect the battle mechanics
type DecorationDanger struct /*implements Decoration*/ {
    cell *Cell
}

// newDecorationDanger creates a new instance of DecorationDanger within a given cell
func newDecorationDanger(cell *Cell) Decoration {
    Assert(cell)
    return &DecorationDanger{cell}
}

// getID returns the ID of the object
func (*DecorationDanger) getID() byte { return 0x33 }

// getNum returns a unique incremental number of the Movable object on the battlefield (or 0 for non-Movable objects)
func (*DecorationDanger) getNum() byte { return 0 }

// getCell returns the cell in which the object is placed
func (decoration *DecorationDanger) getCell() *Cell {
    return decoration.cell
}

// decorationInfo is a "crutch" to make Decoration interface differ from the others; LMK if you know better solution!
func (*DecorationDanger) decorationInfo() string {
    return fmt.Sprintf("{DecorationDanger}")
}
