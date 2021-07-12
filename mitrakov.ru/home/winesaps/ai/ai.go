package ai

import "sync"
import . "mitrakov.ru/home/winesaps/utils" // nolint

// Path of AI movement (e.g. path [0, 1, 2, 3] means that AI goes 3 steps right from the left-top corner)
type pathT []byte

// Ai is a component that represents AI actor. One instance is for one user enemy.
// This component is independent.
type Ai struct {
    sync.Mutex
    pause        uint8
    delayedPause uint8
    toolDelay    byte
    curTool      byte
    curPath      pathT
    graph        *Graph
    xyFunc       func() byte
    bewareFunc   func(byte) bool // check if the AI should beware of a node with a given index 
    resources    map[byte]bool   // xy -> resourceExists
    tools        map[byte]byte   // xy -> toolID
}

// min distance to a node we beware of
const bewareDistance = 4
// if we beware of someone, we gonna run away at `fleeSteps` steps
const fleeSteps = 6

// NewAi creates a new Ai. Please do not create a Ai directly.
// "xyFunc" - function to locate an AI actor on the battle field in case of reset
// "bewareFunc" - function to determine if AI should be scared of a cell with given coordinates
func NewAi(xyFunc func() byte, bewareFunc func(byte) bool) *Ai {
    return &Ai{xyFunc: xyFunc, bewareFunc: bewareFunc, resources: make(map[byte]bool), tools: make(map[byte]byte)}
}

// Init initializes AI with a given graph
func (ai *Ai) Init(graph *Graph) {
    ai.Lock()
    ai.pause = 0
    ai.delayedPause = 0
    ai.toolDelay = 0
    ai.curTool = 0
    ai.curPath = make([]byte, 0)
    ai.graph = graph
    ai.Unlock()
}

// Reset clears current path of the AI
func (ai *Ai) Reset() {
    ai.Lock()
    ai.curPath = make([]byte, 0)
    ai.Unlock()
}

// SetResource sets or unsets the resource in a given graph node
// "idx" - graph node number
// "value" - TRUE to set the resourse, and FALSE to unset one
func (ai *Ai) SetResource(idx byte, value bool) {
    Assert(ai.resources)
    ai.Lock()
    ai.resources[idx] = value
    ai.Unlock()
}

// SetTool sets or unsets the tool in a given graph node
// "idx" - graph node number
// "value" - TRUE to set the tool, and FALSE to unset one
func (ai *Ai) SetTool(idx byte, value byte) {
    Assert(ai.tools)
    ai.Lock()
    ai.tools[idx] = value
    ai.Unlock()
}

// SetCurTool assigns current tool for the AI
// "value" - tool ID
func (ai *Ai) SetCurTool(value byte) {
    ai.Lock()
    ai.curTool = value
    ai.Unlock()
}

// SetPauseSteps sets delay for the AI in "steps". The AI will do nothing during this time
// (by default 5 steps stand for 1 sec)
func (ai *Ai) SetPauseSteps(steps uint8) {
    ai.Lock()
    ai.pause = steps
    ai.Unlock()
}

// SetDelayedPauseSteps is the same as SetPauseSteps, except that the AI will get paused only AFTER he finishes his
// current path
func (ai *Ai) SetDelayedPauseSteps(steps uint8) {
    ai.Lock()
    ai.delayedPause = steps
    ai.Unlock()
}

// Step performs a single step of AI
// nolint: gocyclo
func (ai *Ai) Step() (idxFrom, idxTo byte, useTool bool, err *Error) {
    ai.Lock()
    defer ai.Unlock()
    
    if ai.graph != nil {
        // if we don't know where we are
        if len(ai.curPath) == 0 {
            ai.curPath = []byte{ai.xyFunc()}
        }
        // check delayed pause
        if len(ai.curPath) == 1 && ai.delayedPause > 0 {
            ai.pause = ai.delayedPause
            ai.delayedPause = 0
        }
        // there is no current goal: let's find it
        if len(ai.curPath) == 1 {
            ai.curPath = ai.graph.traverse(ai.curPath[0], ai.resources, false, 0xFF, 0xFF)
        }
        // still no current goal? Maybe dangers block the way? So let's find path taking dangers into account
        if len(ai.curPath) == 1 {
            dangerPath := ai.graph.traverse(ai.curPath[0], ai.resources, true, 0xFF, 0xFF)
            
            if len(dangerPath) > 1 {
                // find danger type and danger index in the path
                danger, dangerIdx := byte(0), 0
                for i, v := range dangerPath {
                    node := ai.graph.GetNode(v)
                    Assert(node)
                    if node.danger > 0 {
                        danger, dangerIdx = node.danger, i
                        break
                    }
                }
                if danger == 0 {
                    return 0, 0, false, NewErr(ai, 2, "AI broken")
                }
                
                // if we've got required tool => prepare to use it in the future, otherwise find it on the battlefield
                if ai.curTool == danger {
                    ai.curPath = dangerPath
                    ai.toolDelay = byte(dangerIdx)
                } else {
                    for k, v := range ai.tools {
                        if v == danger {
                            toolPath := ai.graph.traverse(ai.curPath[0], map[byte]bool{k: true}, false, 0xFF, 0xFF)
                            if len(toolPath) > 1 {
                                ai.curPath = toolPath
                                break
                            }
                        }
                    }
                }
            }
        }
        
        // do we beware of someone?
        if ok, bewareIdx := ai.isBeware(); ok {
            ai.pause = 0
            ai.curPath = ai.graph.traverse(ai.curPath[0], ai.resources, false, bewareIdx, fleeSteps)
        }
        
        // move
        idxFrom, idxTo = ai.curPath[0], ai.curPath[0]
        if ai.pause > 0 {
            ai.pause--
        } else if len(ai.curPath) > 1 {
            idxTo = ai.curPath[1]
            ai.curPath = ai.curPath[1:]
            if ai.toolDelay > 0 {
                ai.toolDelay--
                useTool = ai.toolDelay == 0
            }
        }
        // make the node non-dangerous
        if useTool {
            node := ai.graph.GetNode(idxTo)
            Assert(node)
            node.danger = 0
        }
    } else {
        err = NewErr(ai, 3, "AI is not initialized")
    }
    return
}

// isBeware checks whether the AI is afraid of any node within the limits of "bewareDistance" steps
// Returns (true, index_of_node) if yes, and (false, 255) if not
func (ai *Ai) isBeware() (res bool, idx byte) {
    for i:=1; i<=bewareDistance; i++ {
        if len(ai.curPath) > i && ai.bewareFunc(ai.curPath[i]) {
            return true, ai.curPath[i]
        }
    }
    return false, 0xFF
}
