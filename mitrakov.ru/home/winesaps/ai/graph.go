package ai

import "fmt"
import "bytes"
import "strconv"
import "math/rand"
import . "mitrakov.ru/home/winesaps/utils" // nolint

// NodeT represents a single graph node
type NodeT struct {
    n byte
    colored bool
    danger  byte
    arcs [4]*NodeT
}

// Graph structure represents a graph: set of nodes connected with set of arcs
type Graph struct {
    nodes [256]*NodeT
}

// AddNode adds a node with number "n" to the graph. "danger" parameter indicates what resource should be used to
// eliminate the danger.
// Pass 0 if the node is not dangerous
func (graph *Graph) AddNode(n byte, danger byte) {
    node := new(NodeT)
    node.n = n
    node.danger = danger
    graph.nodes[n] = node
}

// AddArc connects a node number "n" with a node number "pointsTo". "direction" means the following:
// 0 = left
// 1 = right
// 2 = up
// 3 = down
func (graph *Graph) AddArc(n, direction, pointsTo byte) {
    node := graph.nodes[n]
    Assert(node)
    if 0 <= direction && direction < byte(len(node.arcs)) {
        node.arcs[direction] = graph.nodes[pointsTo]
    }
}

// GetNode returns a node by its number
func (graph Graph) GetNode(n byte) *NodeT {
    return graph.nodes[n]
}

// String represents a graph in details
func (graph Graph) String() string {
    var buffer bytes.Buffer
    for i:=0; i<len(graph.nodes); i++ {
        buffer.WriteString(fmt.Sprintf("%v  ", graph.nodes[i]))
    }
    return buffer.String()
}

// GetNext returns the next node according to "direction". Directions are the following:
// 0 = left
// 1 = right
// 2 = up
// 3 = down
func (node NodeT) GetNext(direction byte) *NodeT {
    if 0 <= direction && direction < byte(len(node.arcs)) {
        return node.arcs[direction]
    }
    return nil
}

// String represents a node as a string
func (node NodeT) String() string {
    arcs0, arcs1, arcs2, arcs3, dangerous := "-", "-", "-", "-", ""
    if node.arcs[0] != nil {
        arcs0 = strconv.Itoa(int(node.arcs[0].n))
    }
    if node.arcs[1] != nil {
        arcs1 = strconv.Itoa(int(node.arcs[1].n))
    }
    if node.arcs[2] != nil {
        arcs2 = strconv.Itoa(int(node.arcs[2].n))
    }
    if node.arcs[3] != nil {
        arcs3 = strconv.Itoa(int(node.arcs[3].n))
    }
    if node.danger > 0 {
        dangerous = "d"
    }
    return fmt.Sprintf("%d%s [%s, %s, %s, %s]", node.n, dangerous, arcs0, arcs1, arcs2, arcs3)
}

// traverse recursively traverses a graph trying to find out at least 1 resource, represented by "resources" map.
// Returns a full path found, represented as a byte array of node numbers (inclusively).
// "idx" - start point
// "resources" - resources map (xy -> resource_exists)
// "findDanger" - whether to take dangerous cells into account (default is false)
// "except" - AI should avoid a node with number "except" (pass 0xFF if it's not required)
// "maxLen" - restricts the total path length (default is 0xFF)
// nolint: gocyclo
func (graph *Graph) traverse(idx byte, resources map[byte]bool, findDanger bool, except, maxLen byte) (result []byte) {
    Assert(resources)
    
    // declare traverse recursive function
    var f func(*NodeT, []byte) (bool, []byte)
    f = func (node *NodeT, path []byte) (bool, []byte) {
        if node == nil { // it may happen after sudden teleporting; just move 1 step left or right to recover AI state
            return true, []byte{idx, Ternary(rand.Intn(2) == 0, idx + 1, idx - 1)}
        }
        
        // append new index to path
        path = append(path, node.n)
        
        // if max attained => return as is
        if len(path) == int(maxLen) {
            return true, path
        }
        
        // if resource found => finish traversing
        if v, ok := resources[node.n]; v && ok {
            return true, path
        }
    
        // mark current node as "colored"
        node.colored = true
        arcs := node.arcs
        variants := make([]*NodeT, len(arcs))
        variantsCnt := 0
        
        // find all non-colored arcs
        for i:=0; i<len(arcs); i++ {
            if arcs[i] != nil && !arcs[i].colored && arcs[i].n != except && (findDanger || arcs[i].danger==0) {
                variants[variantsCnt] = arcs[i]
                variantsCnt++
            }
        }
        // shuffling (see stackoverflow.com/questions/12264789)
        for i:=0; i<variantsCnt; i++ {
            j := rand.Intn(i+1)
            variants[i], variants[j] = variants[j], variants[i]
        }
        // fall in recursion
        for i:=0; i<variantsCnt; i++ {
            if ok, p := f(variants[i], path); ok {
                return ok, p
            }
        }
        return false, path
    }
    
    // start traversing
    _, result = f(graph.nodes[idx], make([]byte, 0))
    
    // clean up our graph after usage
    for _, node := range graph.nodes {
        if node != nil {
            node.colored = false
        }
    }
    return
}
