package battle

import "sync"
import "container/list"
import . "mitrakov.ru/home/winesaps/utils" // nolint

// Cell is a single cell that may contain objects like actors, wolves, things, food, ropes, ladders and so on.
type Cell struct {
    sync.RWMutex // to protect objects List
    xy           byte
    bottom       IObject
    objects      list.List //List[*Object]
}

// newCell creates a new instance of Cell. Please do not create a Cell directly.
// "xy" - coordinate of a cell (0-254)
// "value" - binary string (2 higher bytes for bottom, lower 6 bytes - for an object (please note that with this
// binary string you can create no more than 1 object, but in general a cell can hold a lot of objects;
// consider addObject() method))
// "objNum" - external incrementing function to numerate objects (made as function, not as a value, because the
// numbers must be unique across the battlefield).
// nolint: gocyclo
func newCell(xy, value byte, objNum func() byte) *Cell {
    res := &Cell{xy: xy}

    switch value >> 6 {
    case 1:
        res.bottom = newBlock(res)
    case 2:
        res.bottom = newDais(res)
    case 3:
        res.bottom = newWater(res)
    }
    res.append(objNum, value & 0x3F)
    
    return res
}

// append adds an object with a given ID to the cell
// "objNum" - external incrementing function to numerate objects
// nolint: gocyclo
func (cell *Cell) append(objNum func() byte, id byte) {
    // ATTENTION! When modify this function don't forget to also modify aimanager.go
    Assert(cell)
    
    var obj IObject // ONLY VIA INTERFACE! this is for type safety!
    switch id {
    case 0x01:
        obj = newBlock(cell)
    case 0x02:
        obj = newDais(cell)
    case 0x03:
        obj = newWater(cell)
    case 0x04:
        obj = newActor1(objNum(), cell)
    case 0x05:
        obj = newActor2(objNum(), cell)
    case 0x06:
        obj = newWolf(objNum(), cell)
    case 0x07:
        obj = newEntry1(cell)
    case 0x08:
        obj = newEntry2(cell)
    case 0x09:
        obj = newLadderTop(cell)
    case 0x0A:
        obj = newLadderBottom(cell)
    case 0x0B:
        obj = newStair(cell)
    case 0x0C:
        obj = newRopeLine(cell)
    case 0x0D:
        obj = newWaterfall(cell)
    case 0x0E:
        obj = newWaterfallSafe(cell)
    case 0x0F:
        obj = newBeamChunk(objNum(), cell)
    case 0x10:
        obj = newApple(objNum(), cell)
    case 0x11:
        obj = newPear(objNum(), cell)
    case 0x12:
        obj = newMeat(objNum(), cell)
    case 0x13:
        obj = newCarrot(objNum(), cell)
    case 0x14:
        obj = newMushroom(objNum(), cell)
    case 0x15:
        obj = newNut(objNum(), cell)
    case 0x16:
        obj = newFoodActor1(objNum(), cell)
    case 0x17:
        obj = newFoodActor2(objNum(), cell)
    case 0x20:
        obj = newUmbrellaThing(objNum(), cell)
    case 0x21:
        obj = newMineThing(objNum(), cell)
    case 0x22:
        obj = newBeamThing(objNum(), cell)
    case 0x23:
        obj = newAntidoteThing(objNum(), cell)
    case 0x24:
        obj = newFlashbangThing(objNum(), cell)
    case 0x25:
        obj = newTeleportThing(objNum(), cell)
    case 0x26:
        obj = newDetectorThing(objNum(), cell)
    case 0x27:
        obj = newBoxThing(objNum(), cell)
    case 0x28:
        obj = newUmbrella(objNum(), cell)
    case 0x29:
        obj = newMine(objNum(), cell)
    case 0x2A:
        obj = newBeam(objNum(), cell)
    case 0x2B:
        obj = newAntidote(objNum(), cell)
    case 0x2C:
        obj = newFlashbang(objNum(), cell)
    case 0x2D:
        obj = newTeleport(objNum(), cell)
    case 0x2E:
        obj = newDetector(objNum(), cell)
    case 0x2F:
        obj = newBox(objNum(), cell)
    case 0x30:
        obj = newDecorationStatic(cell)
    case 0x31:
        obj = newDecorationDynamic(cell)
    case 0x32:
        obj = newDecorationWarning(cell)
    case 0x33:
        obj = newDecorationDanger(cell)
    }
    if obj != nil {
        cell.Lock()
        cell.objects.PushBack(obj)
        cell.Unlock()
    }
}

// addObject places a given object inside the cell
func (cell *Cell) addObject(obj Movable) {
    Assert(obj)
    cell.Lock()
    cell.objects.PushBack(obj)
    obj.setCell(cell)
    cell.Unlock()
}

// removeObject removes a given object out of the cell
func (cell *Cell) removeObject(obj Movable) Movable {
    cell.Lock()
    defer cell.Unlock()
    for e := cell.objects.Front(); e != nil; e = e.Next() {
        if e.Value == obj {
            cell.objects.Remove(e)
            obj.setCell(nil)
        }
    }
    return obj
}

// hasDais checks whether a cell has Dais at the bottom
func (cell *Cell) hasDais() bool {
    _, ok := cell.bottom.(*Dais)
    return ok
}

// hasWater checks whether a cell has Water at the bottom
func (cell *Cell) hasWater() bool {
    _, ok := cell.bottom.(*Water)
    return ok
}

// hasBlock checks whether a cell contains a Block object (note checking for OBJECT, not for BOTTOM!)
func (cell *Cell) hasBlock() bool {
    cell.RLock()
    defer cell.RUnlock()
    for obj := cell.objects.Front(); obj != nil; obj = obj.Next() {
        if _, ok := obj.Value.(*Block); ok {
            return true
        }
    }
    return false
}

// hasWaterInside checks whether a cell contains a Water object (note checking for OBJECT, not for BOTTOM!)
func (cell *Cell) hasWaterInside() bool {
    cell.RLock()
    defer cell.RUnlock()
    for obj := cell.objects.Front(); obj != nil; obj = obj.Next() {
        if _, ok := obj.Value.(*Water); ok {
            return ok
        }
    }
    return false
}

// hasActor checks whether a cell contains an Actor object (regardless Actor1 or Actor2)
func (cell *Cell) hasActor() (Actor, bool) {
    cell.RLock()
    defer cell.RUnlock()
    for obj := cell.objects.Front(); obj != nil; obj = obj.Next() {
        if v, ok := obj.Value.(Actor); ok {
            return v, ok
        }
    }
    return nil, false
}

// hasWolf checks whether a cell contains a Wolf object
func (cell *Cell) hasWolf() bool {
    cell.RLock()
    defer cell.RUnlock()
    for obj := cell.objects.Front(); obj != nil; obj = obj.Next() {
        if _, ok := obj.Value.(*Wolf); ok {
            return ok
        }
    }
    return false
}

// hasLadderTop checks whether a cell contains a LadderTop object
func (cell *Cell) hasLadderTop() bool {
    cell.RLock()
    defer cell.RUnlock()
    for obj := cell.objects.Front(); obj != nil; obj = obj.Next() {
        if _, ok := obj.Value.(*LadderTop); ok {
            return true
        }
    }
    return false
}

// hasLadderBottom checks whether a cell contains a LadderBottom object
func (cell *Cell) hasLadderBottom() bool {
    cell.RLock()
    defer cell.RUnlock()
    for obj := cell.objects.Front(); obj != nil; obj = obj.Next() {
        if _, ok := obj.Value.(*LadderBottom); ok {
            return true
        }
    }
    return false
}

// Raisable checks whether a cell contains any Raisable object, like Stair, Box, etc.
func (cell *Cell) hasRaisable() bool {
    cell.RLock()
    defer cell.RUnlock()
    for obj := cell.objects.Front(); obj != nil; obj = obj.Next() {
        if _, ok := obj.Value.(Raisable); ok {
            return ok
        }
    }
    return false
}

// hasRopeLine checks whether a cell contains a RopeLine object
func (cell *Cell) hasRopeLine() bool {
    cell.RLock()
    defer cell.RUnlock()
    for obj := cell.objects.Front(); obj != nil; obj = obj.Next() {
        if _, ok := obj.Value.(*RopeLine); ok {
            return true
        }
    }
    return false
}

// hasWaterfall checks whether a cell contains a Waterfall object
func (cell *Cell) hasWaterfall() bool {
    cell.RLock()
    defer cell.RUnlock()
    for obj := cell.objects.Front(); obj != nil; obj = obj.Next() {
        if _, ok := obj.Value.(*Waterfall); ok {
            return ok
        }
    }
    return false
}

// hasBeamChunk checks whether a cell contains a BeamChunk object
func (cell *Cell) hasBeamChunk() bool {
    cell.RLock()
    defer cell.RUnlock()
    for obj := cell.objects.Front(); obj != nil; obj = obj.Next() {
        if _, ok := obj.Value.(*BeamChunk); ok {
            return ok
        }
    }
    return false
}

// hasFood checks whether a cell contains any Food object, like Apple, Pear, Nut, Mushroom, etc.
func (cell *Cell) hasFood() (Food, bool) {
    cell.RLock()
    defer cell.RUnlock()
    for obj := cell.objects.Front(); obj != nil; obj = obj.Next() {
        if v, ok := obj.Value.(Food); ok {
            return v, true
        }
    }
    return nil, false
}

// Thing checks whether a cell contains any Thing object (UmbrellaThing, BeamThing, etc.)
func (cell *Cell) hasThing() (Thing, bool) {
    cell.RLock()
    defer cell.RUnlock()
    for obj := cell.objects.Front(); obj != nil; obj = obj.Next() {
        if v, ok := obj.Value.(Thing); ok {
            return v, true
        }
    }
    return nil, false
}

// hasUmbrella checks whether a cell contains an Umbrella object
func (cell *Cell) hasUmbrella() bool {
    cell.RLock()
    defer cell.RUnlock()
    for obj := cell.objects.Front(); obj != nil; obj = obj.Next() {
        if _, ok := obj.Value.(*Umbrella); ok {
            return ok
        }
    }
    return false
}

// hasMine checks whether a cell contains a Mine object
func (cell *Cell) hasMine() (*Mine, bool) {
    cell.RLock()
    defer cell.RUnlock()
    for obj := cell.objects.Front(); obj != nil; obj = obj.Next() {
        if v, ok := obj.Value.(*Mine); ok {
            return v, ok
        }
    }
    return nil, false
}

// hasBeam checks whether a cell contains a Beam object
func (cell *Cell) hasBeam() (*Beam, bool) {
    cell.RLock()
    defer cell.RUnlock()
    for obj := cell.objects.Front(); obj != nil; obj = obj.Next() {
        if v, ok := obj.Value.(*Beam); ok {
            return v, ok
        }
    }
    return nil, false
}

// hasAntidote checks whether a cell contains an Antidote object
func (cell *Cell) hasAntidote() (*Antidote, bool) {
    cell.RLock()
    defer cell.RUnlock()
    for obj := cell.objects.Front(); obj != nil; obj = obj.Next() {
        if v, ok := obj.Value.(*Antidote); ok {
            return v, ok
        }
    }
    return nil, false
}

// hasFlashbang checks whether a cell contains a Flashbang object
func (cell *Cell) hasFlashbang() (*Flashbang, bool) {
    cell.RLock()
    defer cell.RUnlock()
    for obj := cell.objects.Front(); obj != nil; obj = obj.Next() {
        if v, ok := obj.Value.(*Flashbang); ok {
            return v, ok
        }
    }
    return nil, false
}

// hasTeleport checks whether a cell contains a Teleport object
func (cell *Cell) hasTeleport() (*Teleport, bool) {
    cell.RLock()
    defer cell.RUnlock()
    for obj := cell.objects.Front(); obj != nil; obj = obj.Next() {
        if v, ok := obj.Value.(*Teleport); ok {
            return v, ok
        }
    }
    return nil, false
}

// hasDetector checks whether a cell contains a Detector object
func (cell *Cell) hasDetector() (*Detector, bool) {
    cell.RLock()
    defer cell.RUnlock()
    for obj := cell.objects.Front(); obj != nil; obj = obj.Next() {
        if v, ok := obj.Value.(*Detector); ok {
            return v, ok
        }
    }
    return nil, false
}
