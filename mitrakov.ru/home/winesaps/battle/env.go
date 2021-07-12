package battle

import "log"
import "time"
import "sync"
import "math/rand"
import . "mitrakov.ru/home/winesaps/sid"   // nolint
import . "mitrakov.ru/home/winesaps/utils" // nolint

// Environment is "everything reasonable" aside from Actors and AI, e.g. wolves.
// Please note that this a global singleton object for all battles and all rounds!
type Environment struct {
    sync.RWMutex
    fields map[Sid]*Field
    stop   chan bool
}

// unit of discretisation for the Environment (which means that e.g. all wolves will make 1 step in 250 ms)
const tickDelay = 250 * time.Millisecond
// distance (in cells) which wolves will check for an actor wearing a Voodoo Mask
const voodooDistance = 3

// newEnvironment creates a new instance of Environment. Please do not create the Environment directly.
// "battleManager" - reference to IBattleManager
func newEnvironment(battleManager IBattleManager) *Environment {
    Assert(battleManager)
    
    env := &Environment{fields: make(map[Sid]*Field)}
    env.stop = RunDaemon("env", tickDelay, func() {
        env.RLock()
        for sid, field := range env.fields {
            env.RUnlock()
            box := NewMailBox()
            wolves := field.getWolves()
            var err *Error
            for j := wolves.Front(); j != nil; j = j.Next() {
                if wolf, ok := j.Value.(*Wolf); ok {
                    // see note#6
                    if possiblyAlreadyNewField, ok := env.getField(sid); ok && field == possiblyAlreadyNewField {
                        err = stepWolf(sid, field, wolf, battleManager, box)
                    }
                }
            }
            battleManager.onEvent(box, err)
            env.RLock()
        }
        env.RUnlock()
        
    })
    return env
}

// adds a battlefield reference (expressed by "field") to the Environment, using "sid" as a key.
// after usage DO NOT FORGET to call removeField() in order to release the reference to the "field"
func (env* Environment) addField(sid Sid, field *Field) {
    Assert(field, env.fields)
    
    env.Lock()
    env.fields[sid] = field
    env.Unlock()
    log.Println("Field added; sid", sid)
}

// getField returns a battlefield for a given Session ID (returns FALSE if battlefield not found)
func (env* Environment) getField(sid Sid) (*Field, bool) {
    Assert(env.fields)
    env.RLock()
    field, ok := env.fields[sid]
    env.RUnlock()
    return field, ok 
}

// removeField removes a reference to a battlefield by given Session IDs
func (env* Environment) removeField(sid1, sid2 Sid) {
    Assert(env.fields)
    
    env.Lock()
    delete(env.fields, sid1)
    delete(env.fields, sid2)
    env.Unlock()
    log.Println("Field removed for SIDs:", sid1, sid2)
}

// close shuts Environment down and releases all seized resources
func (env* Environment) close() {
    Assert(env.stop)
    env.stop <- true
}

// getActiveEnvs returns the total count of battlefields retained by the Environment (should be 0 at rest)
func (env* Environment) getActiveEnvs() uint {
    env.RLock()    
    defer env.RUnlock()
    return uint(len(env.fields))
}

// stepWolf a single step processor for a given "wolf" of a given battlefield (specified by "field")
// "sid" - Session ID of any of participants
// "field" - battlefield
// "wolf" - instance of wolf
// "battleManager" - reference to an IBattleManager
// "box" - MailBox to accumulate messages
// nolint: gocyclo
func stepWolf(sid Sid, field *Field, wolf *Wolf, battleManager IBattleManager, box *MailBox) *Error {
    Assert(wolf, field)
    cell := wolf.getCell()
    Assert(cell)
    
    var err0, err1 *Error
    if wolfAfraid(field, cell, wolf.curDir > 0, voodooDistance) {
        wolf.curDir *= -1
        err0 = battleManager.effectChanged(sid, effAfraid, true, wolf.getNum(), box)
    }
    if cell.hasLadderTop() && rand.Intn(2) == 0 && !wolf.justUsedLadder {
        _, err1 = field.move(sid, wolf, int(cell.xy)+Width, box)
        wolf.justUsedLadder = true
    } else if cell.hasLadderBottom() && rand.Intn(2) == 0 && !wolf.justUsedLadder {
        _, err1 = field.move(sid, wolf, int(cell.xy)-Width, box)
        wolf.justUsedLadder = true
    } else if cell.hasRopeLine() && rand.Intn(2) == 0 {
        _, err1 = field.move(sid, wolf, int(cell.xy)-Width, box)
    } else {
        var success bool
        success, err1 = field.move(sid, wolf, int(cell.xy)+wolf.curDir, box)
        if !success {
            wolf.curDir *= -1
        }
        wolf.justUsedLadder = false
    }
    return NewErrs(err0, err1)
}

// wolfAfraid checks whether a wolf on the battlefield "field" is afraid of an actor wearing a Voodoo Mask, starting
// from cell "cell", directed to right or left (specified by "toRight" parameter) up to "n" steps of distance.
// Note that "n" is a recursive parameter, so it shouldn't be to large
func wolfAfraid(field *Field, cell *Cell, toRight bool, n int) bool {
    nextCell := field.getCellByDirection(cell, toRight)
    if nextCell != nil {
        if actor, ok := nextCell.hasActor(); ok {
            return actor.hasSwagga(VoodooMask)
        } else if n > 0 {
            return wolfAfraid(field, nextCell, toRight, n-1)
        }
    }
    return false
}

// note#6 (@mitrakov, 2017-07-20): very strange and unobvious bug! If we've got 2 (or more) wolves on a battlefield,
// and for example, the 1-st wolf has eaten an actor (suppose it had got only 1 live), then "field.move()" unwinds
// ALL THE NECESSARY actions, including "PlayerWound", "NewRoundInfo", "NewState" and so on. In this situation the 2-nd
// wolf will step on the OLD battlefield instead of a new one. This step cause sending INCORRECT "StateChanged" package
// to both clients and their NEW state will be broken
//
