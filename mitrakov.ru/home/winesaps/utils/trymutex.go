package utils

import "sync"

// TryMutex is an extension of Golang sync.Mutex
type TryMutex struct {
    sync.Mutex
    locked bool
    cnt    uint64
}

// OnlyOne ensures that a given function will be executed only once, even though multiple threads are trying to call it
// in the same time. If race condition happens, one thread will call "f", and the others will skip execution.
func (x *TryMutex) OnlyOne(f func()) {
    if !x.locked {           // 1. if locked => return
        cnt := x.cnt
        x.Lock()             // 2. Lock()
        if cnt == x.cnt {
            x.locked = true  // 3. locked = true
            x.cnt++
            f()              // 4. run f()
            x.locked = false // 5. locked = false
        }
        x.Unlock()           // 6. Unlock()
    }
}
