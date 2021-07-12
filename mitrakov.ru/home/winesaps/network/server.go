// Package network Copyright 2017 mitrakov. All right are reserved. Governed by the BSD license
package network

import "fmt"
import "log"
import "net"
import "sync"
import "time"
import "sync/atomic"
import . "mitrakov.ru/home/winesaps/sid"   // nolint
import . "mitrakov.ru/home/winesaps/utils" // nolint

// IServer is an interface to describe components that could send and receive messages
type IServer interface {
    Connect(port uint16) (*net.UDPConn, *Error)
    Start()
    Send(sid Sid, data []byte) *Error
    SendAll(box *MailBox)
    GetRps() uint32
    SetSidHandler(handler ISidHandler)
    SetProtocol(protocol IProtocol)
    Close() *Error
}

// ISidHandler is an interface to handle incoming messages
type ISidHandler interface {
    Handle(array []byte) (sid Sid, response []byte)
}

// IFloodDetector is an interface to detect and ban suspicious addresses
type IFloodDetector interface {
    checkBanned(addr fmt.Stringer) bool
}

// Server is a component that could send and receive messages over a UDP socket.
// Both interface and implementation were placed in the same src intentionally!
// This component is independent.
type Server struct /*implements IServer, IHandler*/ {
    sync.RWMutex
    socket        *net.UDPConn
    clients       map[Sid]uint
    handler       ISidHandler
    protocol      IProtocol

    stop          chan bool
    rps           uint32 // requests per second
}

// size (in bytes) to store Google Play json and signature (about 700b)
const bufSiz = 768

// NewServer creates a new instance of Server. Please don't create a Server manually.
// "handler" - handler for incoming messages (may be NULL)
// "protocol" - an additional protocol over UDP (may be NULL)
func NewServer(handler ISidHandler, protocol IProtocol) *Server {
    // handler, protocol may be nil
    return &Server{clients: make(map[Sid]uint), handler: handler, protocol: protocol}
}

// Connect tries to bind the Server to a given port. This method doesn't block the execution.
// Method throws error, if: 1) Cannot resolve UDPv4 address; 2) Cannot create a socket
func (server *Server) Connect(port uint16) (*net.UDPConn, *Error) {
    udpAddr, err := net.ResolveUDPAddr("udp4", fmt.Sprintf(":%d", port))
    if err == nil {
        server.socket, err = net.ListenUDP("udp", udpAddr)
    }
    log.Println("Server connected to port", port)
    return server.socket, NewErrFromError(server, 6, err)
}

// Start gets the Server started. This method BLOCKS the execution. Ensure the Connect() method is called before this.
// Please note that all errors are handled inside the method (printed to log)
func (server *Server) Start() {
    Assert(server.socket)
    
    curRps := uint32(0)
    server.stop = RunDaemon("RPS", time.Second, func() {
        atomic.StoreUint32(&server.rps, atomic.SwapUint32(&curRps, 0))
    })

    for {
        buf := make([]byte, bufSiz) // if a datagram is larger, ReadFromUDP() will produce an error
        n, addr, err := server.socket.ReadFromUDP(buf)
        if err == nil {
            atomic.AddUint32(&curRps, 1)
            msg := buf[0:n]
            if len(msg) > 5 {
                log.Println(addr, "Recv: ", msg)
            }
            if server.protocol != nil {
                go Check(server.protocol.OnReceived(msg, addr))
            } else {
                log.Println("No protocol found. Since 2017-05-12 server must have a protocol")
            }
        } else {
            log.Println("ReadFromUDP", err) // don't return here {continue listening to a socket}
        }
    }
}

// Send transmits a message to the socket. Ensure the Connect() method is called before this.
// Method throws error, if: 1) SID not found; 2) Send message error.
// "sid" - client's Session ID
// "data" - message
func (server *Server) Send(sid Sid, data []byte) *Error {
    Assert(server.clients)

    server.RLock()
    defer server.RUnlock()
    if crcid, ok := server.clients[sid]; ok {
        return server.send(data, crcid)
    }
    return NewErr(server, 7, "Sid %d not found", sid)
}

// SendAll sends all the messages, consolidated in a given MailBox
func (server *Server) SendAll(box *MailBox) {
    Assert(box, server.clients)

    server.RLock()
    defer server.RUnlock()

    sids, msgs := box.Flush()
    for i := uint(0); i < Min(uint(len(sids)), uint(len(msgs))); i++ {
        sid := sids[i]
        msg := msgs[i]
        if crcid, ok := server.clients[sid]; ok {
            Check(server.send(msg, crcid)) // Do NOT return in case of an error (what about the other clients?)
        }
    }
    return
}
// GetRps returns current "Requests per Second" value
func (server *Server) GetRps() (result uint32) {
    result = atomic.LoadUint32(&server.rps)
    return
}

// SetSidHandler assigns a new message handler for the Server. May be NULL
func (server *Server) SetSidHandler(handler ISidHandler) {
    server.handler = handler
}

// SetProtocol assigns a new transport protocol over UDP to handle the message delivery algorithms. May be NULL
func (server *Server) SetProtocol(protocol IProtocol) {
    server.protocol = protocol
}

// Close shuts down the Server
func (server *Server) Close() *Error {
    if server.socket != nil {
        err := server.socket.Close()
        return NewErrFromError(server, 8, err)
    }
    // TODO maybe also stop listening?
    return nil
}

// onReceived is called when a new incoming message "msg" is received from a client with a given CryptoRandom
// Connection ID "crcid"
func (server *Server) onReceived(crcid uint, msg []byte) /* implements IHandler */ {
    Assert(server.handler)

    // handling
    sid, resp := server.handler.Handle(msg)
    if sid > 0 {
        server.Lock()
        server.clients[sid] = crcid
        server.Unlock()
    }
    if len(resp) > 0 {
        Check(server.send(resp, crcid))
    }
}

// send transmits given data to a client, expressed by a given CryptoRandom Connection ID
func (server *Server) send(data []byte, crcid uint) *Error {
    Assert(server.socket)

    if server.protocol != nil {
        return server.protocol.Send(data, crcid)
    }
    return NewErr(server, 9, "No protocol found! Since 2017-05-12 server must have a protocol")
}
