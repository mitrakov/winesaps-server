package battle

// Swagga is a non-consumable ability: an actor just can put it on and take advantage of it all the time
type Swagga int

// @mitrakov (2017-04-18): don't use ALL_CAPS const naming (gometalinter, stackoverflow.com/questions/22688906)

// Swagga constants
const (
    Snorkel Swagga = 1 + iota
    ClimbingShoes
    SouthWester
    VoodooMask
    SapperShoes
    Sunglasses
)

// Skill is a consumable ability: an actor can convert it into a thing (the Skill will disappear until next round)
type Skill interface {
    getID() byte
    isUsed() bool
    apply(objNum byte) Thing
}


// Miner can produce a MineThing (0x21)
type Miner struct {
    used bool
}

// getID return ID of the Skill
func (*Miner) getID() byte {
    return 0x21
}

// isUsed returns whether this Skill is already used or still available
func (miner *Miner) isUsed() bool {
    return miner.used
}

// apply converts the Skill into a Thing
// "objNum" - unique object number on the battlefield
func (miner *Miner) apply(objNum byte) Thing {
    if miner.used {
        return nil
    }
    miner.used = true
    return newMineThing(objNum, nil)
}

// Builder can produce a BeamThing (0x22)
type Builder struct {
    used bool
}

// getID return ID of the Skill
func (*Builder) getID() byte {
    return 0x22
}

// isUsed returns whether this Skill is already used or still available
func (bld *Builder) isUsed() bool {
    return bld.used
}

// apply converts the Skill into a Thing
// "objNum" - unique object number on the battlefield
func (bld *Builder) apply(objNum byte) Thing {
    if bld.used {
        return nil
    }
    bld.used = true
    return newBeamThing(objNum, nil)
}

// Shaman can produce a AntidoteThing (0x23)
type Shaman struct {
    used bool
}

// getID return ID of the Skill
func (*Shaman) getID() byte {
    return 0x23
}

// isUsed returns whether this Skill is already used or still available
func (shaman *Shaman) isUsed() bool {
    return shaman.used
}

// apply converts the Skill into a Thing
// "objNum" - unique object number on the battlefield
func (shaman *Shaman) apply(objNum byte) Thing {
    if shaman.used {
        return nil
    }
    shaman.used = true
    return newAntidoteThing(objNum, nil)
}

// Grenadier can produce a FlashbangThing (0x24)
type Grenadier struct {
    used bool
}

// getID return ID of the Skill
func (*Grenadier) getID() byte {
    return 0x24
}

// isUsed returns whether this Skill is already used or still available
func (grenadier *Grenadier) isUsed() bool {
    return grenadier.used
}

// apply converts the Skill into a Thing
// "objNum" - unique object number on the battlefield
func (grenadier *Grenadier) apply(objNum byte) Thing {
    if grenadier.used {
        return nil
    }
    grenadier.used = true
    return newFlashbangThing(objNum, nil)
}

// TeleportMan can produce a TeleportThing (0x25)
type TeleportMan struct {
    used bool
}

// getID return ID of the Skill
func (*TeleportMan) getID() byte {
    return 0x25
}

// isUsed returns whether this Skill is already used or still available
func (man *TeleportMan) isUsed() bool {
    return man.used
}

// apply converts the Skill into a Thing
// "objNum" - unique object number on the battlefield
func (man *TeleportMan) apply(objNum byte) Thing {
    if man.used {
        return nil
    }
    man.used = true
    return newTeleportThing(objNum, nil)
}
