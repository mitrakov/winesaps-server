package battle

import "sync"
import "runtime"
import "container/list"
import . "mitrakov.ru/home/winesaps/sid"   // nolint
import . "mitrakov.ru/home/winesaps/utils" // nolint

// battlefield dimensions
const (
    // Width is count of cells in horizontal direction
    Width  = 51
    // Height is count of cells in vertical direction
    Height = 5
)

// characters
const (
    Rabbit = iota + 1
    Hedgehog
    Squirrel
    Cat
)

// CharactersCount is total number of characters
const CharactersCount = Cat
// roundTime is a standard round duration, in sec.
const roundTime = 90  // see note#5

// effect type
type effectT byte

// effects
const (
    effAntidote effectT = iota + 1
    effDazzle
    effAfraid
    effAttention
)

// effect of Antidote, in steps (if an actor takes an Antidote, he can safely take any food in 10 steps)
const antidoteEffect = 10
// length of mines detection for Detectors, in cells (if an actor takes Detector all mines in N cells will be detonated)
const detectionLength = 8

// Field expresses a single battle field with 2 actors and N wolves (N ≥ 0)
type Field struct {
    sync.Mutex
    battleManager    IBattleManager
    cells            [Width * Height]*Cell
    raw              []byte
    movablesDump     map[Movable]byte      // please don't rely upon this map! it's only dump to restore state (1.3.0+)
    movablesDumpLock sync.Mutex            // locker for movablesDump (do not use self-mutex, it may cause dead-locks)
    curObjNum        byte                  // objects incrementing counter
    cellLock         sync.Mutex            // extra lock on addObj/removeObj logical operation (1.3.5+)
    timeSec          byte
}

// newField creates a new instance of Field. Please do not create a Field directly.
// Note that a "Round" corresponds to a "Field" as 1:1
// "battleMgr" - reference to IBattleManager
// "levelname" - level filename
// nolint: gocyclo
func newField(battleMgr IBattleManager, levelname string) (*Field, *Error) {
    Assert(battleMgr)
    var err *Error

    // getting level raw bytearray
    reader := battleMgr.getFileReader()
    Assert(reader)
    level, ok := reader.GetByName(levelname)
    if !ok {
        err = NewErr(&Field{}, 90, "Level not found: %s", levelname)
    }

    // make a copy of obtained array (see note#4)
    raw := make([]byte, len(level))
    copy(raw, level)

    // parsing
    if err == nil {
        if len(raw) >= Width*Height {
            res := &Field{battleManager: battleMgr, raw: raw, movablesDump: make(map[Movable]byte), timeSec: roundTime}
            battleMgr.IncFieldRefs()
            runtime.SetFinalizer(res, func(*Field) {battleMgr.DecFieldRefs()})
            // parse level map
            for i := 0; i < Width*Height; i++ {
                res.cells[i] = newCell(byte(i), raw[i], func() byte { res.curObjNum++; return res.curObjNum })
            }
            // parse additional sections
            for j := Width * Height; j+1 < len(raw); j += 2 {
                sectionCode := raw[j]
                sectionLen := int(raw[j+1])
                switch sectionCode {
                case 1: // parse additional level objects
                    startK := j + 2
                    for k := startK; k+2 < startK+sectionLen && k+2 < len(raw); k += 3 {
                        num := raw[k]
                        id := raw[k+1]
                        xy := raw[k+2]
                        if int(xy) < len(res.cells) {
                            if num > res.curObjNum {
                                res.cells[xy].append(func() byte { return num }, id)
                            } else {
                                return nil, NewErr(res, 91, "Incorrect obj num (%d); must be > %d", num, res.curObjNum)
                            }
                        } else {
                            return nil, NewErr(res, 92, "Incorrect xy (%d)", xy)
                        }
                    }
                case 2: // style pack (useful only for a client)
                case 3: // parse round time
                    if j+2 < len(raw) {
                        res.timeSec = raw[j+2]
                    }
                }
                j += sectionLen
            }
            return res, nil
        }
        return nil, NewErr(new(Field), 93, "Incorrect field file length")
    }
    return nil, err
}

// getNextNum is a function to incr. current object number. It's important because all objects must have unique numbers
func (field *Field) getNextNum() byte {
    field.curObjNum++
    return field.curObjNum
}

// objChanged is called when an object "obj" changes its position to "xy". Set "reset" to TRUE only if the relocation
// is instantaneous, like teleportation or respawning after being hurt; otherwise set "reset" to FALSE
// "sid" - Session ID of initiator of the action
// "box" - MailBox to accumulate messages
func (field *Field) objChanged(sid Sid, obj Movable, xy byte, reset bool, box *MailBox) *Error {
    Assert(field.movablesDump, field.battleManager, obj)
    field.movablesDumpLock.Lock()
    field.movablesDump[obj] = xy
    field.movablesDumpLock.Unlock()
    return field.battleManager.objChanged(sid, obj.getNum(), obj.getID(), xy, reset, box)
}

// objChanged is called when a new object "obj" is arises (e.g. an actor established an umbrella or buries a mine)
// "sid" - Session ID of initiator of the action
// "box" - MailBox to accumulate messages
func (field *Field) objAppended(sid Sid, obj Movable, box *MailBox) *Error {
    Assert(field.movablesDump, field.battleManager, obj, obj.getCell())
    field.movablesDumpLock.Lock()
    field.movablesDump[obj] = obj.getCell().xy
    field.movablesDumpLock.Unlock()
    return field.battleManager.objAppended(sid, obj, box)
}

// isMoveUpPossible checks whether moving up from a given cell is possible (e.g. there is a LadderBottom)
func (field *Field) isMoveUpPossible(cell *Cell) bool {
    Assert(cell)
    return cell.hasLadderBottom() || cell.hasRopeLine()
}

// isMoveDownPossible checks whether moving down from a given cell is possible (e.g. there is a LadderTop)
func (field *Field) isMoveDownPossible(cell *Cell) bool {
    Assert(cell)
    return cell.hasLadderTop()
}

// move performs a single move of object "obj" to new location "idxTo".
// returns TRUE if an actual move has been done;
// if there has been no actual move (e.g. if an actor tried to "move" into a wall), returns FALSE
// "sid" - Session ID of a user who performs this action
// "box" - MailBox to accumulate messages
func (field *Field) move(sid Sid, obj Movable, idxTo int, box *MailBox) (success bool, err *Error) {
    // simple wrapper (because Go doesn't support reentrant mutexes)
    field.Lock()
    defer field.Unlock()
    return field.moveSync(sid, obj, idxTo, box)
}

// moveSync is internal implementation of move() method inside synchronized context.
// NEVER CALL THIS METHOD FROM APPLICATION CODE.
// "sid" - Session ID of a user who performs this action
// "obj" - movable object
// "idxTo" - new location
// "box" - MailBox to accumulate messages
// nolint: gocyclo
func (field *Field) moveSync(sid Sid, obj Movable, idxTo int, box *MailBox) (success bool, err *Error) {
    Assert(obj, obj.getCell())

    if 0 <= idxTo && idxTo < Width*Height {
        idx := byte(idxTo)
        oldCell := obj.getCell()
        newCell, err := field.getCell(idx)
        if err == nil {
            h := idxTo - int(oldCell.xy) // increment of index
            leftRight := h*h == 1
            // face an obstacle
            if newCell.hasBlock() {
                return false, nil
            }
            // climb a rope
            if h == -Width && oldCell.hasRopeLine() {
                err = field.relocate(sid, oldCell, newCell, obj, false, box)
                return true, err
            }
            // scale a dias
            if leftRight && !oldCell.hasDais() && newCell.hasDais() && !oldCell.hasRaisable() {
                if actor, ok := obj.(Actor); ok && !actor.hasSwagga(ClimbingShoes) {
                    return false, nil
                }
            }
            // sink through the floor
            if oldCell.bottom != nil {
                if h == Width && !oldCell.hasLadderTop() {
                    return false, nil
                }
                if h == -Width && !oldCell.hasLadderBottom() {
                    return false, nil
                }
            }
            // left-right edges
            if (oldCell.xy+1)%Width == 0 && (h > 0 && h < Width) { // if right edge
                return false, nil
            }
            if oldCell.xy%Width == 0 && (h < 0 && h > -Width) { // if left edge
                return false, nil
            }
            
            // relocating
            err = field.relocate(sid, oldCell, newCell, obj, false, box)
            // check if there is a firm ground underfoot
            if newCell.bottom != nil || newCell.hasBeamChunk() {
                return true, err
            }
            // else nothing underfoot: fall down!
            return field.moveSync(sid, obj, idxTo+Width, box)
        }
        return false, err
    }
    return false, nil // in fact client CAN send incorrect XY (example: Move(LeftDown) at X=0; Y=0); since 1.3.6
}

// getCell returns a cell by its index; please ALWAYS call this method instead of direct accessing the internal array
func (field *Field) getCell(idx byte) (*Cell, *Error) {
    if int(idx) < len(field.cells) {
        return field.cells[idx], nil
    }
    return nil, NewErr(field, 95, "Incorrect field index %d", idx)
}

// getActor1 returns Actor1 on the battlefield
func (field *Field) getActor1() (*Actor1, bool) {
    for _, c := range field.cells {
        c.Lock()
        defer c.Unlock()
        for j := c.objects.Front(); j != nil; j = j.Next() {
            if res, ok := j.Value.(*Actor1); ok {
                return res, true
            }
        }
    }
    return nil, false
}

// getActor2 returns Actor2 on the battlefield
func (field *Field) getActor2() (*Actor2, bool) {
    for _, c := range field.cells {
        c.Lock()
        defer c.Unlock()
        for j := c.objects.Front(); j != nil; j = j.Next() {
            if res, ok := j.Value.(*Actor2); ok {
                return res, true
            }
        }
    }
    return nil, false
}

// getWolves returns a list of Wolves on the battlefield
func (field *Field) getWolves() (res list.List) {
    for _, c := range field.cells {
        c.Lock()
        for j := c.objects.Front(); j != nil; j = j.Next() {
            if wolf, ok := j.Value.(*Wolf); ok {
                res.PushBack(wolf)
            }
        }
        c.Unlock()
    }
    return
}

// getFavouriteFoodList returns a list of FavouriteFood objects on the battlefield
func (field *Field) getFavouriteFoodList() (res list.List) {
    for _, c := range field.cells {
        c.Lock()
        for j := c.objects.Front(); j != nil; j = j.Next() {
            if food, ok := j.Value.(FavouriteFood); ok {
                res.PushBack(food)
            }
        }
        c.Unlock()
    }
    return
}

// getEntryByActor returns a proper entry point for a given actor (i.e. Entry1 for Actor1 or Entry2 for Actor2)
func (field *Field) getEntryByActor(actor Actor) (Entry, bool) {
    for _, c := range field.cells {
        c.Lock()
        defer c.Unlock()
        for j := c.objects.Front(); j != nil; j = j.Next() {
            if _, ok := actor.(*Actor1); ok {
                if res, ok := j.Value.(*Entry1); ok {
                    return res, true
                }
            }
            if _, ok := actor.(*Actor2); ok {
                if res, ok := j.Value.(*Entry2); ok {
                    return res, true
                }
            }
        }
    }
    return nil, false
}

// getFoodCount returns current count of food items on the battle field, including all poison items
func (field *Field) getFoodCount() byte {
    res := byte(0)
    for _, c := range field.cells {
        c.Lock()
        for j := c.objects.Front(); j != nil; j = j.Next() {
            if _, ok := j.Value.(Food); ok {
                res++
            }
        }
        c.Unlock()
    }
    return res
}

// dropThing drops an old thing "thing" from a given actor (it happens when the actor takes another thing)
// "sid" - Session ID of a user who performs this action
// "box" - MailBox to accumulate messages
func (field *Field) dropThing(sid Sid, actor Actor, thing Thing, box *MailBox) *Error {
    Assert(actor, actor.getCell(), thing)

    cell := actor.getCell()
    cell.addObject(thing)
    return field.objChanged(sid, thing, cell.xy, false, box)
}

// useThing is a method to perform establishing a given thing by a given actor; the Thing is consumed, and corresponding
// object has been created next to the actor, e.g. UmbrellaThing will be turned into Umbrella.
// "sid" - Session ID of a user who performs this action
// "box" - MailBox to accumulate messages
// nolint: gocyclo
func (field *Field) useThing(sid Sid, actor Actor, thing Thing, box *MailBox) *Error {
    Assert(actor, thing, field.battleManager)
    
    myCell := actor.getCell()
    Assert(myCell)

    // get an appropriate cell (if an obstacle ahead, actor uses its own cell)
    cell := field.getCellByDirection(myCell, actor.isDirectedToRight())
    if cell == nil || cell.hasBlock() || cell.hasWater() || (cell.hasDais() && !myCell.hasDais()) { // see note#5
        cell = myCell
    }
    Assert(cell)
    
    // === life hacks ===
    // 1) allow players to establish an umbrella not only in 1 step, but also in 2 steps away from Waterfalls (note#5)
    if _, ok := thing.(*UmbrellaThing); ok {
        if next := field.getCellByDirection(cell, actor.isDirectedToRight()); next != nil && next.hasWaterfall() {
            cell = next
        }
    }
    // 2) allow players to establish a beam over an abyss (instead of letting it down)
    if _, ok := thing.(*BeamThing); ok {
        if cell.bottom == nil {
            cell = myCell
        }
    }
    // ================

    // emplace object
    emplaced := thing.emplace(field.getNextNum(), myCell)
    err1 := field.objAppended(sid, emplaced, box)
    _, err2 := field.move(sid, emplaced, int(cell.xy), box)

    // for mines we must provide a safe single step over the buried mine
    if _, ok := emplaced.(*Mine); ok {
        actor.setEffect(effAttention, 2, nil)
    }
    return NewErrs(err1, err2)
}

// relocate is internal implementation of actual transferring a Movable object from one point to another.
// THIS METHOD IS NOT INTENDED TO BE CALLED FROM APPLICATION CODE APART FROM SPECIAL CASES.
// Please consider move() method instead.
// "sid" - Session ID of initiator of this action
// "oldCell" - old cell
// "newCell" - destination cell
// "obj" - movable object
// "reset" - TRUE, if location has been changed instantaneously (wounded, teleportation, etc.), FALSE otherwise
// "box" - MailBox to accumulate messages
func (field *Field) relocate(sid Sid, oldCell, newCell *Cell, obj Movable, reset bool, box *MailBox) *Error {
    Assert(field.battleManager, oldCell, newCell)

    field.cellLock.Lock() // this lock is needed, because relocate() may be called outside move() context (since 1.3.5)
    newCell.addObject(oldCell.removeObject(obj))
    field.cellLock.Unlock()
    
    err := field.objChanged(sid, obj, newCell.xy, reset, box)
    if err == nil && !reset { // when reset == true, no need to check cell (since 1.3.9)
        return field.checkCell(sid, newCell, box)
    }
    return err
}

// checkCell checks a given cell for any events after relocating objects, like consuming food, taking things and so on.
// "sid" - Session ID of initiator of this action
// "box" - MailBox to accumulate messages
// nolint: gocyclo
func (field *Field) checkCell(sid Sid, cell *Cell, box *MailBox) (err *Error) {
    Assert(cell, field.battleManager)

    if actor, ok := cell.hasActor(); ok {
        // ==== 1. Checks that DO NOT return (e.g. items can be collected simultaneously) ===
        if food, ok := cell.hasFood(); ok && !isPoison(actor, food) {
            cell.removeObject(food)
            err1 := field.objChanged(sid, food, 0xFF, true, box)
            err2 := field.battleManager.foodEaten(sid, box)
            err = NewErrs(err, err1, err2)
        }
        if thing, ok := cell.hasThing(); ok {
            cell.removeObject(thing)
            err1 := field.objChanged(sid, thing, 0xFF, true, box)
            err2 := field.battleManager.thingTaken(sid, thing, box)
            err = NewErrs(err, err1, err2)
        }
        if beam, ok := cell.hasBeam(); ok {
            cell.removeObject(beam)
            err1 := field.objChanged(sid, beam, 0xFF, true, box)
            err2 := field.createBeamChunks(sid, cell, actor.isDirectedToRight(), box)
            err = NewErrs(err, err1, err2)
        }
        if antidote, ok := cell.hasAntidote(); ok {
            actor.setEffect(effAntidote, antidoteEffect, func(effectT) {
                futureBox := NewMailBox()
                futureErr := field.battleManager.effectChanged(sid, effAntidote, false, actor.getNum(), futureBox)
                field.battleManager.onEvent(futureBox, futureErr)
            })
            cell.removeObject(antidote)
            err1 := field.objChanged(sid, antidote, 0xFF, true, box)
            err2 := field.battleManager.effectChanged(sid, effAntidote, true, actor.getNum(), box)
            err = NewErrs(err, err1, err2)
        }
        if bang, ok := cell.hasFlashbang(); ok {
            cell.removeObject(bang)
            err1 := field.objChanged(sid, bang, 0xFF, true, box)
            err2 := field.battleManager.setEffectOnEnemy(sid, effDazzle, box)
            err = NewErrs(err, err1, err2)
        }
        if detector, ok := cell.hasDetector(); ok {
            cell.removeObject(detector)
            err1 := field.objChanged(sid, detector, 0xFF, true, box)
            err2 := field.detectMines(sid, cell, actor.isDirectedToRight(), detectionLength, box)
            err = NewErrs(err, err1, err2)
        }
        // ==== 2. Checks that RETURN (if 2 things may hurt an actor, it loses only 1 live) ===
        // teleport case (please note, that teleport RETURNS to save from mines, water, etc.)
        if teleport, ok := cell.hasTeleport(); ok {
            cell.removeObject(teleport)
            err1 := field.objChanged(sid, teleport, 0xFF, true, box)
            newXy := getMirrorXy(cell.xy)
            newCell, err2 := field.getCellForTeleportation(newXy)
            if err2 == nil {
                err2 = field.relocate(sid, actor.getCell(), newCell, actor, true, box)
            }
            return NewErrs(err, err1, err2)
        }
        if food, ok := cell.hasFood(); ok && isPoison(actor, food) {
            cell.removeObject(food)
            err1 := field.objChanged(sid, food, 0xFF, true, box)
            if actor.hasEffect(effAntidote) {
                err2 := field.battleManager.foodEaten(sid, box)
                err = NewErrs(err, err1, err2) // no return here (we should check mines, waterfalls and so on)
            } else {
                err2 := field.battleManager.hurt(sid, poisoned, box)
                return NewErrs(err, err1, err2)
            }
        }
        if mine, ok := cell.hasMine(); ok && !cell.hasBeamChunk() && !actor.hasSwagga(SapperShoes) {
            if !actor.hasEffect(effAttention) {
                // thanks to this condition the actor can ONCE step onto the mine immediately upon it burried; but in
                // theory the condition allows the enemy to avoid explosion (if it's stepCount = mine.stepCount+1)
                // so let's consider it as a feature
                cell.removeObject(mine)
                err1 := field.objChanged(sid, mine, 0xFF, true, box)
                err2 := field.battleManager.hurt(sid, exploded, box)
                return NewErrs(err, err1, err2)
            }
        }
        if cell.hasWolf() {
            return field.battleManager.eatenByWolf(sid, actor, box)
        }
        if cell.hasWaterfall() && !cell.hasUmbrella() && !actor.hasSwagga(SouthWester) {
            return field.battleManager.hurt(sid, soaked, box)
        }
        if cell.hasWater() && !cell.hasBeamChunk() && !actor.hasSwagga(Snorkel) {
            return field.battleManager.hurt(sid, sunk, box)
        }
    }
    return nil
}

// getCellByDirection returns a neighbour cell for a given cell, either from the left or from the right (expressed by
// "toRight" parameter);
// it may return NULL, e.g. if "curCell" is on the edge
func (field *Field) getCellByDirection(curCell *Cell, toRight bool) (cell *Cell) {
    Assert(curCell)
    xy := curCell.xy

    if toRight {
        if (xy+1)%Width == 0 {
            return nil
        }
        cell = field.cells[xy+1]
    } else {
        if xy%Width == 0 {
            return nil
        }
        cell = field.cells[xy-1]
    }
    return
}

// createBeamChunks is a recursive function to create a bridge, that consists of 3 beam chunks;
// please note, that this method might "fail", e.g. if "cell0" is on the edge and the bridge has no room to grow
// "sid" - Session ID of a user who performs this action
// "cell0" - starting point to grow the bridge
// "toRight" - direction, either grow to right or to left
// "box" - MailBox to accumulate messages
func (field *Field) createBeamChunks(sid Sid, cell0 *Cell, toRight bool, box *MailBox) *Error {
    if cell0 != nil {
        cell1 := field.getCellByDirection(cell0, toRight)
        if cell1 != nil {
            cell2 := field.getCellByDirection(cell1, toRight)
            if cell2 != nil {
                cell3 := field.getCellByDirection(cell2, toRight)
                if cell3 != nil {
                    cell4 := field.getCellByDirection(cell3, toRight)
                    if cell4 != nil {
                        edgeA := cell0.bottom
                        edgeB := cell4.bottom
                        if edgeA != nil && edgeB != nil && edgeA.getID() == edgeB.getID() {
                            chunk1 := newBeamChunk(field.getNextNum(), cell1)
                            chunk2 := newBeamChunk(field.getNextNum(), cell2)
                            chunk3 := newBeamChunk(field.getNextNum(), cell3)
                            cell1.addObject(chunk1)
                            cell2.addObject(chunk2)
                            cell3.addObject(chunk3)
                            err1 := field.objAppended(sid, chunk1, box)
                            err2 := field.objAppended(sid, chunk2, box)
                            err3 := field.objAppended(sid, chunk3, box)
                            return NewErrs(err1, err2, err3)
                        }
                    }
                }
            }
        }
    }
    return nil
}

// detectMines is a recursive function to disarm mines in a given direction starting from a given cell up to "n" steps
// long; please note that it will find and deactivate ALL mines on this line.
// "sid" - Session ID of a user who performs this action
// "toRight" - direction, either detect mines leftwards or rightwards
// "box" - MailBox to accumulate messages
func (field *Field) detectMines(sid Sid, cell *Cell, toRight bool, n int, box *MailBox) (err *Error) {
    if cell != nil && n >= 0 {
        if mine, ok := cell.hasMine(); ok {
            cell.removeObject(mine)
            err = field.objChanged(sid, mine, 0xFF, true, box)
        }
        if err == nil {
            nextCell := field.getCellByDirection(cell, toRight)
            err = field.detectMines(sid, nextCell, toRight, n-1, box)
        }
    }
    return
}

// replaceFavouriteFood replaces "virtual" FavouriteFood objects with "real" Food objects according to given actors
// characters; e.g. if Actor1 is a Rabbit, then all FoodActor1 objects on the battlefield will be replaced with Carrot
// objects
func (field *Field) replaceFavouriteFood(actor1, actor2 Actor) {
    foodActorLst := field.getFavouriteFoodList()
    for j := foodActorLst.Front(); j != nil; j = j.Next() {
        if favouriteFood, ok := j.Value.(FavouriteFood); ok {
            // first determine who loves this "virtual" food
            var actor Actor
            if _, ok := favouriteFood.(*FoodActor1); ok {
                actor = actor1
            } else if _, ok := favouriteFood.(*FoodActor2); ok {
                actor = actor2
            }
            Assert(actor)

            // now replace "virtual" food with the actor's favorite one
            cell := favouriteFood.getCell()
            Assert(cell)
            food := field.createFavouriteFood(actor, favouriteFood.getNum(), cell)
            cell.removeObject(favouriteFood)
            cell.addObject(food)

            // also fix raw field data for sending to clients
            field.raw[cell.xy] = (field.raw[cell.xy] & 0xC0) | food.getID()
        }
    }
}

// createFavouriteFood is a factory method to produce new Food objects according to the actor's character;
// e.g. if an actor is a Hedgehog, it returns a new Mushroom
// "actor" - actor
// "num" - global object number on the battlefield
// "cell" - cell to place a new specific food object
func (field *Field) createFavouriteFood(actor Actor, num byte, cell *Cell) Food {
    Assert(cell)
    switch actor.getCharacter() {
    case Rabbit:
        return newCarrot(num, cell)
    case Hedgehog:
        return newMushroom(num, cell)
    case Squirrel:
        return newNut(num, cell)
    case Cat:
        return newMeat(num, cell)
    default:
        return newApple(num, cell)
    }
}

// getCellForTeleportation finds and returns a cell for teleporting from a given cell
// newXy - new location (0-255) to check
func (field *Field) getCellForTeleportation(newXy byte) (newCell *Cell, err *Error) {
    var f func(int, *Cell) (*Cell, *Error)
    f = func(xy int, cell *Cell) (*Cell, *Error) {
        if xy > 0 {
	        if nc, er := field.getCell(byte(xy)); er == nil {
                if nc.hasBlock() || nc.hasWaterInside() {
	                return f(xy - Width, nc)
                }
                return nc, er
	        }
        }
        return cell, nil
    }

    // 1) avoid teleportation to underground (not all levels support underground)
    if newXy / Width == Height-1 {
        newXy -= Width
    }
    // 2) avoid teleportation INSIDE the block/water (upon water surface is still allowed)
    if newCell, err = field.getCell(newXy); err == nil && (newCell.hasBlock() || newCell.hasWaterInside()) {
        indexToSearch := int(newXy % Width + (Height - 2) * Width)
        return f(indexToSearch, newCell)
    }
    return
}

// dumpMovables creates a binary dump of all Movable objects on the battlefield in internal format.
// this method is primarily written to support "Restore state" feature, introduced in 1.3.0
func (field *Field) dumpMovables() []byte {
    Assert(field.movablesDump)
    
    field.Lock()
    res := make([]byte, 3 * len(field.movablesDump))
    
    var i int
    for obj, xy := range field.movablesDump {
        res[i] = obj.getNum()
        res[i+1] = obj.getID()
        res[i+2] = xy
        i+=3        
    }
    field.Unlock()
    return res
}

// =======================
// === LOCAL FUNCTIONS ===
// =======================

// isPoison checks if given food is poison for a given actor's character, e.g. Nuts and Mushrooms are poison for a Cat
func isPoison(actor Actor, food Food) bool {
    switch actor.getCharacter() {
    case Rabbit:
        return isPoisonForRabbit(food)
    case Hedgehog:
        return isPoisonForHedgehog(food)
    case Squirrel:
        return isPoisonForSquirrel(food)
    case Cat:
        return isPoisonForCat(food)
    }
    return true
}

// isPoisonForRabbit checks if given food is poison for Rabbits
func isPoisonForRabbit(food Food) bool {
    // can eat apples, pears and CARROTS
    if _, ok := food.(*Mushroom); ok {
        return true
    }
    if _, ok := food.(*Nut); ok {
        return true
    }
    if _, ok := food.(*Meat); ok {
        return true
    }
    return false
}

// isPoisonForHedgehog checks if given food is poison for Hedgehogs
func isPoisonForHedgehog(food Food) bool {
    // can eat apples, pears and MUSHROOMS
    if _, ok := food.(*Carrot); ok {
        return true
    }
    if _, ok := food.(*Nut); ok {
        return true
    }
    if _, ok := food.(*Meat); ok {
        return true
    }
    return false
}

// isPoisonForSquirrel checks if given food is poison for Squirrels
func isPoisonForSquirrel(food Food) bool {
    // can eat apples, pears and NUTS
    if _, ok := food.(*Carrot); ok {
        return true
    }
    if _, ok := food.(*Mushroom); ok {
        return true
    }
    if _, ok := food.(*Meat); ok {
        return true
    }
    return false
}

// isPoisonForCat checks if given food is poison for Cats
func isPoisonForCat(food Food) bool {
    // can eat apples, pears and MEAT
    if _, ok := food.(*Carrot); ok {
        return true
    }
    if _, ok := food.(*Mushroom); ok {
        return true
    }
    if _, ok := food.(*Nut); ok {
        return true
    }
    return false
}

// getMirrorXy returns coordinates, "mirrored" to a given "xy"; e.g. for xy=0 it will return xy=254
func getMirrorXy(xy byte) byte {
    x0, y0 := xy % Width, xy / Width
    x := x0 + 2*(Width/2 - x0)
    y := y0 + 2*(Height/2 - y0)
    return y*Width + x
}

// note#4 (@mitrakov, 2017-04-14): earlier I used a "level" slice as "raw" data; but as soon as "Favourite Food" feature
// has been introduced, it [feature] starts to corrupt the original array by replacing favourite food with another
// objects; so since now we FULLY COPY original array to raw data to stay the original array untouched
//
// note#5 (@mitrakov, 2017-07-18): I added some improvements after newbie-beta-testing, e.g: to set up an umbrella, 
// a box or a beam, now there might be tolerance in ±1 step (earlier it MUST be the same cell). Also I was asked for
// increasing round time from 60 up to 90 sec.
//
