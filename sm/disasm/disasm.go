package disasm

import (
	"fmt"
	"strings"

	"github.com/zboralski/spidermonkey-dumper/sm"
	"github.com/zboralski/spidermonkey-dumper/sm/bytecode"
)

const commentCol = 60

// DisasmScriptOpt produces disassembly text with mode-aware error handling.
// ops must not be nil.
func DisasmScriptOpt(s *sm.Script, funcName string, header bool, opt sm.Options, ops *[256]bytecode.OpInfo) (sm.Result[string], error) {
	if ops == nil {
		return sm.Result[string]{}, fmt.Errorf("ops table must not be nil")
	}
	var b strings.Builder
	var diags []sm.Diagnostic
	bc := s.Bytecode
	labels := bytecode.CollectLabels(bc, ops)
	maxSteps := opt.EffectiveMaxSteps()

	if header {
		b.WriteString("loc     op\n")
		b.WriteString("-----   --\n")
	}

	off := 0
	first := true
	steps := 0
	for off < len(bc) {
		steps++
		if steps > maxSteps {
			if opt.Mode == sm.Strict {
				return sm.Result[string]{Value: b.String(), Diags: diags},
					fmt.Errorf("step limit %d exceeded at offset %d", maxSteps, off)
			}
			diags = append(diags, sm.Diagnostic{
				Offset: off,
				Kind:   "overflow",
				Msg:    fmt.Sprintf("step limit %d reached, truncating", maxSteps),
			})
			break
		}

		// Print function name label at mainOffset
		if uint32(off) == s.MainOffset {
			b.WriteString(funcName)
			b.WriteByte('\n')
		}

		op := bc[off]
		info := &ops[op]
		jt := bytecode.JofType(info.Format)

		// Unknown opcode: Name=="" and Length==0
		if info.Name == "" && info.Length == 0 {
			if opt.Mode == sm.Strict {
				return sm.Result[string]{Value: b.String(), Diags: diags},
					fmt.Errorf("unknown opcode 0x%02x at offset %d", op, off)
			}
			diags = append(diags, sm.Diagnostic{
				Offset: off,
				Kind:   "unknown_opcode",
				Msg:    fmt.Sprintf("unknown opcode 0x%02x", op),
			})
			// Emit placeholder and advance 1 byte
			addr := fmt.Sprintf("%05X", off)
			b.WriteString(addr)
			b.WriteString("  ")
			name := fmt.Sprintf("%-12s", fmt.Sprintf("OP_0x%02X", op))
			b.WriteString(name)
			pad := commentCol - len(addr) - 2 - len(name)
			if pad < 1 {
				pad = 1
			}
			b.WriteString(strings.Repeat(" ", pad))
			b.WriteString("; unknown opcode\n")
			off++
			first = false
			continue
		}

		// Label line
		if _, isLabel := labels[off]; isLabel {
			if !first {
				b.WriteByte('\n')
			}
			line := fmt.Sprintf("loc_%05X:", off)
			pad := commentCol - len(line)
			if pad < 1 {
				pad = 1
			}
			b.WriteString(line)
			b.WriteString(strings.Repeat(" ", pad))
			fmt.Fprintf(&b, "; L%d\n", off)
		}

		// Instruction line
		col := 0
		addr := fmt.Sprintf("%05X", off)
		b.WriteString(addr)
		col += len(addr)

		b.WriteString("  ")
		col += 2

		name := fmt.Sprintf("%-12s", info.Name)
		b.WriteString(name)
		col += len(name)

		// Operand
		operand := ""
		comment := ""
		truncated := false

		switch jt {
		case bytecode.JOF_JUMP:
			if jumpOff, ok := bytecode.GetJumpOffset(bc, off); ok {
				tgt := off + int(jumpOff)
				operand = fmt.Sprintf(" loc_%05X (%+d)", tgt, jumpOff)
			} else {
				truncated = true
			}

		case bytecode.JOF_ATOM:
			if idx, ok := bytecode.GetUint32Index(bc, off); ok {
				if int(idx) < len(s.Atoms) {
					operand = fmt.Sprintf(" %q", s.Atoms[idx])
				} else {
					operand = fmt.Sprintf(" <atom#%d>", idx)
				}
			} else {
				truncated = true
			}

		case bytecode.JOF_OBJECT:
			if idx, ok := bytecode.GetUint32Index(bc, off); ok {
				operand = fmt.Sprintf(" <object#%d>", idx)
			} else {
				truncated = true
			}

		case bytecode.JOF_REGEXP:
			if idx, ok := bytecode.GetUint32Index(bc, off); ok {
				if int(idx) < len(s.Regexps) {
					rx := s.Regexps[idx]
					operand = fmt.Sprintf(" /%s/%s", rx.Source, regexpFlags(rx.Flags))
				} else {
					operand = fmt.Sprintf(" <regexp#%d>", idx)
				}
			} else {
				truncated = true
			}

		case bytecode.JOF_UINT16:
			if val, ok := bytecode.GetUint16(bc, off); ok {
				operand = fmt.Sprintf(" %d", val)
			} else {
				truncated = true
			}

		case bytecode.JOF_UINT24:
			if val, ok := bytecode.GetUint24(bc, off); ok {
				operand = fmt.Sprintf(" %d", val)
			} else {
				truncated = true
			}

		case bytecode.JOF_UINT8:
			if off+2 <= len(bc) {
				operand = fmt.Sprintf(" %d", bc[off+1])
			} else {
				truncated = true
			}

		case bytecode.JOF_INT8:
			if val, ok := bytecode.GetInt8(bc, off); ok {
				operand = fmt.Sprintf(" %d", val)
			} else {
				truncated = true
			}

		case bytecode.JOF_INT32:
			if val, ok := bytecode.GetInt32(bc, off); ok {
				operand = fmt.Sprintf(" %d", val)
			} else {
				truncated = true
			}

		case bytecode.JOF_QARG:
			if val, ok := bytecode.GetArgno(bc, off); ok {
				operand = fmt.Sprintf(" %d", val)
				comment = fmt.Sprintf("arg[%d]", val)
			} else {
				truncated = true
			}

		case bytecode.JOF_LOCAL:
			// v28: 3-byte instruction uses uint16 operand
			// v33: 4-byte instruction uses uint24 operand
			if info.Length == 3 {
				if val, ok := bytecode.GetUint16(bc, off); ok {
					operand = fmt.Sprintf(" %d", val)
				} else {
					truncated = true
				}
			} else {
				if val, ok := bytecode.GetLocalno(bc, off); ok {
					operand = fmt.Sprintf(" %d", val)
				} else {
					truncated = true
				}
			}

		case bytecode.JOF_DOUBLE:
			if idx, ok := bytecode.GetUint32Index(bc, off); ok {
				if int(idx) < len(s.Consts) {
					operand = fmt.Sprintf(" %s", formatConst(s.Consts[idx]))
				} else {
					operand = fmt.Sprintf(" <const#%d>", idx)
				}
			} else {
				truncated = true
			}

		case bytecode.JOF_SCOPECOORD:
			if off+5 <= len(bc) {
				hops := bc[off+1]
				slot := uint32(bc[off+2])<<16 | uint32(bc[off+3])<<8 | uint32(bc[off+4])
				operand = fmt.Sprintf(" %d %d", hops, slot)
				comment = fmt.Sprintf("hops=%d slot=%d", hops, slot)
			} else {
				truncated = true
			}

		case bytecode.JOF_TABLESWITCH:
			if defOff, ok := bytecode.GetJumpOffset(bc, off); ok {
				defTgt := off + int(defOff)
				if off+13 <= len(bc) {
					lowVal := int32(bc[off+5])<<24 | int32(bc[off+6])<<16 | int32(bc[off+7])<<8 | int32(bc[off+8])
					highVal := int32(bc[off+9])<<24 | int32(bc[off+10])<<16 | int32(bc[off+11])<<8 | int32(bc[off+12])
					operand = fmt.Sprintf(" default loc_%05X low %d high %d", defTgt, lowVal, highVal)
				} else {
					truncated = true
				}
			} else {
				truncated = true
			}
		}

		if truncated {
			if opt.Mode == sm.Strict {
				return sm.Result[string]{Value: b.String(), Diags: diags},
					fmt.Errorf("truncated operand at offset %d (opcode 0x%02x)", off, op)
			}
			operand = " <truncated>"
			diags = append(diags, sm.Diagnostic{
				Offset: off,
				Kind:   "truncated",
				Msg:    fmt.Sprintf("operand truncated for opcode 0x%02x", op),
			})
		}

		b.WriteString(operand)
		col += len(operand)

		// Pad to comment column
		pad := commentCol - col
		if pad < 1 {
			pad = 1
		}
		b.WriteString(strings.Repeat(" ", pad))

		if comment != "" {
			fmt.Fprintf(&b, "; %s", comment)
		}
		b.WriteByte('\n')

		first = false

		n := bytecode.InstrLen(bc, off, ops)
		if n <= 0 {
			if opt.Mode == sm.Strict {
				return sm.Result[string]{Value: b.String(), Diags: diags},
					fmt.Errorf("cannot determine instruction length at offset %d (opcode 0x%02x)", off, op)
			}
			diags = append(diags, sm.Diagnostic{
				Offset: off,
				Kind:   "invalid",
				Msg:    fmt.Sprintf("unknown instruction length at offset %d (opcode 0x%02x)", off, op),
			})
			off++
			continue
		}
		off += n
	}

	return sm.Result[string]{Value: b.String(), Diags: diags}, nil
}

// tagFunc sets the Func field on diagnostics that don't already have one.
func tagFunc(diags []sm.Diagnostic, name string) {
	for i := range diags {
		if diags[i].Func == "" {
			diags[i].Func = name
		}
	}
}

// DisasmTreeOpt produces disassembly for a script and all inner functions with options.
func DisasmTreeOpt(s *sm.Script, opt sm.Options, ops *[256]bytecode.OpInfo) (sm.Result[string], error) {
	var b strings.Builder
	var allDiags []sm.Diagnostic

	// Source filename header
	if s.Filename != "" {
		fmt.Fprintf(&b, "; %s\n", s.Filename)
	}

	// Main script
	res, err := DisasmScriptOpt(s, "main", true, opt, ops)
	b.WriteString(res.Value)
	tagFunc(res.Diags, "main")
	allDiags = append(allDiags, res.Diags...)
	if err != nil {
		return sm.Result[string]{Value: b.String(), Diags: allDiags}, err
	}
	b.WriteByte('\n')

	// Inner functions (from objects)
	for i, obj := range s.Objects {
		if obj.Kind == sm.CkJSFunction && obj.Function != nil && obj.Function.Script != nil {
			name := obj.Function.Name
			if name == "" {
				name = fmt.Sprintf("anon#%d", i)
			}
			diagName := name
			res, err := DisasmScriptOpt(obj.Function.Script, name, false, opt, ops)
			b.WriteString(res.Value)
			tagFunc(res.Diags, diagName)
			allDiags = append(allDiags, res.Diags...)
			if err != nil {
				return sm.Result[string]{Value: b.String(), Diags: allDiags}, err
			}
			b.WriteByte('\n')
		}
	}

	// Recurse into inner function objects
	for _, obj := range s.Objects {
		if obj.Kind == sm.CkJSFunction && obj.Function != nil && obj.Function.Script != nil {
			res, err := disasmInnerOpt(obj.Function.Script, 1, opt, ops)
			b.WriteString(res.Value)
			allDiags = append(allDiags, res.Diags...)
			if err != nil {
				return sm.Result[string]{Value: b.String(), Diags: allDiags}, err
			}
		}
	}

	return sm.Result[string]{Value: b.String(), Diags: allDiags}, nil
}

// disasmInnerOpt recursively disassembles inner functions with options.
func disasmInnerOpt(s *sm.Script, depth int, opt sm.Options, ops *[256]bytecode.OpInfo) (sm.Result[string], error) {
	if depth > 5 {
		return sm.Result[string]{}, nil
	}
	var b strings.Builder
	var diags []sm.Diagnostic
	for i, obj := range s.Objects {
		if obj.Kind == sm.CkJSFunction && obj.Function != nil && obj.Function.Script != nil {
			name := obj.Function.Name
			if name == "" {
				name = fmt.Sprintf("anon#%d", i)
			}
			diagName := name
			res, err := DisasmScriptOpt(obj.Function.Script, name, false, opt, ops)
			b.WriteString(res.Value)
			tagFunc(res.Diags, diagName)
			diags = append(diags, res.Diags...)
			if err != nil {
				if opt.Mode == sm.Strict {
					return sm.Result[string]{Value: b.String(), Diags: diags}, err
				}
				diags = append(diags, sm.Diagnostic{
					Kind: "invalid",
					Func: diagName,
					Msg:  fmt.Sprintf("inner function %q: %v", name, err),
				})
				continue
			}
			b.WriteByte('\n')
			inner, err := disasmInnerOpt(obj.Function.Script, depth+1, opt, ops)
			b.WriteString(inner.Value)
			diags = append(diags, inner.Diags...)
			if err != nil {
				if opt.Mode == sm.Strict {
					return sm.Result[string]{Value: b.String(), Diags: diags}, err
				}
				diags = append(diags, sm.Diagnostic{
					Kind: "invalid",
					Func: diagName,
					Msg:  fmt.Sprintf("inner recursion: %v", err),
				})
			}
		}
	}
	return sm.Result[string]{Value: b.String(), Diags: diags}, nil
}

// formatConst formats a decoded constant for display.
func formatConst(c sm.Const) string {
	switch c.Kind {
	case sm.ConstInt:
		return fmt.Sprintf("%d", c.Int)
	case sm.ConstDouble:
		return fmt.Sprintf("%g", c.Double)
	case sm.ConstAtom:
		return fmt.Sprintf("%q", c.Atom)
	case sm.ConstTrue:
		return "true"
	case sm.ConstFalse:
		return "false"
	case sm.ConstNull:
		return "null"
	case sm.ConstVoid:
		return "undefined"
	case sm.ConstHole:
		return "<hole>"
	case sm.ConstObject:
		return "<object>"
	default:
		return fmt.Sprintf("<const?%d>", c.Kind)
	}
}

// regexpFlags converts SM33 regexp flag bits to a string.
// SM33 flags: 1=global, 2=ignoreCase, 4=multiline, 8=sticky.
func regexpFlags(flags uint32) string {
	var s []byte
	if flags&1 != 0 {
		s = append(s, 'g')
	}
	if flags&2 != 0 {
		s = append(s, 'i')
	}
	if flags&4 != 0 {
		s = append(s, 'm')
	}
	if flags&8 != 0 {
		s = append(s, 'y')
	}
	return string(s)
}
