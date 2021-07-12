// Package sid Copyright 2017 mitrakov. All right are reserved. Governed by the BSD license
package sid

import "log"
import "sync"
import . "mitrakov.ru/home/winesaps/utils" // nolint

// Sid is a Session ID
type Sid uint16

// TSidManager responsible for a SID distribution
// This component is independent.
type TSidManager struct {
    sync.RWMutex
    curSid Sid // type of curSid [uint16] MUST 1) be a closed ring; 2) correspond to len(busy) [65536]
    busy   [65536]bool
}

// HighSid returns a highest byte of a SID
func HighSid(sid Sid) byte {
    return byte(sid / 256)
}

// LowSid returns a lowest byte of a SID
func LowSid(sid Sid) byte {
    return byte(sid % 256)
}

// GetSid returns an arbitrary free SID. You MUST release it after the use with FreeSid().
// Method throws error in case of empty SID pool
func (mgr *TSidManager) GetSid() (Sid, *Error) {
    mgr.Lock()
    defer mgr.Unlock()

    for end := mgr.curSid - 1; mgr.curSid != end; mgr.curSid++ { //loop over full 65536-valued ring starting with curSid
        i := mgr.curSid
        if i > 0 && !mgr.busy[i] {
            mgr.busy[i] = true
            mgr.curSid++
            return i, nil
        }
    }
    return 0, NewErr(mgr, 1, "All sids are busy!")
}

// FreeSid releases the SID and brings it back to the pool
func (mgr *TSidManager) FreeSid(sid Sid) {
    log.Println("Sid", sid, "freed")
    mgr.Lock()
    mgr.busy[sid] = false
    mgr.Unlock()
}

// GetUsedSidsCount returns count of currently BUSY SIDs
func (mgr *TSidManager) GetUsedSidsCount() uint16 {
    var res uint16
    mgr.RLock()
    for _, v := range mgr.busy {
        if v {
            res++
        }
    }
    mgr.RUnlock()
    return res
}
