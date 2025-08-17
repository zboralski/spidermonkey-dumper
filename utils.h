#pragma once

#include <string>
#include <cstdio>

// File I/O utilities
std::string siblingWithExt(const std::string& path, const char* newExt);
bool readFile(const std::string& path, std::string& out);
bool writeFileAtomic(const std::string& path, const std::string& data);

// String utilities  
std::string stripMarkdownFences(const std::string& in);
std::string redactPath(const std::string& path);

// Output utilities
void outPrintf(const char* fmt, ...) __attribute__((format(printf, 1, 2)));
int outVprintf(const char* fmt, va_list args);
void setOutputFile(FILE* file);