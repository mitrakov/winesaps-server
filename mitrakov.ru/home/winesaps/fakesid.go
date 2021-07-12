// Copyright 2017-2018 Artem Mitrakov. All rights reserved.
package main

import "sync"
import . "mitrakov.ru/home/winesaps/sid"   // nolint
import . "mitrakov.ru/home/winesaps/utils" // nolint

// A FakeSidStore is a special pool to store fake Session IDs (SIDs)
// This component is "dependent"
type FakeSidStore struct {
    sync.RWMutex
    sidManager *TSidManager
    sidSet     map[Sid]bool
}

// NewFakeSidStore creates a new instance of FakeSidStore. Please do not create FakeSidStore directly.
// "sidMgr" - instance of TSidManager
func NewFakeSidStore(sidMgr *TSidManager) *FakeSidStore {
    Assert(sidMgr)
    return &FakeSidStore{sidManager: sidMgr, sidSet: make(map[Sid]bool)}
}

// getFakeSid returns a new vacant SID from pool.
// IMPORTANT! Please DO NOT forget to release it after usage with "freeIfContains" method!
func (fakeSs *FakeSidStore) getFakeSid() (Sid, *Error) {
    Assert(fakeSs.sidManager, fakeSs.sidSet)

    sid, err := fakeSs.sidManager.GetSid()
    if err == nil {
        fakeSs.Lock()
        fakeSs.sidSet[sid] = true
        fakeSs.Unlock()
    }
    return sid, err
}

// contains checks whether a given Session ID is stored in internal pool
func (fakeSs *FakeSidStore) contains(sid Sid) bool {
    Assert(fakeSs.sidSet)

    fakeSs.RLock()
    val, ok := fakeSs.sidSet[sid]
    fakeSs.RUnlock()
    return val && ok
}

// freeIfContains releases a given Session ID and returns it back to pool. It is safe to call this method for any SID,
// regardless of whether it had been actually seized or not
func (fakeSs *FakeSidStore) freeIfContains(sid Sid) {
    Assert(fakeSs.sidManager, fakeSs.sidSet)

    if fakeSs.contains(sid) {
        fakeSs.Lock()
        fakeSs.sidSet[sid] = false
        fakeSs.sidManager.FreeSid(sid)
        fakeSs.Unlock()
    }
}

// getUsedSidsCount returns current count of taken SIDs
func (fakeSs *FakeSidStore) getUsedSidsCount() uint16 {
    var res uint16
    fakeSs.RLock()
    for _, v := range fakeSs.sidSet {
        if v {
            res++
        }
    }
    fakeSs.RUnlock()
    return res
}
