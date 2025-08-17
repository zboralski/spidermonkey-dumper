/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/.
 */

#include <cstdio>
#include <cstdlib>
#include <cstring>
#include <cstdarg>
#include <string>
#include <sstream>
#include <iomanip>
#include <chrono>
#include <random>
#include <filesystem>

#include "dumper.h"
#include "utils.h"
#include "logging.h"
#include "ollama.h"
#include <vector>
#include <map>
#include <unistd.h>
#include <getopt.h>

#include <sys/stat.h>

// Suppress SpiderMonkey offsetof warnings on non-standard-layout types
#pragma clang diagnostic push
#pragma clang diagnostic ignored "-Winvalid-offsetof"
#include <jsapi.h>
#include "jsscript.h"
#include "jsobj.h"
#include "jsfun.h"
#include "jsfriendapi.h"
#include "jsopcode.h"
#include "jsopcodeinlines.h"
#include "vm/Opcodes.h"
#include "jscntxt.h"
#include "vm/ScopeObject.h"
#include "jsatom.h"
#pragma clang diagnostic pop

#include <sys/types.h>
#include <sys/stat.h>
#include <fcntl.h>

// --- File paths ---
static std::string gInputPath;   // original .jsc path
static std::string gDisPath;     // sibling .dis file
static std::string gJsPath;      // sibling .js file (LLM output)

// --- Command line options / feature flags ---
static bool gDecompile = false;       // off by default (opt-in with --decompile)
static bool gDisSugar = true;         // default: include sugar in .dis 
static bool gInnerEnabled = true;     // Enable by default
static bool gShowLines = false;       // show source line numbers
static bool gSugarEnabled = true;     // show syntactic sugar hints
static bool gUseColor = false;        // colorized output
static bool gWritingDis = false;      // true while emitting .dis (suppress colors+decompile)

// --- Runtime state tracking ---
static bool gAfterLoopEntry = false;         // next GETLOCAL is loop index hint
static bool gFirstLineOfFunction = true;     // controls spacing before first label/instruction

// --- Per-instruction history for comment heuristics ---
struct HistOp { JSOp op; int local; bool hasImm; double imm; };
static HistOp gHist[5]; // rolling window of last 5 ops

// Color macros for terminal output
#define COL_ADDR         (gUseColor ? "\033[37m" : "")  // white (regular lines)
#define COL_ADDR_BRANCH  (gUseColor ? "\033[35m" : "")  // magenta (branches)
#define COL_ADDR_CALL    (gUseColor ? "\033[36m" : "")  // cyan (calls)
#define COL_ADDR_HOT     (gUseColor ? "\033[33m" : "")  // yellow (basic-block leaders)
#define COL_ADDR_RET     (gUseColor ? "\033[31m" : "")  // red (returns)
#define COL_BRANCH_BACK  (gUseColor ? "\033[95m" : "")  // bright magenta (backward branch = likely loop)
#define COL_BRANCH_FWD   (gUseColor ? "\033[35m" : "")  // magenta (forward branch)
#define COL_COMM         (gUseColor ? "\033[90m" : "")  // dim gray
#define COL_LABEL        (gUseColor ? "\033[94m" : "")  // bright blue
#define COL_MNEM         (gUseColor ? "\033[1m"  : "")
#define COL_NUM          (gUseColor ? "\033[90m" : "")
#define COL_RESET        (gUseColor ? "\033[0m"  : "")
#define COL_STR          (gUseColor ? "\033[32m" : "")
#define COL_WARN         (gUseColor ? "\033[95m" : "")

static void resetHistory() {
  for (int i = 0; i < 5; ++i) { gHist[i].op = JSOp(0); gHist[i].local = -1; gHist[i].hasImm = false; gHist[i].imm = 0.0; }
  gAfterLoopEntry = false;
}

static void pushHist(JSOp op, int local, bool hasImm, double imm) {
  for (int i = 4; i > 0; --i) gHist[i] = gHist[i-1];
  gHist[0].op = op; gHist[0].local = local; gHist[0].hasImm = hasImm; gHist[0].imm = imm;
}

// Helper to get comparison operator string from opcode
static inline const char* cmpOp(JSOp op) {
  switch (op) {
    case JSOP_LT: return "<";
    case JSOP_LE: return "<=";
    case JSOP_GT: return ">";
    case JSOP_GE: return ">=";
    case JSOP_EQ: return "==";
    case JSOP_NE: return "!=";
    default: return nullptr;
  }
}

// Helper to get try-catch kind string from kind code
static inline const char* tryKindStr(uint8_t k) {
  switch (k) {
    case JSTRY_CATCH:    return "catch";
    case JSTRY_FINALLY:  return "finally";
    case JSTRY_ITER:     return "iter";
    case JSTRY_LOOP:     return "loop";
    default:             return "try";
  }
}

// Build a small "syntactic sugar" hint string for the current opcode.
static std::string sugarForOp(JSContext* cx, JSScript* script, jsbytecode* pc) {
  std::ostringstream out;
  JSOp op = JSOp(*pc);
  (void)cx; // cx is currently only used for atom printing in some cases below

  auto atomToStr = [&](JSAtom* atom) -> const char* {
    static thread_local char buf[256];
    buf[0] = '\0';
    if (!atom) return nullptr;
    JSAutoByteString bytes;
    if (AtomToPrintableString(cx, atom, &bytes)) {
      std::snprintf(buf, sizeof(buf), "%s", bytes.ptr());
      return buf;
    }
    return nullptr;
  };

  switch (op) {
    case JSOP_THIS:
      out << "this";
      break;

    case JSOP_NAME: {
      JSAtom* a = script->getAtom(GET_UINT32_INDEX(pc));
      if (const char* s = atomToStr(a)) out << s;
      break;
    }

    case JSOP_BINDNAME: {
      JSAtom* a = script->getAtom(GET_UINT32_INDEX(pc));
      if (const char* s = atomToStr(a)) out << "(bind " << s << ")";
      break;
    }

    case JSOP_GETARG:
      out << "arg[" << GET_ARGNO(pc) << "]";
      break;

    case JSOP_GETLOCAL:
      out << "local[" << GET_LOCALNO(pc) << "]";
      break;

    case JSOP_SETLOCAL:
      out << "local[" << GET_LOCALNO(pc) << "] = …";
      break;

    case JSOP_GETPROP: {
      JSAtom* a = script->getAtom(GET_UINT32_INDEX(pc));
      if (const char* s = atomToStr(a)) {
        if (strcmp(s, "length") == 0) {
          out << "length"; 
        } else {
          out << "?." << s; // unknown receiver at this point
        }
      }
      break;
    }

    case JSOP_SETPROP: {
      JSAtom* a = script->getAtom(GET_UINT32_INDEX(pc));
      if (const char* s = atomToStr(a)) out << "?." << s << " = …";
      break;
    }

    case JSOP_CALLPROP: {
      JSAtom* a = script->getAtom(GET_UINT32_INDEX(pc));
      if (const char* s = atomToStr(a)) out << "?." << s << "(…)";
      break;
    }

    case JSOP_CALL:
      out << "call(…)";
      break;

    case JSOP_ADD:  out << "+"; break;
    case JSOP_SUB:  out << "-"; break;
    case JSOP_MUL:  out << "*"; break;
    case JSOP_DIV:  out << "/"; break;
    case JSOP_MOD:  out << "%"; break;

    case JSOP_EQ:
    case JSOP_NE:
    case JSOP_LT:
    case JSOP_LE:
    case JSOP_GT:
    case JSOP_GE:
      if (const char* opStr = cmpOp(op)) out << opStr;
      break;

    case JSOP_POS:  out << "+(unary)"; break;
    case JSOP_NEG:  out << "-(unary)"; break;
    case JSOP_NOT:  out << "!(unary)"; break;

    case JSOP_GOTO: {
      ptrdiff_t off = GET_JUMP_OFFSET(pc);
      unsigned loc  = script->pcToOffset(pc);
      out << "jmp loc_"; out << std::uppercase << std::hex << (loc + int(off)) << std::dec;
      break;
    }

    case JSOP_TABLESWITCH:
      out << "switch ( … )";
      break;

    case JSOP_RETURN:
    case JSOP_RETRVAL:
      out << "return";
      break;

    case JSOP_LAMBDA: {
      // lambda <object#N>
      uint32_t idx = GET_UINT32_INDEX(pc);
      out << "function literal <object#" << idx << ">";
      break;
    }

    case JSOP_INITPROP: {
      JSAtom* a = script->getAtom(GET_UINT32_INDEX(pc));
      if (const char* s = atomToStr(a)) out << "init \"" << s << "\"";
      break;
    }

    default:
      break;
  }

  return out.str();
}

// ---- Try-note support -------------------------------------------------------
struct TryRegion {
  uint32_t id;
  uint32_t start;   // bytecode offset of guarded range begin
  uint32_t end;     // bytecode offset of guarded range end (start + length)
  uint32_t depth;   // saved stack depth
  uint8_t  kind;    // JSTRY_* kind
};


// Pre-indexed try boundary markers for O(1) lookup per instruction
struct TryBoundaryIndex {
  std::map<uint32_t, std::vector<TryRegion>> begins;
  std::map<uint32_t, std::vector<TryRegion>> ends;
};

// Collect trynote regions and build pre-indexed boundary maps
static void collectTryRegions(JSScript* script,
                              std::vector<TryRegion>& out,
                              std::vector<char>& isLabel,
                              TryBoundaryIndex& boundaryIndex) {
  out.clear();
  boundaryIndex.begins.clear();
  boundaryIndex.ends.clear();
  if (!script) return;

  auto tn = script->trynotes();
  if (!tn) return;

  uint32_t n = tn->length; // guard below avoids pathological tables
  if (n > script->length() / 2 + 1024) { // generous, scales with script
    // Large trynote table, may be corrupted - skip to avoid hang
    return;
  }

  uint32_t scriptLen = script->length();
  out.reserve(n);
  
  for (uint32_t i = 0; i < n; ++i) {
    const JSTryNote& t = tn->vector[i];
    
    // Bounds check for trynote data
    if (t.start > scriptLen || t.length > scriptLen || t.start + t.length > scriptLen) {
      // Bogus trynote data - skip this entry
      continue;
    }
    
    TryRegion r{};
    r.id    = i;
    r.start = t.start;
    r.end   = t.start + t.length;
    r.depth = t.stackDepth;
    r.kind  = t.kind;

    // Mark as leaders if within bounds
    if (r.start < isLabel.size()) isLabel[r.start] = 1;
    if (r.end < isLabel.size()) isLabel[r.end] = 1;

    // Pre-index boundaries for O(1) lookup
    boundaryIndex.begins[r.start].push_back(r);
    boundaryIndex.ends[r.end].push_back(r);

    out.push_back(r);
  }
}

// Emit begin/end markers for try regions at the boundary offsets - O(1) lookup
static void maybePrintTryBoundary(uint32_t loc, const TryBoundaryIndex& boundaryIndex) {
  auto beginIt = boundaryIndex.begins.find(loc);
  if (beginIt != boundaryIndex.begins.end()) {
    for (const auto& r : beginIt->second) {
      outPrintf("%s; try begin (%s, depth=%u, id=%u)%s\n",
                 COL_COMM, tryKindStr(r.kind), r.depth, r.id, COL_RESET);
    }
  }
  
  auto endIt = boundaryIndex.ends.find(loc);
  if (endIt != boundaryIndex.ends.end()) {
    for (const auto& r : endIt->second) {
      outPrintf("%s; try end   (%s, id=%u)%s\n",
                 COL_COMM, tryKindStr(r.kind), r.id, COL_RESET);
    }
  }
}

// Collect absolute bytecode offsets that are jump targets, so we can print labels.
static void collectLabelTargets(JSContext* cx, JSScript* script, std::vector<char>& isLabel,
                                std::vector<uint8_t>& opAt) {
  (void)cx;
  if (!script) return;
  isLabel.assign(script->length() + 1, 0);
  opAt.assign(script->length() + 1, 0);

  for (jsbytecode* pc = script->code(); pc < script->codeEnd(); ) {
    size_t oplen = js::GetBytecodeLength(pc);
    if (oplen == 0) {
      fprintf(stderr, "collectLabelTargets: bad opcode length at %zu — aborting\n", (size_t)script->pcToOffset(pc));
      break;
    }
    JSOp op = JSOp(*pc);
    const JSCodeSpec* cs = &js_CodeSpec[op];
    unsigned loc = script->pcToOffset(pc);
    opAt[loc] = static_cast<uint8_t>(op);
    if (JOF_TYPE(cs->format) == JOF_JUMP) {
      ptrdiff_t off = GET_JUMP_OFFSET(pc);
      unsigned tgt = loc + int(off);
      if (tgt <= script->length()) isLabel[tgt] = 1;
    } else if (op == JSOP_TABLESWITCH) {
      // Layout: [JSOP_TABLESWITCH][default][low][high][jump * (high-low+1)]
      jsbytecode* pc2 = pc + JUMP_OFFSET_LEN; // after opcode
      if (pc2 + 3*JUMP_OFFSET_LEN > script->codeEnd()) break;
      int32_t defOff = GET_JUMP_OFFSET(pc2); pc2 += JUMP_OFFSET_LEN;
      int32_t low    = GET_JUMP_OFFSET(pc2); pc2 += JUMP_OFFSET_LEN;
      int32_t high   = GET_JUMP_OFFSET(pc2); pc2 += JUMP_OFFSET_LEN;
      if (high < low) continue;
      int64_t n = int64_t(high) - int64_t(low) + 1;
      // guard: absurdly large table relative to script length
      if (n < 0 || n > (int64_t)script->length()) continue;
      if (pc2 + n*JUMP_OFFSET_LEN > script->codeEnd()) break;
      unsigned defTgt = loc + int(defOff);
      if (defTgt <= script->length()) isLabel[defTgt] = 1;
      for (int32_t i = 0; i < (int32_t)n; i++) {
        int32_t off = GET_JUMP_OFFSET(pc2); pc2 += JUMP_OFFSET_LEN;
        unsigned tgt = loc + int(off);
        if (tgt <= script->length()) isLabel[tgt] = 1;
      }
    }
    pc += oplen;
  }
}

// Infer the highest argument index referenced in the script.
static int inferMaxArgIndex(JSScript* script) {
  if (!script) return -1;
  int maxArg = -1;
  for (jsbytecode* pc = script->code(); pc < script->codeEnd(); pc += js::GetBytecodeLength(pc)) {
    JSOp op = JSOp(*pc);
    const JSCodeSpec* cs = &js_CodeSpec[op];
    if (JOF_TYPE(cs->format) == JOF_QARG) {
      int a = int(GET_ARGNO(pc));
      if (a > maxArg) maxArg = a;
    }
  }
  return maxArg;
}

static std::string formatInferredParams(int maxArg) {
  if (maxArg < 0) return std::string();
  std::ostringstream ss; ss << " (/* ";
  for (int i = 0; i <= maxArg; ++i) { if (i) ss << ", "; ss << "arg" << i; }
  ss << " */)"; return ss.str();
}

static bool readAll(const char* path, void** out, uint32_t* n) {
  FILE* f = std::fopen(path, "rb");
  if (!f) {
    logDebugf("readAll failed for %s", path);
    return false;
  }
  std::fseek(f, 0, SEEK_END);
  long len = std::ftell(f);
  std::fseek(f, 0, SEEK_SET);
  if (len <= 0) { std::fclose(f); return false; }
  void* buf = std::malloc((size_t)len);
  if (!buf) { std::fclose(f); return false; }
  if ((long)std::fread(buf, 1, (size_t)len, f) != len) { std::free(buf); std::fclose(f); return false; }
  std::fclose(f);
  *out = buf; *n = (uint32_t)len; return true;
}

static const JSClass global_class = {
  "global", JSCLASS_GLOBAL_FLAGS,
  JS_PropertyStub, JS_DeletePropertyStub, JS_PropertyStub, JS_StrictPropertyStub,
  JS_EnumerateStub, JS_ResolveStub, JS_ConvertStub, nullptr,
  nullptr, nullptr, nullptr, JS_GlobalObjectTraceHook
};


// Direct file writing without stdout hijacking  
bool writeDisassemblyToFile(JSContext* cx, JSScript* script, const char* functionName, const std::string& outPath) {
  // Set up a FILE* that writes to our string buffer
  char* bufPtr = nullptr;
  size_t bufSize = 0;
  FILE* memFile = open_memstream(&bufPtr, &bufSize);
  if (!memFile) {
    logErrorf("failed to create memory stream for disassembly");
    return false;
  }

  // Set global output file and format flags
  bool oldColor = gUseColor;
  bool oldShow  = gShowLines;
  bool oldSugar = gSugarEnabled;
  bool oldWrite = gWritingDis;
  
  setOutputFile(memFile);
  gUseColor   = false;
  gShowLines  = false;
  gSugarEnabled = gDisSugar;   // control sugar for .dis via gDisSugar (default: true) 
  gWritingDis = true;

  // Generate clean, minimal listing to memory buffer
  dumpScriptTree(cx, script, /*depth*/0);

  // Restore all settings
  setOutputFile(nullptr);
  gUseColor   = oldColor;
  gShowLines  = oldShow;
  gSugarEnabled = oldSugar;
  gWritingDis = oldWrite;

  // Close memory stream and get the content
  std::fclose(memFile);
  
  if (!bufPtr) {
    logErrorf("no disassembly content generated");
    return false;
  }
  
  std::string disContent(bufPtr, bufSize);
  std::free(bufPtr);
  
  // Write atomically to prevent corruption
  return writeFileAtomic(outPath, disContent);
}

// --- Minimal disassembler (no DEBUG-only deps) ---
static JSObject* safeGetObject(JSContext* cx, JSScript* script, uint32_t idx) {
  if (!script) {
    logDebugf("safeGetObject: null script (idx=%u)", idx);
    return nullptr;
  }
  auto objs = script->objects();
  if (!objs) {
    logDebugf("safeGetObject: no objects() array (idx=%u)", idx);
    return nullptr;
  }
  size_t len = objs->length;
  if (idx >= len) {
    logDebugf("safeGetObject: idx %u out of range (len=%zu)", idx, len);
    return nullptr;
  }
  JSObject* obj = script->getObject(idx);
  logDebugf("safeGetObject: got object #%u ptr=%p", idx, (void*)obj);
  return obj;
}

// Prints a quoted atom and bumps the caller's column count for visible chars.
static void printQuotedAtom(JSContext* cx, JSAtom* atom, int& col) {
  // Prefer SpiderMonkey's own conversion/escaping paths.
  auto vout = [](int &c, const char* fmt, ...) {
    va_list ap; va_start(ap, fmt);
    int n = outVprintf(fmt, ap);
    va_end(ap);
    if (n > 0) c += n;
  };
  if (!atom) { vout(col, " <atom:null>"); return; }

  // Hard cap for stability in disassembly output
  static const size_t kMaxAtomPrintBytes = 4096;

  // Use SpiderMonkey's built-in printable string conversion (already properly escapes)
  {
    JSAutoByteString bytes;
    if (AtomToPrintableString(cx, atom, &bytes) && bytes.ptr()) {
      const char* s = bytes.ptr();
      size_t n = std::strlen(s);
      if (n > kMaxAtomPrintBytes) n = kMaxAtomPrintBytes;
      vout(col, " \"");
      vout(col, "%.*s", (int)n, s);
      vout(col, "\"");
      return;
    }
  }

  // Last resort
  vout(col, " <atom>");
}

static bool decompileFunction() {
  if (!gDecompile) return true;
  // Decompile the entire .dis file in one pass
  std::string dis;
  if (!gDisPath.empty()) {
    if (!readFile(gDisPath, dis)) {
      // Always report file read errors, not just in verbose mode
      logErrorf("failed to read %s", redactPath(gDisPath).c_str());
      return false;
    }
  } else {
    logDebugf("decompile: no .dis path set");
    return false;
  }

  // Extract function name from file path for context
  std::string functionName = "main";
  if (!gDisPath.empty()) {
    size_t lastSlash = gDisPath.find_last_of('/');
    size_t lastDot = gDisPath.find_last_of('.');
    if (lastSlash != std::string::npos && lastDot != std::string::npos && lastDot > lastSlash) {
      functionName = gDisPath.substr(lastSlash + 1, lastDot - lastSlash - 1);
    }
  }
  
  std::string prompt = buildOllamaPrompt(dis, functionName);
  std::string resp;
  bool ok = generate(gOllamaHost, gOllamaModel, prompt, resp);

  if (!ok) {
    // Always report decompiler failures, not just in verbose mode
    logErrorf("decompiler call failed");
    return false;
  }

  // Clean fences and write pure JS
  std::string jsOut = stripMarkdownFences(resp);
  if (!gJsPath.empty()) {
    if (!writeFileAtomic(gJsPath, jsOut)) {
      logErrorf("failed to write %s", redactPath(gJsPath).c_str());
      return false;
    } else {
      logWarnf("wrote %s", redactPath(gJsPath).c_str());
    }
  }
  logDebugf("decompile: response chars=%zu", jsOut.size());
  outPrintf("%s\n", jsOut.c_str());
  return true;
}


// Helper to print colored jump/call/return location operand (counts visible chars)
static void printColoredLocOperand(int& col, unsigned cur, unsigned tgt,
                                   const std::vector<char>& isLabel,
                                   const std::vector<uint8_t>& opAt) {
  bool backward = tgt < cur;
  const char* colr = COL_ADDR;
  if (tgt < opAt.size()) {
    JSOp top = JSOp(opAt[tgt]);
    switch (top) {
      case JSOP_RETURN:
      case JSOP_RETRVAL: colr = COL_ADDR_RET; break;
      case JSOP_CALL:
      case JSOP_CALLPROP: colr = COL_ADDR_CALL; break;
      default:
        colr = backward ? COL_BRANCH_BACK : COL_BRANCH_FWD;
        break;
    }
  } else {
    colr = backward ? COL_BRANCH_BACK : COL_BRANCH_FWD;
  }
  if (tgt < isLabel.size() && isLabel[tgt]) colr = COL_ADDR_HOT;
  // count only visible characters
  auto vout = [](int &c, const char* fmt, ...) {
    va_list ap; va_start(ap, fmt);
    int n = outVprintf(fmt, ap);
    va_end(ap);
    if (n > 0) c += n;
  };
  vout(col, " ");
  outPrintf("%s", colr);         // color: no column count
  vout(col, "loc_%05X", tgt);
  outPrintf("%s", COL_RESET);    // reset: no column count
}

static void disasmOne(JSContext* cx, JSScript* script, jsbytecode* pc,
                      const std::vector<char>& isLabel, const std::vector<uint8_t>& opAt) {
  // Fixed column where comments start
  const int COMMENT_COL = gShowLines ? 68 : 60; // wider to accommodate longer strings

  // Helper that mirrors printf but also tracks the number of chars written
  auto vout = [](int &col, const char* fmt, ...) {
    va_list ap; va_start(ap, fmt);
    int n = outVprintf(fmt, ap);
    va_end(ap);
    if (n > 0) col += n;
  };

  JSOp op = JSOp(*pc);
  const JSCodeSpec* cs = &js_CodeSpec[op];
  unsigned loc  = script->pcToOffset(pc);
  unsigned line = js::PCToLineNumber(script, pc);
  const char* opname = js_CodeName[op];

  bool curHasImm = false; double curImm = 0.0;

  // Classify address color by opcode type
  auto addrColor = COL_ADDR; // default
  // Label lines are handled separately below; here we colorize by control-flow kind
  auto isJumpLike = [&](){
    switch (op) {
      case JSOP_GOTO:
      case JSOP_IFEQ:
      case JSOP_IFNE:
      case JSOP_TABLESWITCH:
        return true;
      default:
        return false;
    }
  };
  auto isCallLike = [&](){
    switch (op) {
      case JSOP_CALL:
      case JSOP_CALLPROP:
        return true;
      default:
        return false;
    }
  };
  auto isRetLike = [&](){
    switch (op) {
      case JSOP_RETURN:
      case JSOP_RETRVAL:
        return true;
      default:
        return false;
    }
  };
  if (isJumpLike())      addrColor = COL_ADDR_BRANCH;
  else if (isCallLike()) addrColor = COL_ADDR_CALL;
  else if (isRetLike())  addrColor = COL_ADDR_RET;

  // If this PC is a jump target, print a label line first
  if (loc < isLabel.size() && isLabel[loc]) {
    if (!gFirstLineOfFunction) outPrintf("\n"); // add spacing above labels
    int labcol = 0;
    outPrintf("%s", COL_LABEL);
    vout(labcol, "loc_%05X:", loc);
    outPrintf("%s", COL_RESET);
    int pad = COMMENT_COL - labcol; if (pad < 1) pad = 1;
    outPrintf("%*s%s; L%u%s\n", pad, "", COL_COMM, loc, COL_RESET);
  }

  int col = 0; // running count of printed characters for this line

  // Colorize address: yellow for leaders (labels), else by control-flow kind (branch/call/ret/normal)
  if (loc < isLabel.size() && isLabel[loc]) {
    outPrintf("%s", COL_ADDR_HOT);
    vout(col, "%05X", loc);
    outPrintf("%s", COL_RESET);
  } else {
    outPrintf("%s", addrColor);
    vout(col, "%05X", loc);
    outPrintf("%s", COL_RESET);
  }
  if (gShowLines) {
    vout(col, "  %4u  ", line);
  } else {
    vout(col, "  ");
  }
  outPrintf("%s", COL_MNEM);
  vout(col, "%-12s", opname);
  outPrintf("%s", COL_RESET);

  switch (JOF_TYPE(cs->format)) {
    case JOF_BYTE:
      if (op == JSOP_TRY) {
        // Try boundaries are now handled by maybePrintTryBoundary() before instruction printing
        // This instruction just establishes the try frame, the actual regions are marked separately
      }
      break;

    case JOF_JUMP: {
      ptrdiff_t off = GET_JUMP_OFFSET(pc);
      unsigned tgt = loc + int(off);
      printColoredLocOperand(col, loc, tgt, isLabel, opAt);
      vout(col, " (%+d)", int(off));
      break;
    }

    case JOF_SCOPECOORD: {
      JSAtom* name = js::ScopeCoordinateName(cx->runtime()->scopeCoordinateNameCache, script, pc);
      if (!name) {
        vout(col, " <atom:null>");
      } else {
        JSAutoByteString bytes;
        if (AtomToPrintableString(cx, name, &bytes))
          vout(col, " \"%s\"", bytes.ptr());
        else
          vout(col, " <atom>");
      }
      js::ScopeCoordinate sc(pc);
      vout(col, " (hops = %u, slot = %u)", sc.hops(), sc.slot());
      break;
    }

    case JOF_ATOM: {
      JSAtom* atom = script->getAtom(GET_UINT32_INDEX(pc));
      // Reuse the engine-backed print path with a small length cap.
      printQuotedAtom(cx, atom, col);
      break;
    }

    case JOF_DOUBLE: {
      JS::RootedValue v(cx, script->getConst(GET_UINT32_INDEX(pc)));
      JSAutoByteString bytes;
      JSString* s = js::ValueToSource(cx, v);
      if (s && bytes.encodeLatin1(cx, s))
        vout(col, " %s", bytes.ptr());
      else
        vout(col, " <const>");
      break;
    }

    case JOF_OBJECT: {
      uint32_t idx = GET_UINT32_INDEX(pc);
      // Be defensive: do not dereference the object table during disassembly.
      vout(col, " <object#%u>", idx);
      break;
    }

    case JOF_REGEXP: {
      vout(col, " <RegExp>");
      break;
    }

    case JOF_TABLESWITCH: {
      ptrdiff_t off = GET_JUMP_OFFSET(pc);
      jsbytecode* pc2 = pc + JUMP_OFFSET_LEN;
      int32_t low = GET_JUMP_OFFSET(pc2); pc2 += JUMP_OFFSET_LEN;
      int32_t high = GET_JUMP_OFFSET(pc2); pc2 += JUMP_OFFSET_LEN;
      vout(col, " default loc_%05X low %d high %d", script->pcToOffset(pc) + int(off), low, high);
      if (!gWritingDis && gSugarEnabled) {
        // Build a small summary: case value -> target label (first few entries)
        int32_t n = high - low + 1;
        int32_t show = n > 6 ? 6 : n; // cap
        unsigned loc = script->pcToOffset(pc);
        std::ostringstream ss;
        ss << "case ";
        jsbytecode* jpc = pc2;
        for (int32_t i = 0; i < show; i++) {
          int32_t joff = GET_JUMP_OFFSET(jpc); jpc += JUMP_OFFSET_LEN;
          unsigned tgt = loc + int(joff);
          if (i) ss << ", ";
          ss << (low + i) << "->loc_" << std::uppercase << std::hex << std::setw(5) << std::setfill('0') << tgt << std::dec;
        }
        if (n > show) ss << ", …";
        // Place summary at end of line
        int pad = (gShowLines ? 68 : 60) - col; if (pad < 1) pad = 1;
        outPrintf("%*s%s; %s%s\n", pad, "", COL_COMM, ss.str().c_str(), COL_RESET);
        return; // we've already ended the line with summary
      }
      break;
    }

    case JOF_QARG:
      vout(col, " %u", GET_ARGNO(pc));
      break;

    case JOF_LOCAL:
      vout(col, " %u", GET_LOCALNO(pc));
      break;

    case JOF_UINT16: {
      int val = int(GET_UINT16(pc)); outPrintf("%s", COL_NUM); vout(col, " %d", val); outPrintf("%s", COL_RESET); curHasImm = true; curImm = val;
      break;
    }
    case JOF_UINT24: {
      int val = int(GET_UINT24(pc)); outPrintf("%s", COL_NUM); vout(col, " %d", val); outPrintf("%s", COL_RESET); curHasImm = true; curImm = val;
      break;
    }
    case JOF_UINT8: {
      int val = unsigned(GET_UINT8(pc)); outPrintf("%s", COL_NUM); vout(col, " %u", val); outPrintf("%s", COL_RESET); curHasImm = true; curImm = val;
      break;
    }
    case JOF_INT8: {
      int val = int(GET_INT8(pc)); outPrintf("%s", COL_NUM); vout(col, " %d", val); outPrintf("%s", COL_RESET); curHasImm = true; curImm = val;
      break;
    }
    case JOF_INT32: {
      int val = int(GET_INT32(pc)); outPrintf("%s", COL_NUM); vout(col, " %d", val); outPrintf("%s", COL_RESET); curHasImm = true; curImm = val;
      break;
    }
    default:
      break;
  }

  // Handle specific opcodes that need special processing
  switch (op) {
    case JSOP_ONE:
      curHasImm = true; curImm = 1.0;
      break;
    case JSOP_ZERO:
      curHasImm = true; curImm = 0.0;
      break;
    case JSOP_TRUE:
      curHasImm = true; curImm = 1.0;
      break;
    case JSOP_FALSE:
      curHasImm = true; curImm = 0.0;
      break;
    case JSOP_LOOPENTRY:
      // Mark LOOPENTRY to hint the next GETLOCAL as loop index `i`
      gAfterLoopEntry = true;
      break;
    default:
      break;
  }

  // Pad to the fixed comment column (at least one space)
  if (col < COMMENT_COL) {
    int pad = COMMENT_COL - col;
    outPrintf("%*s", pad, "");
  } else {
    outPrintf(" ");
  }

  // Comments only when they add real value
  std::string cmt;

  // Mark loop index as `i` immediately after a LOOPENTRY
  if (op == JSOP_GETLOCAL) {
    int slot = int(GET_LOCALNO(pc));
    if (gAfterLoopEntry) {
      cmt = "i";
      gAfterLoopEntry = false;
    }
    // else: no default local[...] comment (too noisy)
  } else if (op == JSOP_GETARG) {
    cmt = "arg[" + std::to_string(int(GET_ARGNO(pc))) + "]";
  } else if (op == JSOP_GOTO || op == JSOP_IFEQ || op == JSOP_IFNE) {
    // Append a tiny condition hint if the previous op was a compare
    const char* cmp = cmpOp(gHist[0].op);
    if (cmp) cmt = std::string("if (") + cmp + ")";
  }

  // Recognize `local[x]++` sequence: GETLOCAL x, POS?, DUP, ONE, ADD, SETLOCAL x
  bool isInc = false; int incSlot = -1;
  if (op == JSOP_ADD) {
    if (gHist[0].op == JSOP_ONE && gHist[1].op == JSOP_DUP &&
        (gHist[2].op == JSOP_POS || gHist[2].op == JSOP_GETLOCAL) &&
        gHist[3].op == JSOP_GETLOCAL && gHist[3].local >= 0) {
      isInc = true; incSlot = gHist[3].local;
    }
  }
  if (isInc) {
    cmt = std::string("local[") + std::to_string(incSlot) + "]++";
  }

  // --- Minimal sugar rules ---
  // show/hide recognition for setVisible(X)
  if (op == JSOP_CALL && gHist[0].op == JSOP_TRUE && gHist[1].op == JSOP_SWAP && gHist[2].op == JSOP_CALLPROP) {
    cmt = "show";
  } else if (op == JSOP_CALL && gHist[0].op == JSOP_FALSE && gHist[1].op == JSOP_SWAP && gHist[2].op == JSOP_CALLPROP) {
    cmt = "hide";
  }
  // local[x] += K
  if (op == JSOP_SETLOCAL && gHist[0].op == JSOP_ADD && gHist[1].hasImm && gHist[3].op == JSOP_GETLOCAL) {
    int dst = int(GET_LOCALNO(pc));
    if (dst == gHist[3].local) {
      std::ostringstream ss; ss << "local[" << dst << "] += " << (int)gHist[1].imm;
      cmt = ss.str();
    }
  }
  // if (i < arg[n].length)
  if ((op == JSOP_IFEQ || op == JSOP_IFNE) && gHist[0].op == JSOP_LT && gHist[1].op == JSOP_GETPROP && gHist[2].op == JSOP_GETARG && gHist[3].op == JSOP_GETLOCAL) {
    std::ostringstream ss; ss << "if (i < arg[" <<
      (gHist[2].op == JSOP_GETARG ? gHist[2].local : 0) << "].length)";
    cmt = ss.str();
  }

  // Emit comment (or nothing) — no fallback sugar for obvious stuff
  if (gSugarEnabled && !cmt.empty()) {
    outPrintf("%s; %s%s\n", COL_COMM, cmt.c_str(), COL_RESET);
  } else {
    outPrintf("\n");
  }

  // Update history for next instruction
  int localIdx = -1;
  if (op == JSOP_GETLOCAL || op == JSOP_SETLOCAL) {
    localIdx = int(GET_LOCALNO(pc));
  } else if (op == JSOP_GETARG) {
    // Track argument index for JSOP_GETARG to fix sugar bug with arg index tracking
    localIdx = int(GET_ARGNO(pc));
  }
  pushHist(op, localIdx, curHasImm, curImm);

  // After printing any instruction, clear first-line flag so next label gets blank line
  gFirstLineOfFunction = false;
}

// Header state for each script (needs reset per script)
static bool headerPrinted = false;

void disasmScript(JSContext* cx, JSScript* script, const char* functionName) {
  logDebugf("disasmScript: begin (length=%zu bytes)", script ? script->length() : 0);
  if (!script) return;

  // Only print header for main script
  if (!headerPrinted && (!functionName || strcmp(functionName, "main") == 0)) {
    if (gShowLines) {
      outPrintf("loc     line  op\n");
      outPrintf("-----  ----  --\n");
    } else {
      outPrintf("loc     op\n");
      outPrintf("-----   --\n");
    }
    headerPrinted = true;
  }

  resetHistory();

  // First pass: collect jump targets for labels
  std::vector<char> isLabel; std::vector<uint8_t> opAt;
  collectLabelTargets(cx, script, isLabel, opAt);
  
  // Collect try regions and mark their boundaries as leaders
  std::vector<TryRegion> tryRegions;
  TryBoundaryIndex tryBoundaryIndex;
  collectTryRegions(script, tryRegions, isLabel, tryBoundaryIndex);

  // Reset label spacing state for this function
  int inferredMaxArg = inferMaxArgIndex(script);
  std::string inferredParams = formatInferredParams(inferredMaxArg);
  gFirstLineOfFunction = true;
  for (jsbytecode* pc = script->code(); pc < script->codeEnd(); /* advance below */) {
    unsigned loc = script->pcToOffset(pc);
    if (pc == script->main()) {
      if (functionName && strlen(functionName) > 0) {
        outPrintf("%s%s%s", COL_LABEL, functionName, COL_RESET);
        if (!gWritingDis && gSugarEnabled && !inferredParams.empty()) {
          outPrintf("%s%s%s", COL_COMM, inferredParams.c_str(), COL_RESET);
        }
        outPrintf("\n");
      } else {
        outPrintf("%smain%s", COL_LABEL, COL_RESET);
        if (!gWritingDis && gSugarEnabled && !inferredParams.empty()) {
          outPrintf("%s%s%s", COL_COMM, inferredParams.c_str(), COL_RESET);
        }
        outPrintf("\n");
      }
    }
    if ((loc % 1000) == 0) logDebugf("disasmScript: at pc=%zu", script->pcToOffset(pc));
    // Print label first (if any), then boundary comments, then the instruction
    maybePrintTryBoundary(loc, tryBoundaryIndex);
    disasmOne(cx, script, pc, isLabel, opAt);
    // Safe advance; bail on bad length to avoid hang
    size_t len = js::GetBytecodeLength(pc);
    if (len == 0) {
      logErrorf("bad opcode length at %u — aborting disassembly loop", loc);
      break;
    }
    pc += len;
  }
}

// Structure to hold function information
struct FunctionInfo {
  char name[64];
  int lambdaOffset;
  int propOffset;
  uint32_t objectIndex;
};

// Global array to store detected functions
static FunctionInfo detectedFunctions[32];
static int functionCount = 0;

// Structure to map lambda object indices to property names
struct LambdaMapping {
  uint32_t objectIndex;
  char propertyName[64];
  unsigned bytecodeOffset;
};

static LambdaMapping lambdaMappings[32];
static int lambdaCount = 0;

// Disassembly formatter
void dumpScriptAnalysis(JSContext* cx, JSScript* script, int depth, const char* functionName) {
  if (!script || depth > 5) return;
  
  if (depth == 0) logDebugf("main script");
  else            logDebugf("nested function (depth %d)", depth);
  
  // Reset micro-heuristic state for this script (for inner functions too)
  resetHistory();

  // Use our custom formatter
  dumpScriptFormat(cx, script, functionName);


}

// Walk inner (lambda) functions and dump their bytecode without executing them
static void dumpInnerFunctions(JSContext* cx, JSScript* script, int depth) {
  logDebugf("dumpInnerFunctions: depth=%d", depth);
  JS::RootedScript rscript(cx, script);
  if (!rscript)
    return;
  if (!gInnerEnabled) {
    logDebugf("dumpInnerFunctions: disabled (use --inner or DUMPER_INNER=1 to enable)");
    return;
  }
  
  // Prevent excessive recursion that can cause crashes
  if (depth >= 3) {
    deferWarnf("dumpInnerFunctions: max recursion depth reached (%d), stopping", depth);
    return;
  }

  auto dumpOne = [&](JS::HandleFunction fun, size_t tagIndex, uint32_t objectIndex) {
    if (!fun || !fun->isInterpreted())
      return;
    
    // Defensive: try to get script, but handle failures gracefully
    JS::RootedScript inner(cx, nullptr);
    try {
      inner = fun->getOrCreateScript(cx);
    } catch (...) {
      logDebugf("dumpOne: exception getting script for function at tagIndex=%zu", tagIndex);
      return;
    }
    
    if (!inner) {
      logDebugf("dumpOne: failed to get script for function at tagIndex=%zu", tagIndex);
      return;
    }

    // Try to print a useful name when available
    JS::Rooted<JSString*> disp(cx, nullptr);
    JSAutoByteString nameBytes;
    const char* nameStr = nullptr;
    
    try {
      disp = JS_GetFunctionDisplayId(fun);
      if (disp && nameBytes.encodeLatin1(cx, disp))
        nameStr = nameBytes.ptr();
    } catch (...) {
      logDebugf("dumpOne: exception getting function name at tagIndex=%zu", tagIndex);
      nameStr = nullptr;
    }

    const char* displayName = nullptr;
    if (nameStr) {
      displayName = nameStr;
    } else {
      const char* propName = getLambdaPropertyName(objectIndex);
      if (propName) {
        displayName = propName;
      }
    }
    
    // Convert MainGame<.ctor to MainGame.ctor format
    char cleanName[256];
    if (displayName) {
      strncpy(cleanName, displayName, 255);
      cleanName[255] = '\0';
      
      // Replace <. with just .
      char* pos = strstr(cleanName, "<.");
      if (pos) {
        memmove(pos, pos + 1, strlen(pos));
      }
      
      outPrintf("\n");
      
      logDebugf("Function: %s, Depth: %d, TagIndex: %zu", cleanName, depth + 1, tagIndex);
    } else {
      strcpy(cleanName, "unknown");
    }

    try {
      // Always show bytecode for inner functions when inner analysis is enabled
      dumpScriptAnalysis(cx, inner.get(), depth + 1, cleanName);
    } catch (...) {
      logErrorf("dumpOne: exception dumping script at tagIndex=%zu", tagIndex);
      logErrorf("Failed to analyze nested function (depth %d)", depth + 1);
    }
  };

  // Guard: avoid suspiciously long or obviously bogus object tables.
  if (rscript->objects() && (rscript->objects()->length > 100000 ||
                             rscript->objects()->length > rscript->length())) {
    logDebugf("dumpInnerFunctions: objects()->length looks suspicious (%u) vs script length %zu; skipping table walk",
        rscript->objects()->length, rscript->length());
  }

  bool foundAny = false;

  // Bytecode scan for function literals (JSOP_OBJECT / JSOP_LAMBDA / JOF_OBJECT)
  if (!foundAny) {
    logDebugf("dumpInnerFunctions: scanning bytecode for JSOP_OBJECT/JSOP_LAMBDA");
    jsbytecode* pc = rscript->code();
    jsbytecode* end = rscript->codeEnd();
    size_t tagIdx = 0;
    while (pc < end) {
      JSOp op = JSOp(*pc);
      if (JOF_TYPE(js_CodeSpec[op].format) == JOF_OBJECT) {
        uint32_t index = GET_UINT32_INDEX(pc);
        unsigned off = rscript->pcToOffset(pc);
        logDebugf("found JOF_OBJECT opcode at offset %u, idx=%u", off, index);
        // Resolve cautiously: only when the objects() table looks sane and index is in range.
        if (rscript->objects() &&
            rscript->objects()->length < 100000 &&
            rscript->objects()->length <= rscript->length() &&
            index < rscript->objects()->length) {
          JSObject* raw = safeGetObject(cx, rscript.get(), index);
          if (raw) {
            JS::RootedObject obj(cx, raw);
            // Still be defensive: some entries may not be functions.
            if (obj->is<JSFunction>()) {
              JS::RootedFunction fun(cx, &obj->as<JSFunction>());
              JS::RootedScript inner(cx, fun->getOrCreateScript(cx));
              if (inner) {
                logDebugf("dumpInnerFunctions: dumping function from literal index=%u", index);
                dumpOne(fun, tagIdx, index);
                foundAny = true;
              } else {
                logDebugf("dumpInnerFunctions: getOrCreateScript returned null for idx=%u", index);
              }
            } else {
              logDebugf("dumpInnerFunctions: object idx=%u is not a function", index);
            }
          }
        } else {
          logDebugf("dumpInnerFunctions: objects() table looks unsafe (len=%u, scriptLen=%zu); skipping resolution",
              rscript->objects() ? rscript->objects()->length : 0, rscript->length());
        }
        tagIdx++;
      }
      pc += js::GetBytecodeLength(pc);
    }
  }
}

// Function to scan bytecode and map lambda indices to property names
void mapLambdasToProperties(JSContext* cx, JSScript* script) {
  logDebugf("mapLambdasToProperties: start");
  lambdaCount = 0;
  memset(lambdaMappings, 0, sizeof(lambdaMappings));
  
  if (!script) return;
  
  jsbytecode* pc = script->code();
  jsbytecode* end = script->codeEnd();
  
  while (pc < end && lambdaCount < 32) {
    JSOp op = JSOp(*pc);
    unsigned offset = script->pcToOffset(pc);
    
    // Look for lambda <object#N> followed by initprop "propertyName"
    if (op == JSOP_LAMBDA && JOF_TYPE(js_CodeSpec[op].format) == JOF_OBJECT) {
      uint32_t objectIndex = GET_UINT32_INDEX(pc);
      
      // Look ahead for the next initprop instruction
      jsbytecode* nextPc = pc + js::GetBytecodeLength(pc);
      if (nextPc < end) {
        JSOp nextOp = JSOp(*nextPc);
        if (nextOp == JSOP_INITPROP && JOF_TYPE(js_CodeSpec[nextOp].format) == JOF_ATOM) {
          JSAtom* atom = script->getAtom(GET_UINT32_INDEX(nextPc));
          if (atom) {
            JSAutoByteString bytes;
            if (AtomToPrintableString(cx, atom, &bytes)) {
              lambdaMappings[lambdaCount].objectIndex = objectIndex;
              strncpy(lambdaMappings[lambdaCount].propertyName, bytes.ptr(), 63);
              lambdaMappings[lambdaCount].propertyName[63] = '\0';
              lambdaMappings[lambdaCount].bytecodeOffset = offset;
              logDebugf("mapLambdasToProperties: lambda object#%u -> property '%s' at offset %u", 
                  objectIndex, bytes.ptr(), offset);
              lambdaCount++;
            }
          }
        }
      }
    }
    pc += js::GetBytecodeLength(pc);
  }
  logDebugf("mapLambdasToProperties: found %d lambda->property mappings", lambdaCount);
}

// Function to get property name for a lambda object index
const char* getLambdaPropertyName(uint32_t objectIndex) {
  for (int i = 0; i < lambdaCount; i++) {
    if (lambdaMappings[i].objectIndex == objectIndex) {
      return lambdaMappings[i].propertyName;
    }
  }
  return nullptr;
}

// Function to parse real SpiderMonkey bytecode and extract function names
static void parseBytecodeForFunctions(JSContext* cx, JSScript* script, const char* functionName = nullptr) {
  logDebugf("parseBytecodeForFunctions: start");
  JS::RootedScript rscript(cx, script);
  functionCount = 0;
  
  // Reset function array
  memset(detectedFunctions, 0, sizeof(detectedFunctions));
  
  // First, map lambda indices to property names
  mapLambdasToProperties(cx, rscript.get());
  
  const char* nameToUse = functionName ? functionName : "main";
  disasmScript(cx, rscript.get(), nameToUse);
  outPrintf("\n");
  
  logDebugf("parseBytecodeForFunctions: found %d functions", functionCount);
  
  // If no functions found in objects, fall back to analyzing constants
  if (functionCount == 0) {
    // Look for string constants that might be function names
    logDebugf("No interpreted functions found in objects array");
    logDebugf("Analyzing string constants for function names...");
    logDebugf("parseBytecodeForFunctions: fallback to string-constant heuristic");
    // Only add synthetic name in debug mode to avoid noise
    if (gDebugEnabled) {
      strcpy(detectedFunctions[0].name, "main_entry");
      detectedFunctions[0].lambdaOffset = 0x000;
      detectedFunctions[0].propOffset = 0x005;
      functionCount = 1;
    }
  }
}

// Use actual SpiderMonkey data only
void dumpScriptFormat(JSContext* cx, JSScript* script, const char* functionName) {
  logDebugf("dumpScriptFormat: enter");
  JS::RootedScript rscript(cx, script);
  
  // Reset header state for each script
  headerPrinted = false;
  
  // Clear any previous deferred warnings
  gDeferredWarnings.clear();
  
  // Parse the bytecode to extract function information
  parseBytecodeForFunctions(cx, rscript.get(), functionName);

  // Recurse into nested interpreted functions (lambdas/inner functions)
  if (gInnerEnabled) {
    dumpInnerFunctions(cx, rscript.get(), /*depth=*/0);
  }
  
  // Flush any deferred warnings after all processing is complete
  flushDeferredWarnings();
  
  // Flush stderr to ensure all immediate logs are visible
  fflush(stderr);

  logDebugf("analysis done");
}

void dumpScriptTree(JSContext* cx, JSScript* script, int depth) {
  dumpScriptAnalysis(cx, script, depth, "main");
}

static void usage(const char* prog) {
  std::fprintf(stderr, "SpiderMonkey JavaScript bytecode dumper\n\n");
  std::fprintf(stderr, "USAGE:\n");
  std::fprintf(stderr, "  %s [OPTIONS] file.jsc\n\n", prog);
  std::fprintf(stderr, "OPTIONS:\n");
  std::fprintf(stderr, "  -v, --debug              Enable debug output\n");
  std::fprintf(stderr, "      --no-inner           Disable inner function analysis\n");
  std::fprintf(stderr, "      --color              Force color output\n");
  std::fprintf(stderr, "      --no-color           Disable color output\n");
  std::fprintf(stderr, "      --lines              Show line numbers\n");
  std::fprintf(stderr, "      --no-lines           Hide line numbers\n");
  std::fprintf(stderr, "      --no-sugar           Disable syntactic sugar recognition\n");
  std::fprintf(stderr, "      --no-dis-sugar       Make .dis plain (default includes sugar)\n");
  std::fprintf(stderr, "      --decompile          Enable LLM-based decompilation\n");
  std::fprintf(stderr, "      --ollama-host URL    Ollama server URL (default: http://localhost:11434)\n");
  std::fprintf(stderr, "      --ollama-model NAME  Ollama model name (default: llama31-abliterated-q8:latest)\n");
  std::fprintf(stderr, "      --ollama-timeout SEC Ollama request timeout in seconds (default: 300)\n");
  std::fprintf(stderr, "      --ollama-retries NUM Ollama retry attempts on failure (default: 3)\n");
  std::fprintf(stderr, "      --ollama-num-ctx NUM Ollama context window size in tokens (default: 65536, max: 131072)\n");
  std::fprintf(stderr, "  -h, --help               Show this help message\n\n");
  std::fprintf(stderr, "EXAMPLES:\n");
  std::fprintf(stderr, "  %s file.jsc                    # Basic disassembly\n", prog);
  std::fprintf(stderr, "  %s --debug file.jsc            # Debug output\n", prog);
  std::fprintf(stderr, "  %s --decompile file.jsc        # Decompile with LLM\n", prog);
  std::fprintf(stderr, "  %s --no-color file.jsc         # No color output\n", prog);
}

int main(int argc, char** argv) {
  const char* prog = argv[0];
  
  // Auto-enable colors only when writing to a TTY
  gUseColor = isatty(fileno(stdout)) != 0;
  
  // Enable debug if DUMPER_DEBUG is set and not "0"
  const char* envdbg = std::getenv("DUMPER_DEBUG");
  if (envdbg && envdbg[0] && strcmp(envdbg, "0") != 0) gDebugEnabled = true;
  const char* envinner = std::getenv("DUMPER_INNER");
  if (envinner && envinner[0] && strcmp(envinner, "0") != 0) gInnerEnabled = true;

  curl_global_init(CURL_GLOBAL_DEFAULT);

  enum {
    OPT_NO_INNER = 1000,
    OPT_COLOR,
    OPT_NO_COLOR,
    OPT_LINES,
    OPT_NO_LINES,
    OPT_NO_SUGAR,
    OPT_DECOMPILE,
    OPT_NO_DIS_SUGAR,
    /* keep values contiguous for getopt_long */
    OPT_OLLAMA_HOST,
    OPT_OLLAMA_MODEL,
    OPT_OLLAMA_TIMEOUT,
    OPT_OLLAMA_RETRIES,
    OPT_OLLAMA_NUM_CTX
  };

  static struct option long_options[] = {
    {"debug",              no_argument,       0, 'v'},
    {"no-inner",           no_argument,       0, OPT_NO_INNER},
    {"color",              no_argument,       0, OPT_COLOR},
    {"no-color",           no_argument,       0, OPT_NO_COLOR},
    {"lines",              no_argument,       0, OPT_LINES},
    {"no-lines",           no_argument,       0, OPT_NO_LINES},
    {"no-sugar",           no_argument,       0, OPT_NO_SUGAR},
    {"no-dis-sugar",       no_argument,       0, OPT_NO_DIS_SUGAR},
    {"decompile",          no_argument,       0, OPT_DECOMPILE},
    {"ollama-host",        required_argument, 0, OPT_OLLAMA_HOST},
    {"ollama-model",       required_argument, 0, OPT_OLLAMA_MODEL},
    {"ollama-timeout",     required_argument, 0, OPT_OLLAMA_TIMEOUT},
    {"ollama-retries",     required_argument, 0, OPT_OLLAMA_RETRIES},
    {"ollama-num-ctx",     required_argument, 0, OPT_OLLAMA_NUM_CTX},
    {"help",               no_argument,       0, 'h'},
    {0, 0, 0, 0}
  };

  int c;
  int option_index = 0;
  
  while ((c = getopt_long(argc, argv, "vh", long_options, &option_index)) != -1) {
    switch (c) {
      case 'v':
        gDebugEnabled = true;
        break;
      case 'h':
        usage(prog);
        curl_global_cleanup();
        return 0;
      case OPT_NO_INNER:
        gInnerEnabled = false;
        break;
      case OPT_COLOR:
        gUseColor = true;
        break;
      case OPT_NO_COLOR:
        gUseColor = false;
        break;
      case OPT_LINES:
        gShowLines = true;
        break;
      case OPT_NO_LINES:
        gShowLines = false;
        break;
      case OPT_NO_SUGAR:
        gSugarEnabled = false;
        break;
      case OPT_NO_DIS_SUGAR:
        gDisSugar = false;
        break;
      case OPT_DECOMPILE:
        gDecompile = true;
        break;
      case OPT_OLLAMA_HOST:
        gOllamaHost = optarg;
        break;
      case OPT_OLLAMA_MODEL:
        gOllamaModel = optarg;
        break;
      case OPT_OLLAMA_TIMEOUT:
        gOllamaTimeout = std::atoi(optarg);
        if (gOllamaTimeout <= 0) {
          logErrorf("Invalid timeout value: %s (must be positive)", optarg);
          return 1;
        }
        break;
      case OPT_OLLAMA_RETRIES:
        gOllamaRetries = std::atoi(optarg);
        if (gOllamaRetries < 0) {
          logErrorf("Invalid retries value: %s (must be non-negative)", optarg);
          return 1;
        }
        break;
      case OPT_OLLAMA_NUM_CTX:
        gOllamaNumCtx = std::atoi(optarg);
        if (gOllamaNumCtx < 1024 || gOllamaNumCtx > 131072) {
          logErrorf("Invalid context window size: %s (must be 1024-131072)", optarg);
          return 1;
        }
        break;
      case '?':
        // getopt_long already printed an error message
        usage(prog);
        curl_global_cleanup();
        return 2;
      default:
        logErrorf("Internal error: unhandled option %d", c);
        curl_global_cleanup();
        return 2;
    }
  }

  // Check for exactly one remaining argument (the input file)
  if (optind != argc - 1) {
    if (optind == argc) {
      logErrorf("Missing input file");
    } else {
      logErrorf("Too many arguments");
    }
    usage(prog);
    curl_global_cleanup();
    return 2;
  }
  
  const char* file = argv[optind];
  logDebugf("args: file=%s", file);

  gInputPath = file;
  gDisPath = siblingWithExt(gInputPath, ".dis");
  gJsPath  = siblingWithExt(gInputPath, ".js");



  if (!JS_Init()) { logErrorf("JS_Init failed");
    curl_global_cleanup();
    return 1; }
  JSRuntime* rt = JS_NewRuntime(64 * 1024 * 1024);
  JSContext* cx = JS_NewContext(rt, 32 * 1024);
  if (!rt || !cx) { logErrorf("rt/cx failed");
    curl_global_cleanup();
    return 1; }
  logDebugf("JS runtime/context created");

  {
    JSAutoRequest ar(cx);
    JS::CompartmentOptions opts;
    JS::RootedObject global(cx, JS_NewGlobalObject(cx, &global_class, nullptr,
                                                   JS::FireOnNewGlobalHook, opts));
    if (!global) { logErrorf("NewGlobalObject failed");
      curl_global_cleanup();
      return 1; }
    JSAutoCompartment ac(cx, global);
    JS_InitStandardClasses(cx, global);
    logDebugf("global created and standard classes initialized");

    void* bytes = nullptr; uint32_t n = 0;
    if (!readAll(file, &bytes, &n)) { logErrorf("read failed for %s", redactPath(file).c_str());
      curl_global_cleanup();
      return 1; }
    logDebugf("read %u bytes from %s", n, file);

    JSScript* top = JS_DecodeScript(cx, bytes, n, nullptr);
    if (!top) {
      logErrorf("JS_DecodeScript failed");
      std::free(bytes);
      curl_global_cleanup();
      return 1;
    }
    logDebugf("JS_DecodeScript: success");

    if (writeDisassemblyToFile(cx, top, "main", gDisPath)) {
      logWarnf("wrote %s", redactPath(gDisPath).c_str());
    } else {
      logErrorf("failed to write %s", redactPath(gDisPath).c_str());
      std::free(bytes);
      curl_global_cleanup();
      return 1;
    }

    dumpScriptTree(cx, top);

    if (gDecompile) {
      (void)decompileFunction(); // callee logs success/failure
    }

    std::free(bytes);
  }

  JS_DestroyContext(cx);
  JS_DestroyRuntime(rt);
  JS_ShutDown();
  curl_global_cleanup();
  return 0;
}
