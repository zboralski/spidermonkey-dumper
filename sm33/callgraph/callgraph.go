package callgraph

import (
	"fmt"

	"github.com/zboralski/spidermonkey-dumper/sm33"
	"github.com/zboralski/spidermonkey-dumper/sm33/bytecode"
)

// Edge represents a call from one function to another.
type Edge struct {
	Caller string
	Callee string
	Args   []string // literal arguments observed at call site
}

// Graph holds the callgraph for a decoded script.
type Graph struct {
	Nodes []string // function names
	Edges []Edge
}

// opcode constants for call pattern detection.
const (
	opGetprop  = 53
	opCall     = 58
	opName     = 59
	opNew      = 82
	opFuncall  = 108
	opFunapply = 79
	opGetgname = 154
	opCallprop = 184
)

// opcode constants for literal-pushing instructions.
const (
	opString = 61  // JOF_ATOM — pushes string from atom table
	opDouble = 60  // JOF_DOUBLE — pushes double from consts
	opInt8   = 215 // JOF_INT8
	opInt32  = 216 // JOF_INT32
	opUint16 = 88  // JOF_UINT16
	opUint24 = 188 // JOF_UINT24
	opZero   = 62
	opOne    = 63
	opNull   = 64
	opTrue   = 67
	opFalse  = 66
)

// Build constructs a callgraph from a decoded Script.
func Build(s *sm33.Script) *Graph {
	g := &Graph{}
	g.walkScript(s, "main")
	g.dedup()
	return g
}

// callInfo pairs a callee name with observed literal arguments.
type callInfo struct {
	callee string
	args   []string
}

// walkScript extracts calls from a single script and recurses into inner functions.
func (g *Graph) walkScript(s *sm33.Script, name string) {
	g.Nodes = append(g.Nodes, name)

	// Scan bytecode for call patterns
	calls := scanCalls(s)
	for _, ci := range calls {
		g.Edges = append(g.Edges, Edge{Caller: name, Callee: ci.callee, Args: ci.args})
	}

	// Recurse into inner functions
	for i, obj := range s.Objects {
		if obj.Kind != sm33.CkJSFunction || obj.Function == nil {
			continue
		}
		fn := obj.Function
		innerName := fn.Name
		if innerName == "" {
			innerName = fmt.Sprintf("anon#%d", i)
		}

		// The defining script contains this function
		g.Edges = append(g.Edges, Edge{Caller: name, Callee: innerName})

		if fn.Script != nil && !fn.IsLazy {
			g.walkScript(fn.Script, innerName)
		} else {
			g.Nodes = append(g.Nodes, innerName)
		}
	}
}

// scanCalls finds call targets and their literal arguments by scanning bytecode.
func scanCalls(s *sm33.Script) []callInfo {
	bc := s.Bytecode
	var calls []callInfo
	seen := map[string]bool{}

	// Track last atom seen from getprop/getgname/name
	lastAtom := ""
	lastAtomOff := -1

	// Track recently pushed literals (strings, numbers, booleans)
	var litBuf []string

	off := 0
	for off < len(bc) {
		op := bc[off]
		n := bytecode.InstrLen(bc, off)
		if n <= 0 {
			break
		}

		switch op {
		// Literal-pushing opcodes — accumulate for arg tracking
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

		// Call opcodes — emit edge with captured literals
		case opCallprop:
			if idx, ok := bytecode.GetUint32Index(bc, off); ok {
				if int(idx) < len(s.Atoms) {
					atom := s.Atoms[idx]
					if !seen[atom] {
						seen[atom] = true
						calls = append(calls, callInfo{callee: atom, args: cloneLits(litBuf)})
					}
				}
			}
			litBuf = litBuf[:0]
			lastAtom = ""

		case opGetprop, opGetgname, opName:
			if idx, ok := bytecode.GetUint32Index(bc, off); ok {
				if int(idx) < len(s.Atoms) {
					lastAtom = s.Atoms[idx]
					lastAtomOff = off
				}
			}

		case opCall, opNew, opFuncall, opFunapply:
			if lastAtom != "" && off-lastAtomOff < 20 {
				if !seen[lastAtom] {
					seen[lastAtom] = true
					calls = append(calls, callInfo{callee: lastAtom, args: cloneLits(litBuf)})
				}
			}
			litBuf = litBuf[:0]
			lastAtom = ""
		}

		off += n
	}

	return calls
}

// appendLit adds a literal to the buffer, capped at 6.
func appendLit(buf []string, lit string) []string {
	if len(buf) >= 6 {
		return buf
	}
	return append(buf, lit)
}

// cloneLits returns a copy of the literal buffer, or nil if empty.
func cloneLits(buf []string) []string {
	if len(buf) == 0 {
		return nil
	}
	cp := make([]string, len(buf))
	copy(cp, buf)
	return cp
}

// formatConstLit renders a Const as a short literal string.
func formatConstLit(c sm33.Const) string {
	switch c.Kind {
	case sm33.ConstInt:
		return fmt.Sprintf("%d", c.Int)
	case sm33.ConstDouble:
		return fmt.Sprintf("%g", c.Double)
	case sm33.ConstAtom:
		s := c.Atom
		if len(s) > 24 {
			s = s[:24] + "\u2026"
		}
		return "\"" + s + "\""
	case sm33.ConstTrue:
		return "true"
	case sm33.ConstFalse:
		return "false"
	case sm33.ConstNull:
		return "null"
	default:
		return ""
	}
}

// dedup removes duplicate nodes.
func (g *Graph) dedup() {
	seen := map[string]bool{}
	var nodes []string
	for _, n := range g.Nodes {
		if !seen[n] {
			seen[n] = true
			nodes = append(nodes, n)
		}
	}
	g.Nodes = nodes
}
