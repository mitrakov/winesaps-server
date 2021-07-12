package main

import "sync"
import "time"
import "sync/atomic"
import . "mitrakov.ru/home/winesaps/sid" // nolint
import . "mitrakov.ru/home/winesaps/utils" // nolint

// WaitingRoom is a component to establish a buffer area where a user can wait for an opponent for some time (e.g.
// 10 sec), and spawn an AI for that user on time out
// This component is "dependent"
type WaitingRoom struct {
    sync.Mutex
    seconds   int
    pending   Sid
    stop      chan bool
    aiSpawned uint32
}

// max wait time, in seconds (on time out an AI will be spawned)
const maxWait = 10

// NewWaitingRoom creates a new WaitingRoom. Please do not create a WaitingRoom directly
// "controller" - instance of Controller (non-NULL)
func NewWaitingRoom(controller *Controller) *WaitingRoom {
    Assert(controller)
    
    room := new(WaitingRoom)
    room.stop = RunDaemon("room", time.Second, func() {
        room.Lock()
        room.seconds--
        if room.seconds == 0 {
            sid := room.pending
            room.pending = 0
            controller.attackAi(sid)
            atomic.AddUint32(&room.aiSpawned, 1)
        }
        room.Unlock()
    })

    return room
}

// getPendingOrWait returns a pending opponent for a battle (if it exists), or put this user to a queue for waiting for
// another opponent (if doesn't)
// "sid" - user's Session ID
func (room *WaitingRoom) getPendingOrWait(sid Sid) (Sid, bool) {
    room.Lock()
    defer room.Unlock()
    if room.pending > 0 {
        res := room.pending
        room.pending = 0
        room.seconds = -1
        return res, true
    }
    room.pending = sid
    room.seconds = maxWait
    return 0, false
}

// getSpawnedAiCount returns current count of spawned AIs
func (room *WaitingRoom) getSpawnedAiCount() uint32 {
    return atomic.LoadUint32(&room.aiSpawned)
}

// getPendingCount returns current count of users awaiting in a queue
func (room *WaitingRoom) getPendingCount() int {
    room.Lock()
    defer room.Unlock()
    return TernaryInt(room.pending > 0, 1, 0)
}

// close shuts WaitingRoom down and releases all seized resources
func (room *WaitingRoom) close() {
    Assert(room.stop)
    room.stop <- true
}
