package battle

import "sync"
import "time"
import "strings"
import "sync/atomic"
import "path/filepath"
import . "mitrakov.ru/home/winesaps/sid"   // nolint
import . "mitrakov.ru/home/winesaps/utils" // nolint
import "mitrakov.ru/home/winesaps/filereader"

// IBattleManager is an interface for all battle management operations
type IBattleManager interface {
    SetController(controller IController)
    Attack(aggressor, defender Sid, aggressorName, defenderName string) (*MailBox, *Error)
    Accept(aggressor, defender Sid, char1, char2 byte, aggAbilities, defAbilities []byte, levelnames []string, 
        wins byte, quickBattle, removeCall bool) (*MailBox, *Error)
    Reject(aggressor, defender Sid, cowardName string) (*MailBox, *Error)
    CancelCall(aggressor Sid) (*MailBox, *Error)
    Move(sid Sid, direction byte) (*MailBox, *Error)
    UseThing(Sid) (*MailBox, *Error)
    UseSkill(sid Sid, skillID byte) (*MailBox, *Error)
    GiveUp(Sid) (enemySid Sid, box *MailBox, e *Error)
    GetActorXy(sid Sid, actor1 bool) (byte, *Error)
    WolfExists(sid Sid, xy byte) (bool, *Error)
    GetFieldRaw(sid Sid) (raw []byte, e *Error)
    GetMovablesDump(sid Sid) (dump []byte, e *Error)
    GetBattlesCount() uint
    GetBattlesCountTotal() uint32
    IncBattleRefs()
    DecBattleRefs()
    GetBattleRefs() (uint32, uint32)
    IncRoundRefs()
    DecRoundRefs()
    GetRoundRefs() (uint32, uint32)
    IncFieldRefs()
    DecFieldRefs()
    GetFieldRefs() (uint32, uint32)
    GetEnvironmentSize() uint
    Close()
    onEvent(box *MailBox, err *Error)
    getFileReader() *filereader.FileReader
    getEnvironment() *Environment
    getBattle(sid Sid) (*Battle, bool)
    objChanged(sid Sid, objNum, objID, xy byte, reset bool, box *MailBox) *Error
    foodEaten(sid Sid, box *MailBox) *Error
    thingTaken(sid Sid, thing Thing, box *MailBox) *Error
    objAppended(sid Sid, object IObject, box *MailBox) *Error
    roundFinished(winnerSid Sid, box *MailBox) *Error
    eatenByWolf(sid Sid, actor Actor, box *MailBox) *Error
    hurt(sid Sid, cause hurtCause, box *MailBox) *Error
    effectChanged(sid Sid, id effectT, added bool, objNumber byte, box *MailBox) *Error
    setEffectOnEnemy(sid Sid, id effectT, box *MailBox) *Error
}

// IPacker interface comprises of methods for converting some events into a bytearray
type IPacker interface {
    PackCall(aggressor Sid, aggressorName string) []byte
    PackStopCallRejected(cowardName string) []byte
    PackStopCallMissed(aggressorName string) []byte
    PackStopCallExpired(defenderName string) []byte
    PackFullState(state []byte) []byte
    PackRoundInfo(sid, aggressor Sid, roundNum, timeSec, char1, char2, myLives, enemyLives byte, fname string) []byte
    PackAbilityList(abilities []byte) []byte
    PackStateChanged(objNum, objID, xy byte, reset bool) []byte
    PackScoreChanged(score1, score2 byte) []byte
    PackEffectChanged(effID byte, added bool, objNumber byte) []byte
    PackWound(sid, woundSid Sid, cause, myLives, enemyLives byte) []byte
    PackThingTaken(sid, ownerSid Sid, thingID byte) []byte
    PackObjectAppended(id, objNum, xy byte) []byte
    PackRoundFinished(sid, winnerSid Sid, totalScore1, totalScore2 byte) []byte
    PackGameOver(sid, winnerSid Sid, totalScore1, totalScore2 byte, reward uint32) []byte
}

// IController contains methods for IBattleManager callbacks
type IController interface {
    Event(*MailBox, *Error)
    GameOver(winnerSid, loserSid Sid, score1, score2 byte, quickBattle bool, box *MailBox) (reward uint32, err *Error)
}

// callT is a structure that is temporarily created while an Aggressor is trying to challenge a Defender.
// by default the life cycle of "callT" is about 10 sec.
type callT struct {
    aggressor     Sid
    defender      Sid
    aggressorName string
    defenderName  string
    calls         int
}

// BatManager is an implementation of IBattleManager.
// Both interface and implementation were placed in the same src intentionally!
// This component is independent.
type BatManager struct /* implements IBattleManager */ {
    sync.RWMutex
    fileReader   *filereader.FileReader
    environment  *Environment
    packer       IPacker
    controller   IController
    battles      map[Sid]*Battle
    activeCalls  map[Sid]*callT
    stop         chan bool
    battlesCount   uint32
    battleRefsUp   uint32
    battleRefsDown uint32
    roundRefsUp    uint32
    roundRefsDown  uint32
    fieldRefsUp    uint32
    fieldRefsDown  uint32
}

// @mitrakov (2017-04-18): don't use ALL_CAPS const naming (gometalinter, stackoverflow.com/questions/22688906)

// period is an interval during which a BattleManager checks up incoming calls
const period = time.Second
// maxCalls is a maximum count of calls, so that total awaiting time for an Aggressor is (maxCalls * period)
const maxCalls = 20

// hurt cause
type hurtCause byte

// hurt causes constants
const (
    poisoned hurtCause = iota
    sunk
    soaked
    devoured
    exploded
)

// move direction
type moveDirection byte

// move direction constants
const (
    moveLeftDown moveDirection = iota
    moveLeft
    moveLeftUp
    moveRightDown
    moveRight
    moveRightUp
)

// NewBattleManager creates a new instance of BattleManager and returns a reference to IBattleManager interface.
// Please do not create a BatManager directly.
// "reader" - file reader
// "packer" - reference to IPacker implementation
// "ctrl" - reference to IController implementation
func NewBattleManager(reader *filereader.FileReader, packer IPacker, ctrl IController) IBattleManager {
    Assert(reader)

    battleMgr := new(BatManager)
    battleMgr.fileReader = reader
    battleMgr.environment = newEnvironment(battleMgr)
    battleMgr.packer = packer
    battleMgr.controller = ctrl
    battleMgr.battles = make(map[Sid]*Battle)
    battleMgr.activeCalls = make(map[Sid]*callT)
    battleMgr.stop = RunDaemon("battle", period, func() {
        battleMgr.Lock() // here we use full loop WLock() to protect algorithm (not only activeCalls map)
        for k, v := range battleMgr.activeCalls {
            if v.calls >= maxCalls {
                delete(battleMgr.activeCalls, k) // it is safe: stackoverflow.com/questions/23229975
                box := NewMailBox()
                box.Put(v.defender, packer.PackStopCallMissed(v.aggressorName))
                box.Put(v.aggressor, packer.PackStopCallExpired(v.defenderName))
                battleMgr.controller.Event(box, nil)
            } else {
                v.calls++
            }
        }
        battleMgr.Unlock()
    })

    return battleMgr
}

// onEvent is a handler for all future events of the battle system, like timeouts, effect fade outs and so on
// "box" - MailBox with all accumulated messages to be send to a client
// "err" - current error (ideally should be NULL)
func (battleMgr *BatManager) onEvent(box *MailBox, err *Error) {
    Assert(battleMgr.controller, box)
    battleMgr.controller.Event(box, err)
}

// getFileReader returns a FileReader assosiated with this IBattleManager
func (battleMgr *BatManager) getFileReader() *filereader.FileReader {
    return battleMgr.fileReader
}

// getEnvironment returns an Environment assosiated with this IBattleManager
func (battleMgr *BatManager) getEnvironment() *Environment {
    return battleMgr.environment
}

// getBattle returns a battle by a given Session ID of one of its participants.
// You can specify any Session ID of either an Aggressor or a Defender
func (battleMgr *BatManager) getBattle(sid Sid) (battle *Battle, ok bool) {
    Assert(battleMgr.battles)

    battleMgr.RLock()
    battle, ok = battleMgr.battles[sid]
    battleMgr.RUnlock()
    return
}

// SetController assigns a non-NULL IController for this IBattleManager
func (battleMgr *BatManager) SetController(controller IController) {
    Assert(controller)
    battleMgr.controller = controller
}

// Attack initiates the attack. The battle won't be started until a Defender accepts the challenge.
// "aggressor" - aggressor Session ID
// "defender" - defender Session ID
// "aggressorName" - aggressor name
// "defenderName" - defender name
func (battleMgr *BatManager) Attack(aggressor, defender Sid, aggressorName, defenderName string) (*MailBox, *Error) {
    Assert(battleMgr.activeCalls)
    box := NewMailBox()

    ok, err := battleMgr.areAvailable(aggressor, defender)
    if ok {
        battleMgr.Lock()
        battleMgr.activeCalls[aggressor] = &callT{aggressor, defender, aggressorName, defenderName, 0}
        battleMgr.Unlock()
            
        box.Put(defender, battleMgr.packer.PackCall(aggressor, aggressorName))
        return box, nil
    }
    return box, err
}

// Accept confirms the Defender's will to fight against the Aggressor that had initiated the attack by calling "Attack"
// method some time ago. If the Aggressor cancelled its intention, or timeout happened, this method will do nothing.
// "aggressor" - Aggressor Session ID
// "defender" - Defender Session ID
// "char1" - character of Aggressor (Rabbit, Hedgehog, Squirrel or Cat)
// "char2" - character of Defender (Rabbit, Hedgehog, Squirrel or Cat)
// "aggAbilities" - skills and swaggas of Aggressor
// "defAbilities" - skills and swaggas of Defender
// "levelnames" - array of level names (ensure that the length is enough! E.g. for wins = 3 this array.size should be 5)
// "wins" - count of round wins to win the battle
// "quickBattle" - TRUE for quick battles; this argument does not affect the underlying battle system
// "removeCall" - TRUE to remove call (for PvP battles)
func (battleMgr *BatManager) Accept(aggressor, defender Sid, char1, char2 byte, aggAbilities, defAbilities []byte, 
    levelnames []string, wins byte, quickBattle, removeCall bool) (box *MailBox, err *Error) {
    Assert(battleMgr.controller)
    box = NewMailBox()

    if removeCall {
        err = battleMgr.deleteCall(aggressor, defender)
    }
    if err == nil {
        var ok bool
        if ok, err = battleMgr.areAvailable(aggressor, defender); ok {
            var battle *Battle
            battle, err = newBattle(aggressor, defender, char1, char2, levelnames, wins, quickBattle, 
                aggAbilities, defAbilities, battleMgr)
            if err == nil {
                battleMgr.Lock()
                battleMgr.battles[aggressor] = battle
                battleMgr.battles[defender] = battle
                battleMgr.Unlock()
                atomic.AddUint32(&battleMgr.battlesCount, 1)
                err = battleMgr.startRound(battle.getRound(), box)
            }
        }
    }
    return
}

// Reject discards the Defender's will to fight against the Aggressor that had initiated the attack by calling "Attack"
// method some time ago. Aggressor will give the corresponding notification.
// "aggressor" - Aggressor Session ID
// "defender" - Defender Session ID
// "cowardName" - name of a user, who rejected the battle
func (battleMgr *BatManager) Reject(aggressor, defender Sid, cowardName string) (box *MailBox, err *Error) {
    box = NewMailBox()
    if err = battleMgr.deleteCall(aggressor, defender); err == nil {
        box.Put(aggressor, battleMgr.packer.PackStopCallRejected(cowardName))
    }
    return
}

// CancelCall discards the call by Aggressor initiative (he just changed his mind to fight).
// "aggressor" - Aggressor Session ID
func (battleMgr *BatManager) CancelCall(aggressor Sid) (*MailBox, *Error) {
    Assert(battleMgr.activeCalls)
    box := NewMailBox()

    battleMgr.Lock() // here we use full WLock() to protect algorithm (not only activeCalls map)
    defer battleMgr.Unlock()
    if item, ok := battleMgr.activeCalls[aggressor]; ok {
        delete(battleMgr.activeCalls, aggressor)
        box.Put(item.defender, battleMgr.packer.PackStopCallMissed(item.aggressorName))
        return box, nil
    }
    return box, NewErr(battleMgr, 53, "No call found (sid=%d)", aggressor)
}

// Move performs a single "Move" action for a participant with a Session ID in a given direction
func (battleMgr *BatManager) Move(sid Sid, direction byte) (*MailBox, *Error) {
    box := NewMailBox()
    if battle, ok := battleMgr.getBattle(sid); ok {
        round := battle.getRound()
        Assert(round)
        return box, round.move(sid, moveDirection(direction), box)
    }
    return box, NewErr(battleMgr, 77, "Battle not found: sid=%d", sid)
}

// UseThing performs "Use thing" action for a participant with a given Session ID.
// If an actor hasn't got any things, this method will do nothing
func (battleMgr *BatManager) UseThing(sid Sid) (*MailBox, *Error) {
    box := NewMailBox()
    if battle, ok := battleMgr.getBattle(sid); ok {
        round := battle.getRound()
        Assert(round)
        err := round.useThing(sid, box)
        if err == nil {
            box.Put(battle.detractor1.sid, battleMgr.packer.PackThingTaken(battle.detractor1.sid, sid, 0))
            box.Put(battle.detractor2.sid, battleMgr.packer.PackThingTaken(battle.detractor2.sid, sid, 0))
        }
        return box, err
    }
    return box, NewErr(battleMgr, 55, "Battle not found: sid=%d", sid)
}

// UseSkill performs "Use skill" action for a participant with a given Session ID by a given "skillID".
// This method may produce "Skill not found" error.
func (battleMgr *BatManager) UseSkill(sid Sid, skillID byte) (*MailBox, *Error) {
    box := NewMailBox()
    if battle, ok := battleMgr.getBattle(sid); ok {
        round := battle.getRound()
        Assert(round)
        thing, err := round.useSkill(sid, skillID, box)
        if err == nil {
            if thing != nil { // thing may be NULL (in case skill produced nothing)
                thingID := thing.getID()
                box.Put(battle.detractor1.sid, battleMgr.packer.PackThingTaken(battle.detractor1.sid, sid, thingID))
                box.Put(battle.detractor2.sid, battleMgr.packer.PackThingTaken(battle.detractor2.sid, sid, thingID))
            }
            var abilities []byte
            abilities, err = round.getCurrentAbilities(sid)
            if err == nil {
                box.Put(sid, battleMgr.packer.PackAbilityList(abilities))
            }
        }
        return box, err
    }
    return box, NewErr(battleMgr, 57, "Battle not found: sid=%d", sid)
}

// GiveUp is a method to surrender. It means that the battle will be over, and the opponent will become the winner
// "sid" - coward's Session ID
func (battleMgr *BatManager) GiveUp(sid Sid) (Sid, *MailBox, *Error) {
    box := NewMailBox()
    if battle, ok := battleMgr.getBattle(sid); ok {
        if enemy, ok := battle.getEnemy(sid); ok {
            enemy.score = battle.wins - 1
            return enemy.sid, box, battleMgr.roundFinished(enemy.sid, box)
        }
        return 0, box, NewErr(battleMgr, 58, "Enemy not found: sid=%d", sid)
    }
    return 0, box, NewErr(battleMgr, 59, "Battle not found: sid=%d", sid)
}

// GetActorXy returns a position of an actor on the battlefield.
// Specify "actor1" = TRUE for Actor1, and "actor1" = FALSE for Actor2.
// Session ID is needed only to lookup the battle and may be a SID of any participants.
func (battleMgr *BatManager) GetActorXy(sid Sid, actor1 bool) (byte, *Error) {
    field, err := battleMgr.getCurrentField(sid)
    if err == nil {
        var actor Actor
        var ok bool
        if actor1 {
            actor, ok = field.getActor1()
        } else {
            actor, ok = field.getActor2()
        }
        if ok {
            return actor.getCell().xy, nil
        }
        return 0xFF, NewErr(battleMgr, 54, "Actor not found: sid=%d", sid)
    }
    return 0xFF, err
}

// WolfExists checks whether a wolf is located in a given position.
// Session ID is needed only to lookup the battle and may be a SID of any participants.
func (battleMgr *BatManager) WolfExists(sid Sid, xy byte) (bool, *Error) {
    field, err := battleMgr.getCurrentField(sid)
    if err == nil {
        var cell *Cell
        if cell, err = field.getCell(xy); err == nil {
            return cell.hasWolf(), nil
        }
        return false, err
    }
    return false, err
}

// GetFieldRaw returns a battlefield as a raw bytearray.
// Session ID is needed only to lookup the battle and may be a SID of any participants.
// since 1.3.0
func (battleMgr *BatManager) GetFieldRaw(sid Sid) (raw []byte, e *Error) {
    field, err := battleMgr.getCurrentField(sid)
    if err == nil {
        return field.raw, nil
    }
    return []byte{}, err
}

// GetMovablesDump returns a binary dump of all Movable objects on the battlefield: actors, wolves, food, things, etc.
// Session ID is needed only to lookup the battle and may be a SID of any participants.
// since 1.3.0
func (battleMgr *BatManager) GetMovablesDump(sid Sid) (dump []byte, e *Error) {
    field, err := battleMgr.getCurrentField(sid)
    if err == nil {
        return field.dumpMovables(), nil
    }
    return []byte{}, err
}

// GetBattlesCount returns current count of battles
func (battleMgr *BatManager) GetBattlesCount() uint {
    battleMgr.RLock()
    defer battleMgr.RUnlock()
    return uint(len(battleMgr.battles)/2) // len(map) is thread-safe but Data Race may occur
}

// GetBattlesCountTotal returns total count of battles since App startup
func (battleMgr *BatManager) GetBattlesCountTotal() uint32 {
    return atomic.LoadUint32(&battleMgr.battlesCount)
}

// IncBattleRefs increases a counter of battles allocations
func (battleMgr *BatManager) IncBattleRefs() {
    atomic.AddUint32(&battleMgr.battleRefsUp, 1)
}

// DecBattleRefs increases a counter of battles utilizations
func (battleMgr *BatManager) DecBattleRefs() {
    atomic.AddUint32(&battleMgr.battleRefsDown, 1)
}

// GetBattleRefs returns count of battles allocations and utilizations. Ideally those 2 numbers should be equal
func (battleMgr *BatManager) GetBattleRefs() (uint32, uint32) {
    return atomic.LoadUint32(&battleMgr.battleRefsUp), atomic.LoadUint32(&battleMgr.battleRefsDown)
}

// IncRoundRefs increases a counter of rounds allocations
func (battleMgr *BatManager) IncRoundRefs() {
    atomic.AddUint32(&battleMgr.roundRefsUp, 1)
}

// DecRoundRefs increases a counter of rounds utilizations
func (battleMgr *BatManager) DecRoundRefs() {
    atomic.AddUint32(&battleMgr.roundRefsDown, 1)
}

// GetRoundRefs returns count of rounds allocations and utilizations. Ideally those 2 numbers should be equal
func (battleMgr *BatManager) GetRoundRefs() (uint32, uint32) {
    return atomic.LoadUint32(&battleMgr.roundRefsUp), atomic.LoadUint32(&battleMgr.roundRefsDown)
}

// IncFieldRefs increases a counter of battlefields allocations
func (battleMgr *BatManager) IncFieldRefs() {
    atomic.AddUint32(&battleMgr.fieldRefsUp, 1)
}

// DecFieldRefs increases a counter of battlefields utilizations
func (battleMgr *BatManager) DecFieldRefs() {
    atomic.AddUint32(&battleMgr.fieldRefsDown, 1)
}

// GetFieldRefs returns count of battlefields allocations and utilizations. Ideally those 2 numbers should be equal
func (battleMgr *BatManager) GetFieldRefs() (uint32, uint32) {
    return atomic.LoadUint32(&battleMgr.fieldRefsUp), atomic.LoadUint32(&battleMgr.fieldRefsDown)
}

// GetEnvironmentSize returns count of battlefields, referenced by the Environment
func (battleMgr *BatManager) GetEnvironmentSize() uint {
    battleMgr.RLock()
    defer battleMgr.RUnlock()
    return battleMgr.environment.getActiveEnvs()
}

// Close shuts IBattleManager down and releases all seized resources
func (battleMgr *BatManager) Close() {
    Assert(battleMgr.stop, battleMgr.environment)
    battleMgr.stop <- true
    battleMgr.environment.close()
}

// =============================
// ===    PRIVATE METHODS    ===
// =============================

// objChanged is a callback on "Object Changed" event, e.g. an actor performed a single move.
// Session ID is needed only to lookup the battle and may be a SID of any participants.
// "sid" - Session ID of any of participants
// "objNum" - global object number on the battlefield
// "objID" - object ID
// "xy" - new location (0-255)
// "reset" - TRUE, if location has been changed instantaneously (wounded, teleportation, etc.), FALSE otherwise
// "box" - MailBox to accumulate messages
func (battleMgr *BatManager) objChanged(sid Sid, objNum, objID, xy byte, reset bool, box *MailBox) *Error {
    // @mitrakov (2017-07-20): objID is added only for additional checking on client-side
    if battle, ok := battleMgr.getBattle(sid); ok {
        box.Put(battle.detractor1.sid, battleMgr.packer.PackStateChanged(objNum, objID, xy, reset))
        box.Put(battle.detractor2.sid, battleMgr.packer.PackStateChanged(objNum, objID, xy, reset))
        return nil
    }
    return NewErr(battleMgr, 60, "Battle not found: sid=%d", sid)
}

// objAppended is a callback on "Object Appended" event, e.g. an actor has just used an UmbrellaThing and a new Umbrella
// has been produced.
// Session ID is needed only to lookup the battle and may be a SID of any participants.
// "sid" - Session ID of any of participants
// "object" - new object appended
// "box" - MailBox to accumulate messages
func (battleMgr *BatManager) objAppended(sid Sid, object IObject, box *MailBox) *Error {
    Assert(object)
    if battle, ok := battleMgr.getBattle(sid); ok {
        xy := byte(0xFF)             // if an actor possesses the object, then its xy = 0xFF
        if object.getCell() != nil { // else the object is located on the field
            xy = object.getCell().xy // don't use 'Ternary' here (it causes NullPointerException)
        }
        // send
        box.Put(battle.detractor1.sid, battleMgr.packer.PackObjectAppended(object.getID(), object.getNum(), xy))
        box.Put(battle.detractor2.sid, battleMgr.packer.PackObjectAppended(object.getID(), object.getNum(), xy))
        return nil
    }
    return NewErr(battleMgr, 65, "Battle not found: sid=%d", sid)
}

// foodEaten is a callback on "Food eaten by actor" event.
// "sid" - Session ID of actor who ate food
// "box" - MailBox to accumulate messages
func (battleMgr *BatManager) foodEaten(sid Sid, box *MailBox) *Error {
    if battle, ok := battleMgr.getBattle(sid); ok {
        round := battle.getRound()
        Assert(round)
        if player, ok := round.getPlayerBySid(sid); ok {
            player.score++
            box.Put(battle.detractor1.sid, battleMgr.packer.PackScoreChanged(round.player1.score, round.player2.score))
            box.Put(battle.detractor2.sid, battleMgr.packer.PackScoreChanged(round.player1.score, round.player2.score))
            return round.checkRoundFinished(box)
        }
        return NewErr(battleMgr, 61, "Player not found (sid=%d)", sid)
    }
    return NewErr(battleMgr, 62, "Battle not found: sid=%d", sid)
}

// thingTaken is a callback on "Thing taken by actor" event
// "sid" - Session ID of actor who took a thing
// "thing" - thing taken
// "box" - MailBox to accumulate messages
func (battleMgr *BatManager) thingTaken(sid Sid, thing Thing, box *MailBox) *Error {
    if battle, ok := battleMgr.getBattle(sid); ok {
        round := battle.getRound()
        Assert(round)
        err := round.setThingToPlayer(sid, thing, box)
        if err == nil {
            box.Put(battle.detractor1.sid, battleMgr.packer.PackThingTaken(battle.detractor1.sid, sid, thing.getID()))
            box.Put(battle.detractor2.sid, battleMgr.packer.PackThingTaken(battle.detractor2.sid, sid, thing.getID()))
        }
        return err
    }
    return NewErr(battleMgr, 64, "Battle not found: sid=%d", sid)
}

// roundFinished is a callback on "Round finished" event.
// "winnerSid" - Session ID of the round winner
// "box" - MailBox to accumulate messages
func (battleMgr *BatManager) roundFinished(winnerSid Sid, box *MailBox) (err *Error) {
    Assert(battleMgr.controller)
    if battle, ok := battleMgr.getBattle(winnerSid); ok {
        var gameOver bool
        gameOver, err = battle.checkBattle(winnerSid)
        if err == nil {
            detractor1, detractor2 := battle.detractor1, battle.detractor2
            Assert(detractor1, detractor2)
            sid1, sid2 := detractor1.sid, detractor2.sid
            score1, score2 := detractor1.score, detractor2.score
            box.Put(sid1, battleMgr.packer.PackRoundFinished(sid1, winnerSid, score1, score2))
            box.Put(sid2, battleMgr.packer.PackRoundFinished(sid2, winnerSid, score1, score2))
            if !gameOver {
                var round *Round
                round, err = battle.nextRound()
                if err == nil {
                    err = battleMgr.startRound(round, box)
                }
            } else {
                var reward uint32
                battle.stop()
                battleMgr.Lock()
                delete(battleMgr.battles, sid1)
                delete(battleMgr.battles, sid2)
                battleMgr.Unlock()
                if loser, ok := battle.getEnemy(winnerSid); ok {
                    reward, err = battleMgr.controller.GameOver(winnerSid, loser.sid, score1, score2, battle.quick, box)
                } else {
                    err = NewErr(battleMgr, 67, "Loser not found: sid=%d", winnerSid)
                }
                box.Put(sid1, battleMgr.packer.PackGameOver(sid1, winnerSid, score1, score2, reward))
                box.Put(sid2, battleMgr.packer.PackGameOver(sid2, winnerSid, score1, score2, reward))
            }
        }
        return
    }
    return NewErr(battleMgr, 68, "Battle not found: sid=%d", winnerSid)
}

// hurt is a callback on "Actor hurt" event.
// IMPORTANT: for "Eaten by wolf" event please use eatenByWolf() method instead!
// "sid" - Session ID of a wounded actor
// "cause" - cause of hurt
// "box" - MailBox to accumulate messages
func (battleMgr *BatManager) hurt(sid Sid, cause hurtCause, box *MailBox) *Error {
    if battle, ok := battleMgr.getBattle(sid); ok {
        round := battle.getRound()
        Assert(round, round.player1, round.player2, battle.detractor1, battle.detractor2)
        isAlive, err := round.wound(sid)
        if err == nil {
            sid1, sid2 := battle.detractor1.sid, battle.detractor2.sid
            lives1, lives2 := round.player1.lives, round.player2.lives
            box.Put(sid1, battleMgr.packer.PackWound(sid1, sid, byte(cause), lives1, lives2))
            box.Put(sid2, battleMgr.packer.PackWound(sid2, sid, byte(cause), lives2, lives1))
            if isAlive {
                round.restore(sid, box)
            } else {
                if enemy, ok := battle.getEnemy(sid); ok {
                    return battleMgr.roundFinished(enemy.sid, box)
                }
                err = NewErr(battleMgr, 71, "Enemy not found: sid=%d", sid)
            }
        }
        return err
    }
    return NewErr(battleMgr, 72, "Battle not found: sid=%d", sid)
}

// eatenByWolf is a callback on "Actor has been eaten by wolf" event.
// IMPORTANT: use this method for described event rather than hurt() method!
// The reason is that here "sid" belongs to aggressor (only to find a battle), but ACTUAL sid is
// calculated via getPlayerByActor() method
// "sid" - Session ID of any of participants
// "actor" - wounded actor
// "box" - MailBox to accumulate messages
func (battleMgr *BatManager) eatenByWolf(sid Sid, actor Actor, box *MailBox) *Error {
    if battle, ok := battleMgr.getBattle(sid); ok {
        round := battle.getRound()
        Assert(round)
        if player, ok := round.getPlayerByActor(actor); ok {
            return battleMgr.hurt(player.sid, devoured, box)
        }
        return NewErr(battleMgr, 69, "Player not found")
    }
    return NewErr(battleMgr, 70, "Battle not found: sid=%d", sid)
}

// effectChanged is a callback on "Effect changed" event
// Session ID is needed only to lookup the battle and may be a SID of any participants.
// "sid" - Session ID of any of participants
// "id" - effect ID
// "added" - whether the effect added (TRUE) or dismissed (FALSE)
// "objNumber" - global object number on the battlefield
// "box" - MailBox to accumulate messages
func (battleMgr *BatManager) effectChanged(sid Sid, id effectT, added bool, objNumber byte, box *MailBox) *Error {
    if battle, ok := battleMgr.getBattle(sid); ok {
        box.Put(battle.detractor1.sid, battleMgr.packer.PackEffectChanged(byte(id), added, objNumber))
        box.Put(battle.detractor2.sid, battleMgr.packer.PackEffectChanged(byte(id), added, objNumber))
        return nil
    }
    return NewErr(battleMgr, 73, "Battle not found: sid=%d", sid)
}

// setEffectOnEnemy imposes effect (specified by effect ID) on a enemy of a player, specified by a given Session ID
// "box" - MailBox to accumulate messages
func (battleMgr *BatManager) setEffectOnEnemy(sid Sid, id effectT, box *MailBox) *Error {
    if battle, ok := battleMgr.getBattle(sid); ok {
        round := battle.getRound()
        Assert(round)
        if enemy, ok := battle.getEnemy(sid); ok {
            if player, ok := round.getPlayerBySid(enemy.sid); ok {
                actor := player.actor
                Assert(actor)
                if !actor.hasSwagga(Sunglasses) {
                    actor.setEffect(id, 1, nil) // only formality; in fact it's an empty effect on server-side
                    return battleMgr.effectChanged(sid, id, true, actor.getNum(), box)
                }
                return nil
            }
            return NewErr(battleMgr, 74, "Player not found: sid=%d", sid)
        }
        return NewErr(battleMgr, 75, "Enemy not found: sid=%d", sid)
    }
    return NewErr(battleMgr, 76, "Battle not found: sid=%d", sid)
}

// ===============================
// === NON-INTERFACE FUNCTIONS ===
// ===============================

// areAvailable checks whether two players, specified by their Session IDs, are vacant for a battle.
// If one of them is already in the battle, returns FALSE
// "aggressor" - Aggressor Session ID
// "defender" - Defender Session ID
func (battleMgr *BatManager) areAvailable(aggressor, defender Sid) (bool, *Error) {
    if aggressor == defender {
        return false, NewErr(battleMgr, 50, "You cannot attack yourself (%d)", aggressor)
    } else if _, ok := battleMgr.getBattle(aggressor); ok {                                // it is also possible!
        return false, NewErr(battleMgr, 51, "Aggressor is busy (%d)", aggressor)
    } else if _, ok := battleMgr.getBattle(defender); ok {
        return false, NewErr(battleMgr, 52, "Defender is busy (%d)", defender)
    }
    return true, nil
}

// startRound begins a new round (it may be 1-st round of the battle, or just the next round after another)
// "round" - round to start
// "box" - MailBox to accumulate messages
func (battleMgr *BatManager) startRound(round *Round, box *MailBox) *Error {
    Assert(round, round.field, round.player1, round.player2, round.player1.actor, round.player2.actor)

    base := round.field.raw
    sid1, sid2 := round.player1.sid, round.player2.sid
    char1, char2 := round.player1.actor.getCharacter(), round.player2.actor.getCharacter()
    lives1, lives2 := round.player1.lives, round.player2.lives
    abilities1, err1 := round.getCurrentAbilities(sid1)
    abilities2, err2 := round.getCurrentAbilities(sid2)
    t := round.field.timeSec
    fname := strings.TrimSuffix(round.levelName, filepath.Ext(round.levelName))
    if err1 == nil && err2 == nil {
        box.Put(sid1, battleMgr.packer.PackRoundInfo(sid1, sid1, round.number, t, char1, char2, lives1, lives2, fname))
        box.Put(sid2, battleMgr.packer.PackRoundInfo(sid2, sid1, round.number, t, char1, char2, lives2, lives1, fname))
        box.Put(sid1, battleMgr.packer.PackFullState(base))
        box.Put(sid2, battleMgr.packer.PackFullState(base))
        box.Put(sid1, battleMgr.packer.PackAbilityList(abilities1))
        box.Put(sid2, battleMgr.packer.PackAbilityList(abilities2))
    }
    return NewErrs(err1, err2)
}

// deleteCall removes a call (Aggressor -> Defender) from the active calls queue.
// Method will return error if there has been no thitherto registered calls by Aggressor
// "aggressor" - Aggressor Session ID
// "defender" - Defender Session ID
func (battleMgr *BatManager) deleteCall(aggressor, defender Sid) *Error {
    Assert(battleMgr.activeCalls)

    battleMgr.Lock() // here we use full WLock() to protect algorithm (not only activeCalls map)
    defer battleMgr.Unlock()
    if item, ok := battleMgr.activeCalls[aggressor]; ok {
        if item.defender == defender {
            delete(battleMgr.activeCalls, aggressor)
            return nil
        }
        return NewErr(battleMgr, 78, "Incorrect defender sid (%d != %d)", item.defender, defender)
    }
    return NewErr(battleMgr, 79, "Fake Accept/Reject detected (Sid1 = %d, Sid2 = %d)", aggressor, defender)
}

// getCurrentField returns a reference to a current battlefield.
// Session ID is needed only to lookup the battle and may be a SID of any participants.
func (battleMgr *BatManager) getCurrentField(sid Sid) (*Field, *Error) {
    if battle, ok := battleMgr.getBattle(sid); ok {
        round := battle.curRound
        Assert(round)
        return round.field, nil
    }
    return nil, NewErr(battleMgr, 56, "Battle not found: sid=%d", sid)
}
