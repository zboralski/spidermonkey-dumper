#pragma once

#include <vector>
#include <string>

// Global state
extern bool gDebugEnabled;
extern std::vector<std::string> gDeferredWarnings;

// Core logging functions
void logDebugf(const char* fmt, ...) __attribute__((format(printf, 1, 2)));
void logWarnf(const char* fmt, ...) __attribute__((format(printf, 1, 2)));
void logErrorf(const char* fmt, ...) __attribute__((format(printf, 1, 2)));

// Deferred logging functions
void deferWarnf(const char* fmt, ...) __attribute__((format(printf, 1, 2)));
void deferErrorf(const char* fmt, ...) __attribute__((format(printf, 1, 2)));
void deferDebugf(const char* fmt, ...) __attribute__((format(printf, 1, 2)));

// Deferred warning management
void flushDeferredWarnings();