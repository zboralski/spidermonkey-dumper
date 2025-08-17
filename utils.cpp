#include "utils.h"
#include <filesystem>
#include <vector>
#include <cstdio>
#include <cstdarg>
#include <unistd.h>
#include <fcntl.h>
#include <sys/stat.h>

extern bool gDebugEnabled;

// Global output file for safePrintf
static FILE* gSafeOutputFile = nullptr;

std::string siblingWithExt(const std::string& p, const char* newExt) {
  std::filesystem::path path(p);
  std::filesystem::path newPath = path.parent_path() / path.stem();
  newPath += newExt;
  return newPath.string();
}

bool readFile(const std::string& path, std::string& out) {
  FILE* f = std::fopen(path.c_str(), "rb");
  if (!f) return false;
  std::fseek(f, 0, SEEK_END);
  long n = std::ftell(f);
  std::fseek(f, 0, SEEK_SET);
  if (n < 0) { std::fclose(f); return false; }
  out.resize((size_t)n);
  if (n && std::fread(&out[0], 1, (size_t)n, f) != (size_t)n) { std::fclose(f); return false; }
  std::fclose(f);
  return true;
}

// Atomic file write: temp → flush → sync → rename
bool writeFileAtomic(const std::string& path, const std::string& data) {
  std::filesystem::path fsPath(path);
  std::filesystem::path dir = fsPath.parent_path();
  if (dir.empty()) dir = ".";
  std::string base = fsPath.stem().string();
  
  // Template: <dir>/<base>.XXXXXX
  std::filesystem::path templatePath = dir / (base + ".XXXXXX");
  std::string templateStr = templatePath.string();
  std::vector<char> tempTemplate(templateStr.begin(), templateStr.end());
  tempTemplate.push_back('\0');
  
  int fd = mkstemp(tempTemplate.data());
  if (fd == -1) return false;
  
  std::string tempPath(tempTemplate.data());
  
  if (fchmod(fd, 0600) != 0) {
    close(fd);
    std::remove(tempPath.c_str());
    return false;
  }
  
  FILE* f = fdopen(fd, "wb");
  if (!f) {
    close(fd);
    std::remove(tempPath.c_str());
    return false;
  }
  
  std::string tempPathStr(tempTemplate.data());

  if (std::fwrite(data.data(), 1, data.size(), f) != data.size()) {
    std::fclose(f);
    std::remove(tempPathStr.c_str());
    return false;
  }
  
  if (std::fflush(f) != 0) {
    std::fclose(f);
    std::remove(tempPathStr.c_str());
    return false;
  }
  
  if (fsync(fileno(f)) != 0) {
    std::fclose(f);
    std::remove(tempPathStr.c_str());
    return false;
  }
  
  std::fclose(f);
  
  if (std::rename(tempPathStr.c_str(), path.c_str()) != 0) {
    std::remove(tempPathStr.c_str());
    return false;
  }
  
  return true;
}

// Strip Markdown code fences (``` or ```js) from a model response, preserving code
std::string stripMarkdownFences(const std::string& in) {
  std::string out; out.reserve(in.size());
  size_t i = 0;
  while (i < in.size()) {
    // fence line start?
    if ((i == 0 || in[i-1] == '\n') && i + 2 < in.size()
        && in[i] == '`' && in[i+1] == '`' && in[i+2] == '`') {
      // skip ```[lang]* to EOL
      i += 3;
      while (i < in.size() && in[i] != '\n') ++i;
      if (i < in.size() && in[i] == '\n') ++i;
      continue;
    }
    out.push_back(in[i++]);
  }
  return out;
}

// Path redaction helper: show only filename in non-debug logs, full path in debug
std::string redactPath(const std::string& path) {
  if (gDebugEnabled) {
    return path; // Show full path in debug mode
  }
  
  // Extract just the filename for non-debug logs
  std::filesystem::path p(path);
  return p.filename().string();
}

// Printf wrapper that writes to gSafeOutputFile or stdout
void outPrintf(const char* fmt, ...) {
  va_list args;
  va_start(args, fmt);
  vfprintf(gSafeOutputFile ? gSafeOutputFile : stdout, fmt, args);
  va_end(args);
}

int outVprintf(const char* fmt, va_list args) {
  return vfprintf(gSafeOutputFile ? gSafeOutputFile : stdout, fmt, args);
}

void setOutputFile(FILE* file) {
  gSafeOutputFile = file;
}