package battle

import "time"
import "runtime"
import . "mitrakov.ru/home/winesaps/sid"   // nolint
import . "mitrakov.ru/home/winesaps/utils" // nolint

// Round is a time-restricted single part of the battle (i.e. a Battle consists of N rounds, until one of Players
// reaches specific count of wins)
type Round struct {
    TryMutex
    number        byte
    foodTotal     byte
    battleManager IBattleManager
    player1       *Player
    player2       *Player
    field         *Field
    levelName     string
    stop          chan bool
}

// newRound creates a new instance of Round. Please do not create a Round directly.
// "aggressor" - aggressor's Session ID
// "defender" - defender's Session ID
// "char1" - aggressor's character (Rabbit, Squirrel, etc.)
// "char2" - defender's character (Rabbit, Squirrel, etc.)
// "number" - round number (zero based)
// "levelname" - level filename
// "skills1" - aggressor's skills
// "skills2" - defender's skills
// "swaggas1" - aggressor's swaggas
// "swaggas2" - defender's swaggas
// "batMgr" - reference to IBattleManager
func newRound(aggressor, defender Sid, char1, char2, number byte, levelname string, skills1,
    skills2 []Skill, swaggas1, swaggas2 []Swagga, batMgr IBattleManager) (*Round, *Error) {
    Assert(batMgr)
    
    env := batMgr.getEnvironment()
    Assert(env)

    field, err := newField(batMgr, levelname)
    if err == nil {
        env.addField(aggressor, field)
        actor1, ok1 := field.getActor1()
        actor2, ok2 := field.getActor2()
        if ok1 && ok2 {
            actor1.setCharacter(char1)
            actor2.setCharacter(char2)
            for _, s := range swaggas1 {
                actor1.addSwagga(s)
            }
            for _, s := range swaggas2 {
                actor2.addSwagga(s)
            }
            field.replaceFavouriteFood(actor1, actor2)
            player1 := newPlayer(aggressor, actor1, skills1)
            player2 := newPlayer(defender, actor2, skills2)
            food := field.getFoodCount()
            res := &Round{TryMutex{}, number, food, batMgr, player1, player2, field, levelname, nil}
            batMgr.IncRoundRefs()
            runtime.SetFinalizer(res, func(*Round) {batMgr.DecRoundRefs()})
            res.stop = RunTask("round_timer", time.Duration(field.timeSec) * time.Second, func() {
                res.timeOut()
            })
            return res, nil
        }
        return nil, NewErr(new(Round), 112, "No actors found")
    }
    return nil, err
}

// getPlayerBySid returns one of 2 Players, involved in the Round, by a given Session ID (or NULL, if no Player
// corresponds to the given Session ID)
func (round *Round) getPlayerBySid(sid Sid) (*Player, bool) {
    Assert(round.player1, round.player2)

    if round.player1.sid == sid {
        return round.player1, true
    }
    if round.player2.sid == sid {
        return round.player2, true
    }
    return nil, false
}

// getPlayerByActor returns one of 2 Players, involved in the Round, by a given Actor (or NULL, if no Player
// corresponds to the given Actor)
func (round *Round) getPlayerByActor(actor Actor) (*Player, bool) {
    Assert(actor, round.player1, round.player2)

    if round.player1.actor == actor {
        return round.player1, true
    }
    if round.player2.actor == actor {
        return round.player2, true
    }
    return nil, false
}

// checkRoundFinished checks whether the Round finished by analysing current food count
// "box" - MailBox to accumulate messages
func (round *Round) checkRoundFinished(box *MailBox) (err *Error) {
    Assert(round.player1, round.player2, round.battleManager)

    round.OnlyOne(func() {
        if round.player1.score > round.foodTotal/2 {
            err = round.battleManager.roundFinished(round.player1.sid, box)
        } else if round.player2.score > round.foodTotal/2 {
            err = round.battleManager.roundFinished(round.player2.sid, box)
        } else if round.field.getFoodCount() == 0 {
            err = round.finishRoundForced(box)
        }
    })
    return
}

// timeOut is a callback for Round timeout
func (round *Round) timeOut() {
    Assert(round.battleManager)

    round.OnlyOne(func() {
        box := NewMailBox()
        err := round.finishRoundForced(box)
        round.battleManager.onEvent(box, err)
    })
    return
}

// finishRoundForced forcefully shuts the Round down without checking current food count.
// By default the winner is determined by the following algorithm:
// 1) check score (who has more - wins the Round)
// 2) if scores are equal, check lives (who has more - wins the Round)
// 3) if scores and lives are equal, the defender wins
// "box" - MailBox to accumulate messages
func (round *Round) finishRoundForced(box *MailBox) (err *Error) {
    Assert(round.player1, round.player2, round.battleManager)
    if round.player1.score > round.player2.score {
        return round.battleManager.roundFinished(round.player1.sid, box)
    } else if round.player2.score > round.player1.score {
        return round.battleManager.roundFinished(round.player2.sid, box)
    } else { // draw: let's check who has more lives
        if (round.player1.lives > round.player2.lives) {
            return round.battleManager.roundFinished(round.player1.sid, box)
        }
        // note: if draw and lives are equals let's suppose the defender (player2) wins
        return round.battleManager.roundFinished(round.player2.sid, box)
    }
}

// move performs a single move of an Actor, expressed by a given Session ID, towards a given direction.
// "box" - MailBox to accumulate messages
// nolint: gocyclo
func (round *Round) move(sid Sid, direction moveDirection, box *MailBox) *Error {
    if player, ok := round.getPlayerBySid(sid); ok {
        // get components
        field := round.field
        actor := player.actor
        Assert(field, actor)
        cell := actor.getCell()
        Assert(cell)
        
        // calculate delta
        delta := 0
        switch (direction) {
            case moveLeftDown:
                delta = TernaryInt(field.isMoveDownPossible(cell), Width, -1)
            case moveLeft:
                delta = -1
            case moveLeftUp:
                delta = TernaryInt(field.isMoveUpPossible(cell), -Width, -1)
            case moveRightDown:
                delta = TernaryInt(field.isMoveDownPossible(cell), Width, 1)
            case moveRight:
                delta = 1
            case moveRightUp:
                delta = TernaryInt(field.isMoveUpPossible(cell), -Width, 1)
            default:
        }
        
        // set actor's direction (left/right)
        if delta == 1 {
            actor.setDirectionRight(true)
        }
        if delta == -1 {
            actor.setDirectionRight(false)
        }
        
        // go!
        ok, err := field.move(sid, actor, int(actor.getCell().xy)+delta, box)
        
        // if movement ok => inc actor's internal step counter to get effects working
        if ok {
            actor.addStep()
        }
        return err
    }
    return NewErr(round, 113, "Player not found; sid=%d", sid)
}

// wound reaves 1 live from a Player with a given Session ID. If the Player has extra lives, it returns TRUE, and the
// Actor will be respawned at the Entry Point. If the lives counter = 0, it returns FALSE, which means the Round is
// over, and the Player lost the Round
func (round *Round) wound(sid Sid) (isAlive bool, err *Error) {
    if player, ok := round.getPlayerBySid(sid); ok {
        player.lives--
        return player.lives > 0, nil
    }
    return false, NewErr(round, 114, "Player not found (sid=%d)", sid)
}

// restore respawns the Actor, expressed by a given Session ID, at the Entry Point
// "box" - MailBox to accumulate messages
func (round *Round) restore(sid Sid, box *MailBox) *Error {
    Assert(round.field)
    
    if player, ok := round.getPlayerBySid(sid); ok {
        actor := player.actor
        Assert(actor)
        if entry, ok := round.field.getEntryByActor(actor); ok {
            Assert(actor.getCell(), entry.getCell())
            return round.field.relocate(sid, actor.getCell(), entry.getCell(), actor, true, box)
        }
        return NewErr(round, 115, "Entry not found (sid=%d)", sid)
    }
    return NewErr(round, 116, "Player not found (sid=%d)", sid)
}

// setThingToPlayer assigns a given Thing to a Player with a given Session ID. If the Player had the other Thing before,
// it will be dropped on the same cell.
// "box" - MailBox to accumulate messages
func (round *Round) setThingToPlayer(sid Sid, thing Thing, box *MailBox) *Error {
    Assert(round.field)

    if player, ok := round.getPlayerBySid(sid); ok {
        oldThing := player.setThing(thing)
        if oldThing != nil {
            return round.field.dropThing(sid, player.actor, oldThing, box)
        }
        return nil
    }
    return NewErr(round, 117, "Player not found (sid=%d)", sid)
}

// useThing utilizes a Thing that was carried by a Player with a given Session ID.
// If the Player had no Things, the method does nothing (and doesn't throw any errors as well).
// Please note this action "consumes" a Thing.
// "box" - MailBox to accumulate messages
func (round *Round) useThing(sid Sid, box *MailBox) *Error {
    if player, ok := round.getPlayerBySid(sid); ok {
        thing := player.setThing(nil)
        if thing != nil {
            return round.field.useThing(sid, player.actor, thing, box)
        }
        return nil
    }
    return NewErr(round, 118, "Player not found (sid=%d)", sid)
}

// useSkill utilizes Skill, expressed by given skill ID, for a Player with a given Session ID.
// As a result, a new Thing will appear and automatically be assigned to the Player.
// Please note this action "consumes" the Skill.
// "box" - MailBox to accumulate messages
func (round *Round) useSkill(sid Sid, skillID byte, box *MailBox) (Thing, *Error) {
    Assert(round.field, round.battleManager)

    if player, ok := round.getPlayerBySid(sid); ok {
        if skill := player.getSkill(skillID); skill != nil {
            thing := skill.apply(round.field.getNextNum())
            if thing != nil {
                err1 := round.battleManager.objAppended(sid, thing, box)
                err2 := round.setThingToPlayer(sid, thing, box)
                return thing, NewErrs(err1, err2)
            }
            return nil, nil // no error here: skill may cast nothing
        }
        return nil, NewErr(round, 119, "Skill not found (sid=%d, skillId = %d)", sid, skillID)
    }
    return nil, NewErr(round, 120, "Player not found (sid=%d)", sid)
}

// getCurrentAbilities returns a current list of Ability IDs for a Player with a given Session ID.
// Note that this list may be shorter as the Player consumes the Abilities during the Round
// Throws error, if Player not found
func (round *Round) getCurrentAbilities(sid Sid) (abilities []byte, err *Error) {
    if player, ok := round.getPlayerBySid(sid); ok {
        actor := player.actor
        Assert(actor)
        swaggas := actor.getSwaggas()
        for s := swaggas.Front(); s != nil; s = s.Next() {
            if v, ok := s.Value.(Swagga); ok {
                abilities = append(abilities, byte(v))
            }
        }
        for s := player.skills.Front(); s != nil; s = s.Next() {
            if v, ok := s.Value.(Skill); ok {
                if !v.isUsed() {
                    abilities = append(abilities, v.getID())
                }
            }
        }
    } else {
        err = NewErr(round, 121, "Player not found (sid=%d)", sid)
    }
    return
}
