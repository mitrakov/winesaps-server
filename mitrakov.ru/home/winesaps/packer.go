package main

import . "mitrakov.ru/home/winesaps/sid"   // nolint
import . "mitrakov.ru/home/winesaps/utils" // nolint

// Packer is a component that allows to convert calls to a bytearray message (sending push messages to clients)
// This component is "dependent"
type Packer byte /*implements battle.IPacker, user.IPacker*/

// ============================================
// === battle.IPacker method implementations ===
// ============================================

// PackCall packs the message for "CALL" command (7)
// "aggressor" - aggressor Session ID
// "aggressorName" - agressor name
func (Packer) PackCall(aggressor Sid, aggressorName string) []byte {
    return append([]byte{byte(call), HighSid(aggressor), LowSid(aggressor)}, aggressorName...)
}

// PackStopCallRejected packs the message for "STOP CALL" command (10) with reason = "REJECTED" (0)
// "cowardName" - name of a user who's rejected the invitation
func (Packer) PackStopCallRejected(cowardName string) []byte {
    return append([]byte{byte(stopCall), rejected}, cowardName...)
}

// PackStopCallMissed packs the message for "STOP CALL" command (10) with reason = "MISSED" (1)
// "aggressorName" - name of a user who's wanted to invite us to a battle
func (Packer) PackStopCallMissed(aggressorName string) []byte {
    return append([]byte{byte(stopCall), missed}, aggressorName...)
}

// PackStopCallExpired packs the message for "STOP CALL" command (10) with reason = "TIMER EXPIRED" (2)
// "defenderName" - name of a user who's not responded to us in time
func (Packer) PackStopCallExpired(defenderName string) []byte {
    return append([]byte{byte(stopCall), timerExpired}, defenderName...)
}

// PackFullState packs the message for "FULL STATE" command (16)
// "state" - full state expressed as a byte array
func (Packer) PackFullState(state []byte) []byte {
    return append([]byte{byte(fullState)}, state...)
}

// PackRoundInfo packs the message for "ROUND INFO" command (17)
// "sid" - client's Session ID
// "aggressor" - aggressor's Session ID (will be the same as "sid", if that user is the aggressor)
// "num" - round number (zero based)
// "t" - round duration (in seconds)
// "char1" - character of Aggressor
// "char2" - character of Defender
// "myLives" - my lives count
// "enemyLives" - the enemy's lives count
// "fname" - name of the battle field
func (Packer) PackRoundInfo(sid, aggressor Sid, num, t, char1, char2, myLives, enemyLives byte, fname string) []byte {
    meAggressor := Ternary(sid == aggressor, 1, 0)
    return append([]byte{byte(roundInfo), num, t, meAggressor, char1, char2, myLives, enemyLives}, fname...)
}

// PackAbilityList packs the message for "ABILITY LIST" command (18)
// "abilities" - abilities of the user expressed as a byte array
func (Packer) PackAbilityList(abilities []byte) []byte {
    return append([]byte{byte(abilityList), byte(len(abilities))}, abilities...)
}

// PackStateChanged packs the message for "STATE CHANGED" command (23)
// "objNum" - global object number on the battle field
// "objID" - object ID
// "xy" - new location (0-255)
// "reset" - TRUE, if location has been changed instantaneously (wounded, teleportation, etc.), FALSE otherwise
func (Packer) PackStateChanged(objNum, objID, xy byte, reset bool) []byte {
    return []byte{byte(stateChanged), objNum, objID, xy, Ternary(reset, 1, 0)}
}

// PackScoreChanged packs the message for "SCORE CHANGED" command (24)
// "score1" - score of Aggressor
// "score2" - score of Defender
func (Packer) PackScoreChanged(score1, score2 byte) []byte {
    return []byte{byte(scoreChanged), score1, score2}
}

// PackEffectChanged packs the message for "EFFECT CHANGED" command (25)
// "effID" - effect ID
// "added" - TRUE if added, and FALSE if taken off
// "objNumber" - global object number on the battle field
func (Packer) PackEffectChanged(effID byte, added bool, objNumber byte) []byte {
    return []byte{byte(effectChanged), effID, Ternary(added, 1, 0), objNumber}
}

// PackWound packs the message for "PLAYER WOUNDED" command (26)
// "sid" - client's Session ID
// "woundSid" - Session ID of a user who's been wounded (will be the same as "sid", if that user has been wonded)
// "cause" - wound cause
// "myLives" - my lives left
// "enemyLives" - the enemy's lives left
func (Packer) PackWound(sid, woundSid Sid, cause, myLives, enemyLives byte) []byte {
    return []byte{byte(playerWounded), Ternary(sid == woundSid, 1, 0), cause, myLives, enemyLives}
}

// PackThingTaken packs the message for "THING TAKEN" command (27)
// "sid" - client's Session ID
// "ownerSid" - Session ID of a user who's taken a thing (will be the same as "sid", if that user has taken the thing)
// "thingID" - thing ID
func (Packer) PackThingTaken(sid, ownerSid Sid, thingID byte) []byte {
    return []byte{byte(thingTaken), Ternary(sid == ownerSid, 1, 0), thingID}
}

// PackObjectAppended packs the message for "OBJECT APPENDED" command (28)
// "id" - object ID
// "objNum" - global object number on the battle field
// "xy" - location of the new object
func (Packer) PackObjectAppended(id, objNum, xy byte) []byte {
    return []byte{byte(objectAppended), id, objNum, xy}
}

// PackRoundFinished packs the message for "FINISHED" command (29) with the parameter "GAME OVER" = 0
// "sid" - client's Session ID
// "winnerSid" - Session ID of a winner (will be the same as "sid", if that user is the winner)
// "totalScore1" - total score of Aggressor
// "totalScore2" - total score of Defender
func (Packer) PackRoundFinished(sid, winnerSid Sid, totalScore1, totalScore2 byte) []byte {
    return []byte{byte(finished), 0, Ternary(sid == winnerSid, 1, 0), totalScore1, totalScore2}
}

// PackGameOver packs the message for "FINISHED" command (29) with the parameter "GAME OVER" = 1
// "sid" - client's Session ID
// "winnerSid" - Session ID of a winner (will be the same as "sid", if that user is the winner)
// "totalScore1" - total score of Aggressor
// "totalScore2" - total score of Defender
// "reward" - reward for the winner, in gems
func (Packer) PackGameOver(sid, winnerSid Sid, totalScore1, totalScore2 byte, reward uint32) []byte {
    if sid == winnerSid {
        a, b, c, d := byte(reward>>24), byte(reward>>16), byte(reward>>8), byte(reward)
        return []byte{byte(finished), 1, 1, totalScore1, totalScore2, a, b, c, d}
    }
    return []byte{byte(finished), 1, 0, totalScore1, totalScore2}
}

// =========================================
// === user.IPacker method implementations ===
// =========================================

// PackUserInfo packs the message for "USER INFO" command (4)
// "info" - user's information, expressed as a byte array
func (Packer) PackUserInfo(info []byte) []byte {
    return append([]byte{byte(userInfo), noErr}, info...)
}

// PackPromocodeDone packs the message for "PROMOCODE DONE" command (37)
// "inviter" - TRUE, if we had invited a friend, or FALSE if we had been invited by a friend
// "name" - friend's name
// "gems" - reward for promo code, in gems
func (Packer) PackPromocodeDone(inviter bool, name string, gems uint32) []byte {
    inv := Ternary(inviter, 1, 0)
    data := []byte{byte(promocodeDone), inv, byte(gems >> 24), byte(gems >> 16), byte(gems >> 8), byte(gems)}
    return append(data, name...)
}
