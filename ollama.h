#pragma once

#include <string>
#include <curl/curl.h>
#include <cstddef>

// Ollama configuration globals
extern std::string gOllamaHost;
extern std::string gOllamaModel;
extern int gOllamaTimeout;
extern int gOllamaRetries;
extern int gOllamaNumCtx;

// HTTP response context
struct CurlCtx {
  std::string body;
  size_t bytes{0};
  size_t lastNotified{0};
  bool gotHeaders{false};
  size_t contentLength{0};
  bool sawFirstByte{false};
};

// libcurl callbacks
size_t curlHeader(char* buffer, size_t size, size_t nitems, void* userdata);
size_t curlWrite(char* ptr, size_t size, size_t nmemb, void* userdata);

// Main LLM integration function
bool generate(const std::string& host, const std::string& model, 
              const std::string& prompt, std::string& response);

// Prompt building
std::string buildOllamaPrompt(const std::string& disasm, const std::string& functionName = "main");