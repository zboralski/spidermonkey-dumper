package callgraph

import (
	"fmt"

	"github.com/zboralski/lattice"
	"github.com/zboralski/spidermonkey-dumper/sm"
	"github.com/zboralski/spidermonkey-dumper/sm/bytecode"
)

// builder holds internal state for graph construction.
type builder struct {
	graph *lattice.Graph
	ops   *[256]bytecode.OpInfo
}

// Build constructs a callgraph from a decoded Script.
// Panics if ops is nil.
func Build(s *sm.Script, ops *[256]bytecode.OpInfo) *lattice.Graph {
	if ops == nil {
		panic("callgraph.Build: ops must not be nil")
	}
	b := &builder{
		graph: &lattice.Graph{},
		ops:   ops,
	}
	b.walkScript(s, "main")
	b.graph.Dedup()
	return b.graph
}

// callInfo pairs a callee name with observed literal arguments.
type callInfo struct {
	callee string
	args   []string
}

// walkScript extracts calls from a single script and recurses into inner functions.
func (b *builder) walkScript(s *sm.Script, name string) {
	b.graph.Nodes = append(b.graph.Nodes, name)

	// Scan bytecode for call patterns
	calls := scanCalls(s, b.ops)
	for _, ci := range calls {
		b.graph.Edges = append(b.graph.Edges, lattice.Edge{Caller: name, Callee: ci.callee, Args: ci.args})
	}

	// Recurse into inner functions
	for i, obj := range s.Objects {
		if obj.Kind != sm.CkJSFunction || obj.Function == nil {
			continue
		}
		fn := obj.Function
		innerName := fn.Name
		if innerName == "" {
			innerName = fmt.Sprintf("anon#%d", i)
		}

		// The defining script contains this function
		b.graph.Edges = append(b.graph.Edges, lattice.Edge{Caller: name, Callee: innerName})

		if fn.Script != nil && !fn.IsLazy {
			b.walkScript(fn.Script, innerName)
		} else {
			b.graph.Nodes = append(b.graph.Nodes, innerName)
		}
	}
}

// scanCalls finds call targets and their literal arguments by scanning bytecode.
// Opcode matching uses the ops table by name, so this works across SM versions.
func scanCalls(s *sm.Script, ops *[256]bytecode.OpInfo) []callInfo {
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
		n := bytecode.InstrLen(bc, off, ops)
		if n <= 0 {
			// Unknown instruction — skip 1 byte and keep scanning for calls
			off++
			continue
		}
		name := ops[op].Name

		switch name {
		// Literal-pushing opcodes — accumulate for arg tracking
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

		// Call opcodes — emit edge with captured literals
		case "callprop":
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

		case "getprop", "getgname", "name",
			"callname", "callgname":
			// v28 uses callname/callgname as combined name+this push opcodes.
			if idx, ok := bytecode.GetUint32Index(bc, off); ok {
				if int(idx) < len(s.Atoms) {
					lastAtom = s.Atoms[idx]
					lastAtomOff = off
				}
			}

		case "call", "new", "funcall", "funapply":
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
func formatConstLit(c sm.Const) string {
	switch c.Kind {
	case sm.ConstInt:
		return fmt.Sprintf("%d", c.Int)
	case sm.ConstDouble:
		return fmt.Sprintf("%g", c.Double)
	case sm.ConstAtom:
		s := c.Atom
		if len(s) > 24 {
			s = s[:24] + "\u2026"
		}
		return "\"" + s + "\""
	case sm.ConstTrue:
		return "true"
	case sm.ConstFalse:
		return "false"
	case sm.ConstNull:
		return "null"
	default:
		return ""
	}
}
