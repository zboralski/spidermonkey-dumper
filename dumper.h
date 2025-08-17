#ifndef DUMPER_H
#define DUMPER_H

// Prefer forward declarations to avoid forcing jsapi.h on all users.
struct JSContext;
struct JSScript;

#include <string>

/// Public API (external linkage). Default args only here, not in .cpp.
void dumpScriptFormat(JSContext* cx, JSScript* script, const char* functionName = nullptr);
void dumpScriptAnalysis(JSContext* cx, JSScript* script, int depth = 0, const char* functionName = nullptr);
void mapLambdasToProperties(JSContext* cx, JSScript* script);
const char* getLambdaPropertyName(uint32_t objectIndex);
void disasmScript(JSContext* cx, JSScript* script, const char* functionName = nullptr);
void dumpScriptTree(JSContext* cx, JSScript* script, int depth = 0);
bool writeDisassemblyToFile(JSContext* cx, JSScript* script, const char* functionName, const std::string& outPath);

#endif // DUMPER_H