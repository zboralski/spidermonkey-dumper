package callgraph

import (
	"fmt"
	"sort"
	"strings"

	"github.com/zboralski/spidermonkey-dumper/sm33"
	"github.com/zboralski/spidermonkey-dumper/sm33/bytecode"
)

// Control flow opcodes.
const (
	opGoto        = 6
	opIfeq        = 7
	opIfne        = 8
	opReturn      = 5
	opRetrval     = 153
	opThrow       = 112
	opOr          = 68
	opAnd         = 69
	opGosub       = 116
	opCase        = 121
	opDefault     = 122
	opTableswitch = 70
)

// Comparison opcodes — emit property access context.
const (
	opEq       = 18
	opNe       = 19
	opStrictEq = 72
	opStrictNe = 73
)

// CallSite records a call found during bytecode scanning.
type CallSite struct {
	Offset int
	Callee string
	Args   []string
}

// Successor describes a control flow edge to another basic block.
type Successor struct {
	BlockID int
	Cond    string // "" (unconditional), "T" (true), "F" (false)
}

// PropAccess records a property read or name lookup that isn't a call target.
type PropAccess struct {
	Name string
}

// BasicBlock is a straight-line sequence of bytecodes.
type BasicBlock struct {
	ID    int
	Start int
	End   int // exclusive
	Calls []CallSite
	Props []PropAccess // property accesses not consumed by calls
	Succs []Successor
	Term  bool // ends with return/throw
}

// FuncCFG is a per-function control flow graph.
type FuncCFG struct {
	Name     string
	Blocks   []*BasicBlock
	Children []int // indices of child functions in CFGGraph.Funcs
}

// CFGGraph holds the full program CFG.
type CFGGraph struct {
	Funcs []*FuncCFG
}

// BuildCFG constructs a control flow graph from a decoded Script.
func BuildCFG(s *sm33.Script) *CFGGraph {
	g := &CFGGraph{}
	g.walkCFG(s, "main")
	return g
}

func (g *CFGGraph) walkCFG(s *sm33.Script, name string) {
	parentIdx := len(g.Funcs)
	cfg := buildFuncCFG(s, name)
	g.Funcs = append(g.Funcs, cfg)

	for i, obj := range s.Objects {
		if obj.Kind != sm33.CkJSFunction || obj.Function == nil {
			continue
		}
		fn := obj.Function
		innerName := fn.Name
		if innerName == "" {
			innerName = fmt.Sprintf("anon#%d", i)
		}
		childIdx := len(g.Funcs)
		g.Funcs[parentIdx].Children = append(g.Funcs[parentIdx].Children, childIdx)
		if fn.Script != nil && !fn.IsLazy {
			g.walkCFG(fn.Script, innerName)
		} else {
			g.Funcs = append(g.Funcs, &FuncCFG{
				Name:   innerName,
				Blocks: []*BasicBlock{{ID: 0}},
			})
		}
	}
}

// buildFuncCFG splits a function's bytecode into basic blocks and annotates calls.
func buildFuncCFG(s *sm33.Script, name string) *FuncCFG {
	bc := s.Bytecode
	if len(bc) == 0 {
		return &FuncCFG{Name: name, Blocks: []*BasicBlock{{ID: 0}}}
	}

	// 1. Collect block boundary offsets
	labels := bytecode.CollectLabels(bc)
	blockStarts := map[int]bool{0: true}
	for label := range labels {
		if label >= 0 && label < len(bc) {
			blockStarts[label] = true
		}
	}

	// Instructions after branches/returns also start blocks
	off := 0
	for off < len(bc) {
		op := bc[off]
		n := bytecode.InstrLen(bc, off)
		if n <= 0 {
			break
		}
		switch op {
		case opGoto, opIfeq, opIfne, opOr, opAnd, opCase, opDefault, opGosub,
			opReturn, opRetrval, opThrow, opTableswitch:
			next := off + n
			if next < len(bc) {
				blockStarts[next] = true
			}
		}
		off += n
	}

	// 2. Sort starts, build blocks
	starts := make([]int, 0, len(blockStarts))
	for s := range blockStarts {
		starts = append(starts, s)
	}
	sort.Ints(starts)

	offsetToBlock := map[int]int{} // offset → block index
	blocks := make([]*BasicBlock, len(starts))
	for i, start := range starts {
		end := len(bc)
		if i+1 < len(starts) {
			end = starts[i+1]
		}
		blocks[i] = &BasicBlock{ID: i, Start: start, End: end}
		offsetToBlock[start] = i
	}

	// 3. Walk each block: find calls, property accesses, and successors
	for _, block := range blocks {
		off := block.Start
		var litBuf []string
		lastAtom := ""
		lastAtomOff := -1
		var propChain []string // tracks .foo.bar chains

		for off < block.End {
			op := bc[off]
			n := bytecode.InstrLen(bc, off)
			if n <= 0 {
				break
			}

			switch op {
			// Literal pushes
			case opString:
				if idx, ok := bytecode.GetUint32Index(bc, off); ok {
					if int(idx) < len(s.Atoms) {
						lit := s.Atoms[idx]
						if len(lit) > 24 {
							lit = lit[:24] + "\u2026"
						}
						litBuf = appendLit(litBuf, "\""+lit+"\"")
					}
				}
			case opDouble:
				if idx, ok := bytecode.GetUint32Index(bc, off); ok {
					if int(idx) < len(s.Consts) {
						litBuf = appendLit(litBuf, formatConstLit(s.Consts[idx]))
					}
				}
			case opInt8:
				if v, ok := bytecode.GetInt8(bc, off); ok {
					litBuf = appendLit(litBuf, fmt.Sprintf("%d", v))
				}
			case opInt32:
				if v, ok := bytecode.GetInt32(bc, off); ok {
					litBuf = appendLit(litBuf, fmt.Sprintf("%d", v))
				}
			case opUint16:
				if v, ok := bytecode.GetUint16(bc, off); ok {
					litBuf = appendLit(litBuf, fmt.Sprintf("%d", v))
				}
			case opUint24:
				if v, ok := bytecode.GetUint24(bc, off); ok {
					litBuf = appendLit(litBuf, fmt.Sprintf("%d", v))
				}
			case opZero:
				litBuf = appendLit(litBuf, "0")
			case opOne:
				litBuf = appendLit(litBuf, "1")
			case opNull:
				litBuf = appendLit(litBuf, "null")
			case opTrue:
				litBuf = appendLit(litBuf, "true")
			case opFalse:
				litBuf = appendLit(litBuf, "false")

			// Calls
			case opCallprop:
				if idx, ok := bytecode.GetUint32Index(bc, off); ok {
					if int(idx) < len(s.Atoms) {
						block.Calls = append(block.Calls, CallSite{
							Offset: off, Callee: s.Atoms[idx], Args: cloneLits(litBuf),
						})
					}
				}
				litBuf = litBuf[:0]
				lastAtom = ""
				propChain = propChain[:0]

			case opGetprop, opGetgname, opName:
				if idx, ok := bytecode.GetUint32Index(bc, off); ok {
					if int(idx) < len(s.Atoms) {
						atom := s.Atoms[idx]
						lastAtom = atom
						lastAtomOff = off
						propChain = append(propChain, atom)
					}
				}

			case opCall, opNew, opFuncall, opFunapply:
				if lastAtom != "" && off-lastAtomOff < 20 {
					block.Calls = append(block.Calls, CallSite{
						Offset: off, Callee: lastAtom, Args: cloneLits(litBuf),
					})
				}
				litBuf = litBuf[:0]
				lastAtom = ""
				propChain = propChain[:0]

			// Comparisons — emit property chain with compared value
			case opEq, opNe, opStrictEq, opStrictNe:
				if len(propChain) > 0 {
					chain := strings.Join(propChain, ".")
					cmpOp := "=="
					if op == opNe || op == opStrictNe {
						cmpOp = "!="
					}
					if op == opStrictEq || op == opStrictNe {
						cmpOp += "="
					}
					label := chain
					if len(litBuf) > 0 {
						label += " " + cmpOp + " " + litBuf[len(litBuf)-1]
					}
					block.Props = append(block.Props, PropAccess{Name: label})
					propChain = propChain[:0]
					litBuf = litBuf[:0]
				}

			// Successors (control flow)
			case opGoto:
				if jumpOff, ok := bytecode.GetJumpOffset(bc, off); ok {
					target := off + int(jumpOff)
					if bid, ok := offsetToBlock[target]; ok {
						block.Succs = append(block.Succs, Successor{BlockID: bid})
					}
				}
				block.Term = true

			case opIfeq:
				// ifeq: jump if falsy → F branch, fall through → T branch
				if jumpOff, ok := bytecode.GetJumpOffset(bc, off); ok {
					target := off + int(jumpOff)
					fallThrough := off + n
					if bid, ok := offsetToBlock[fallThrough]; ok {
						block.Succs = append(block.Succs, Successor{BlockID: bid, Cond: "T"})
					}
					if bid, ok := offsetToBlock[target]; ok {
						block.Succs = append(block.Succs, Successor{BlockID: bid, Cond: "F"})
					}
				}
				block.Term = true

			case opIfne:
				// ifne: jump if truthy → T branch, fall through → F branch
				if jumpOff, ok := bytecode.GetJumpOffset(bc, off); ok {
					target := off + int(jumpOff)
					fallThrough := off + n
					if bid, ok := offsetToBlock[fallThrough]; ok {
						block.Succs = append(block.Succs, Successor{BlockID: bid, Cond: "F"})
					}
					if bid, ok := offsetToBlock[target]; ok {
						block.Succs = append(block.Succs, Successor{BlockID: bid, Cond: "T"})
					}
				}
				block.Term = true

			case opOr, opAnd:
				// Short-circuit: jump or fall through
				if jumpOff, ok := bytecode.GetJumpOffset(bc, off); ok {
					target := off + int(jumpOff)
					fallThrough := off + n
					if bid, ok := offsetToBlock[fallThrough]; ok {
						block.Succs = append(block.Succs, Successor{BlockID: bid})
					}
					if bid, ok := offsetToBlock[target]; ok {
						block.Succs = append(block.Succs, Successor{BlockID: bid})
					}
				}
				block.Term = true

			case opCase:
				if jumpOff, ok := bytecode.GetJumpOffset(bc, off); ok {
					target := off + int(jumpOff)
					fallThrough := off + n
					if bid, ok := offsetToBlock[fallThrough]; ok {
						block.Succs = append(block.Succs, Successor{BlockID: bid})
					}
					if bid, ok := offsetToBlock[target]; ok {
						block.Succs = append(block.Succs, Successor{BlockID: bid})
					}
				}
				block.Term = true

			case opDefault, opGosub:
				if jumpOff, ok := bytecode.GetJumpOffset(bc, off); ok {
					target := off + int(jumpOff)
					if bid, ok := offsetToBlock[target]; ok {
						block.Succs = append(block.Succs, Successor{BlockID: bid})
					}
				}
				block.Term = true

			case opReturn, opRetrval, opThrow:
				block.Term = true
			}

			off += n
		}

		// Flush remaining property chain (not consumed by call or comparison)
		if len(propChain) > 0 {
			block.Props = append(block.Props, PropAccess{Name: strings.Join(propChain, ".")})
		}

		// Non-terminal blocks fall through to next block
		if !block.Term {
			if bid, ok := offsetToBlock[block.End]; ok {
				block.Succs = append(block.Succs, Successor{BlockID: bid})
			}
		}
		// Handle goto/jump that set Term but are really just unconditional jumps
		// — already handled above in the switch cases

	}

	return &FuncCFG{Name: name, Blocks: blocks}
}
