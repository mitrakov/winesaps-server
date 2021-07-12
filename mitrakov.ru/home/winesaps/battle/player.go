package battle

import "container/list"
import "mitrakov.ru/home/winesaps/sid"

// Player is a participant of a battle scoped to a single round (new round = new instance of Player).
// There can be exactly 2 instances of Player per Round
type Player struct {
    sid      sid.Sid
    score    byte
    lives    byte
    actor    Actor
    thing    Thing
    skills   list.List // List[Skills]
}

// newPlayer creates a new instance of Player. Please do not create a Player directly.
// "sid" - player's Session ID
// "actor" - player's actor (an "Actor" corresponds to a "Player" as 1:1)
// "skills" - player's skills
func newPlayer(sid sid.Sid, actor Actor, skills []Skill) *Player {
    list := list.List{}
    for _, skill := range skills {
        list.PushBack(skill)
    }
    return &Player{sid, 0, 2, actor, nil, list}
}

// setThing puts a new Thing into a pocket, replacing the existing Thing out (if any). The old Thing will be dropped
// on the same cell. Please note that Players have only EXACTLY 1 slot for Things!
func (player *Player) setThing(thing Thing) (oldThing Thing) {
    oldThing = player.thing
    player.thing = thing
    return
}

// getSkill returns Skill by ID, if it is present in the Player's skill list. If not => returns NULL
func (player *Player) getSkill(id byte) Skill {
    for e := player.skills.Front(); e != nil; e = e.Next() {
        if skill, ok := e.Value.(Skill); ok {
            if skill.getID() == id {
                return skill
            }
        }
    }
    return nil
}
