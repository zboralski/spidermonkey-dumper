package sm33

// XDR object class kinds.
const (
	CkBlockObject = 0
	CkWithObject  = 1
	CkJSFunction  = 2
	CkJSObject    = 3
)

// Maximum decode recursion depth.
const MaxDecodeDepth = 64

// Maximum allocation count for any single parsed count.
const MaxAllocCount = 1 << 20

// MaxReadBytes caps any single bytes() allocation (16 MB).
const MaxReadBytes = 1 << 24

// TryNote describes a try/catch/finally region.
type TryNote struct {
	Kind       uint8
	StackDepth uint32
	Start      uint32
	Length     uint32
}

// Script is a decoded SpiderMonkey 33 script.
type Script struct {
	// Header
	Nargs        uint16
	Nblocklocals uint16
	Nvars        uint32
	MainOffset   uint32 // prologLength
	Version      uint32

	// Source info
	Filename    string
	SourceStart uint32
	SourceEnd   uint32
	Lineno      uint32
	Column      uint32
	Nslots      uint32
	StaticLevel uint32

	// Bytecode
	Bytecode []byte
	Srcnotes []byte

	// Atoms referenced by bytecode
	Atoms []string

	// Constants referenced by bytecode
	Consts []Const

	// Regexps referenced by bytecode
	Regexps []Regexp

	// Inner objects (functions)
	Objects []*Object

	// Try notes
	TryNotes []TryNote

	// Binding names (args + vars)
	Bindings []string
}

// ConstKind identifies the type of a script constant.
type ConstKind uint8

const (
	ConstInt    ConstKind = iota // int32 value
	ConstDouble                 // float64 value
	ConstAtom                   // string atom
	ConstTrue                   // boolean true
	ConstFalse                  // boolean false
	ConstNull                   // null
	ConstVoid                   // undefined
	ConstHole                   // JS_ARRAY_HOLE
	ConstObject                 // object literal
)

// Const is a decoded script constant.
type Const struct {
	Kind   ConstKind
	Int    int32   // valid when Kind == ConstInt
	Double float64 // valid when Kind == ConstDouble
	Atom   string  // valid when Kind == ConstAtom
}

// Regexp is a decoded script regexp.
type Regexp struct {
	Source string
	Flags  uint32
}

// Object is a decoded XDR object entry.
type Object struct {
	Kind     uint32
	Function *Function // non-nil when Kind == CkJSFunction
}

// Function is a decoded inner function.
type Function struct {
	Name   string // empty if anonymous
	Nargs  uint16
	Flags  uint16
	Script *Script
	IsLazy bool
}
