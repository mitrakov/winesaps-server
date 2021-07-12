// Package sid Copyright 2017 mitrakov. All right are reserved. Governed by the BSD license
package sid

import "sync"
import "math/rand"                         // token is not critical data so we may use math.rand instead of crypto.rand
import . "mitrakov.ru/home/winesaps/utils" // nolint

// TokenManager responsible for generating new tokens. Token is designed for validation purposes only (not for
// security), because Session ID (2 bytes) is not enough to identify both user and its session.
// E.g. if a client has been re-connected, a new token should be generated for him.
// This component is independent.
type TokenManager struct {
    sync.RWMutex
    tokens map[Sid]uint32
}

// NewTokenManager creates a new instance of a TokenManager. Please don't create a TokenManager manually.
func NewTokenManager() *TokenManager {
    return &TokenManager{tokens: make(map[Sid]uint32)}
}

// TokenA returns the highest byte of a token
func TokenA (token uint32) byte {
    return byte(token >> 24)
}

// TokenB returns the second byte of a token
func TokenB (token uint32) byte {
    return byte(token >> 16)
}

// TokenC returns the first byte of a token
func TokenC (token uint32) byte {
    return byte(token >> 8)
}

// TokenD returns the lowest byte of a token
func TokenD (token uint32) byte {
    return byte(token)
}

// NewToken generates a new randomly generated 32-bit token for a given SID (and stores it for the future reference)
func (mgr *TokenManager) NewToken(sid Sid) (token uint32) {
    Assert(mgr.tokens)
    
    token = rand.Uint32()
    mgr.Lock()
    mgr.tokens[sid] = token
    mgr.Unlock()
    return
}

// GetToken returns a previously issued token by a given SID (if there are no match, returns "false")
func (mgr *TokenManager) GetToken(sid Sid) (token uint32, ok bool) {
    Assert(mgr.tokens)
    
    mgr.RLock()
    token, ok = mgr.tokens[sid]
    mgr.RUnlock()
    return
}
