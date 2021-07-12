package battle

import "mitrakov.ru/home/winesaps/sid"

// Detractor is an "abstract global" participant of a battle; it is still neither a player, not an actor.
// There can be exactly 2 instances per Battle
type Detractor struct {
    sid       sid.Sid
    character byte
    score     byte
    abilities []byte
}

// newDetractor creates a new instance of Detractor. Please do not create a Detractor directly.
// "sid" - detractor's Session ID
// "character" - detractor's character (Rabbit, Hedgehog, Squirrel or Cat)
// "abilities" - detractor's skills and swaggas
func newDetractor(sid sid.Sid, character byte, abilities []byte) *Detractor {
    return &Detractor{sid, character, 0, abilities}
}
