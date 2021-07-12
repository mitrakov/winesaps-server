// Package network Copyright 2017 mitrakov. All right are reserved. Governed by the BSD license
package network

import "fmt"
import "log"
import "time"
import "sync"
import . "mitrakov.ru/home/winesaps/utils" // nolint

// @mitrakov (2017-04-18): don't use ALL_CAPS const naming (gometalinter, stackoverflow.com/questions/22688906)

// max count of packets to treat the behaviour as "suspicious" in the limits of "intervalSec" seconds
const maxSamples = 60
// interval of time to count the number of received packets; if it becomes > "maxSamples" => this is suspicious
const intervalSec = 10

// SimpleDetector is a primitive misbehaviour detector (flood, DoS, etc.)
// This component is independent.
type SimpleDetector struct /*implements IFloodDetector*/ {
    sync.Mutex
    samples map[string]*sampleT
    banned  map[string]bool
}

// NewSimpleDetector creates a new instance of a SimpleDetector. Please don't create a SimpleDetector manually.
func NewSimpleDetector() IFloodDetector {
    return &SimpleDetector{samples: make(map[string]*sampleT), banned: make(map[string]bool)}
}

// sampleT is a helper structure to hold a counter and a base timestamp
type sampleT struct {
    cnt       int
    timestamp time.Time
}

// checkBanned checks whether the address "addr" should be banned, or already banned. If it's okay, returns FALSE.
func (flood *SimpleDetector) checkBanned(addr fmt.Stringer) bool {
    Assert(flood.samples, flood.banned)
    key := addr.String()
    
    // protect our maps
    flood.Lock()
    defer flood.Unlock()

    // check if already banned
    if _, ok := flood.banned[key]; ok {
        return true
    }

    // check if behaviour is suspicious
    if s, ok := flood.samples[key]; ok {
        s.cnt++
        if time.Since(s.timestamp) > intervalSec*time.Second {
            if s.cnt > maxSamples {
                flood.banned[key] = true
                log.Printf("Address %s banned as suspicious\n", addr)
                return true
            }
            s.cnt = 0
            s.timestamp = time.Now()
        }
    } else {
        flood.samples[key] = &sampleT{0, time.Now()}
    }
    return false
}
