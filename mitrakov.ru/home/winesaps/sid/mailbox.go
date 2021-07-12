package sid

import "sync"
import "container/list"
import . "mitrakov.ru/home/winesaps/utils" // nolint

// dataT is a helper data class to store N messages and a single prefix for them
type dataT struct {
    prefix []byte
    chunks list.List // List[ByteArray]
}

// MailBox is a special container to accumulate "pending-to-send" data, in order to send N messages in a single UDP
// packet instead of N different messages. It will help to improve latency between client and server
type MailBox struct {
    sync.RWMutex
    msgs map[Sid]*dataT
}

// NewMailBox creates a new instance of MailBox. Please don't create a MailBox manually.
func NewMailBox() *MailBox {
    return &MailBox{msgs: make(map[Sid]*dataT)}
}

// SetPrefix assigns a new prefix for a given SID, thereby prepends data in the beginning of the resulting message.
// NOTE: the prefix is single, i.e. this action will overwrite the existing prefix.
// "sid" - MailBox key (because a MailBox can contain data for several SIDs)
// "prefix" - arbitrary byte array to be prepended
func (box *MailBox) SetPrefix(sid Sid, prefix []byte) *MailBox {
    Assert(box.msgs)
    box.Lock()
    box.getData(sid).prefix = prefix
    box.Unlock()
    return box
}

// Put adds new message for a given Session ID, thereby appends data at the end of the resulting message.
// You can call Put several times, and the order of messages will be preserved
// "sid" - MailBox key (because a MailBox can contain data for several SIDs)
// "msg" - message to be appended
func (box *MailBox) Put(sid Sid, msg []byte) *MailBox {
    Assert(box.msgs)
    box.Lock()
    box.getData(sid).chunks.PushBack(msg)
    box.Unlock()
    return box
}

// Pick retrieves all messages for a given Session ID, but doesn't remove them from the MailBox
func (box *MailBox) Pick(sid Sid) (res [][]byte) {
    Assert(box.msgs)
    dt := box.getData(sid)
    box.RLock()
    for j := dt.chunks.Front(); j != nil; j = j.Next() {
        if msg, ok := j.Value.([]byte); ok {
            res = append(res, msg)
        }
    }
    box.RUnlock()
    return
}

// GetSids returns all the Session IDs, accumulated in the MailBox so far
func (box *MailBox) GetSids() []Sid {
    Assert(box.msgs)
    box.RLock()
    res := make([]Sid, len(box.msgs))
    i := 0
    for sid := range box.msgs {
        res[i] = sid
        i++
    }
    box.RUnlock()
    return res
}

// Remove deletes all messages (as well as a prefix) for a given Session ID
func (box *MailBox) Remove(sid Sid) {
    Assert(box.msgs)
    box.Lock()
    delete(box.msgs, sid)
    box.Unlock()
}

// Flush unpacks the MailBox to array of messages, so that each element of the array corresponds to a given Session ID.
// The length of result types "sids" and "messages" is guaranteed to be the same.
// This action will ERASE the MailBox (and theoretically it can be re-used again)
func (box *MailBox) Flush() (sids []Sid, messages [][]byte) {
    Assert(box.msgs)
    box.Lock()
    
    for sid, dt := range box.msgs {
        data := dt.prefix
        for j := dt.chunks.Front(); j != nil; j = j.Next() {
            if msg, ok := j.Value.([]byte); ok {
                data = append(data, byte(len(msg)/256), byte(len(msg)%256))
                data = append(data, msg...)
            }
        }
        sids = append(sids, sid)
        messages = append(messages, data)
    }
    box.msgs = make(map[Sid]*dataT)
    
    box.Unlock()
    return
}

// getData returns "dataT" structure by a given Session ID.
// Always use this method and never access "msgs" map directly.
func (box *MailBox) getData(sid Sid) *dataT {
    // please ensure the context is synchronized!
    if data, ok := box.msgs[sid]; ok {
        return data
    }
    data := new(dataT)
    box.msgs[sid] = data
    return data
}
