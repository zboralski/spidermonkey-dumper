package xdr

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
	"unicode/utf16"

	"github.com/zboralski/spidermonkey-dumper/sm33"
)

const XdrMagic = 0xb973c02c // 0xb973c0de - 178

// ScriptBits flags (bit positions).
const (
	sbNoScriptRval              = 0
	sbSavedCallerFun            = 1
	sbStrict                    = 2
	sbContainsDynamicNameAccess = 3
	sbFunHasExtensibleScope     = 4
	sbFunNeedsDeclEnvObject     = 5
	sbFunHasAnyAliasedFormal    = 6
	sbArgumentsHasVarBinding    = 7
	sbNeedsArgsObj              = 8
	sbIsGeneratorExp            = 9
	sbIsLegacyGenerator         = 10
	sbIsStarGenerator           = 11
	sbOwnSource                 = 12
	sbExplicitUseStrict         = 13
	sbSelfHosted                = 14
	sbIsCompileAndGo            = 15
	sbHasSingleton              = 16
	sbTreatAsRunOnce            = 17
	sbHasLazyScript             = 18
)

// Const tags from XDRScriptConst.
const (
	scriptInt    = 0
	scriptDouble = 1
	scriptAtom   = 2
	scriptTrue   = 3
	scriptFalse  = 4
	scriptNull   = 5
	scriptObject = 6
	scriptVoid   = 7
	scriptHole   = 8
)

// reader wraps a byte slice with a cursor for sequential reads.
// In BestEffort mode, reads past EOF return zero-values and record diagnostics.
// In Strict mode, reads past EOF return io.ErrUnexpectedEOF.
type reader struct {
	data         []byte
	pos          int
	mode         sm33.Mode
	maxReadBytes int
	diags        []sm33.Diagnostic
	depth        int
}

func newReader(data []byte, mode sm33.Mode, maxReadBytes int) *reader {
	if maxReadBytes <= 0 {
		maxReadBytes = sm33.MaxReadBytes
	}
	return &reader{data: data, mode: mode, maxReadBytes: maxReadBytes}
}

func (r *reader) remaining() int {
	return len(r.data) - r.pos
}

func (r *reader) truncated(n int, what string) error {
	if r.mode == sm33.BestEffort {
		r.diags = append(r.diags, sm33.Diagnostic{
			Offset: r.pos,
			Kind:   "truncated",
			Msg:    fmt.Sprintf("%s: need %d bytes, have %d", what, n, r.remaining()),
		})
		r.pos = len(r.data)
		return nil
	}
	return fmt.Errorf("%s at offset %d: %w", what, r.pos, io.ErrUnexpectedEOF)
}

func (r *reader) u8() (uint8, error) {
	if r.pos >= len(r.data) {
		return 0, r.truncated(1, "u8")
	}
	v := r.data[r.pos]
	r.pos++
	return v, nil
}

func (r *reader) u16() (uint16, error) {
	if r.pos+2 > len(r.data) {
		return 0, r.truncated(2, "u16")
	}
	v := binary.LittleEndian.Uint16(r.data[r.pos:])
	r.pos += 2
	return v, nil
}

func (r *reader) u32() (uint32, error) {
	if r.pos+4 > len(r.data) {
		return 0, r.truncated(4, "u32")
	}
	v := binary.LittleEndian.Uint32(r.data[r.pos:])
	r.pos += 4
	return v, nil
}

func (r *reader) bytes(n int) ([]byte, error) {
	if n < 0 {
		if r.mode == sm33.BestEffort {
			r.diags = append(r.diags, sm33.Diagnostic{
				Offset: r.pos,
				Kind:   "invalid",
				Msg:    fmt.Sprintf("bytes: negative count %d", n),
			})
			r.pos = len(r.data)
			return []byte{}, nil
		}
		return nil, fmt.Errorf("bytes: negative count %d at offset %d: %w", n, r.pos, io.ErrUnexpectedEOF)
	}
	if n > r.maxReadBytes {
		if r.mode == sm33.BestEffort {
			r.diags = append(r.diags, sm33.Diagnostic{
				Offset: r.pos,
				Kind:   "clamped",
				Msg: fmt.Sprintf("bytes(%d): clamped to %d (increase max read cap if this is expected)",
					n, r.maxReadBytes),
			})
			n = r.maxReadBytes
		} else {
			return nil, fmt.Errorf("bytes(%d) exceeds max %d at offset %d (increase MaxReadBytes if this is expected): %w",
				n, r.maxReadBytes, r.pos, io.ErrUnexpectedEOF)
		}
	}
	if r.pos+n > len(r.data) {
		if r.mode == sm33.BestEffort {
			avail := r.remaining()
			b := make([]byte, avail)
			copy(b, r.data[r.pos:r.pos+avail])
			r.diags = append(r.diags, sm33.Diagnostic{
				Offset: r.pos,
				Kind:   "truncated",
				Msg:    fmt.Sprintf("bytes(%d): have %d", n, avail),
			})
			r.pos = len(r.data)
			return b, nil
		}
		return nil, r.truncated(n, fmt.Sprintf("bytes(%d)", n))
	}
	b := make([]byte, n)
	copy(b, r.data[r.pos:r.pos+n])
	r.pos += n
	return b, nil
}

func (r *reader) cstring() (string, error) {
	start := r.pos
	for r.pos < len(r.data) {
		if r.data[r.pos] == 0 {
			s := string(r.data[start:r.pos])
			r.pos++ // skip NUL
			return s, nil
		}
		r.pos++
	}
	if r.mode == sm33.BestEffort {
		s := string(r.data[start:r.pos])
		r.diags = append(r.diags, sm33.Diagnostic{
			Offset: start,
			Kind:   "truncated",
			Msg:    "unterminated cstring",
		})
		return s, nil
	}
	return "", fmt.Errorf("unterminated cstring at offset %d: %w", start, io.ErrUnexpectedEOF)
}

// readAtom reads an XDR atom: uint32(length<<1|isLatin1) + chars.
func (r *reader) readAtom() (string, error) {
	val, err := r.u32()
	if err != nil {
		return "", fmt.Errorf("atom header: %w", err)
	}
	length := val >> 1
	isLatin1 := val & 1

	if isLatin1 != 0 {
		b, err := r.bytes(int(length))
		if err != nil {
			return "", fmt.Errorf("atom latin1 data: %w", err)
		}
		return string(b), nil
	}
	// UTF-16: 2 bytes per char (little-endian), decode surrogate pairs
	raw, err := r.bytes(int(length) * 2)
	if err != nil {
		return "", fmt.Errorf("atom utf16 data: %w", err)
	}
	// Use actual bytes returned (may be shorter in BestEffort mode)
	nchars := len(raw) / 2
	u16s := make([]uint16, nchars)
	for i := 0; i < nchars; i++ {
		u16s[i] = binary.LittleEndian.Uint16(raw[i*2:])
	}
	return string(utf16.Decode(u16s)), nil
}

// clampCount validates a parsed count against remaining bytes and absolute cap.
// In Strict mode, returns error if count exceeds limits.
// In BestEffort mode, clamps count and records diagnostic.
func (r *reader) clampCount(count uint32, minEntryBytes int, what string) (uint32, error) {
	if minEntryBytes < 1 {
		minEntryBytes = 1
	}
	maxByBytes := uint32(r.remaining() / minEntryBytes)
	cap := maxByBytes
	if cap > sm33.MaxAllocCount {
		cap = sm33.MaxAllocCount
	}
	if count > cap {
		if r.mode == sm33.Strict {
			return 0, fmt.Errorf("%s count %d exceeds limit (max by remaining: %d, abs cap: %d)",
				what, count, maxByBytes, sm33.MaxAllocCount)
		}
		r.diags = append(r.diags, sm33.Diagnostic{
			Offset: r.pos,
			Kind:   "clamped",
			Msg:    fmt.Sprintf("%s count %d clamped to %d", what, count, cap),
		})
		count = cap
	}
	return count, nil
}

// checkDepth validates recursion depth. In Strict mode, returns error.
// In BestEffort, records diagnostic and returns exceeded=true with nil error.
func (r *reader) checkDepth(what string) (exceeded bool, err error) {
	if r.depth > sm33.MaxDecodeDepth {
		if r.mode == sm33.Strict {
			return true, fmt.Errorf("%s: recursion depth %d exceeds limit %d", what, r.depth, sm33.MaxDecodeDepth)
		}
		r.diags = append(r.diags, sm33.Diagnostic{
			Offset: r.pos,
			Kind:   "overflow",
			Msg:    fmt.Sprintf("%s: recursion depth %d exceeded limit %d", what, r.depth, sm33.MaxDecodeDepth),
		})
		return true, nil
	}
	return false, nil
}

// DecodeFileOpt reads a .jsc file and decodes with options.
func DecodeFileOpt(path string, opt sm33.Options) (sm33.Result[*sm33.Script], error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return sm33.Result[*sm33.Script]{}, err
	}
	return DecodeOpt(data, opt)
}

// DecodeOpt parses XDR-encoded bytecode with options.
func DecodeOpt(data []byte, opt sm33.Options) (sm33.Result[*sm33.Script], error) {
	r := newReader(data, opt.Mode, opt.EffectiveMaxReadBytes())

	magic, err := r.u32()
	if err != nil {
		return sm33.Result[*sm33.Script]{Diags: r.diags}, fmt.Errorf("reading magic: %w", err)
	}
	if magic != XdrMagic {
		if opt.Mode == sm33.Strict {
			return sm33.Result[*sm33.Script]{Diags: r.diags}, fmt.Errorf("bad XDR magic: got 0x%08x, want 0x%08x", magic, XdrMagic)
		}
		r.diags = append(r.diags, sm33.Diagnostic{
			Offset: 0,
			Kind:   "invalid",
			Msg:    fmt.Sprintf("bad XDR magic: got 0x%08x, want 0x%08x", magic, XdrMagic),
		})
	}

	s, err := decodeScript(r)
	if err != nil {
		return sm33.Result[*sm33.Script]{Diags: r.diags}, err
	}
	return sm33.Result[*sm33.Script]{Value: s, Diags: r.diags}, nil
}

// DecodeFile reads a .jsc file and decodes the top-level script (Strict mode).
func DecodeFile(path string) (*sm33.Script, error) {
	res, err := DecodeFileOpt(path, sm33.DefaultOptions())
	return res.Value, err
}

// Decode parses XDR-encoded bytecode into a Script (Strict mode).
func Decode(data []byte) (*sm33.Script, error) {
	res, err := DecodeOpt(data, sm33.DefaultOptions())
	return res.Value, err
}

// decodeScript reads XDRScript fields.
func decodeScript(r *reader) (*sm33.Script, error) {
	r.depth++
	exceeded, err := r.checkDepth("decodeScript")
	if err != nil {
		r.depth--
		return nil, err
	}
	if exceeded {
		r.depth--
		return &sm33.Script{}, nil
	}
	defer func() { r.depth-- }()

	s := &sm33.Script{}

	s.Nargs, err = r.u16()
	if err != nil {
		return nil, fmt.Errorf("nargs: %w", err)
	}
	s.Nblocklocals, err = r.u16()
	if err != nil {
		return nil, fmt.Errorf("nblocklocals: %w", err)
	}
	s.Nvars, err = r.u32()
	if err != nil {
		return nil, fmt.Errorf("nvars: %w", err)
	}

	length, err := r.u32()
	if err != nil {
		return nil, fmt.Errorf("length: %w", err)
	}

	s.MainOffset, err = r.u32()
	if err != nil {
		return nil, fmt.Errorf("prologLength: %w", err)
	}
	s.Version, err = r.u32()
	if err != nil {
		return nil, fmt.Errorf("version: %w", err)
	}

	natoms, err := r.u32()
	if err != nil {
		return nil, fmt.Errorf("natoms: %w", err)
	}
	nsrcnotes, err := r.u32()
	if err != nil {
		return nil, fmt.Errorf("nsrcnotes: %w", err)
	}
	nconsts, err := r.u32()
	if err != nil {
		return nil, fmt.Errorf("nconsts: %w", err)
	}
	nobjects, err := r.u32()
	if err != nil {
		return nil, fmt.Errorf("nobjects: %w", err)
	}
	nregexps, err := r.u32()
	if err != nil {
		return nil, fmt.Errorf("nregexps: %w", err)
	}
	ntrynotes, err := r.u32()
	if err != nil {
		return nil, fmt.Errorf("ntrynotes: %w", err)
	}
	nblockscopes, err := r.u32()
	if err != nil {
		return nil, fmt.Errorf("nblockscopes: %w", err)
	}
	// nTypeSets - read and discard
	if _, err = r.u32(); err != nil {
		return nil, fmt.Errorf("nTypeSets: %w", err)
	}
	// funLength - read and discard
	if _, err = r.u32(); err != nil {
		return nil, fmt.Errorf("funLength: %w", err)
	}

	scriptBits, err := r.u32()
	if err != nil {
		return nil, fmt.Errorf("scriptBits: %w", err)
	}

	// XDRScriptBindings
	nameCount := uint32(s.Nargs) + s.Nvars
	nameCount, err = r.clampCount(nameCount, 5, "bindings")
	if err != nil {
		return nil, err
	}
	s.Bindings = make([]string, nameCount)
	for i := uint32(0); i < nameCount; i++ {
		s.Bindings[i], err = r.readAtom()
		if err != nil {
			return nil, fmt.Errorf("binding atom %d: %w", i, err)
		}
	}
	// Binding descriptors (1 byte each)
	for i := uint32(0); i < nameCount; i++ {
		if _, err = r.u8(); err != nil {
			return nil, fmt.Errorf("binding descriptor %d: %w", i, err)
		}
	}

	// ScriptSource (only if OwnSource)
	if scriptBits&(1<<sbOwnSource) != 0 {
		s.Filename, err = decodeScriptSource(r)
		if err != nil {
			return nil, fmt.Errorf("script source: %w", err)
		}
	}

	// Source location
	s.SourceStart, err = r.u32()
	if err != nil {
		return nil, fmt.Errorf("sourceStart: %w", err)
	}
	s.SourceEnd, err = r.u32()
	if err != nil {
		return nil, fmt.Errorf("sourceEnd: %w", err)
	}
	s.Lineno, err = r.u32()
	if err != nil {
		return nil, fmt.Errorf("lineno: %w", err)
	}
	s.Column, err = r.u32()
	if err != nil {
		return nil, fmt.Errorf("column: %w", err)
	}
	s.Nslots, err = r.u32()
	if err != nil {
		return nil, fmt.Errorf("nslots: %w", err)
	}
	s.StaticLevel, err = r.u32()
	if err != nil {
		return nil, fmt.Errorf("staticLevel: %w", err)
	}

	// Bytecode
	s.Bytecode, err = r.bytes(int(length))
	if err != nil {
		return nil, fmt.Errorf("bytecode: %w", err)
	}
	// Source notes
	s.Srcnotes, err = r.bytes(int(nsrcnotes))
	if err != nil {
		return nil, fmt.Errorf("srcnotes: %w", err)
	}

	// Atoms
	natoms, err = r.clampCount(natoms, 4, "atoms")
	if err != nil {
		return nil, err
	}
	s.Atoms = make([]string, natoms)
	for i := uint32(0); i < natoms; i++ {
		s.Atoms[i], err = r.readAtom()
		if err != nil {
			return nil, fmt.Errorf("atom %d: %w", i, err)
		}
	}

	// Consts
	nconsts, err = r.clampCount(nconsts, 4, "consts")
	if err != nil {
		return nil, err
	}
	if nconsts > 0 {
		s.Consts = make([]sm33.Const, nconsts)
		for i := uint32(0); i < nconsts; i++ {
			s.Consts[i], err = decodeConst(r)
			if err != nil {
				return nil, fmt.Errorf("const %d: %w", i, err)
			}
		}
	}

	// Objects
	nobjects, err = r.clampCount(nobjects, 4, "objects")
	if err != nil {
		return nil, err
	}
	s.Objects = make([]*sm33.Object, nobjects)
	for i := uint32(0); i < nobjects; i++ {
		s.Objects[i], err = decodeObject(r)
		if err != nil {
			return nil, fmt.Errorf("object %d: %w", i, err)
		}
	}

	// Regexps
	nregexps, err = r.clampCount(nregexps, 8, "regexps")
	if err != nil {
		return nil, err
	}
	if nregexps > 0 {
		s.Regexps = make([]sm33.Regexp, nregexps)
		for i := uint32(0); i < nregexps; i++ {
			s.Regexps[i], err = decodeRegexp(r)
			if err != nil {
				return nil, fmt.Errorf("regexp %d: %w", i, err)
			}
		}
	}

	// TryNotes (in reverse order in the XDR stream)
	ntrynotes, err = r.clampCount(ntrynotes, 13, "trynotes")
	if err != nil {
		return nil, err
	}
	if ntrynotes > 0 {
		s.TryNotes = make([]sm33.TryNote, ntrynotes)
		for i := int(ntrynotes) - 1; i >= 0; i-- {
			tn := &s.TryNotes[i]
			tn.Kind, err = r.u8()
			if err != nil {
				return nil, fmt.Errorf("trynote %d kind: %w", i, err)
			}
			tn.StackDepth, err = r.u32()
			if err != nil {
				return nil, fmt.Errorf("trynote %d stackDepth: %w", i, err)
			}
			tn.Start, err = r.u32()
			if err != nil {
				return nil, fmt.Errorf("trynote %d start: %w", i, err)
			}
			tn.Length, err = r.u32()
			if err != nil {
				return nil, fmt.Errorf("trynote %d length: %w", i, err)
			}
		}
	}

	// Block scopes (skip)
	nblockscopes, err = r.clampCount(nblockscopes, 16, "blockscopes")
	if err != nil {
		return nil, err
	}
	for i := uint32(0); i < nblockscopes; i++ {
		// 4 uint32s: index, start, length, parent
		for j := 0; j < 4; j++ {
			if _, err = r.u32(); err != nil {
				return nil, fmt.Errorf("blockscope %d field %d: %w", i, j, err)
			}
		}
	}

	// HasLazyScript → XDRRelazificationInfo (not XDRLazyScript)
	if scriptBits&(1<<sbHasLazyScript) != 0 {
		if err = skipRelazificationInfo(r); err != nil {
			return nil, fmt.Errorf("relazification info: %w", err)
		}
	}

	return s, nil
}

// decodeScriptSource reads ScriptSource::performXDR data and returns the filename.
// Source text is skipped (all known SM33 Cocos2d-x files use retrievable=1,
// meaning source is loaded from the .js file at runtime rather than embedded).
func decodeScriptSource(r *reader) (string, error) {
	hasSource, err := r.u8()
	if err != nil {
		return "", err
	}
	retrievable, err := r.u8()
	if err != nil {
		return "", err
	}

	var srcLength, compLength uint32
	if hasSource != 0 && retrievable == 0 {
		srcLength, err = r.u32()
		if err != nil {
			return "", err
		}
		compLength, err = r.u32()
		if err != nil {
			return "", err
		}
		// argumentsNotIncluded
		if _, err = r.u8(); err != nil {
			return "", err
		}
		var byteLen uint32
		if compLength != 0 {
			byteLen = compLength
		} else {
			byteLen = srcLength * 2 // jschar = 2 bytes
		}
		if _, err = r.bytes(int(byteLen)); err != nil {
			return "", err
		}
	}

	// haveSourceMap
	haveSourceMap, err := r.u8()
	if err != nil {
		return "", err
	}
	if haveSourceMap != 0 {
		mapLen, err := r.u32()
		if err != nil {
			return "", err
		}
		// jschar = 2 bytes per char
		if _, err = r.bytes(int(mapLen) * 2); err != nil {
			return "", err
		}
	}

	// haveDisplayURL
	haveDisplayURL, err := r.u8()
	if err != nil {
		return "", err
	}
	if haveDisplayURL != 0 {
		urlLen, err := r.u32()
		if err != nil {
			return "", err
		}
		if _, err = r.bytes(int(urlLen) * 2); err != nil {
			return "", err
		}
	}

	// haveFilename
	haveFilename, err := r.u8()
	if err != nil {
		return "", err
	}
	var filename string
	if haveFilename != 0 {
		filename, err = r.cstring()
		if err != nil {
			return "", err
		}
	}

	return filename, nil
}

// decodeObject reads one XDR object entry.
func decodeObject(r *reader) (*sm33.Object, error) {
	classKind, err := r.u32()
	if err != nil {
		return nil, err
	}
	obj := &sm33.Object{Kind: classKind}

	switch classKind {
	case sm33.CkBlockObject, sm33.CkWithObject:
		// enclosingStaticScopeIndex
		if _, err = r.u32(); err != nil {
			return nil, err
		}
		if classKind == sm33.CkBlockObject {
			if err = skipStaticBlockObject(r); err != nil {
				return nil, err
			}
		}

	case sm33.CkJSFunction:
		// funEnclosingScopeIndex
		if _, err = r.u32(); err != nil {
			return nil, err
		}
		obj.Function, err = decodeInterpretedFunction(r)
		if err != nil {
			return nil, err
		}

	case sm33.CkJSObject:
		if err = skipObjectLiteral(r); err != nil {
			return nil, err
		}

	default:
		if r.mode == sm33.BestEffort {
			r.diags = append(r.diags, sm33.Diagnostic{
				Offset: r.pos,
				Kind:   "invalid",
				Msg:    fmt.Sprintf("unknown class kind %d", classKind),
			})
			return obj, nil
		}
		return nil, fmt.Errorf("unknown class kind %d", classKind)
	}

	return obj, nil
}

// decodeInterpretedFunction reads XDRInterpretedFunction.
func decodeInterpretedFunction(r *reader) (*sm33.Function, error) {
	r.depth++
	exceeded, err := r.checkDepth("decodeInterpretedFunction")
	if err != nil {
		r.depth--
		return nil, err
	}
	if exceeded {
		r.depth--
		return &sm33.Function{Name: "<depth-exceeded>", IsLazy: true}, nil
	}
	defer func() { r.depth-- }()

	firstword, err := r.u32()
	if err != nil {
		return nil, err
	}

	f := &sm33.Function{}

	hasAtom := firstword&0x1 != 0
	isLazy := firstword&0x4 != 0

	if hasAtom {
		f.Name, err = r.readAtom()
		if err != nil {
			return nil, fmt.Errorf("function atom: %w", err)
		}
	}

	flagsword, err := r.u32()
	if err != nil {
		return nil, err
	}
	f.Nargs = uint16(flagsword >> 16)
	f.Flags = uint16(flagsword & 0xFFFF)
	f.IsLazy = isLazy

	if isLazy {
		if err = skipLazyScript(r); err != nil {
			return nil, fmt.Errorf("lazy script: %w", err)
		}
	} else {
		f.Script, err = decodeScript(r)
		if err != nil {
			return nil, fmt.Errorf("function script: %w", err)
		}
	}

	return f, nil
}

// decodeConst reads one XDRScriptConst.
func decodeConst(r *reader) (sm33.Const, error) {
	tag, err := r.u32()
	if err != nil {
		return sm33.Const{}, err
	}
	switch tag {
	case scriptInt:
		v, err := r.u32()
		if err != nil {
			return sm33.Const{}, err
		}
		return sm33.Const{Kind: sm33.ConstInt, Int: int32(v)}, nil
	case scriptDouble:
		b, err := r.bytes(8)
		if err != nil {
			return sm33.Const{}, err
		}
		if len(b) < 8 {
			// BestEffort: bytes() returned short slice, already recorded truncation diagnostic
			return sm33.Const{Kind: sm33.ConstDouble}, nil
		}
		bits := binary.LittleEndian.Uint64(b)
		return sm33.Const{Kind: sm33.ConstDouble, Double: math.Float64frombits(bits)}, nil
	case scriptAtom:
		s, err := r.readAtom()
		if err != nil {
			return sm33.Const{}, err
		}
		return sm33.Const{Kind: sm33.ConstAtom, Atom: s}, nil
	case scriptTrue:
		return sm33.Const{Kind: sm33.ConstTrue}, nil
	case scriptFalse:
		return sm33.Const{Kind: sm33.ConstFalse}, nil
	case scriptNull:
		return sm33.Const{Kind: sm33.ConstNull}, nil
	case scriptVoid:
		return sm33.Const{Kind: sm33.ConstVoid}, nil
	case scriptHole:
		return sm33.Const{Kind: sm33.ConstHole}, nil
	case scriptObject:
		if err = skipObjectLiteral(r); err != nil {
			return sm33.Const{}, err
		}
		return sm33.Const{Kind: sm33.ConstObject}, nil
	default:
		if r.mode == sm33.BestEffort {
			r.diags = append(r.diags, sm33.Diagnostic{
				Offset: r.pos,
				Kind:   "invalid",
				Msg:    fmt.Sprintf("unknown const tag %d", tag),
			})
			return sm33.Const{}, nil
		}
		return sm33.Const{}, fmt.Errorf("unknown const tag %d", tag)
	}
}

// skipConst reads and discards one XDRScriptConst (used by skipObjectLiteral dense elements).
func skipConst(r *reader) error {
	_, err := decodeConst(r)
	return err
}

// decodeRegexp reads one XDRScriptRegExpObject.
func decodeRegexp(r *reader) (sm33.Regexp, error) {
	source, err := r.readAtom()
	if err != nil {
		return sm33.Regexp{}, err
	}
	flags, err := r.u32()
	if err != nil {
		return sm33.Regexp{}, err
	}
	return sm33.Regexp{Source: source, Flags: flags}, nil
}

// skipStaticBlockObject reads and discards a StaticBlockObject.
func skipStaticBlockObject(r *reader) error {
	count, err := r.u32()
	if err != nil {
		return err
	}
	// offset (localOffset)
	if _, err = r.u32(); err != nil {
		return err
	}
	count, err = r.clampCount(count, 8, "block vars")
	if err != nil {
		return err
	}
	for i := uint32(0); i < count; i++ {
		if _, err = r.readAtom(); err != nil {
			return fmt.Errorf("block var %d atom: %w", i, err)
		}
		if _, err = r.u32(); err != nil {
			return fmt.Errorf("block var %d aliased: %w", i, err)
		}
	}
	return nil
}

// skipObjectLiteral reads and discards an XDRObjectLiteral.
func skipObjectLiteral(r *reader) error {
	isArray, err := r.u32()
	if err != nil {
		return err
	}

	if isArray != 0 {
		if _, err = r.u32(); err != nil {
			return err
		}
	} else {
		if _, err = r.u32(); err != nil {
			return err
		}
	}

	// capacity
	if _, err = r.u32(); err != nil {
		return err
	}

	// initialized (dense elements count)
	initialized, err := r.u32()
	if err != nil {
		return err
	}
	initialized, err = r.clampCount(initialized, 4, "dense elements")
	if err != nil {
		return err
	}
	for i := uint32(0); i < initialized; i++ {
		if err = skipConst(r); err != nil {
			return fmt.Errorf("dense element %d: %w", i, err)
		}
	}

	// nslot (named properties)
	nslot, err := r.u32()
	if err != nil {
		return err
	}
	nslot, err = r.clampCount(nslot, 8, "object slots")
	if err != nil {
		return err
	}
	for i := uint32(0); i < nslot; i++ {
		idType, err := r.u32()
		if err != nil {
			return err
		}
		if idType == 0 { // JSID_TYPE_STRING
			if _, err = r.readAtom(); err != nil {
				return fmt.Errorf("slot %d atom: %w", i, err)
			}
		} else { // JSID_TYPE_INT
			if _, err = r.u32(); err != nil {
				return fmt.Errorf("slot %d int id: %w", i, err)
			}
		}
		if err = skipConst(r); err != nil {
			return fmt.Errorf("slot %d value: %w", i, err)
		}
	}

	return nil
}

// readPackedFields reads a LazyScript uint64 packedFields and extracts counts.
func readPackedFields(r *reader) (numFreeVars, numInnerFuncs uint32, err error) {
	lo, err := r.u32()
	if err != nil {
		return 0, 0, err
	}
	hi, err := r.u32()
	if err != nil {
		return 0, 0, err
	}
	numFreeVars = (lo >> 8) & 0xFFFFFF
	numInnerFuncs = hi & 0x7FFFFF
	return numFreeVars, numInnerFuncs, nil
}

// skipRelazificationInfo reads and discards XDRRelazificationInfo.
func skipRelazificationInfo(r *reader) error {
	numFreeVars, _, err := readPackedFields(r)
	if err != nil {
		return err
	}
	numFreeVars, err = r.clampCount(numFreeVars, 4, "relazification free vars")
	if err != nil {
		return err
	}
	for i := uint32(0); i < numFreeVars; i++ {
		if _, err = r.readAtom(); err != nil {
			return err
		}
	}
	return nil
}

// skipLazyScript reads and discards XDRLazyScript.
func skipLazyScript(r *reader) error {
	// begin, end, lineno, column
	for i := 0; i < 4; i++ {
		if _, err := r.u32(); err != nil {
			return err
		}
	}

	numFreeVars, numInnerFuncs, err := readPackedFields(r)
	if err != nil {
		return err
	}

	numFreeVars, err = r.clampCount(numFreeVars, 4, "lazy free vars")
	if err != nil {
		return err
	}
	for i := uint32(0); i < numFreeVars; i++ {
		if _, err = r.readAtom(); err != nil {
			return err
		}
	}

	numInnerFuncs, err = r.clampCount(numInnerFuncs, 8, "lazy inner funcs")
	if err != nil {
		return err
	}
	for i := uint32(0); i < numInnerFuncs; i++ {
		if _, err = decodeInterpretedFunction(r); err != nil {
			return err
		}
	}

	return nil
}
