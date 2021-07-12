// Copyright 2017-2018 Artem Mitrakov. All rights reserved.
package main

import "log"
import "math/rand"
import "mitrakov.ru/home/winesaps/user"
import . "mitrakov.ru/home/winesaps/sid"      // nolint
import "mitrakov.ru/home/winesaps/battle"
import . "mitrakov.ru/home/winesaps/utils"    // nolint
import "mitrakov.ru/home/winesaps/network"
import "mitrakov.ru/home/winesaps/filereader"

// A Controller is a special component that couples all independent components together, such as UserManager,
// BattleManager, Networking components, etc. Please Note that ALL components in Winesaps server are independent, and
// designed to be "loose coupled"
// This component is "dependent"
type Controller struct /*implements battle.IController*/ {
    userManager   user.IUserManager
    battleManager battle.IBattleManager
    server        network.IServer
    reader        *filereader.FileReader
    tokenManager  *TokenManager
    aiManager     *AiManager
    fakeSidStore  *FakeSidStore
}

// NewController creates a new Controller. Please do not create a Controller directly.
// "usrMgr" - reference to an IUserManager
// "batMgr" - reference to an IBattleManager
// "server" - reference to an IServer
// "reader" - reference to a FileReader
// "tokenMgr" - reference to a TokenManager
// "aiMgr" - reference to an AiManager
// "fakeSs" - reference to a FakeSidStore
func NewController(usrMgr user.IUserManager, batMgr battle.IBattleManager, server network.IServer,
    reader *filereader.FileReader, tokenMgr *TokenManager, aiMgr *AiManager, fakeSs *FakeSidStore) *Controller {
    Assert(usrMgr, batMgr, server, reader, tokenMgr, aiMgr, fakeSs)
    return &Controller{usrMgr, batMgr, server, reader, tokenMgr, aiMgr, fakeSs}
}

// Event is a common handler for user.IController and battle.IController interfaces.
// "box" - MailBox with all accumulated messages to be send to a client
// "err" - current error (ideally should be NULL)
func (ctrl *Controller) Event(box *MailBox, err *Error) {
    Assert(box, ctrl.server, ctrl.tokenManager, ctrl.fakeSidStore)
    Check(err)
    for _, sid := range box.GetSids() {
        if ctrl.fakeSidStore.contains(sid) {
            ctrl.aiManager.handleEvent(sid, box)
        } else if token, ok := ctrl.tokenManager.GetToken(sid); ok {
            box.SetPrefix(sid, pack(sid, token, 0))
        }
    }
    ctrl.server.SendAll(box)
}

// GameOver is a handler for "Game Finished" event of battle.IController interface
// "winnerSid" - winner Session ID
// "loserSid" - loser Session ID
// "score1" - total score 1
// "score2" - total score 2
// "quickBattle" - quick battle marker (true for QuickBattle mode, false for PvP mode)
func (ctrl *Controller) GameOver(winnerSid, loserSid Sid, score1, score2 byte, quickBattle bool,
    box *MailBox) (res uint32, err *Error) {
    Assert(ctrl.userManager, ctrl.fakeSidStore, ctrl.aiManager)
    res, err = ctrl.userManager.RewardUsers(winnerSid, loserSid, score1, score2, quickBattle, box)
    if ctrl.fakeSidStore.contains(winnerSid) {
        ctrl.aiManager.removeAi(winnerSid)
        ctrl.fakeSidStore.freeIfContains(winnerSid)
    }
    if ctrl.fakeSidStore.contains(loserSid) {
        ctrl.aiManager.removeAi(loserSid)
        ctrl.fakeSidStore.freeIfContains(loserSid)
    }
    return
}

// attackAi initiates a new battle "User vs. AI"
// "sid" - user's Session ID
func (ctrl *Controller) attackAi(sid Sid) {
    Assert(ctrl.battleManager, ctrl.userManager)

    if aggressor, ok := ctrl.userManager.GetUserBySid(sid); ok {
        abilities, err := ctrl.userManager.GetUserAbilities(aggressor)
        if err == nil {
            var levels []string
            levels, err = getLevels(ctrl.reader, 5)
            if err == nil {
                var aiSid Sid
                aiSid, err = ctrl.fakeSidStore.getFakeSid()
                if err == nil {
                    var box *MailBox
                    char1 := aggressor.Character
                    char2 := byte(rand.Intn(battle.CharactersCount) + 1)
                    ctrl.aiManager.addNewAi(aiSid, char2)
                    box, err = ctrl.battleManager.Accept(sid, aiSid, char1, char2, abilities, make([]byte, 0),
                        levels, 3, true, false)
                    box.Put(sid, append([]byte{byte(enemyName)}, getName()...))
                    ctrl.Event(box, err)
                    // IMPORTANT! if smth goes wrong => we must free fake SID
                    if err != nil {
                        ctrl.aiManager.removeAi(aiSid)
                        ctrl.fakeSidStore.freeIfContains(aiSid)
                    }
                }
            }
        }
        Check(err)
    } else {
        log.Println("ERROR: Aggressor not found", sid)
    }
}

// getName returns a random name for AI
func getName() string {
    names := []string{"Tom", "Bob", "Tim", "Fox", "Bro", "Man", "Pal", "Ace", "Ada", "Amy", "Ash", "Eve", "Eva", "Roy", 
        "Ray", "Lee", "Rex", "Rob", "Ron", "Tod", "Leo", "Van", "Fon", "Vin", "Wat", "Zak", "Mac", "Gus", "Ian", "Ira", 
        "Kim", "Joe"}
    return names[rand.Intn(len(names))]
}
