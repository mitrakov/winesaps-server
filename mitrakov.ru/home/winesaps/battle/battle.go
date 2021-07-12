package battle

import "sync"
import "runtime"
import . "mitrakov.ru/home/winesaps/sid"   // nolint
import . "mitrakov.ru/home/winesaps/utils" // nolint

// Battle is a struct that represents a single battle
type Battle struct {
    sync.RWMutex
    battleManager IBattleManager
    detractor1    *Detractor
    detractor2    *Detractor
    curRound      *Round
    wins          byte
    quick         bool
    levelnames    []string
}

// newBattle creates a new instance of Battle. Please do not create a Battle directly.
// "aggressor" - Aggressor Session ID
// "defender" - Defender Session ID
// "aggressorChar" - character of Aggressor (Rabbit, Hedgehog, Squirrel or Cat)
// "defenderChar" - character of Defender (Rabbit, Hedgehog, Squirrel or Cat)
// "levelnames" - array of level names (ensure that the length is enough! E.g. for wins = 3 this array.size should be 5)
// "wins" - count of round wins to win the battle
// "quickBattle" - TRUE for quick battles; this argument DOES NOT affect the battle and only propagated to callbacks
// "aggressorAbilities" - skills and swaggas of Aggressor
// "defenderAbilities" - skills and swaggas of Defender
// "battleMgr" - reference to IBattleManager
func newBattle(aggressor, defender Sid, aggressorChar, defenderChar byte, levelnames []string, wins byte,
    quickBattle bool, aggressorAbilities, defenderAbilities []byte, battleMgr IBattleManager) (*Battle, *Error) {

    if len(levelnames) > 0 {
        detractor1 := newDetractor(aggressor, aggressorChar, aggressorAbilities)
        detractor2 := newDetractor(defender, defenderChar, defenderAbilities)
        skills1, swaggas1 := extractAbilities(aggressorAbilities)
        skills2, swaggas2 := extractAbilities(defenderAbilities)
        round, err := newRound(aggressor, defender, aggressorChar, defenderChar, 0, levelnames[0], skills1,
            skills2, swaggas1, swaggas2, battleMgr)
        res := &Battle{sync.RWMutex{}, battleMgr, detractor1, detractor2, round, wins, quickBattle, levelnames}
        battleMgr.IncBattleRefs()
        runtime.SetFinalizer(res, func(*Battle) {battleMgr.DecBattleRefs()})
        return res, err
    }
    return nil, NewErr(&Battle{}, 110, "Empty levels list")
}

// extractAbilities takes a list of all abilities and splits them into 2 groups: skills and swaggas
// nolint: gocyclo
func extractAbilities(abilities []byte) (skills []Skill, swaggas []Swagga) {
    for _, ability := range abilities {
        switch Swagga(ability) {
        case Snorkel:
            fallthrough
        case ClimbingShoes:
            fallthrough
        case SouthWester:
            fallthrough
        case VoodooMask:
            fallthrough
        case SapperShoes:
            fallthrough
        case Sunglasses:
            swaggas = append(swaggas, Swagga(ability))
        }
        switch ability {
        case 0x21:
            skills = append(skills, new(Miner))
        case 0x22:
            skills = append(skills, new(Builder))
        case 0x23:
            skills = append(skills, new(Shaman))
        case 0x24:
            skills = append(skills, new(Grenadier))
        case 0x25:
            skills = append(skills, new(TeleportMan))
        }
    }
    return
}

// getRound returns current round
func (battle *Battle) getRound() *Round {
    battle.RLock()
    defer battle.RUnlock()
    return battle.curRound
}

// checkBattle increases score for a winner of current round and returns TRUE if the battle is finished
// "winnerSid" - winner Session ID
func (battle *Battle) checkBattle(winnerSid Sid) (finished bool, err *Error) {
    // increase score
    switch winnerSid {
    case battle.detractor1.sid:
        battle.detractor1.score++
    case battle.detractor2.sid:
        battle.detractor2.score++
    default:
        err = NewErr(battle, 110, "Incorrect winner sid %d", winnerSid)
    }
    if battle.detractor1.score >= battle.wins || battle.detractor2.score >= battle.wins {
        finished = true
    }
    return
}

// nextRound starts the next round of the battle (and returns a reference to this new round)
func (battle *Battle) nextRound() (round *Round, err *Error) {
    battle.stop()

    // copy parameters from the previous round
    oldRound := battle.getRound()
    number := byte(0)
    if oldRound != nil {
        number = oldRound.number + 1
    }
    if number < byte(len(battle.levelnames)) {
        levelname := battle.levelnames[number]
        // get parameters from within battle
        detractor1 := battle.detractor1
        detractor2 := battle.detractor2
        skills1, swaggas1 := extractAbilities(detractor1.abilities) // see note below
        skills2, swaggas2 := extractAbilities(detractor2.abilities) // see note below
        // create a new round
        round, err = newRound(detractor1.sid, detractor2.sid, detractor1.character, detractor2.character, number,
            levelname, skills1, skills2, swaggas1, swaggas2, battle.battleManager)
        if err == nil {
            battle.Lock()
            battle.curRound = round
            battle.Unlock()
        }
    } else {
        err = NewErr(battle, 111, "Incorrect levels length")
    }

    return
    // note: firstly I desided to keep skills/swaggas inside a battle (and not to extract the abilities each round),
    // but skills are reference type, therefore they are marked "used" after using, and they appeared to be "used" in
    // the next rounds; there are 2 ways: to reset them or to recreate them by extracting from the abilities again;
    // eventually I've chosen the second way.
}

// getEnemy returns an opponent of a given detractor (expressed by its Session ID)
func (battle *Battle) getEnemy(mySid Sid) (*Detractor, bool) {
    Assert(battle.detractor1, battle.detractor2)

    if battle.detractor1.sid == mySid {
        return battle.detractor2, true
    }
    if battle.detractor2.sid == mySid {
        return battle.detractor1, true
    }
    return nil, false
}

// stop shuts the battle down and releases all the seized resources
func (battle *Battle) stop() {
    round := battle.getRound()
    Assert(round)
    round.stop <- true
    
    Assert(battle.battleManager, battle.detractor1, battle.detractor2)
    env := battle.battleManager.getEnvironment()
    Assert(env)
    env.removeField(battle.detractor1.sid, battle.detractor2.sid)
}
