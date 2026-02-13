package callgraph

import (
	"fmt"
	"sort"
	"strings"

	"github.com/zboralski/lattice"
	"github.com/zboralski/spidermonkey-dumper/sm"
	"github.com/zboralski/spidermonkey-dumper/sm/bytecode"
)

// cfgBuilder holds internal state for CFG construction.
type cfgBuilder struct {
	graph *lattice.CFGGraph
	ops   *[256]bytecode.OpInfo
}

// BuildCFG constructs a control flow graph from a decoded Script.
// Panics if ops is nil.
func BuildCFG(s *sm.Script, ops *[256]bytecode.OpInfo) *lattice.CFGGraph {
	if ops == nil {
		panic("callgraph.BuildCFG: ops must not be nil")
	}
	b := &cfgBuilder{
		graph: &lattice.CFGGraph{},
		ops:   ops,
	}
	b.walkCFG(s, "main")
	return b.graph
}

func (b *cfgBuilder) walkCFG(s *sm.Script, name string) {
	parentIdx := len(b.graph.Funcs)
	cfg := buildFuncCFG(s, name, b.ops)
	b.graph.Funcs = append(b.graph.Funcs, cfg)

	for i, obj := range s.Objects {
		if obj.Kind != sm.CkJSFunction || obj.Function == nil {
			continue
		}
		fn := obj.Function
		innerName := fn.Name
		if innerName == "" {
			innerName = fmt.Sprintf("anon#%d", i)
		}
		childIdx := len(b.graph.Funcs)
		b.graph.Funcs[parentIdx].Children = append(b.graph.Funcs[parentIdx].Children, childIdx)
		if fn.Script != nil && !fn.IsLazy {
			b.walkCFG(fn.Script, innerName)
		} else {
			b.graph.Funcs = append(b.graph.Funcs, &lattice.FuncCFG{
				Name:   innerName,
				Blocks: []*lattice.BasicBlock{{ID: 0}},
			})
		}
	}
}

// isBlockTerminator returns true if the named opcode ends a basic block.
func isBlockTerminator(name string) bool {
	switch name {
	case "goto", "ifeq", "ifne", "or", "and", "case", "default", "gosub",
		"return", "retrval", "throw", "tableswitch":
		return true
	}
	return false
}

// buildFuncCFG splits a function's bytecode into basic blocks and annotates calls.
// All opcode matching uses the ops table by name for version independence.
func buildFuncCFG(s *sm.Script, name string, ops *[256]bytecode.OpInfo) *lattice.FuncCFG {
	bc := s.Bytecode
	if len(bc) == 0 {
		return &lattice.FuncCFG{Name: name, Blocks: []*lattice.BasicBlock{{ID: 0}}}
	}

	// 1. Collect block boundary offsets
	labels := bytecode.CollectLabels(bc, ops)
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
		n := bytecode.InstrLen(bc, off, ops)
		if n <= 0 {
			// Unknown instruction — skip 1 byte and keep discovering blocks
			off++
			continue
		}
		if isBlockTerminator(ops[op].Name) {
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
	blocks := make([]*lattice.BasicBlock, len(starts))
	for i, start := range starts {
		end := len(bc)
		if i+1 < len(starts) {
			end = starts[i+1]
		}
		blocks[i] = &lattice.BasicBlock{ID: i, Start: start, End: end}
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
			n := bytecode.InstrLen(bc, off, ops)
			if n <= 0 {
				// Unlike block discovery (which skips 1 byte to find boundaries),
				// we break here: advancing 1 byte risks misinterpreting mid-instruction
				// bytes as opcodes, corrupting call/successor analysis for this block.
				break
			}
			opName := ops[op].Name

			switch opName {
			// Literal pushes
			case "string":
				if idx, ok := bytecode.GetUint32Index(bc, off); ok {
					if int(idx) < len(s.Atoms) {
						lit := s.Atoms[idx]
						if len(lit) > 24 {
							lit = lit[:24] + "\u2026"
						}
						litBuf = appendLit(litBuf, "\""+lit+"\"")
					}
				}
			case "double":
				if idx, ok := bytecode.GetUint32Index(bc, off); ok {
					if int(idx) < len(s.Consts) {
						litBuf = appendLit(litBuf, formatConstLit(s.Consts[idx]))
					}
				}
			case "int8":
				if v, ok := bytecode.GetInt8(bc, off); ok {
					litBuf = appendLit(litBuf, fmt.Sprintf("%d", v))
				}
			case "int32":
				if v, ok := bytecode.GetInt32(bc, off); ok {
					litBuf = appendLit(litBuf, fmt.Sprintf("%d", v))
				}
			case "uint16":
				if v, ok := bytecode.GetUint16(bc, off); ok {
					litBuf = appendLit(litBuf, fmt.Sprintf("%d", v))
				}
			case "uint24":
				if v, ok := bytecode.GetUint24(bc, off); ok {
					litBuf = appendLit(litBuf, fmt.Sprintf("%d", v))
				}
			case "zero":
				litBuf = appendLit(litBuf, "0")
			case "one":
				litBuf = appendLit(litBuf, "1")
			case "null":
				litBuf = appendLit(litBuf, "null")
			case "true":
				litBuf = appendLit(litBuf, "true")
			case "false":
				litBuf = appendLit(litBuf, "false")

			// Calls
			case "callprop":
				if idx, ok := bytecode.GetUint32Index(bc, off); ok {
					if int(idx) < len(s.Atoms) {
						block.Calls = append(block.Calls, lattice.CallSite{
							Offset: off, Callee: s.Atoms[idx], Args: cloneLits(litBuf),
						})
					}
				}
				litBuf = litBuf[:0]
				lastAtom = ""
				propChain = propChain[:0]

			case "getprop", "getgname", "name",
				"callname", "callgname":
				// v28 uses callname/callgname as combined name+this push opcodes.
				if idx, ok := bytecode.GetUint32Index(bc, off); ok {
					if int(idx) < len(s.Atoms) {
						atom := s.Atoms[idx]
						lastAtom = atom
						lastAtomOff = off
						propChain = append(propChain, atom)
					}
				}

			case "call", "new", "funcall", "funapply":
				if lastAtom != "" && off-lastAtomOff < 20 {
					block.Calls = append(block.Calls, lattice.CallSite{
						Offset: off, Callee: lastAtom, Args: cloneLits(litBuf),
					})
				}
				litBuf = litBuf[:0]
				lastAtom = ""
				propChain = propChain[:0]

			// Comparisons — emit property chain with compared value
			case "eq", "ne", "stricteq", "strictne":
				if len(propChain) > 0 {
					chain := strings.Join(propChain, ".")
					cmpOp := "=="
					if opName == "ne" || opName == "strictne" {
						cmpOp = "!="
					}
					if opName == "stricteq" || opName == "strictne" {
						cmpOp += "="
					}
					label := chain
					if len(litBuf) > 0 {
						label += " " + cmpOp + " " + litBuf[len(litBuf)-1]
					}
					block.Props = append(block.Props, lattice.PropAccess{Name: label})
					propChain = propChain[:0]
					litBuf = litBuf[:0]
				}

			// Successors (control flow)
			case "goto":
				if jumpOff, ok := bytecode.GetJumpOffset(bc, off); ok {
					target := off + int(jumpOff)
					if bid, ok := offsetToBlock[target]; ok {
						block.Succs = append(block.Succs, lattice.Successor{BlockID: bid})
					}
				}
				block.Term = true

			case "ifeq":
				// ifeq: jump if falsy → F branch, fall through → T branch
				if jumpOff, ok := bytecode.GetJumpOffset(bc, off); ok {
					target := off + int(jumpOff)
					fallThrough := off + n
					if bid, ok := offsetToBlock[fallThrough]; ok {
						block.Succs = append(block.Succs, lattice.Successor{BlockID: bid, Cond: "T"})
					}
					if bid, ok := offsetToBlock[target]; ok {
						block.Succs = append(block.Succs, lattice.Successor{BlockID: bid, Cond: "F"})
					}
				}
				block.Term = true

			case "ifne":
				// ifne: jump if truthy → T branch, fall through → F branch
				if jumpOff, ok := bytecode.GetJumpOffset(bc, off); ok {
					target := off + int(jumpOff)
					fallThrough := off + n
					if bid, ok := offsetToBlock[fallThrough]; ok {
						block.Succs = append(block.Succs, lattice.Successor{BlockID: bid, Cond: "F"})
					}
					if bid, ok := offsetToBlock[target]; ok {
						block.Succs = append(block.Succs, lattice.Successor{BlockID: bid, Cond: "T"})
					}
				}
				block.Term = true

			case "or", "and":
				// Short-circuit: jump or fall through
				if jumpOff, ok := bytecode.GetJumpOffset(bc, off); ok {
					target := off + int(jumpOff)
					fallThrough := off + n
					if bid, ok := offsetToBlock[fallThrough]; ok {
						block.Succs = append(block.Succs, lattice.Successor{BlockID: bid})
					}
					if bid, ok := offsetToBlock[target]; ok {
						block.Succs = append(block.Succs, lattice.Successor{BlockID: bid})
					}
				}
				block.Term = true

			case "case":
				if jumpOff, ok := bytecode.GetJumpOffset(bc, off); ok {
					target := off + int(jumpOff)
					fallThrough := off + n
					if bid, ok := offsetToBlock[fallThrough]; ok {
						block.Succs = append(block.Succs, lattice.Successor{BlockID: bid})
					}
					if bid, ok := offsetToBlock[target]; ok {
						block.Succs = append(block.Succs, lattice.Successor{BlockID: bid})
					}
				}
				block.Term = true

			case "default", "gosub":
				if jumpOff, ok := bytecode.GetJumpOffset(bc, off); ok {
					target := off + int(jumpOff)
					if bid, ok := offsetToBlock[target]; ok {
						block.Succs = append(block.Succs, lattice.Successor{BlockID: bid})
					}
				}
				block.Term = true

			case "return", "retrval", "throw":
				block.Term = true
			}

			off += n
		}

		// Flush remaining property chain (not consumed by call or comparison)
		if len(propChain) > 0 {
			block.Props = append(block.Props, lattice.PropAccess{Name: strings.Join(propChain, ".")})
		}

		// Non-terminal blocks fall through to next block
		if !block.Term {
			if bid, ok := offsetToBlock[block.End]; ok {
				block.Succs = append(block.Succs, lattice.Successor{BlockID: bid})
			}
		}
	}

	return &lattice.FuncCFG{Name: name, Blocks: blocks}
}
