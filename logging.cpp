#include "logging.h"
#include <cstdio>
#include <cstdarg>

// Global state
bool gDebugEnabled = false;
std::vector<std::string> gDeferredWarnings;

void logDebugf(const char* fmt, ...) {
  if (!gDebugEnabled) return;
  va_list args;
  va_start(args, fmt);
  fprintf(stderr, "[+] ");
  vfprintf(stderr, fmt, args);
  fprintf(stderr, "\n");
  va_end(args);
}

void logWarnf(const char* fmt, ...) {
  va_list args;
  va_start(args, fmt);
  fprintf(stderr, "[*] ");
  vfprintf(stderr, fmt, args);
  fprintf(stderr, "\n");
  va_end(args);
}

// Deferred warning function that buffers messages
void deferWarnf(const char* fmt, ...) {
  va_list args;
  va_start(args, fmt);
  char buffer[1024];
  vsnprintf(buffer, sizeof(buffer), fmt, args);
  gDeferredWarnings.push_back(std::string("[*] " + std::string(buffer)));
  va_end(args);
}

void logErrorf(const char* fmt, ...) {
  va_list args;
  va_start(args, fmt);
  fprintf(stderr, "[-] ");
  vfprintf(stderr, fmt, args);
  fprintf(stderr, "\n");
  va_end(args);
}

// Additional deferred logging functions for completeness
void deferErrorf(const char* fmt, ...) {
  va_list args;
  va_start(args, fmt);
  char buffer[1024];
  vsnprintf(buffer, sizeof(buffer), fmt, args);
  gDeferredWarnings.push_back(std::string("[-] " + std::string(buffer)));
  va_end(args);
}

void deferDebugf(const char* fmt, ...) {
  if (!gDebugEnabled) return;
  va_list args;
  va_start(args, fmt);
  char buffer[1024];
  vsnprintf(buffer, sizeof(buffer), fmt, args);
  gDeferredWarnings.push_back(std::string("[+] " + std::string(buffer)));
  va_end(args);
}

void flushDeferredWarnings() {
  for (const auto& warning : gDeferredWarnings) {
    // Messages already include their prefixes ([*], [-], [+])
    if (warning.find("[") == 0) {
      fprintf(stderr, "%s\n", warning.c_str());
    } else {
      // Fallback for messages without proper prefixes
      fprintf(stderr, "[*] %s\n", warning.c_str());
    }
  }
  gDeferredWarnings.clear();
}