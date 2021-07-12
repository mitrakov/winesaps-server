// Copyright 2017-2018 Artem Mitrakov. All rights reserved.
// Welcome to Winesaps Server! This is a Main File.
// See notes below for more details.
package main

import "log"
import "fmt"
import "strconv"
import "net/http"
import _ "net/http/pprof"
import "github.com/vaughan0/go-ini"
import "mitrakov.ru/home/winesaps/sid"
import "mitrakov.ru/home/winesaps/user"
import "mitrakov.ru/home/winesaps/battle"
import "mitrakov.ru/home/winesaps/network"
import . "mitrakov.ru/home/winesaps/utils" // nolint
import "mitrakov.ru/home/winesaps/checker"
import "mitrakov.ru/home/winesaps/filereader"

// Buffer size (in bytes) for reading level files; the most files are 258 bytes long, but they might be larger
const levelBufSiz = 512

// Entry Point
// nolint: gocyclo
func main() {
    // ==========================================================================
    // PREPARING LOGGER AND PROFILING
    // ==========================================================================

    go http.ListenAndServe(":8080", nil)
    log.SetFlags(log.LstdFlags | log.Lmicroseconds)
    log.Println("Winesaps server v1.3.11 (2018-09-29)")
    
    // ==========================================================================
    // READING INI-FILE
    // ==========================================================================

    // scan INI-file (GENERAL)
    file, err := ini.LoadFile("settings.ini")
    Check(err)
    pwd, ok := file.Get("GENERAL", "db.pwd")
    if !ok {
        panic("Cannot find db password")
    }
    localArg, ok := file.Get("GENERAL", "local.parameter")
    if !ok {
        panic("Cannot find db local parameter")
    }
    publicKey, ok := file.Get("GENERAL", "public.key")
    if !ok {
        panic("Cannot find RSA Public Key")
    }
    statTokenStr, ok := file.Get("GENERAL", "stat.token")
    if !ok {
        panic("Cannot find statistics token")
    }
    statToken, err := strconv.ParseUint(statTokenStr, 16, 0)
    Check(err)
    minVersionStr, ok := file.Get("GENERAL", "client.version.min")
    if !ok {
        panic("Cannot find min client version")
    }
    var minVersionH, minVersionM, minVersionL uint8
    _, err = fmt.Sscanf(minVersionStr, "%d.%d.%d", &minVersionH, &minVersionM, &minVersionL)
    Check(err)
    minClientVersion := (uint(minVersionH) << 16) | (uint(minVersionM) << 8) | uint(minVersionL)
    curVersionStr, ok := file.Get("GENERAL", "client.version.cur")
    if !ok {
        panic("Cannot find current client version")
    }
    var curVersionH, curVersionM, curVersionL uint8
    _, err = fmt.Sscanf(curVersionStr, "%d.%d.%d", &curVersionH, &curVersionM, &curVersionL)
    Check(err)
    curClientVersion := (uint(curVersionH) << 16) | (uint(curVersionM) << 8) | (uint(curVersionL))
    
    // scan INI-file (SKU)
    skuMap := make(map[string]uint32)
    for _, sku := range []string{"gems_pack_small", "gems_pack", "gems_pack_big"} {
        str, norm := file.Get("SKU", sku)
        if !norm {
            panic("Cannot find SKU")
        }
        gems, er := strconv.ParseUint(str, 10, 0)
        Check(er)
        skuMap[sku] = uint32(gems)
    }
    
    // scan INI-file (REWARD)
    promoRewardStr, ok := file.Get("REWARD", "promocode")
    if !ok {
        panic("Cannot find promocode reward")
    }
    promoReward, err := strconv.ParseUint(promoRewardStr, 10, 0)
    Check(err)
    reward := uint32(promoReward)
    rewardMap := make(map[int]uint32)
    for i, name := range []string{"rating.gold", "rating.silver", "rating.bronze"} {
        str, ok := file.Get("REWARD", name)
        if !ok {
            panic("Cannot find rating reward")
        }
        gems, er := strconv.ParseUint(str, 10, 0)
        Check(er)
        rewardMap[i] = uint32(gems)
    }
    
    // ==========================================================================
    // DEPENDENCY INJECTION (TODO: think of external tools)
    // ==========================================================================

    // DbManager
    dbManager, err := NewDbManager("tommy", pwd)
    Check(err)

    // SidManager
    sidManager := new(sid.TSidManager)

    // TokenManager
    tokenManager := sid.NewTokenManager()
    
    // Signature Checker
    publicKey = fmt.Sprintf("-----BEGIN PUBLIC KEY-----\n%s\n-----END PUBLIC KEY-----", publicKey)
    checker, err := checker.NewSignatureChecker(publicKey)
    Check(err)

    // FakeSidStore
    fakeSidStore := NewFakeSidStore(sidManager)

    // FileReader
    reader, err := filereader.NewFileReader("levels", "level", levelBufSiz)
    Check(err)

    // Server
    server := network.NewServer(nil, nil)

    // Packer
    packer := new(Packer)

    // UserManager
    usrManager := user.NewUserManager(sidManager, checker, dbManager, packer, nil, localArg, skuMap, rewardMap, reward)

    // BattleManager
    battleManager := battle.NewBattleManager(reader, packer, nil)
    
    // Ai
    aiManager := NewAiManager(nil, battleManager)
    
    // Controller
    controller := NewController(usrManager, battleManager, server, reader, tokenManager, aiManager, fakeSidStore)

    // Waiting Room
    room := NewWaitingRoom(controller)
    
    // Statistics
    statistics := NewStatistics(uint32(statToken), sidManager, usrManager, battleManager, server, nil, fakeSidStore, 
        room)

    // Handler
    handler := newHandler(usrManager, battleManager, server, reader, tokenManager, aiManager, fakeSidStore, room,
        statistics, minClientVersion, curClientVersion)

    // add cross references
    server.SetSidHandler(handler)
    usrManager.SetController(controller)
    battleManager.SetController(controller)
    aiManager.setController(controller)

    // ==========================================================================
    // STARTING SERVER
    // ==========================================================================

    socket, err := server.Connect(33996)
    Check(err)
    protocol := network.NewSwUDP(socket, server)
    server.SetProtocol(protocol)
    statistics.setProtocol(protocol)
    server.Start()

    // ==========================================================================
    // SHUTTING DOWN SERVER
    // ==========================================================================

    aiManager.close()
    room.close()
    usrManager.Close()
    battleManager.Close()
    protocol.Close()
    err = server.Close()
    Check(err)
    err = dbManager.Close()
    Check(err)
}

//
// note#2 (@mitrakov 2017-03-30): I often use append() to empty slice instead of writing to 'bytes.Buffer' or writing
// to allocated array by index; why? Let's see to the benchmark:
//
// 1. a:=[]byte{};           for (...) {a = append(a, i)}             2.76 ns/op
// 2. a:=bytes.Buffer{};     for (...) {a.WriteByte(i)}              12.11 ns/op
// 3. a:=make([]byte, 0, N); for (...) {a = append(a, i)}             1.77 ns/op
// 4. a:=make([]byte, N);    for (...) {a[i] = i}                     0.88 ns/op
//
// as we can see the way (1) takes 2.8 ns that is longer than (3) or (4); but in case of (4) we must KNOW the precise
// length of array (it's not always possible), and in case of (3) we should PREDICT the possible capacity of underlying
// array (it's easier to estimate, but we could waste our precious memory); the difference is not too high
// (2.8 vs 1.8), so I decided to confidently use the way (1) when impossible to calculate the array size.
//
// note#3 (@mitrakov, 2017-04-04): if protocol SwUDP drops down a connection, I decided NOT to remove the user's SID;
// why? firstly, if the user really had disconnected, the ping-pong mechanism soon or later kicks him/her out (see
// UserManager.kickOutInactiveUsers); secondly, if the user just changes network (e.g. from mobile to WiFi), his/her
// connection drops down, but a sid/token pair will be still actual after re-connection
//
