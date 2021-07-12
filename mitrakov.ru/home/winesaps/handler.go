// Copyright 2017-2018 Artem Mitrakov. All rights reserved.
package main

import "log"
import "time"
import "bytes"
import "strings"
import "strconv"
import "runtime"
import "math/rand"
import "mitrakov.ru/home/winesaps/user"
import "mitrakov.ru/home/winesaps/battle"
import "mitrakov.ru/home/winesaps/network"
import "mitrakov.ru/home/winesaps/filereader"
import . "mitrakov.ru/home/winesaps/sid"   // nolint
import . "mitrakov.ru/home/winesaps/utils" // nolint

// A Handler is a special component to process incoming messages from Winesaps clients
// This component is "dependent"
type Handler struct /*implements network.IHandler*/ {
    userManager      user.IUserManager
    battleManager    battle.IBattleManager
    server           network.IServer
    reader           *filereader.FileReader
    tokenManager     *TokenManager
    aiManager        *AiManager
    fakeSidStore     *FakeSidStore
    room             *WaitingRoom
    statistics       *Statistics
    serverStop       bool
    minClientVersion uint
    curClientVersion uint
}

// Command according to Server API Doc
type cmd byte

// List of Server API Commands
const ( // nolint
    // @mitrakov (2017-04-18): don't use ALL_CAPS const naming (gometalinter, stackoverflow.com/questions/22688906)
    unspecError     cmd = iota
    signUp              // 1
    signIn              // 2
    signOut             // 3
    userInfo            // 4
    changeCharacter     // 5
    attack              // 6
    call                // 7
    accept              // 8
    reject              // 9
    stopCall            // 10
    cancelCall          // 11
    receiveLevel        // 12
    rangeOfProducts     // 13
    buyProduct          // 14
    enemyName           // 15
    fullState           // 16
    roundInfo           // 17
    abilityList         // 18
    move                // 19
    useThing            // 20
    useSkill            // 21
    giveUp              // 22
    stateChanged        // 23
    scoreChanged        // 24
    effectChanged       // 25
    playerWounded       // 26
    thingTaken          // 27
    objectAppended      // 28
    finished            // 29
    restoreState        // 30
    reserved1F          // nolint
    rating              // 32
    friendList          // 33
    addFriend           // 34
    removeFriend        // 35
    checkPromocode      // 36
    promocodeDone       // 37
    getSkuGems          // 38
    checkPurchase       // 39
    getClientVersion    // 40
    changePassword      // 41
)

// "REQUEST STATISTICS" Server API Command
const statRequest cmd = 240
// "EXECUTE FUNCTION" Server API Command
const callFunction cmd = 241
// Special Command to return the same response as a request
const loopback cmd = 242

// Handler error list
const (
    errNotHandled        byte = iota + 240
    errIncorrectLen           // 241
    errNotEnoughArgs          // 242
    errIncorrectArg           // 243
    errIncorrectAuthtype      // 244
    errUserNotFound           // 245
    errIncorrectToken         // 246
    errEnemyNotFound          // 247
    errWaitForEnemy           // 248
    errIncorrectName          // 249
    errIncorrectPassword      // 250
    errIncorrectEmail         // 251
    errNameAlreadyExists      // 252
    errFnCodeNotFound         // 253
    errServerGonnaStop        // 254
)

// Attack Type
type attackType byte

// List of possible Attack types
const (
    attackByName attackType = iota
    attackLatest
    attackQuick
)

// List of possible Stop Call cases
const (
    rejected = iota
    missed
    timerExpired
)

// SUCCESS constant
const noErr = 0
// Min length of password (now in fact it is not used because a client sends an md5-hash of a password)
const minPasswordLen = 8
// Offset for arguments in incoming bytearray: 10 bytes: sid (2b), token (4b), flags, msgLength (2b), cmdCode
const argsOffset = 10
// Pagination for a list of friends (in case a user has a lot of friends)
const friendListFragment = 25

// newHandler creates a new Handler. Please do not create a Handler directly.
// "usrMgr" - reference to an IUserManager
// "battleMgr" - reference to an IBattleManager
// "server" - reference to an IServer
// "reader" - reference to a FileReader
// "tokenMgr" - reference to a TokenManager
// "aiMgr" - reference to an AiManager
// "fakeSs" - reference to a FakeSidStore
// "room" - reference to a WaitingRoom
// "stat" - Statistics module (for monitoring only)
// "minClientVersion" - minimal supported client version expressed as an uint32 (e.g. "1.2.3" = 1 << 16 | 2 << 8 | 3)
// "curClientVersion" - current client version expressed as an uint32 (e.g. "1.2.3" = 1 << 16 | 2 << 8 | 3)
func newHandler(usrMgr user.IUserManager, battleMgr battle.IBattleManager, server network.IServer, 
    reader *filereader.FileReader, tokenMgr *TokenManager, aiMgr *AiManager, fakeSs *FakeSidStore, room *WaitingRoom, 
    stat *Statistics, minClientVersion, curClientVersion uint) *Handler {
    Assert(usrMgr, battleMgr, server, reader, tokenMgr, aiMgr, fakeSs, room, stat)
    return &Handler{usrMgr, battleMgr, server, reader, tokenMgr, aiMgr, fakeSs, room, stat, false, minClientVersion, 
        curClientVersion}
}

// Handle is a main handler method for network.ISidHandler interface
// "array" - incoming message
// nolint: gocyclo
func (handler *Handler) Handle(array []byte) (Sid, []byte) {
    t0 := time.Now()
    Assert(handler.userManager, handler.tokenManager)

    if len(array) < argsOffset {
        return 0, []byte{0, 0, 0, 0, 0, 0, 1, byte(unspecError)}
    }

    sidH := array[0]
    sidL := array[1]
    tokenA := array[2]
    tokenB := array[3]
    tokenC := array[4]
    tokenD := array[5]
    flags := array[6]
    // msgLenH := array[7]         ignored (client is allowed to send single messages only)
    // msgLenL := array[8]         ignored (client is allowed to send single messages only)
    code := cmd(array[9])
    sid := Sid(sidH)*256 + Sid(sidL)
    token := uint32(tokenA)<<24 | uint32(tokenB)<<16 | uint32(tokenC)<<8 | uint32(tokenD)

    if usr, ok := handler.userManager.GetUserBySid(sid); ok {
        if tok, ok := handler.tokenManager.GetToken(sid); ok && tok == token {
            switch code {
            case signOut:
                return sid, handler.signOut(usr, token, flags, code)
            case userInfo:
                return sid, handler.userInfo(usr, token, flags, code)
            case attack:
                return sid, handler.attack(usr, token, flags, code, array[argsOffset:])
            case accept:
                return sid, handler.accept(usr, token, flags, code, array[argsOffset:])
            case reject:
                return sid, handler.reject(usr, token, flags, code, array[argsOffset:])
            case cancelCall:
                return sid, handler.cancelCall(sid, token, flags, code)
            case receiveLevel:
                return sid, handler.receiveLevel(usr, token, flags, code, array[argsOffset:])
            case changeCharacter:
                return sid, handler.changeCharacter(usr, token, flags, code, array[argsOffset:])
            case friendList:
                return sid, handler.friendList(usr, token, flags, code, array[argsOffset:])
            case addFriend:
                return sid, handler.addFriend(usr, token, flags, code, array[argsOffset:])
            case removeFriend:
                return sid, handler.removeFriend(usr, token, flags, code, array[argsOffset:])
            case rangeOfProducts:
                return sid, handler.rangeOfProducts(sid, token, flags, code)
            case buyProduct:
                return sid, handler.buyProduct(usr, token, flags, code, array[argsOffset:])
            case rating:
                return sid, handler.getRating(usr, token, flags, code, array[argsOffset:])
            case fullState: // since v1.3.0
                return sid, handler.getCurrentField(sid, token, flags, code)
            case move:
                return sid, handler.move(sid, token, flags, code, array[argsOffset:])
            case useThing:
                return sid, handler.useThing(sid, token, flags, code)
            case useSkill:
                return sid, handler.useSkill(sid, token, flags, code, array[argsOffset:])
            case restoreState:
                return sid, handler.restoreState(sid, token, flags, code)
            case giveUp:
                return sid, handler.giveUp(sid, token, flags, code)
            case getSkuGems:
                return sid, handler.getSkuGems(sid, token, flags, code)
            case checkPurchase:
                return sid, handler.checkPurchase(usr, token, flags, code, array[argsOffset:])
            case getClientVersion:
                return sid, handler.clientVersion(sid, token, flags, code)
            case changePassword:
                return sid, handler.changePassword(usr, token, flags, code, array[argsOffset:])
            }
        }
        return 0, packN(sid, token, flags|1, 2, byte(code), errIncorrectToken) // see note#1
    } else if sid == 0 {
        switch code {
        case signUp:
            return handler.signUp(sid, token, flags, code, array[argsOffset:])
        case signIn:
            return handler.signIn(sid, token, flags, code, array[argsOffset:])
        case checkPromocode:
            return sid, handler.checkPromocode(sid, token, flags, code, array[argsOffset:])
        case getClientVersion:
            return sid, handler.clientVersion(sid, token, flags, code)
        case statRequest:
            return sid, handler.getStatistics(sid, token, flags, code, t0)
        case callFunction:
            return sid, handler.callFunction(sid, token, flags, code, array[argsOffset:])
        case loopback:
            return sid, array
        }
    } else {
        return 0, packN(sid, token, flags|1, 2, byte(code), errUserNotFound) // see note#1
    }

    return 0, packN(sid, token, flags|1, 2, byte(code), errNotHandled) // see note#1
}

// signUp is a handler for "SIGN UP" command (1)
// "sid" - client's Session ID
// "token" - client's 32-bit validation token
// "flags" - message flags
// "code" - command code
// "usrData" - arbitrary user data of the message
func (handler *Handler) signUp(sid Sid, token uint32, flags byte, code cmd, usrData []byte) (Sid, []byte) {
    Assert(handler.userManager, handler.tokenManager, handler.server)

    items := bytes.Split(usrData, []byte{0})
    if len(items) == 5 {
        name := string(items[0])
        password := string(items[1])
        agentInfo := string(items[2])
        email := string(items[3])
        promocode := string(items[4])
        if len(password) >= minPasswordLen { // additional check; in theory a client must send HEX md5-hash (32b)
            user, err := handler.userManager.SignUp(name, email, password, agentInfo, promocode)
            if err == nil {
                newToken := handler.tokenManager.NewToken(user.Sid)
                return user.Sid, packN(user.Sid, newToken, flags|1, 2, byte(code), noErr)
            }
            Check(err)
            return sid, packN(sid, token, flags|1, 2, byte(code), getSignUpErrCode(err))
        }
        return sid, packN(sid, token, flags|1, 2, byte(code), errIncorrectPassword)
    }
    return sid, packN(sid, token, flags|1, 2, byte(code), errNotEnoughArgs)
}

// signIn is a handler for "SIGN IN" command (2)
// "sid" - client's Session ID
// "token" - client's 32-bit validation token
// "flags" - message flags
// "code" - command code
// "usrData" - arbitrary user data of the message
func (handler *Handler) signIn(sid Sid, token uint32, flags byte, code cmd, usrData []byte) (Sid, []byte) {
    Assert(handler.userManager, handler.tokenManager)
    if len(usrData) > 1 {
        authType := usrData[0]
        if authType == 1 { // 1 = Local auth
            authData := usrData[1:]
            items := bytes.Split(authData, []byte{0})
            if len(items) == 3 {
                name := string(items[0])
                password := string(items[1])
                agentInfo := string(items[2])
                user, err, oldSid := handler.userManager.SignIn(name, password, agentInfo)
                if err == nil {
                    if oldSid > 0 { // if oldSid exists => send SignOut to him
                        box := NewMailBox()
                        box.Put(oldSid, []byte{byte(signOut), noErr})
                        handler.setPrefixes(box, oldSid, 0)
                        handler.server.SendAll(box)
                    }
                    newToken := handler.tokenManager.NewToken(user.Sid)
                    return user.Sid, packN(user.Sid, newToken, flags|1, 2, byte(code), noErr)
                }
                Check(err)
                return sid, packN(sid, token, flags|1, 2, byte(code), GetErrorCode(err))
            }
            return sid, packN(sid, token, flags|1, 2, byte(code), errNotEnoughArgs)
        }
        return sid, packN(sid, token, flags|1, 2, byte(code), errIncorrectAuthtype)
    }
    return sid, packN(sid, token, flags|1, 2, byte(code), errIncorrectLen)
}

// signOut is a handler for "SIGN OUT" command (3)
// "user" - user
// "token" - client's 32-bit validation token
// "flags" - message flags
// "code" - command code
func (handler *Handler) signOut(user *user.User, token uint32, flags byte, code cmd) (response []byte) {
    Assert(user)
    handler.userManager.SignOut(user)
    return packN(user.Sid, token, flags|1, 2, byte(code), noErr)
}

// is a handler for "USER INFO" command (4)
// "user" - user
// "token" - client's 32-bit validation token
// "flags" - message flags
// "code" - command code
func (handler *Handler) userInfo(user *user.User, token uint32, flags byte, code cmd) (response []byte) {
    Assert(user, handler.userManager)

    info, err := handler.userManager.GetUserInfo(user)
    if err == nil {
        return append(packN(user.Sid, token, flags|1, len(info)+2, byte(code), noErr), info...)
    }
    Check(err)
    return packN(user.Sid, token, flags|1, 2, byte(code), GetErrorCode(err))
}

// changeCharacter is a handler for "CHANGE CHARACTER" command (5)
// "user" - user
// "token" - client's 32-bit validation token
// "flags" - message flags
// "code" - command code
// "usrData" - arbitrary user data of the message
func (handler *Handler) changeCharacter(user *user.User, token uint32, flags byte, code cmd, usrData []byte) []byte {
    Assert(handler.userManager, user)

    if len(usrData) == 1 {
        character := usrData[0]
        err := handler.userManager.ChangeCharacter(user, character)
        Check(err)
        return packN(user.Sid, token, flags|1, 2, byte(code), GetErrorCode(err))
    }
    return packN(user.Sid, token, flags|1, 2, byte(code), errIncorrectLen)
}

// attack is a handler for "ATTACK" command (6)
// "aggressor" - aggressor user
// "token" - client's 32-bit validation token
// "flags" - message flags
// "code" - command code
// "usrData" - arbitrary user data of the message
func (handler *Handler) attack(aggressor *user.User, token uint32, flags byte, code cmd, usrData []byte) []byte {
    if len(usrData) > 0 {
        if !handler.serverStop {
            switch attackType(usrData[0]) {
            case attackByName:
                return handler.attackByName(aggressor, token, flags, code, usrData)
            case attackLatest:
                return handler.attackLatest(aggressor, token, flags, code)
            case attackQuick:
                return handler.attackQuick(aggressor, token, flags, code)
            }
            return packN(aggressor.Sid, token, flags|1, 2, byte(code), errIncorrectArg)
        }
        return packN(aggressor.Sid, token, flags|1, 2, byte(code), errServerGonnaStop)
    }
    return packN(aggressor.Sid, token, flags|1, 2, byte(code), errIncorrectLen)
}

// attackByName is a handler for "ATTACK" command (6) with a "ByName" argument
// "aggressor" - aggressor user
// "token" - client's 32-bit validation token
// "flags" - message flags
// "code" - command code
// "usrData" - arbitrary user data of the message
func (handler *Handler) attackByName(aggressor *user.User, token uint32, flags byte, code cmd, usrData []byte) []byte {
    Assert(aggressor, handler.userManager, handler.battleManager, handler.server)

    if len(usrData) > 1 {
        name := string(usrData[1:])
        if victim, ok := handler.userManager.GetUserByName(name); ok {
            box, err := handler.battleManager.Attack(aggressor.Sid, victim.Sid, aggressor.Name, victim.Name)
            if err == nil {
                box.Put(aggressor.Sid, append([]byte{byte(code), noErr}, name...))
                handler.setPrefixes(box, aggressor.Sid, flags)
                handler.server.SendAll(box)
                return nil
            }
            Check(err)
            return packN(aggressor.Sid, token, flags|1, 2, byte(code), GetErrorCode(err))
        }
        log.Println("ERROR: Enemy not found", name)
        return packN(aggressor.Sid, token, flags|1, 2, byte(code), errEnemyNotFound)
    }
    return packN(aggressor.Sid, token, flags|1, 2, byte(code), errIncorrectLen)
}

// attackLatest is a handler for "ATTACK" command (6) with a "Latest" argument
// "aggressor" - aggressor user
// "token" - client's 32-bit validation token
// "flags" - message flags
// "code" - command code
func (handler *Handler) attackLatest(aggressor *user.User, token uint32, flags byte, code cmd) (response []byte) {
    Assert(aggressor, handler.userManager, handler.battleManager, handler.server)

    if victim, ok := handler.userManager.GetUserByID(aggressor.LastEnemy); ok {
        box, err := handler.battleManager.Attack(aggressor.Sid, victim.Sid, aggressor.Name, victim.Name)
        if err == nil {
            box.Put(aggressor.Sid, append([]byte{byte(code), noErr}, victim.Name...))
            handler.setPrefixes(box, aggressor.Sid, flags)
            handler.server.SendAll(box)
            return nil
        }
        Check(err)
        return packN(aggressor.Sid, token, flags|1, 2, byte(code), GetErrorCode(err))
    }
    log.Println("ERROR: Enemy not found.", aggressor.LastEnemy)
    return packN(aggressor.Sid, token, flags|1, 2, byte(code), errEnemyNotFound)
}

// attackQuick is a handler for "ATTACK" command (6) with a "Random" argument
// "user" - user
// "token" - client's 32-bit validation token
// "flags" - message flags
// "code" - command code
func (handler *Handler) attackQuick(user *user.User, token uint32, flags byte, code cmd) (response []byte) {
    Assert(user, handler.userManager, handler.battleManager, handler.server, handler.room)

    if enemySid, ok := handler.room.getPendingOrWait(user.Sid); ok {
        if enemy, ok := handler.userManager.GetUserBySid(enemySid); ok {
            levels, err := getLevels(handler.reader, 5)
            if err == nil {
                abilities1, err1 := handler.userManager.GetUserAbilities(enemy)
                abilities2, err2 := handler.userManager.GetUserAbilities(user)
                err = NewErrs(err1, err2)
                if err == nil {
                    char1, char2 := enemy.Character, user.Character
                    box, err1 := handler.battleManager.Accept(enemySid, user.Sid, char1, char2, abilities1, 
                        abilities2, levels, 3, true, false)
                    err2 := handler.userManager.Accept(enemy, user)
                    err = NewErrs(err1, err2)
                    if err == nil {
                        box.Put(enemy.Sid, append([]byte{byte(enemyName)}, user.Name...))
                        box.Put(user.Sid, append([]byte{byte(enemyName)}, enemy.Name...))
                        box.Put(user.Sid, []byte{byte(code), noErr})
                        handler.setPrefixes(box, user.Sid, flags)
                        handler.server.SendAll(box)
                        return nil
                    }
                }
            }
            Check(err)
            return packN(user.Sid, token, flags|1, 2, byte(code), GetErrorCode(err))
        }
        log.Println("ERROR: Enemy not found!", enemySid)
        return packN(user.Sid, token, flags|1, 2, byte(code), errEnemyNotFound)
    }
    return packN(user.Sid, token, flags|1, 2, byte(code), errWaitForEnemy)
}

// accept is a handler for "ACCEPT" command (8)
// "defender" - defender user
// "token" - client's 32-bit validation token
// "flags" - message flags
// "code" - command code
// "usrData" - arbitrary user data of the message
func (handler *Handler) accept(defender *user.User, token uint32, flags byte, code cmd, usrData []byte) []byte {
    Assert(defender, handler.userManager, handler.battleManager, handler.server)

    if len(usrData) == 2 {
        aggressorSid := Sid(usrData[0])*256 + Sid(usrData[1])
        if aggressor, ok := handler.userManager.GetUserBySid(aggressorSid); ok {
            levels, err := getLevels(handler.reader, 5)
            if err == nil {
                abilities1, err1 := handler.userManager.GetUserAbilities(aggressor)
                abilities2, err2 := handler.userManager.GetUserAbilities(defender)
                err = NewErrs(err1, err2)
                if err == nil {
                    char1, char2 := aggressor.Character, defender.Character
                    box, err1 := handler.battleManager.Accept(aggressor.Sid, defender.Sid, char1, char2, 
                        abilities1, abilities2, levels, 3, false, true)
                    err2 := handler.userManager.Accept(aggressor, defender)
                    err = NewErrs(err1, err2)
                    if err == nil {
                        box.Put(defender.Sid, []byte{byte(code), noErr})
                        handler.setPrefixes(box, defender.Sid, flags)
                        handler.server.SendAll(box)
                        return nil
                    }
                }
            }
            Check(err)
            return packN(defender.Sid, token, flags|1, 2, byte(code), GetErrorCode(err))
        }
        log.Println("ERROR: Aggressor not found: sid=", aggressorSid)
        return packN(defender.Sid, token, flags|1, 2, byte(code), errEnemyNotFound)
    }
    return packN(defender.Sid, token, flags|1, 2, byte(code), errIncorrectLen)
}

// reject is a handler for "REJECT" command (9)
// "user" - user
// "token" - client's 32-bit validation token
// "flags" - message flags
// "code" - command code
// "usrData" - arbitrary user data of the message
func (handler *Handler) reject(user *user.User, token uint32, flags byte, code cmd, usrData []byte) (response []byte) {
    Assert(user, handler.userManager, handler.battleManager, handler.server)

    if len(usrData) == 2 {
        aggressorSid := Sid(usrData[0])*256 + Sid(usrData[1])
        if _, ok := handler.userManager.GetUserBySid(aggressorSid); ok {
            box, err := handler.battleManager.Reject(aggressorSid, user.Sid, user.Name)
            if err == nil {
                box.Put(user.Sid, []byte{byte(code), noErr})
                handler.setPrefixes(box, user.Sid, flags)
                handler.server.SendAll(box)
                return nil
            }
            Check(err)
            return packN(user.Sid, token, flags|1, 2, byte(code), GetErrorCode(err))
        }
        log.Println("ERROR: Aggressor not found; sid=", aggressorSid)
        return packN(user.Sid, token, flags|1, 2, byte(code), errEnemyNotFound)
    }
    return packN(user.Sid, token, flags|1, 2, byte(code), errIncorrectLen)
}

// cancelCall is a handler for "CANCEL CALL" command (11)
// "sid" - client's Session ID
// "token" - client's 32-bit validation token
// "flags" - message flags
// "code" - command code
func (handler *Handler) cancelCall(sid Sid, token uint32, flags byte, code cmd) (response []byte) {
    Assert(handler.battleManager, handler.server)

    box, err := handler.battleManager.CancelCall(sid)
    if err == nil {
        box.Put(sid, []byte{byte(code), noErr})
        handler.setPrefixes(box, sid, flags)
        handler.server.SendAll(box)
        return nil
    }
    Check(err)
    return packN(sid, token, flags|1, 2, byte(code), GetErrorCode(err))
}

// receiveLevel is a handler for "RECEIVE LEVEL" command (12)
// "user" - user
// "token" - client's 32-bit validation token
// "flags" - message flags
// "code" - command code
// "usrData" - arbitrary user data of the message
func (handler *Handler) receiveLevel(user *user.User, token uint32, flags byte, code cmd, usrData []byte) []byte {
    Assert(user, handler.battleManager, handler.fakeSidStore, handler.server)
    
    if len(usrData) > 0 {
        if !handler.serverStop {
            levelName := string(usrData)
            abilities := make([]byte, 0)
            enemyChar := byte(rand.Intn(3) + 1)
            if enemyChar == user.Character {
                enemyChar = 4
            }
            fakeSid, err := handler.fakeSidStore.getFakeSid()
            if err == nil {
                var box *MailBox
                box, err = handler.battleManager.Accept(user.Sid, fakeSid, user.Character, enemyChar, abilities, 
                    abilities, []string{levelName}, 1, false, false)
                if err == nil {
                    box.Put(user.Sid, []byte{byte(code), noErr})
                    handler.setPrefixes(box, user.Sid, flags)
                    handler.server.SendAll(box)
                    return nil
                } // else
                handler.fakeSidStore.freeIfContains(fakeSid) // IMPORTANT! if smth goes wrong, we must free fake SID
            }
            Check(err)
            return packN(user.Sid, token, flags|1, 2, byte(code), GetErrorCode(err))
        }
        return packN(user.Sid, token, flags|1, 2, byte(code), errServerGonnaStop)
    }
    return packN(user.Sid, token, flags|1, 2, byte(code), errIncorrectLen)
}

// rangeOfProducts is a handler for "RANGE OF PRODUCTS" command (13)
// "sid" - client's Session ID
// "token" - client's 32-bit validation token
// "flags" - message flags
// "code" - command code
func (handler *Handler) rangeOfProducts(sid Sid, token uint32, flags byte, code cmd) (response []byte) {
    Assert(handler.userManager)

    array, err := handler.userManager.GetAllAbilities()
    if err == nil {
        return append(packN(sid, token, flags|1, len(array)+2, byte(code), noErr), array...)
    }
    Check(err)
    return packN(sid, token, flags|1, 2, byte(code), GetErrorCode(err))
}

// buyProduct is a handler for "BUY PRODUCT" command (14)
// "user" - user
// "token" - client's 32-bit validation token
// "flags" - message flags
// "code" - command code
// "usrData" - arbitrary user data of the message
func (handler *Handler) buyProduct(user *user.User, token uint32, flags byte, code cmd, usrData []byte) []byte {
    Assert(user, handler.userManager)

    if len(usrData) == 2 {
        product := usrData[0]
        days := usrData[1]
        err := handler.userManager.BuyProduct(user, product, days)
        if err == nil {
            var info []byte
            info, err = handler.userManager.GetUserInfo(user)
            if err == nil {
                return append(packN(user.Sid, token, flags|1, len(info)+2, byte(code), noErr), info...)
            }
        }
        Check(err)
        return packN(user.Sid, token, flags|1, 2, byte(code), GetErrorCode(err))
    }
    return packN(user.Sid, token, flags|1, 2, byte(code), errIncorrectLen)
}

// getCurrentField is a handler for "FULL STATE" command (16)
// @since 1.3.0
// "sid" - client's Session ID
// "token" - client's 32-bit validation token
// "flags" - message flags
// "code" - command code
func (handler *Handler) getCurrentField(sid Sid, token uint32, flags byte, code cmd) (response []byte) {
    Assert(handler.battleManager)
    base, err := handler.battleManager.GetFieldRaw(sid)
    Check(err)
    result := packN(sid, token, flags|1, len(base)+1, byte(code)) // it doesn't contain err for backwards compatibility
    return append(result, base...)
}

// move is a handler for "MOVE" command (19)
// "sid" - client's Session ID
// "token" - client's 32-bit validation token
// "flags" - message flags
// "code" - command code
// "usrData" - arbitrary user data of the message
func (handler *Handler) move(sid Sid, token uint32, flags byte, code cmd, usrData []byte) (response []byte) {
    Assert(handler.battleManager, handler.server)

    if len(usrData) == 1 {
        direction := usrData[0]
        box, err := handler.battleManager.Move(sid, direction)
        if err == nil {
            for _, s := range box.GetSids() {
                if handler.fakeSidStore.contains(s) {
                    handler.aiManager.handleEvent(s, box)
                }
            }
            box.Put(sid, []byte{byte(code), noErr})
            handler.setPrefixes(box, sid, flags)
            handler.server.SendAll(box)
            return nil
        }
        Check(err)
        return packN(sid, token, flags|1, 2, byte(code), GetErrorCode(err))
    }
    return packN(sid, token, flags|1, 2, byte(code), errIncorrectLen)
}

// useThing is a handler for "USE THING" command (20)
// "sid" - client's Session ID
// "token" - client's 32-bit validation token
// "flags" - message flags
// "code" - command code
func (handler *Handler) useThing(sid Sid, token uint32, flags byte, code cmd) (response []byte) {
    Assert(handler.battleManager, handler.server)

    box, err := handler.battleManager.UseThing(sid)
    if err == nil {
        box.Put(sid, []byte{byte(code), noErr})
        handler.setPrefixes(box, sid, flags)
        handler.server.SendAll(box)
        return nil
    }
    Check(err)
    return packN(sid, token, flags|1, 2, byte(code), GetErrorCode(err))
}

// useSkill is a handler for "USE SKILL" command (21)
// "sid" - client's Session ID
// "token" - client's 32-bit validation token
// "flags" - message flags
// "code" - command code
// "usrData" - arbitrary user data of the message
func (handler *Handler) useSkill(sid Sid, token uint32, flags byte, code cmd, usrData []byte) (response []byte) {
    Assert(handler.battleManager, handler.server)

    if len(usrData) == 1 {
        id := usrData[0]
        box, err := handler.battleManager.UseSkill(sid, id)
        if err == nil {
            box.Put(sid, []byte{byte(code), noErr})
            handler.setPrefixes(box, sid, flags)
            handler.server.SendAll(box)
            return nil
        }
        Check(err)
        return packN(sid, token, flags|1, 2, byte(code), GetErrorCode(err))
    }
    return packN(sid, token, flags|1, 2, byte(code), errIncorrectLen)
}

// giveUp is a handler for "GIVE UP" command (22)
// "sid" - client's Session ID
// "token" - client's 32-bit validation token
// "flags" - message flags
// "code" - command code
func (handler *Handler) giveUp(sid Sid, token uint32, flags byte, code cmd) (response []byte) {
    Assert(handler.battleManager, handler.fakeSidStore, handler.server)

    _, box, err := handler.battleManager.GiveUp(sid)
    if err == nil {
        box.Put(sid, []byte{byte(code), noErr})
        handler.setPrefixes(box, sid, flags)
        handler.server.SendAll(box)
        return nil
    }
    Check(err)
    return packN(sid, token, flags|1, 2, byte(code), GetErrorCode(err))
}

// restoreState is a handler for "RESTORE STATE" command (30)
// @since 1.3.0
// "sid" - client's Session ID
// "token" - client's 32-bit validation token
// "flags" - message flags
// "code" - command code
func (handler *Handler) restoreState(sid Sid, token uint32, flags byte, code cmd) (response []byte) {
    Assert(handler.battleManager)
    
    dump, err := handler.battleManager.GetMovablesDump(sid)
    if err == nil {
        return append(packN(sid, token, flags|1, len(dump)+2, byte(code), noErr), dump...)
    }
    Check(err)
    return packN(sid, token, flags|1, 2, byte(code), GetErrorCode(err))
}

// getRating is a handler for "RATING" command (32)
// "user" - user
// "token" - client's 32-bit validation token
// "flags" - message flags
// "code" - command code
// "usrData" - arbitrary user data of the message
func (handler *Handler) getRating(user *user.User, token uint32, flags byte, code cmd, usrData []byte) []byte {
    Assert(user, handler.userManager)

    if len(usrData) == 1 {
        ratingType := usrData[0]
        rating, err := handler.userManager.GetRating(user, ratingType)
        if err == nil {
            return append(packN(user.Sid, token, flags|1, len(rating)+3, byte(code), noErr, ratingType), rating...)
        }
        Check(err)
        return packN(user.Sid, token, flags|1, 2, byte(code), GetErrorCode(err))
    }
    return packN(user.Sid, token, flags|1, 2, byte(code), errIncorrectLen)
}

// friendList is a handler for "FRIEND LIST" command (33)
// "user" - user
// "token" - client's 32-bit validation token
// "flags" - message flags
// "code" - command code
// "usrData" - arbitrary user data of the message
func (handler *Handler) friendList(user *user.User, token uint32, flags byte, code cmd, usrData []byte) []byte {
    Assert(user, handler.userManager, handler.server)

    // since 1.2.0 we additionally add statuses (1=offline, 2=online)
    showStatuses := len(usrData) == 1 && usrData[0] == 1

    characters, friends, err := handler.userManager.GetUserFriends(user)
    total := Min(uint(len(characters)), uint(len(friends)))
    if err == nil {
        fragNumber := byte(1)
        res := []byte{}
        for i := uint(0); i < total; i++ {
            if i > 0 && i%friendListFragment == 0 {
                header := packN(user.Sid, token, flags|1, len(res)+3, byte(code), noErr, fragNumber)
                handler.server.Send(user.Sid, append(header, res...)) // do NOT use MailBox here! It may cause overflow
                fragNumber++
                res = []byte{}
            }
            character := characters[i]
            friend := friends[i]
            if showStatuses {
                _, ok := handler.userManager.GetUserByName(friend)
                res = append(res, Ternary(ok, 2, 1))
            }
            res = append(res, character)
            res = append(res, []byte(friend)...)
            res = append(res, 0)
        }
        return append(packN(user.Sid, token, flags|1, len(res)+3, byte(code), noErr, fragNumber), res...)
    }
    Check(err)
    return packN(user.Sid, token, flags|1, 2, byte(code), GetErrorCode(err))
}

// addFriend is a handler for "ADD FRIEND" command (34)
// "user" - user
// "token" - client's 32-bit validation token
// "flags" - message flags
// "code" - command code
// "usrData" - arbitrary user data of the message
func (handler *Handler) addFriend(user *user.User, token uint32, flags byte, code cmd, usrData []byte) []byte {
    Assert(user, handler.userManager)

    if len(usrData) > 0 {
        name := string(usrData)
        character, err := handler.userManager.AddFriend(user, name)
        Check(err)
        res := packN(user.Sid, token, flags|1, len(name)+3, byte(code), GetErrorCode(err), character)
        return append(res, name...)
    }
    return packN(user.Sid, token, flags|1, 2, byte(code), errIncorrectLen)
}

// removeFriend is a handler for "REMOVE FRIEND" command (35)
// "user" - user
// "token" - client's 32-bit validation token
// "flags" - message flags
// "code" - command code
// "usrData" - arbitrary user data of the message
func (handler *Handler) removeFriend(user *user.User, token uint32, flags byte, code cmd, usrData []byte) []byte {
    Assert(user, handler.userManager)

    if len(usrData) > 0 {
        name := string(usrData)
        err := handler.userManager.RemoveFriend(user, name)
        Check(err)
        res := packN(user.Sid, token, flags|1, len(name)+2, byte(code), GetErrorCode(err))
        return append(res, name...)
    }
    return packN(user.Sid, token, flags|1, 2, byte(code), errIncorrectLen)
}

// checkPromocode is a handler for "CHECK PROMOCODE" command (36)
// "sid" - client's Session ID
// "token" - client's 32-bit validation token
// "flags" - message flags
// "code" - command code
// "usrData" - arbitrary user data of the message
func (handler *Handler) checkPromocode(sid Sid, token uint32, flags byte, code cmd, usrData []byte) (response []byte) {
    Assert(handler.userManager)

    if len(usrData) > 0 {
        promocode := string(usrData)
        _, ok, err := handler.userManager.IsPromocodeValid(promocode)
        Check(err)
        return packN(sid, token, flags|1, 3, byte(code), noErr, Ternary(ok, 1, 0)) // we return "noErr" here!
    }
    return packN(sid, token, flags|1, 2, byte(code), errIncorrectLen)
}

// getSkuGems is a handler for "GET SKU GEMS" command (38)
// "sid" - client's Session ID
// "token" - client's 32-bit validation token
// "flags" - message flags
// "code" - command code
func (handler *Handler) getSkuGems(sid Sid, token uint32, flags byte, code cmd) (response []byte) {
    Assert(handler.userManager)

    data := []byte{}
    for sku, gems := range handler.userManager.GetSkuGems() {
        gems0 := byte(gems >> 24)
        gems1 := byte(gems >> 16)
        gems2 := byte(gems >> 8)
        gems3 := byte(gems)
        data = append(data, sku...)
        data = append(data, 0, gems0, gems1, gems2, gems3)
    }
    return append(packN(sid, token, flags|1, len(data)+1, byte(code)), data...)
}

// checkPurchase is a handler for "CHECK PURCHASE" command (39)
// "user" - user
// "token" - client's 32-bit validation token
// "flags" - message flags
// "code" - command code
// "usrData" - arbitrary user data of the message
func (handler *Handler) checkPurchase(user *user.User, token uint32, flags byte, code cmd, usrData []byte) []byte {
    Assert(user, handler.userManager, handler.server)

    if len(usrData) > 2 {
        items := bytes.Split(usrData, []byte{0})
        if len(items) == 2 {
            json := string(items[0])
            signature := string(items[1])
            gems, box, err := handler.userManager.CheckPayment(user, json, signature)
            if err == nil {
                gems0 := byte(gems >> 24)
                gems1 := byte(gems >> 16)
                gems2 := byte(gems >> 8)
                gems3 := byte(gems)
                box.Put(user.Sid, append([]byte{byte(code), noErr, gems0, gems1, gems2, gems3}, "Coupon"...))
                handler.setPrefixes(box, user.Sid, flags)
                handler.server.SendAll(box)
                return nil
            }
            Check(err)
            return packN(user.Sid, token, flags|1, 2, byte(code), GetErrorCode(err))
        }
        return packN(user.Sid, token, flags|1, 2, byte(code), errNotEnoughArgs)
    }
    return packN(user.Sid, token, flags|1, 2, byte(code), errIncorrectLen)
}

// clientVersion is a handler for "GET CLIENT VERSION" command (40)
// "sid" - client's Session ID
// "token" - client's 32-bit validation token
// "flags" - message flags
// "code" - command code
func (handler *Handler) clientVersion(sid Sid, token uint32, flags byte, code cmd) (response []byte) {
    a := byte((handler.minClientVersion >> 16) & 0xFF)
    b := byte((handler.minClientVersion >> 8) & 0xFF)
    c := byte((handler.minClientVersion & 0xFF))
    x := byte((handler.curClientVersion >> 16) & 0xFF)
    y := byte((handler.curClientVersion >> 8) & 0xFF)
    z := byte((handler.curClientVersion & 0xFF))
    return packN(sid, token, flags|1, 7, byte(code), a, b, c, x, y, z)
}

// changePassword is a handler for "CHANGE PASSWORD" command (41)
// @since 1.2.0
// "user" - user
// "token" - client's 32-bit validation token
// "flags" - message flags
// "code" - command code
// "usrData" - arbitrary user data of the message
func (handler *Handler) changePassword(user *user.User, token uint32, flags byte, code cmd, usrData []byte) []byte {
    Assert(handler.userManager)
    
    if len(usrData) > 2 {
        passwords := bytes.Split(usrData, []byte{0})
        if len(passwords) == 2 {
            oldPwd, newPwd := string(passwords[0]), string(passwords[1])
            err := handler.userManager.ChangePassword(user, oldPwd, newPwd)
            Check(err)
            return packN(user.Sid, token, flags|1, 2, byte(code), GetErrorCode(err))
        }
        return packN(user.Sid, token, flags|1, 2, byte(code), errNotEnoughArgs)
    }
    return packN(user.Sid, token, flags|1, 2, byte(code), errIncorrectLen)
}

// getStatistics is a handler for "STATISTICS" command (240)
// "sid" - client's Session ID
// "token" - client's 32-bit validation token
// "flags" - message flags
// "code" - command code
// "t0" - start timestamp (to calculate elapsed time of the query)
func (handler *Handler) getStatistics(sid Sid, token uint32, flags byte, code cmd, t0 time.Time) (response []byte) {
    Assert(handler.statistics)

    stats, err := handler.statistics.getStats(token, t0)
    Check(err)
    return append(packN(sid, token, flags|1, len(stats)+2, byte(code), GetErrorCode(err)), stats...)
}

// callFunction is a handler for "CALL FUNCTION" command (241)
// nolint: gocyclo
// "sid" - client's Session ID
// "token" - client's 32-bit validation token
// "flags" - message flags
// "code" - command code
// "usrData" - arbitrary user data of the message
func (handler *Handler) callFunction(sid Sid, token uint32, flags byte, code cmd, usrData []byte) []byte {
    Assert(handler.userManager)

    if len(usrData) > 0 {
        fnCode := usrData[0]
        switch fnCode {
        case 0x31: // '1' (kicks out a user by name)
            if len(usrData) > 1 {
                name := string(usrData[1:])
                if usr, ok := handler.userManager.GetUserByName(name); ok {
                    handler.userManager.SignOut(usr)
                    return packN(sid, token, flags|1, 2, byte(code), noErr)
                }
                return packN(sid, token, flags|1, 2, byte(code), errUserNotFound)
            }
            return packN(sid, token, flags|1, 2, byte(code), errIncorrectLen)
        case 0x32: // '2' (hint to run Garbage Collector)
            runtime.GC()
            return packN(sid, token, flags|1, 2, byte(code), noErr)
        case 0x33: // '3' (soft stopping a server)
            handler.serverStop = !handler.serverStop
            return packN(sid, token, flags|1, 2, byte(code), noErr)
        case 0x34: // '4' (get user by number)
            if len(usrData) > 1 {
                number := string(usrData[1:])
                if n, err1 := strconv.Atoi(number); err1 == nil {
                    usr, err2 := handler.userManager.GetUserNumberN(uint(n))
                    if err2 == nil {
                        name := usr.Name
                        return append(packN(sid, token, flags|1, 2+len(name), byte(code), noErr), name...)
                    }
                    return packN(sid, token, flags|1, 2, byte(code), GetErrorCode(err2))
                }
                return packN(sid, token, flags|1, 2, byte(code), errIncorrectArg)
            }
            return packN(sid, token, flags|1, 2, byte(code), errIncorrectLen)
        default:
            return packN(sid, token, flags|1, 2, byte(code), errFnCodeNotFound)
        }
    }
    return packN(sid, token, flags|1, 2, byte(code), errIncorrectLen)
}

// =======================
// === LOCAL FUNCTIONS ===
// =======================

// setPrefixes prepends ALL messages in the box with all required prefixes (sid, token, flags), and returns this box
// "responseSid" - response Session ID
// "box" - MailBox to set prefixes in
// "flags" - message flags
func (handler *Handler) setPrefixes(box *MailBox, responseSid Sid, flags byte) *MailBox {
    Assert(box)
    
    for _, sid := range box.GetSids() {
        if token, ok := handler.tokenManager.GetToken(sid); ok {
            prefix := pack(sid, token, Ternary(sid == responseSid, flags|1, flags))
            box.SetPrefix(sid, prefix)
        }
    }
    return box
}

// pack converts "sid", "token" and "flags" into a bytearray
func pack(sid Sid, token uint32, flags byte) []byte {
    return []byte{HighSid(sid), LowSid(sid), TokenA(token), TokenB(token), TokenC(token), TokenD(token), flags}
}

// packN converts "sid", "token", "flags" and extra arbitrary data ("args") into a bytearray
// please note that "size" is NOT a length of "args", but length of a total message, e.g. this code packs "aaaTommy":
//
// name := "Tommy"
// append(packN(sid, token, flags, len(name)+3, byte('a'), byte('a'), byte('a')), name...) // note "len(name)+3"
func packN(sid Sid, token uint32, flags byte, size int, args ...byte) []byte {
    res := []byte{HighSid(sid), LowSid(sid), TokenA(token), TokenB(token), TokenC(token), TokenD(token), flags, 
        byte(size/256), byte(size%256)}
    return append(res, args...)
}

// getSignUpErrCode converts SignUp error "err" into a byte code
func getSignUpErrCode(err *Error) byte {
    Assert(err)
    if strings.Contains(err.Text, "Incorrect name") {
        return errIncorrectName
    }
    if strings.Contains(err.Text, "Incorrect email") {
        return errIncorrectEmail
    }
    if strings.Contains(err.Text, "Duplicate entry") {
        return errNameAlreadyExists
    }
    return err.Code
}

// getLevels returns no more than "count" level names as a string array
// "reader" - instance of FileReader
// "count" - count of level names
func getLevels(reader *filereader.FileReader, count int) (levels []string, err *Error) {
    Assert(reader)

    levels = make([]string, count)
    for i := 0; i < len(levels); i++ {
        // note: since 1.3.8 we don't take wins count into account and return all [non-tutorial] levels for all users
        levels[i], err = reader.GetRandomExcept("tutorial.level", "training.level")
        if err != nil {
            return
        }
    }
    return
}

//
// note#1 (@mitrakov, 2017-03-29): here we MUST return sid = 0! If we return an old sid, it causes vulnerability!
// Our 'Network' maps every [non-zero] sid to a remote UDP address; suppose a hacker knows that a user with sid = 56
// exists, and he/she may send a fake datagram with sid = 56; the handler detects IncorrectTokenError and returns it to
// the hacker, but 'Network' reassigns the UDP address to the hacker, so that the actual user will receive nothing
// P.S. described above regards only to the sid for 'Network'; a sid inside 'pack(...)' may be an old one
//
