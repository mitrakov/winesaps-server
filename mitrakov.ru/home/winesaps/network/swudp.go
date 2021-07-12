// Package network Copyright 2017 mitrakov. All right are reserved. Governed by the BSD license
package network

import "net"
import "log"
import "sync"
import "time"
import . "mitrakov.ru/home/winesaps/utils" // nolint

// IProtocol is an interface for network protocols that may be implemented over UDP
type IProtocol interface {
    Send(data []byte, crcid uint) *Error
    OnReceived(data []byte, addr *net.UDPAddr) *Error
    OnSenderConnected()
    OnReceiverConnected(crcid uint, addr *net.UDPAddr) *Error
    ConnectionFailed(crcid uint)
    GetSendersCount() uint
    GetReceiversCount() uint
    Close()
}

// IHandler is an interface for receivers of IProtocol
type IHandler interface {
    onReceived(crcid uint, msg []byte)
}

// =======================
// === SWUDP FUNCTIONS ===
// =======================

// SwUDP is a "Simple Wrapper over UDP" protocol designed by @mitrakov to provide guaranteed delivery of messages.
// See SwUDP v1.2 specification for more details
type SwUDP struct /*implements IProtocol*/ {
    sync.RWMutex
    socket    *net.UDPConn
    senders   map[uint]*senderT
    receivers map[uint]*receiverT
    addresses map[uint]*net.UDPAddr
    handler   IHandler
    stop1     chan bool
    stop2     chan bool
}

// @mitrakov (2017-04-18): don't use ALL_CAPS const naming (gometalinter, stackoverflow.com/questions/22688906)

// SwUDP Ring size (i.e. range of IDs is 0-255)
const n = 256
// SwUDP Syn Flag
const syn = 0
// SwUDP Error Ack
const errAck = 1
// SwUDP Maximum send attempts count
const maxAttempts = 9
// SwUDP Tick duration, in ms
const period = 10 * time.Millisecond
// SwUDP Maximum of pending messages to store in receiver buffer in case of packet loss
const maxPending = 5
// SwUDP Minimum threshold for Smoothed Round Trip Time, in ticks
const minSRTT float32 = 2
// SwUDP Default Smoothed Round Trip Time, in ticks
const defaultSRTT float32 = 5
// SwUDP Maximum threshold for Smoothed Round Trip Time, in ticks
const maxSRTT float32 = 12.5
// SwUDP RTT Coefficient (RTT = Round Trip Time, in ticks)
const rc = 0.8
// SwUDP Assurance coefficient
const ac = 2.2
// Polling interval for SwUDP Guard (if a client doesn't respond more than "guardPeriod" then we kick it off)
const guardPeriod = 10 * time.Minute

// NewSwUDP creates a new instance of SwUDP protocol implementation. Please don't create an SwUDP manually.
// "socket" - standard UDP connection
// "handler" - listener for incoming messages
func NewSwUDP(socket *net.UDPConn, handler IHandler) IProtocol {
    Assert(socket, handler)

    res := &SwUDP{socket: socket, senders: make(map[uint]*senderT), receivers: make(map[uint]*receiverT), 
        addresses: make(map[uint]*net.UDPAddr), handler: handler}
    res.stop1 = RunDaemon("swudp", period, func() {
        res.RLock()
        for _, s := range res.senders {
            res.RUnlock()
            s.trigger()
            res.RLock()
        }
        res.RUnlock()
    })
    res.stop2 = RunDaemon("swudp_guard", guardPeriod, func() {
        res.RLock()
        for crcid, r := range res.receivers {
            res.RUnlock()
            if time.Since(r.lastTime) > guardPeriod {
                res.ConnectionFailed(crcid)
            }
            res.RLock()
        }
        for crcid, s := range res.senders {
            res.RUnlock()
            if time.Since(s.lastTime) > guardPeriod {
                res.ConnectionFailed(crcid)
            }
            res.RLock()
        }
        res.RUnlock()
    })

    return res
}

// Send sends a message
// "data" - message
// "crcid" - SwUDP CryptoRandom Connection ID
func (p *SwUDP) Send(data []byte, crcid uint) *Error {
    sender := p.getSender(crcid)
    Assert(sender)
    p.RLock()
    defer p.RUnlock()
    if addr, ok := p.addresses[crcid]; ok {
        return sender.send(data, addr)
    }
    return NewErr(p, 10, "Address not found for %d", crcid)
}

// OnReceived is a callback on a new message received
// "data" - message
// "addr" - UDP socket address
func (p *SwUDP) OnReceived(data []byte, addr *net.UDPAddr) *Error {
    if len(data) >= 5 {
        id := data[0]
        crcid := (uint(data[1]) << 24) | (uint(data[2]) << 16) | (uint(data[3]) << 8) | uint(data[4])
    
        // remember the address
        p.Lock()
        p.addresses[crcid] = addr
        p.Unlock()
    
        // check
        if len(data) == 5 { // Ack (id + crcid)
            sender := p.getSender(crcid)
            Assert(sender)
            sender.onAck(id)
            return nil
        } // else Input message
        receiver := p.getReceiver(crcid)
        Assert(receiver)
        return receiver.onMsg(id, crcid, data[5:], addr)
    }
    return NewErr(p, 11, "Incorrect data")
}

// OnReceiverConnected is a callback on a new receiver connected event
// "crcid" - SwUDP CryptoRandom Connection ID
// "addr" - UDP socket address
func (p *SwUDP) OnReceiverConnected(crcid uint, addr *net.UDPAddr) *Error {
    log.Println(addr, "Receiver connected!", crcid)
    sender := p.getSender(crcid)
    Assert(sender)
    return sender.connect(crcid, addr)
}

// OnSenderConnected is a callback on a new sender connected event
func (p *SwUDP) OnSenderConnected() {
    log.Println("Sender connected!")
}

// ConnectionFailed is a callback on a connection failed event
// "crcid" - SwUDP CryptoRandom Connection ID
func (p *SwUDP) ConnectionFailed(crcid uint) {
    p.Lock()
    delete(p.senders, crcid)
    delete(p.receivers, crcid)
    s, r := len(p.senders), len(p.receivers)
    p.Unlock()
    log.Println("Connection failed! ", s, "senders and", r, " receivers left")
}

// GetSendersCount returns current count of Senders. This value might be inaccurate if some clients "fell off"
func (p *SwUDP) GetSendersCount() uint {
    p.RLock()
    defer p.RUnlock()
    return uint(len(p.senders))    // len(map) is thread-safe but Data Race may occur
}

// GetReceiversCount returns current count of Receivers. This value might be inaccurate if some clients "fell off"
func (p *SwUDP) GetReceiversCount() uint {
    p.RLock()
    defer p.RUnlock()
    return uint(len(p.receivers))  // len(map) is thread-safe but Data Race may occur
}

// Close shuts SwUDP down and releases all seized resources
func (p *SwUDP) Close() {
    Assert(p.stop1, p.stop2)
    p.stop1 <- true
    p.stop2 <- true
}

// getSender returns a Sender by its CryptoRandom Connection ID. Note: DO NOT access "senders" array directly!
func (p *SwUDP) getSender(crcid uint) *senderT {
    p.Lock()
    defer p.Unlock()
    if sender, ok := p.senders[crcid]; ok {
        sender.lastTime = time.Now()
        return sender
    }
    p.senders[crcid] = newSender(p.socket, p)
    return p.senders[crcid]
}

// getReceiver returns a Receiver by its CryptoRandom Connection ID. Note: DO NOT access "receivers" array directly!
func (p *SwUDP) getReceiver(crcid uint) *receiverT {
    p.Lock()
    defer p.Unlock()
    if receiver, ok := p.receivers[crcid]; ok {
        receiver.lastTime = time.Now()
        return receiver
    }
    p.receivers[crcid] = newReceiver(p.socket, p.handler, p)
    return p.receivers[crcid]
}

// ====================
// === COMMON TYPES ===
// ====================

// SwUDP item
type itemT struct {
    ack        bool
    startRtt   uint
    ticks      uint
    attempt    uint
    nextRepeat uint
    msg        []byte
    addr       *net.UDPAddr
}

// ========================
// === SENDER FUNCTIONS ===
// ========================

// SwUDP Sender
type senderT struct {
    sync.Mutex
    id          byte
    expectedAck byte
    connected   bool
    srtt        float32
    totalTicks  uint
    crcid       uint
    buffer      [n]*itemT
    socket      *net.UDPConn
    protocol    IProtocol
    lastTime    time.Time
}

// newSender creates a new instance of senderT. Please don't create a senderT manually.
// "socket" - standard UDP connection
// "protocol" - back reference to IProtocol
func newSender(socket *net.UDPConn, protocol IProtocol) *senderT {
    Assert(socket, protocol)
    return &senderT{socket: socket, protocol: protocol, lastTime: time.Now()}
}

// connect connects to a remote SwUDP receiver
// "crcID" - SwUDP CryptoRandom Connection ID
// "addr" - UDP socket address
func (s *senderT) connect(crcID uint, addr *net.UDPAddr) (err *Error) {
    s.Lock()
    s.crcid = crcID
    s.id = syn
    s.expectedAck = syn
    s.srtt = defaultSRTT
    s.totalTicks = 0
    s.connected = false
    for j := range s.buffer {
        s.buffer[j] = nil
    }
    msg := []byte{s.id, byte(s.crcid >> 24), byte(s.crcid >> 16), byte(s.crcid >> 8), byte(s.crcid), 0xFD} // fake data
    s.buffer[s.id] = &itemT{msg: msg, addr: addr}
    log.Println(addr, "Send: ", msg)
    _, er := s.socket.WriteToUDP(msg, addr)
    err = NewErrFromError(s, 12, er)
    s.Unlock()
    return
}

// send sends the given message to the remote SwUDP receiver
// "msg" - message
// "addr" - UDP socket address
func (s *senderT) send(msg []byte, addr *net.UDPAddr) *Error {
    if s.connected {
        s.Lock()
        s.id = next(s.id)
        data := append([]byte{s.id, byte(s.crcid >> 24), byte(s.crcid >> 16), byte(s.crcid >> 8), byte(s.crcid)},
            msg...)
        s.buffer[s.id] = &itemT{startRtt: s.totalTicks, msg: data, addr: addr}
        s.Unlock()
        log.Println(addr, "Send: ", data)
        _, er := s.socket.WriteToUDP(data, addr)
        return NewErrFromError(s, 13, er)
    }
    return NewErr(s, 14, "Not connected %v", addr)
}

// onAck is a callback on Ack packet received
// "ack" - Ack packet from the remote SwUDP receiver
func (s *senderT) onAck(ack byte) {
    s.Lock()
    if s.buffer[ack] != nil {
        s.buffer[ack].ack = true
        if ack == s.expectedAck {
            rtt := s.totalTicks - s.buffer[ack].startRtt + 1
            newSrtt := rc*s.srtt + (1-rc)*float32(rtt)
            s.srtt = MinF(MaxF(newSrtt, minSRTT), maxSRTT)
            s.accept()
        }
    }
    if ack == syn {
        s.connected = true
        s.protocol.OnSenderConnected()
    } else if ack == errAck {
        s.connected = false
        for j := range s.buffer {
            s.buffer[j] = nil
        }
        s.protocol.ConnectionFailed(s.crcid)
    }
    s.Unlock()
}

// accept accepts expected Ack and removes the corresponding message from the buffer
func (s *senderT) accept() {
    // no need to lock
    if s.buffer[s.expectedAck] != nil {
        if s.buffer[s.expectedAck].ack {
            s.buffer[s.expectedAck] = nil
            s.expectedAck = next(s.expectedAck)
            s.accept()
        }
    }
}

// trigger is called by timer procedure, that tries to retransmit non-Acked packets to the remote SwUDP receiver
func (s *senderT) trigger() {
    s.Lock()
    defer s.Unlock()

    s.totalTicks++
    i := s.expectedAck
    if s.buffer[i] != nil && !s.buffer[i].ack {
        if s.buffer[i].attempt > maxAttempts {
            s.connected = false
            for j := range s.buffer {
                s.buffer[j] = nil
            }
            s.protocol.ConnectionFailed(s.crcid)
            return
        } else if s.buffer[i].ticks == s.buffer[i].nextRepeat {
            s.buffer[i].attempt++
            s.buffer[i].nextRepeat += uint(ac * s.srtt * float32(s.buffer[i].attempt))
            if s.buffer[i].attempt > 1 {
                s.buffer[i].startRtt = s.totalTicks
                _, er := s.socket.WriteToUDP(s.buffer[i].msg, s.buffer[i].addr) // msg already contains id and crcid
                Check(er)
            }
        }
        s.buffer[i].ticks++
    }
}

// ==========================
// === RECEIVER FUNCTIONS ===
// ==========================

// SwUDP Receiver
type receiverT struct {
    sync.Mutex
    expected  byte
    connected bool
    pending   byte
    buffer    [n]*itemT
    socket    *net.UDPConn
    handler   IHandler
    protocol  IProtocol
    lastTime  time.Time
}

// newReceiver creates a new instance of receiverT. Please don't create a receiverT manually.
// "socket" - standard UDP connection
// "handler" - handler for incoming messages
// "protocol" - back reference to IProtocol
func newReceiver(socket *net.UDPConn, handler IHandler, protocol IProtocol) *receiverT {
    Assert(socket, handler, protocol)
    return &receiverT{socket: socket, handler: handler, protocol: protocol, lastTime: time.Now()}
}

// onMsg is a callback on a new message received
// "id" - SwUDP packet ID
// "crcid" - SwUDP CryptoRandom Connection ID
// "msg" - message
// "addr" - UDP socket address
func (r *receiverT) onMsg(id byte, crcid uint, msg []byte, addr *net.UDPAddr) (err *Error) {
    r.Lock()
    ack := []byte{id, byte(crcid >> 24), byte(crcid >> 16), byte(crcid >> 8), byte(crcid)}
    if id == syn {
        _, er1 := r.socket.WriteToUDP(ack, addr)
        for j := range r.buffer {
            r.buffer[j] = nil
        }
        r.expected = next(id)
        r.connected = true
        r.pending = 0
        er2 := r.protocol.OnReceiverConnected(crcid, addr)
        err = NewErrs(NewErrFromError(r, 15, er1), er2)
    } else if r.connected {
        _, er := r.socket.WriteToUDP(ack, addr)
        err = NewErrFromError(r, 16, er)
        if id == r.expected {
            r.handler.onReceived(crcid, msg)
            r.expected = next(id)
            r.pending = 0
            r.accept(crcid)
        } else if after(id, r.expected) {
            if r.pending++; r.pending < maxPending {
                r.buffer[id] = &itemT{msg: msg}
            } else {
                r.connected = false
                for j := range r.buffer {
                    r.buffer[j] = nil
                }
                r.protocol.ConnectionFailed(crcid)
            }
        }
    } else {
        ack[0] = errAck
        _, er := r.socket.WriteToUDP(ack, addr)
        err = NewErrFromError(r, 16, er)
    }
    r.Unlock()
    return
}

// accept transmits the successful packet to the handler and removes it from the buffer
// "crcid" - CryptoRandom Connection ID
func (r *receiverT) accept(crcid uint) {
    if r.buffer[r.expected] != nil {
        r.handler.onReceived(crcid, r.buffer[r.expected].msg)
        r.buffer[r.expected] = nil
        r.expected = next(r.expected)
        r.accept(crcid)
    }
}

// next returns the next SwUDP ID (number in range 2-255)
// "n" - current SwUDP ID
func next(n byte) byte {
    result := n + 1 // in case of byte: 255 -> 0
    ok := result != syn && result != errAck
    if ok {
        return result
    }
    return next(result)
}

// after returns whether ID1 is after or before ID2
// "x" - SwUDP ID1
// "y" - SwUDP ID2
func after(x, y byte) bool {
    return (y - x) > n/2 // in case of byte: 3-255 = 4 (4 < N/2 => return false)
}
