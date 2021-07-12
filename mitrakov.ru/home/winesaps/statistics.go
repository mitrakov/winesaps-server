package main

import "time"
import "mitrakov.ru/home/winesaps/user"
import "mitrakov.ru/home/winesaps/battle"
import "mitrakov.ru/home/winesaps/network"
import . "mitrakov.ru/home/winesaps/sid"   // nolint
import . "mitrakov.ru/home/winesaps/utils" // nolint

// Statistics is a supplementary component to send server-side statistics data for monitoring
// This component is "dependent"
type Statistics struct {
    token         uint32
    started       time.Time
    sidManager    *TSidManager
    battleManager battle.IBattleManager
    userManager   user.IUserManager
    server        network.IServer
    protocol      network.IProtocol
    fakeSS        *FakeSidStore
    room          *WaitingRoom
}

// Category Type expressed as a byte
type categoryT byte

// Statistics categories
const (
    catTimeElapsedMsec categoryT = iota
    catUptimeMin
    catRps
    catCurrentUsedSids
    catCurrentBattles
    catCurrentUsers
    catTotalBattles
    catTotalUsers
    catSendersCount
    catReceiversCount
    catFakeSids
	catTotalAi
    catBattleRefsUp
    catBattleRefsDown
    catRoundRefsUp
    catRoundRefsDown
    catFieldRefsUp
    catFieldRefsDown
    catCurrentEnvSize
    catWaitingCount
)

// NewStatistics creates a new Statistics. Please do not create a Statistics directly
// "token" - secret token for a Statistics client
// "sidMgr" - reference to a TSidManager
// "usrMgr" - reference to an IUserManager
// "battleMgr" - reference to an IBattleManager
// "server" - reference to an IServer
// "protocol" - reference to an IProtocol
// "fakeSS" - reference to a FakeSidStore
// "room" - reference to a WaitingRoom
func NewStatistics(token uint32, sidMgr *TSidManager, usrMgr user.IUserManager, battleMgr battle.IBattleManager, 
        server network.IServer, protocol network.IProtocol, fakeSS *FakeSidStore, room *WaitingRoom) *Statistics {
    // args may be NULL
    return &Statistics{token, time.Now(), sidMgr, battleMgr, usrMgr, server, protocol, fakeSS, room}
}

// setSidManager assigns a non-NULL TSidManager for Statistics
func (stat *Statistics) setSidManager(sidManager *TSidManager) {
    Assert(sidManager)
    stat.sidManager = sidManager
}

// setBattleManager assigns a non-NULL IBattleManager for Statistics
func (stat *Statistics) setBattleManager(battleManager battle.IBattleManager) {
    Assert(battleManager)
    stat.battleManager = battleManager
}

// setUserManager assigns a non-NULL IUserManager for Statistics
func (stat *Statistics) setUserManager(userManager user.IUserManager) {
    Assert(userManager)
    stat.userManager = userManager
}

// setProtocol assigns a non-NULL IProtocol for Statistics
func (stat *Statistics) setProtocol(protocol network.IProtocol) {
    Assert(protocol)
    stat.protocol = protocol
}

// getStats is the main function to retrieve current server state
// "token" - client's token (must be equal to the server-side token)
// "t0" - start time (to calculate approximate elapsed time of the response)
func (stat *Statistics) getStats(token uint32, t0 time.Time) ([]byte, *Error) {
    Assert(stat.sidManager, stat.battleManager, stat.userManager, stat.server, stat.protocol)

    if token == stat.token {
        uptimeMin := Min(uint(time.Since(stat.started).Minutes()), 65535)
        rps := Min(uint(stat.server.GetRps()), 65535)
        sids := stat.sidManager.GetUsedSidsCount()
        battles := Min(stat.battleManager.GetBattlesCount(), 65535)
        users := stat.userManager.GetUsersCount()
        totBattles := Min(uint(stat.battleManager.GetBattlesCountTotal()), 65535)
        totUsers := Min(stat.userManager.GetUsersCountTotal(), 65535)
        senders := Min(stat.protocol.GetSendersCount(), 65535)
        receivers := Min(stat.protocol.GetReceiversCount(), 65535)
        fakeSids := stat.fakeSS.getUsedSidsCount()
        totalAi := Min(uint(stat.room.getSpawnedAiCount()), 65535)
        batRefUp, batRefDown := stat.battleManager.GetBattleRefs()
        roundsRefUp, roundsRefDown := stat.battleManager.GetRoundRefs()
        fieldsRefUp, fieldsRefDown := stat.battleManager.GetFieldRefs()
        currentEnv := stat.battleManager.GetEnvironmentSize()
        waiting := stat.room.getPendingCount()
        msec := Min(uint(time.Since(t0)/time.Microsecond), 65535)
        return []byte{
            byte(catTimeElapsedMsec), byte(msec / 256),          byte(msec % 256),
            byte(catUptimeMin),       byte(uptimeMin / 256),     byte(uptimeMin % 256),
            byte(catRps),             byte(rps / 256),           byte(rps % 256),
            byte(catCurrentUsedSids), byte(sids / 256),          byte(sids % 256),
            byte(catCurrentBattles),  byte(battles / 256),       byte(battles % 256),
            byte(catCurrentUsers),    byte(users / 256),         byte(users % 256),
            byte(catTotalBattles),    byte(totBattles / 256),    byte(totBattles % 256),
            byte(catTotalUsers),      byte(totUsers / 256),      byte(totUsers % 256),
            byte(catSendersCount),    byte(senders / 256),       byte(senders % 256),
            byte(catReceiversCount),  byte(receivers / 256),     byte(receivers % 256),
            byte(catFakeSids),        byte(fakeSids / 256),      byte(fakeSids % 256),
            byte(catTotalAi),         byte(totalAi / 256),       byte(totalAi % 256),
            byte(catBattleRefsUp),    byte(batRefUp / 256),      byte(batRefUp % 256),
            byte(catBattleRefsDown),  byte(batRefDown / 256),    byte(batRefDown % 256),
            byte(catRoundRefsUp),     byte(roundsRefUp / 256),   byte(roundsRefUp % 256),
            byte(catRoundRefsDown),   byte(roundsRefDown / 256), byte(roundsRefDown % 256),
            byte(catFieldRefsUp),     byte(fieldsRefUp / 256),   byte(fieldsRefUp % 256),
            byte(catFieldRefsDown),   byte(fieldsRefDown / 256), byte(fieldsRefDown % 256),
            byte(catCurrentEnvSize),  byte(currentEnv / 256),    byte(currentEnv % 256),
            byte(catWaitingCount),    byte(waiting / 256),       byte(waiting % 256)}, nil
    }
    return []byte{}, NewErr(stat, 29, "Incorrect token %d != %d", token, stat.token)
}
