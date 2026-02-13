package xdr

import (
	"fmt"

	"github.com/zboralski/spidermonkey-dumper/sm"
)

// decodeScriptV28 reads XDRScript fields for SpiderMonkey 28.
// Verified against js/src/jsscript.cpp from cocos2d-jsc-decompiler v28 branch.
//
// Key differences from v33:
//   - argsVars packed into single uint32: (nargs<<16 | nvars)
//   - No nblocklocals field
//   - version field packs nfixed in upper 16 bits
//   - No column field
//   - nslots packs staticLevel in upper 16 bits
//   - nblockscopes IS present (unlike earlier assumption)
//   - TryNotes: kindAndDepth packed into single uint32
//   - Block scopes section IS present
//   - Atoms are UTF-16 only (handled by readAtom via r.ver)
func decodeScriptV28(r *reader) (*sm.Script, error) {
	r.depth++
	exceeded, err := r.checkDepth("decodeScriptV28")
	if err != nil {
		r.depth--
		return nil, err
	}
	if exceeded {
		r.depth--
		return &sm.Script{}, nil
	}
	defer func() { r.depth-- }()

	s := &sm.Script{}

	// v28: argsVars is a single uint32 = (nargs << 16) | nvars
	argsVars, err := r.u32()
	if err != nil {
		return nil, fmt.Errorf("argsVars: %w", err)
	}
	s.Nargs = uint16(argsVars >> 16)
	s.Nvars = argsVars & 0xFFFF

	length, err := r.u32()
	if err != nil {
		return nil, fmt.Errorf("length: %w", err)
	}

	s.MainOffset, err = r.u32()
	if err != nil {
		return nil, fmt.Errorf("prologLength: %w", err)
	}
	// v28: version packs nfixed in upper 16 bits
	versionPacked, err := r.u32()
	if err != nil {
		return nil, fmt.Errorf("version: %w", err)
	}
	s.Version = versionPacked & 0xFFFF

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

	// Source location — v28 has only sourceStart, sourceEnd, lineno, nslots
	// No column field. nslots packs staticLevel in upper 16 bits.
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
	// v28: nslots = (staticLevel << 16) | nslots — packed
	nslotsPacked, err := r.u32()
	if err != nil {
		return nil, fmt.Errorf("nslots: %w", err)
	}
	s.Nslots = nslotsPacked & 0xFFFF
	s.StaticLevel = nslotsPacked >> 16

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
		s.Consts = make([]sm.Const, nconsts)
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
	s.Objects = make([]*sm.Object, nobjects)
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
		s.Regexps = make([]sm.Regexp, nregexps)
		for i := uint32(0); i < nregexps; i++ {
			s.Regexps[i], err = decodeRegexp(r)
			if err != nil {
				return nil, fmt.Errorf("regexp %d: %w", i, err)
			}
		}
	}

	// TryNotes (in reverse order in the XDR stream)
	// v28: kindAndDepth packed as uint32 = (kind << 16) | stackDepth
	ntrynotes, err = r.clampCount(ntrynotes, 12, "trynotes")
	if err != nil {
		return nil, err
	}
	if ntrynotes > 0 {
		s.TryNotes = make([]sm.TryNote, ntrynotes)
		for i := int(ntrynotes) - 1; i >= 0; i-- {
			tn := &s.TryNotes[i]
			kindAndDepth, err := r.u32()
			if err != nil {
				return nil, fmt.Errorf("trynote %d kindAndDepth: %w", i, err)
			}
			tn.Kind = uint8(kindAndDepth >> 16)
			tn.StackDepth = kindAndDepth & 0xFFFF
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

	// Block scopes (v28 DOES have this section)
	nblockscopes, err = r.clampCount(nblockscopes, 16, "blockscopes")
	if err != nil {
		return nil, err
	}
	for i := uint32(0); i < nblockscopes; i++ {
		// Each BlockScopeNote: index, start, length, parent — 4x uint32
		if _, err = r.u32(); err != nil {
			return nil, fmt.Errorf("blockscope %d index: %w", i, err)
		}
		if _, err = r.u32(); err != nil {
			return nil, fmt.Errorf("blockscope %d start: %w", i, err)
		}
		if _, err = r.u32(); err != nil {
			return nil, fmt.Errorf("blockscope %d length: %w", i, err)
		}
		if _, err = r.u32(); err != nil {
			return nil, fmt.Errorf("blockscope %d parent: %w", i, err)
		}
	}

	// HasLazyScript → XDRRelazificationInfo
	if scriptBits&(1<<sbHasLazyScript) != 0 {
		if err = skipRelazificationInfo(r); err != nil {
			return nil, fmt.Errorf("relazification info: %w", err)
		}
	}

	return s, nil
}
