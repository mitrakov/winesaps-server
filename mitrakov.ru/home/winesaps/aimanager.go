// Copyright 2017-2018 Artem Mitrakov. All rights reserved.
package main

import "log"
import "sync"
import "time"
import "math/rand"
import . "mitrakov.ru/home/winesaps/ai"       // nolint
import . "mitrakov.ru/home/winesaps/sid"      // nolint
import . "mitrakov.ru/home/winesaps/utils"    // nolint
import . "mitrakov.ru/home/winesaps/battle"   // nolint

// An aiInfoT is used to describe a single AI player
type aiInfoT struct {
    sync.RWMutex
    ai          *Ai
    myChar      byte
    myNumber    byte
    totalScore1 byte
    totalScore2 byte
    objects     map[byte]byte
}

// An AiManager is a single entity to control the lifecycle of all AI players.
// This component is "dependent"
type AiManager struct {
    sync.RWMutex
    controller    *Controller
    battleManager IBattleManager
    ais           map[Sid]*aiInfoT
    stop          chan bool
}

// tickDelay controls how often AI players act, e.g. 200 msec means that they perform 5 steps per second
const tickDelay = 200 * time.Millisecond
// minHandicap is min count of steps which an AI will stay idly by, in case if it is leading
const minHandicap = 15
// maxRandHandicap is a max random parameter to be added to minHandicap
const maxRandHandicap = 10

// NewAiManager creates a new instance of AiManager. Please do not create an AiManager directly.
// "controller" - event handler
// "battleMgr" - reference to an IBattleManager
func NewAiManager(controller *Controller, battleMgr IBattleManager) *AiManager {
    mgr := &AiManager{controller: controller, battleManager: battleMgr, ais: make(map[Sid]*aiInfoT)}
    mgr.stop = RunDaemon("ai", tickDelay, func() {
        mgr.RLock()
        for sid, info := range mgr.ais {
            mgr.RUnlock()
            idxFrom, idxTo, useThing, err := info.ai.Step()
            if err == nil {
                if useThing {
                    mgr.controller.Event(mgr.battleManager.UseThing(sid))
                }
                if idxFrom != idxTo {
                    mgr.move(sid, idxFrom, idxTo)
                }
            } else {
                log.Println("ERROR:", err)
            }
            mgr.RLock()
        }
        mgr.RUnlock()
    })

    return mgr
}

// setController sets a NON-NULL controller for AiManager
func (mgr *AiManager) setController(controller *Controller) {
    Assert(controller)
    mgr.controller = controller
}

// addNewAi creates a new AI player and maps a given sid to that AI
func (mgr *AiManager) addNewAi(sid Sid, character byte) {
    Assert(mgr.ais, mgr.battleManager)

    f := func() byte {
        xy, err := mgr.battleManager.GetActorXy(sid, false)
        Check(err)
        return xy
    }
    g := func(xy byte) bool {
        res, err := mgr.battleManager.WolfExists(sid, xy)
        Check(err)
        return res
    }
    mgr.Lock()
    mgr.ais[sid] = &aiInfoT{ai: NewAi(f, g), myChar: character, objects: make(map[byte]byte)}
    mgr.Unlock()
}

// removeAi removes AI player by a given sid
func (mgr *AiManager) removeAi(sid Sid) {
    Assert(mgr.ais)
    mgr.Lock()
    delete(mgr.ais, sid)
    mgr.Unlock()
}

// handleEvent handles all events from a given box, addressed to an AI player with a given sid
// nolint: gocyclo
func (mgr *AiManager) handleEvent(sid Sid, box *MailBox) *MailBox {
    Assert(box)
    for _, msg := range box.Pick(sid) {
        if len(msg) > 0 {
            switch cmd(msg[0]) {
                case fullState:
                    if len(msg) > 1 {
                        mgr.setFullState(sid, msg[1:])
                    }
                case stateChanged:
                    if len(msg) > 4 {
                        mgr.stateChanged(sid, msg[1], msg[3], msg[4] == 1) // msg[2] (objID) not used
                    }
                case thingTaken:
                    if len(msg) > 2 {
                        mgr.setThing(sid, msg[1], msg[2])
                    }
                case effectChanged:
                    if len(msg) > 3 {
                        mgr.setEffect(sid, msg[1], msg[2] == 1, msg[3])
                    }
                case scoreChanged:
                    if len(msg) > 2 {
                        mgr.setScore(sid, msg[1], msg[2])
                    }
                case finished:
                    if len(msg) > 4 {
                        mgr.setTotalScore(sid, msg[3], msg[4])
                    }
            }
        }
    }
    box.Remove(sid) // no need to send addressed to AI messages by network
    return box
}

// getAiInfo is a thread-safe getter for "ais" map; please use this method instead of direct accessing "ais"
func (mgr *AiManager) getAiInfo(sid Sid) (info *aiInfoT, ok bool) {
    Assert(mgr.ais)
    mgr.RLock()
    info, ok = mgr.ais[sid]
    mgr.RUnlock()
    return
}

// close shuts AiManager down
func (mgr *AiManager) close() {
    Assert(mgr.stop)
    mgr.stop <- true
}

// move is a handler for MOVE command
func (mgr *AiManager) move(sid Sid, indexFrom, indexTo byte) {
    Assert(mgr.battleManager, mgr.controller)

    // calculate the direction
    dir := byte(0)
    delta := int(indexTo) - int(indexFrom)
    if delta == Width {
        dir = 0
    } else if delta == -Width {
        dir = 2
    } else if delta%Width == 1 {
        dir = 4
    } else if delta%Width == -1 || delta%Width == Width-1 {
        dir = 1
    } else {
        log.Println("ERROR: Incorrect AI delta! from = ", indexFrom, "; to = ", indexTo)
        return
    }
    
    // move!
    mgr.controller.Event(mgr.battleManager.Move(sid, dir))
}

// setFullState is a handler for FULL STATE command
// nolint: gocyclo
func (mgr *AiManager) setFullState(sid Sid, state []byte) {
    if aiInfo, ok := mgr.getAiInfo(sid); ok {
        graph := parseLevel(state, aiInfo.myChar)
        aiInfo.ai.Init(graph)
        curObjNum := byte(0)
        for i := byte(0); i < Width*Height; i++ {
            aiInfo.ai.SetResource(i, false)
            aiInfo.ai.SetTool(i, 0)
            obj := state[i] & 0x3F
            switch obj {
            case 0x05:
                curObjNum++
                aiInfo.Lock()
                aiInfo.myNumber = curObjNum
                aiInfo.Unlock()
            case 0x10:
                fallthrough
            case 0x11:
                fallthrough
            case 0x12:
                fallthrough
            case 0x13:
                fallthrough
            case 0x14:
                fallthrough
            case 0x15:
                curObjNum++
                aiInfo.Lock()
                aiInfo.objects[curObjNum] = i
                aiInfo.ai.SetResource(i, true)
                aiInfo.Unlock()
            case 0x20:
                fallthrough
            case 0x21:
                fallthrough
            case 0x22:
                fallthrough
            case 0x23:
                fallthrough
            case 0x24:
                fallthrough
            case 0x25:
                fallthrough
            case 0x26:
                fallthrough
            case 0x27:
                curObjNum++
                aiInfo.Lock()
                aiInfo.objects[curObjNum] = i
                aiInfo.ai.SetTool(i, obj)
                aiInfo.Unlock()
            case 0x04:
                fallthrough
            case 0x06:
                fallthrough
            case 0x0F:
                fallthrough
            case 0x16:
                fallthrough
            case 0x17:
                fallthrough
            case 0x28:
                fallthrough
            case 0x29:
                fallthrough
            case 0x2A:
                fallthrough
            case 0x2B:
                fallthrough
            case 0x2C:
                fallthrough
            case 0x2D:
                fallthrough
            case 0x2E:
                fallthrough
            case 0x2F:
                curObjNum++
            }
        }
        aiInfo.ai.SetPauseSteps(15) // 15 steps = 3 sec
    }
}

// stateChanged is a handler for STATE CHANGED command
func (mgr *AiManager) stateChanged(sid Sid, objNum, xy byte, reset bool) {
    if aiInfo, ok := mgr.getAiInfo(sid); ok {
        aiInfo.RLock()
        actorRestarted := objNum == aiInfo.myNumber && reset
        objRemoved := xy == 0xFF
        if actorRestarted {
            aiInfo.ai.Reset()
        } else if objRemoved {
            if oldXy, ok := aiInfo.objects[objNum]; ok {
                aiInfo.ai.SetResource(oldXy, false)
                aiInfo.ai.SetTool(oldXy, 0)
            }
        }
        aiInfo.RUnlock()
    }
}

// setThing is a handler for THING TAKEN command
func (mgr *AiManager) setThing(sid Sid, me byte, thing byte) {
    if me == 1 {
        if aiInfo, ok := mgr.getAiInfo(sid); ok {
            if thing == /*FlashBangThing*/ 0x24 { // use Flashbang immediately to stab in the back to a player!
                mgr.controller.Event(mgr.battleManager.UseThing(sid))
            } else {
                aiInfo.ai.SetCurTool(thing)
            }
        }
    }
}

// setEffect is a handler for EFFECT CHANGED command
func (mgr *AiManager) setEffect(sid Sid, effect byte, added bool, number byte) {
    if added && effect == /*effDazzle*/ 2 {
        if aiInfo, ok := mgr.getAiInfo(sid); ok && aiInfo.myNumber == number {
            aiInfo.ai.SetPauseSteps(15) // pause on 15 steps (3 sec)
        }
    }
}

// setScore is a handler for SCORE CHANGED command
func (mgr *AiManager) setScore(sid Sid, score1, score2 byte) {
    if aiInfo, ok := mgr.getAiInfo(sid); ok {
        if isHandicapNeeded(aiInfo, score1, score2) {
            steps := uint8(rand.Intn(maxRandHandicap)) + minHandicap // delay AI for several steps
            aiInfo.ai.SetDelayedPauseSteps(steps)
        }
    }
}

// setTotalScore is a handler for FINISHED command
func (mgr *AiManager) setTotalScore(sid Sid, score1, score2 byte) {
    if aiInfo, ok := mgr.getAiInfo(sid); ok {
        aiInfo.totalScore1, aiInfo.totalScore2 = score1, score2
    }
}

// parseLevel converts a level (expressed as bytearray) into a Graph data structure
// nolint: gocyclo
func parseLevel(data []byte, character byte) *Graph {
    Assert(data)
    
    res := new(Graph)
    // nodes
    for i:=0; i<Height; i++ {
        for j:=0; j<Width; j++ {
            idx := byte(i*Width+j)
            bottom := data[idx] >> 6
            obj := data[idx] & 0x3F
            if bottom > 0 || obj == 12 {                 // bottom non-empty (or rope exists)
                if obj != 1 && obj != 3 {                // object not block, not water
                    var danger byte
                    if bottom == 3 && obj != 15 {        // water without BeamChunk
                        danger = 34
                    } else if obj == 13 {                // waterfall
                        danger = 32
                    } else if isPoison(character, obj) { // poison
                        danger = 35
                    }
                    res.AddNode(idx, danger)
                }
            }
        }
    }
    // arcs
    for i:=0; i<Height; i++ {
        for j:=0; j<Width; j++ {
            idx := byte(i*Width+j)
            node := res.GetNode(idx)
            if node != nil {
                bottom := data[idx] >> 6
                obj := data[idx] & 0x3F
                // left arc
                if idx % Width != 0 {
                    bottomLeft := data[idx-1] >> 6
                    objLeft := data[idx-1] & 0x3F
                    if objLeft != 1 && objLeft != 3 {                    // not block, not water
                        fromBlockToDais := bottom == 1 && bottomLeft == 2
                        if !fromBlockToDais || obj == 11 {               // obj==11 is Stair
                            res.AddArc(idx, 0, idx-1)
                            for k:=1; node.GetNext(0) == nil && int(idx)+k*Width-1 < len(data); k++ { // k must be int!
                                res.AddArc(idx, 0, idx+byte(k)*Width-1)
                            }
                        }
                    }
                }
                // right arc
                if (idx+1) % Width != 0 {
                    bottomRight := data[idx+1] >> 6
                    objRight := data[idx+1] & 0x3F
                    if objRight != 1 && objRight != 3 {                     // right object not block, not water
                        fromBlockToDais := bottom == 1 && bottomRight == 2
                        if !fromBlockToDais || obj == 11 {                  // obj==11 is Stair
                            res.AddArc(idx, 1, idx+1)
                            for k:=1; node.GetNext(1) == nil && int(idx)+k*Width+1 < len(data); k++ { // k must be int!
                                res.AddArc(idx, 1, idx+byte(k)*Width+1)
                            }
                        }
                    }
                }
                // up arc
                if idx-Width >= 0 && (obj == 10 || obj == 12) { // LadderBottom or RopeLine
                    res.AddArc(idx, 2, idx-Width)
                }
                // down arc
                if int(idx+Width) < len(data) && obj == 9 {     // LadderTop
                    res.AddArc(idx, 3, idx+Width)
                }
            }
        }
    }
    return res
}

// isPoison checks whether given food (expressed by foodID) is poison for a given character
// nolint: gocyclo
func isPoison(character, foodID byte) bool {
    switch character {
        case Rabbit:
            return foodID == 0x12 || foodID == 0x14 || foodID == 0x15
        case Hedgehog:
            return foodID == 0x12 || foodID == 0x13 || foodID == 0x15
        case Squirrel:
            return foodID == 0x12 || foodID == 0x13 || foodID == 0x14
        case Cat:
            return foodID == 0x13 || foodID == 0x14 || foodID == 0x15
    }
    return false
}

// isHandicapNeeded checks whether an AI should "soft play" to a user according to the current score
func isHandicapNeeded(aiInfo *aiInfoT, score1, score2 byte) bool {
    /*comboScore1, comboScore2 := 100*aiInfo.totalScore1+score1, 100*aiInfo.totalScore2+score2
    return comboScore2 > comboScore1*/ return true // "true" is experimental since 1.3.11
}
