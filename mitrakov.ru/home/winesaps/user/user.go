package user

import "time"
import "sync"
import "mitrakov.ru/home/winesaps/sid"

// User is a struct for a 'user' DB row
type User struct {
    sync.RWMutex
    Character   byte
    Sid         sid.Sid
    Gems        uint32
    TrustPoints uint32
    Name        string
    Email       string
    AuthType    string // ENUM: 'Local'
    AuthData    string
    Salt        string
    Promocode   string
    AgentInfo   string
    ID          uint64
    LastEnemy   uint64
    LastLogin   time.Time
    LastActive  time.Time
}

// getLastActive is a thread-safe getter for LastActive attribute
func (user *User) getLastActive() time.Time {
    user.RLock()
    defer user.RUnlock()
    return user.LastActive
}

// setLastActiveNow sets current time for LastActive attribute. Please note that this action does NOT affect the DB
func (user *User) setLastActiveNow() {
    user.Lock()
    user.LastActive = time.Now()
    user.Unlock()
}
