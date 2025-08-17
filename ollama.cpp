#include "ollama.h"
#include "logging.h"
#include <sstream>
#include <chrono>
#include <random>
#include <unistd.h>
#include <cstring>
#include <nlohmann/json.hpp>
using nlohmann::json;

// Ollama configuration globals
std::string gOllamaHost = "http://localhost:11434";
std::string gOllamaModel = "llama31-abliterated-q8:latest";
int gOllamaTimeout = 300;
int gOllamaRetries = 3;
int gOllamaNumCtx = 65536;

// Header callback: detect status line and Content-Length
size_t curlHeader(char* buffer, size_t size, size_t nitems, void* userdata) {
  size_t n = size * nitems;
  CurlCtx* ctx = static_cast<CurlCtx*>(userdata);
  // Parse HTTP status
  if (n >= 12 && std::memcmp(buffer, "HTTP/", 5) == 0) {
    if (!ctx->gotHeaders) {
      ctx->gotHeaders = true;
      logDebugf("decompile: headers");
    }
  }
  // Parse Content-Length if present
  static const char kCL[] = "Content-Length:";
  if (n > sizeof(kCL)-1 && strncasecmp(buffer, kCL, sizeof(kCL)-1) == 0) {
    const char* p = buffer + sizeof(kCL)-1;
    while (*p == ' ' || *p == '\t') ++p;
    ctx->contentLength = (size_t)strtoull(p, nullptr, 10);
    logDebugf("decompile: content-length: %zu bytes", ctx->contentLength);
  }
  return n;
}

// Body callback: accumulate and print first-byte/periodic progress
size_t curlWrite(char* ptr, size_t size, size_t nmemb, void* userdata) {
  extern bool gDebugEnabled;
  size_t n = size * nmemb;
  CurlCtx* ctx = static_cast<CurlCtx*>(userdata);
  if (!ctx->sawFirstByte) {
    ctx->sawFirstByte = true;
    logDebugf("decompile: receiving");
  }
  ctx->body.append(ptr, n);
  ctx->bytes += n;

  // Every 32KB (or when content-length is small), show a short progress line
  const size_t STEP = 32 * 1024;
  if (gDebugEnabled &&
      (ctx->bytes - ctx->lastNotified >= STEP || (ctx->contentLength && ctx->bytes == ctx->contentLength))) {
      if (ctx->contentLength) {
        double pct = (double)ctx->bytes * 100.0 / (double)ctx->contentLength;
        logDebugf("decompile: received: %zu / %zu bytes (%.1f%%)", ctx->bytes, ctx->contentLength, pct);
      } else {
        logDebugf("decompile: received: %zu bytes", ctx->bytes);
      }
      ctx->lastNotified = ctx->bytes;
    }
  return n;
}

// Helper function to check if an error is retryable
static bool isRetryableError(CURLcode rc, long http_code) {
  // Retryable network/timeout errors
  if (rc == CURLE_OPERATION_TIMEDOUT || rc == CURLE_COULDNT_CONNECT || 
      rc == CURLE_COULDNT_RESOLVE_HOST || rc == CURLE_RECV_ERROR ||
      rc == CURLE_SEND_ERROR || rc == CURLE_PARTIAL_FILE) {
    return true;
  }
  // Retryable HTTP errors (server overload)
  if (http_code == 500 || http_code == 502 || http_code == 503 || http_code == 504) {
    return true;
  }
  return false;
}

bool generate(const std::string& host, const std::string& model,
                     const std::string& prompt, std::string& out) {
  // Build JSON body once
  json jbody = {
    {"model", model},
    {"prompt", prompt},
    {"stream", false},
    {"options", {{"num_ctx", gOllamaNumCtx}}}
  };
  std::string body = jbody.dump();
  std::string url = host + "/api/generate";

  logDebugf("decompile: connect %s (%s)", host.c_str(), model.c_str());
  logDebugf("decompile: request bytes: %zu", body.size());
  
  // Show timeout in progress message
  int timeoutMins = gOllamaTimeout / 60;
  int timeoutSecs = gOllamaTimeout % 60;
  if (timeoutMins > 0) {
    logWarnf("query %s (timeout: %dm%ds, retries: %d)", model.c_str(), timeoutMins, timeoutSecs, gOllamaRetries);
  } else {
    logWarnf("query %s (timeout: %ds, retries: %d)", model.c_str(), timeoutSecs, gOllamaRetries);
  }

  // Retry loop with jittered exponential backoff and wall-time caps
  auto startTime = std::chrono::steady_clock::now();
  for (int attempt = 0; attempt <= gOllamaRetries; attempt++) {
    CURL* curl = curl_easy_init();
    if (!curl) return false;

    struct curl_slist* hdrs = nullptr;
    hdrs = curl_slist_append(hdrs, "Content-Type: application/json");
    hdrs = curl_slist_append(hdrs, "Accept: application/json");
    hdrs = curl_slist_append(hdrs, "Expect:");

    CurlCtx ctx;

    // Aggressive anti-hang defaults for better user experience
    curl_easy_setopt(curl, CURLOPT_TCP_KEEPALIVE, 1L);
    curl_easy_setopt(curl, CURLOPT_TCP_KEEPIDLE, 15L);
    curl_easy_setopt(curl, CURLOPT_TCP_KEEPINTVL, 10L);
    curl_easy_setopt(curl, CURLOPT_NOSIGNAL, 1L);
    curl_easy_setopt(curl, CURLOPT_CONNECTTIMEOUT, 5L);     // Faster connection timeout
    curl_easy_setopt(curl, CURLOPT_SERVER_RESPONSE_TIMEOUT, 30L); // TTFB watchdog: max 30s for first byte
    logDebugf("TTFB watchdog: 30s timeout for first byte, connect timeout: 5s");
    curl_easy_setopt(curl, CURLOPT_LOW_SPEED_TIME, (long)gOllamaTimeout); // configurable processing time
    curl_easy_setopt(curl, CURLOPT_LOW_SPEED_LIMIT, 1L);    // Very low minimum speed requirement
    curl_easy_setopt(curl, CURLOPT_TIMEOUT, (long)gOllamaTimeout); // configurable timeout

    curl_easy_setopt(curl, CURLOPT_URL, url.c_str());
    curl_easy_setopt(curl, CURLOPT_HTTPHEADER, hdrs);
    curl_easy_setopt(curl, CURLOPT_POSTFIELDS, body.c_str());

    // Feedback hooks
    curl_easy_setopt(curl, CURLOPT_HEADERFUNCTION, curlHeader);
    curl_easy_setopt(curl, CURLOPT_HEADERDATA, &ctx);
    curl_easy_setopt(curl, CURLOPT_WRITEFUNCTION, curlWrite);
    curl_easy_setopt(curl, CURLOPT_WRITEDATA, &ctx);

    // Optional curl verbose in debug mode
    extern bool gDebugEnabled;
    if (gDebugEnabled) {
      curl_easy_setopt(curl, CURLOPT_VERBOSE, 1L);
    }

    CURLcode rc = curl_easy_perform(curl);
    long http_code = 0;
    double t_conn = 0.0, t_start = 0.0, t_total = 0.0;
    curl_easy_getinfo(curl, CURLINFO_RESPONSE_CODE, &http_code);
    curl_easy_getinfo(curl, CURLINFO_CONNECT_TIME, &t_conn);
    curl_easy_getinfo(curl, CURLINFO_STARTTRANSFER_TIME, &t_start);
    curl_easy_getinfo(curl, CURLINFO_TOTAL_TIME, &t_total);

    curl_slist_free_all(hdrs);
    curl_easy_cleanup(curl);

    logDebugf("decompile: HTTP %ld (connect %.2fs, TTFB %.2fs, total %.2fs)", http_code, t_conn, t_start, t_total);

    // Success case
    if (rc == CURLE_OK && http_code / 100 == 2) {
      // Parse JSON response robustly
      try {
        json j = json::parse(ctx.body);
        if (j.contains("response") && j["response"].is_string()) {
          out = j["response"].get<std::string>();
          return true;
        }
        if (j.contains("error") && j["error"].is_string()) {
          logErrorf("Ollama error: %s", j["error"].get<std::string>().c_str());
          return false; // Don't retry on model errors
        }
      } catch (...) {
        logErrorf("failed to parse JSON response");
        return false; // Don't retry on parse errors
      }
      return false;
    }

    // Check if we should retry
    bool shouldRetry = isRetryableError(rc, http_code) && (attempt < gOllamaRetries);
    
    // Always show HTTP status and response body excerpt for decompile failures
    if (http_code >= 400) {
      std::string bodyExcerpt = ctx.body;
      if (bodyExcerpt.length() > 200) {
        bodyExcerpt = bodyExcerpt.substr(0, 200) + "...";
      }
      logWarnf("[decompile] HTTP %ld; body: %s", http_code, bodyExcerpt.c_str());
    }
    
    // Log detailed error information
    if (rc == CURLE_OPERATION_TIMEDOUT) {
      logErrorf("Request timed out after %ds. Ollama may be overloaded or model not loaded.", gOllamaTimeout);
    } else if (rc != CURLE_OK) {
      logErrorf("Network error: %s", curl_easy_strerror(rc));
    } else if (http_code == 500) {
      logErrorf("Server error (HTTP 500). Ollama may be overloaded or prompt too large.");
      logWarnf("see ~/.ollama/logs/server.log for details");
    } else if (http_code >= 400) {
      logErrorf("HTTP %ld error", http_code);
    }
    
    if (!ctx.body.empty()) {
      std::string preview = ctx.body.substr(0, 200);
      for (char& ch : preview) { if (ch == '\n' || ch == '\r') ch = ' '; }
      logErrorf("decompile error: %s", preview.c_str());
    }

    if (!shouldRetry) {
      fflush(stdout);
      return false;
    }

    // Jittered exponential backoff with wall-time caps
    // Base: 1s, 2s, 4s, 8s... with ±20% jitter and 60s max per delay
    int baseBackoffSecs = 1 << attempt;
    int maxBackoffSecs = std::min(baseBackoffSecs, 60); // Cap at 60s per delay
    
    // Add ±20% jitter to prevent thundering herd
    static std::random_device rd;
    static std::mt19937 gen(rd());
    std::uniform_real_distribution<> jitterDist(0.8, 1.2); // ±20%
    double jitteredBackoffSecs = maxBackoffSecs * jitterDist(gen);
    
    // Check wall-time cap: don't retry if total time would exceed 10 minutes
    auto now = std::chrono::steady_clock::now();
    auto elapsed = std::chrono::duration_cast<std::chrono::seconds>(now - startTime).count();
    const int MAX_TOTAL_RETRY_TIME = 600; // 10 minutes wall time cap
    
    if (elapsed + (int)jitteredBackoffSecs > MAX_TOTAL_RETRY_TIME) {
      logWarnf("Abandoning retry: would exceed %d-minute wall-time cap", MAX_TOTAL_RETRY_TIME / 60);
      return false;
    }
    
    logWarnf("Retrying in %.1fs... (attempt %d/%d, elapsed %lds)", 
             jitteredBackoffSecs, attempt + 2, gOllamaRetries + 1, (long)elapsed);
    fflush(stdout);
    
    // Sleep with millisecond precision for jitter
    int sleepMs = (int)(jitteredBackoffSecs * 1000);
    usleep(sleepMs * 1000);
  }

  logErrorf("All retry attempts failed");
  return false;
}


// Prompt building (always available)
std::string buildOllamaPrompt(const std::string& disasm, const std::string& functionName) {
  extern int gOllamaNumCtx;
  extern bool gDebugEnabled;
  
  std::ostringstream p;
  p << "Decompile this SpiderMonkey bytecode into valid JavaScript.\n\n"
    
    << "OUTPUT FORMAT - respond with ONLY this structure:\n"
    << "/*\n"
    << " * Function: " << functionName << "\n"
    << " * Behavior: [brief description]\n"
    << " */\n"
    << "function " << functionName << "() {\n"
    << "    // JavaScript code here\n"
    << "}\n\n"
    
    << "CRITICAL RULES:\n"
    << "- Output ONLY the comment block + function\n"
    << "- NO explanations, prose, or markdown outside the code\n"
    << "- Convert all bytecode operations to equivalent JavaScript\n"
    << "- Use descriptive variable names when possible\n\n"
    
    << "Bytecode:\n";
  
  // Token-aware budgeting: reserve ~20% headroom for prompt overhead + JavaScript output
  // Estimate: 1 token ≈ 4 characters (conservative for technical text)
  const size_t CHARS_PER_TOKEN = 4;
  const double HEADROOM_RATIO = 0.20;
  const size_t MAX_DISASM_SIZE = static_cast<size_t>((gOllamaNumCtx * CHARS_PER_TOKEN) * (1.0 - HEADROOM_RATIO));
  if (gDebugEnabled) {
    size_t estTokensDis = disasm.size() / CHARS_PER_TOKEN;
    size_t estTokensPrompt = (p.str().size() + disasm.size()) / CHARS_PER_TOKEN;
    logDebugf("prompt budget: ctx=%d tokens, disasm≈%zu tok, est total≈%zu tok (headroom %.0f%%)",
              gOllamaNumCtx, estTokensDis, estTokensPrompt, HEADROOM_RATIO * 100.0);
  }
  if (disasm.size() <= MAX_DISASM_SIZE) {
    p << disasm << "\n";
  } else {
    logWarnf("disassembly truncated for %d-token context: %zu bytes → %zu bytes", gOllamaNumCtx, disasm.size(), MAX_DISASM_SIZE);
    // Truncate disassembly but keep first and last parts for context
    size_t half = MAX_DISASM_SIZE / 2;
    p << disasm.substr(0, half) << "\n... [TRUNCATED " << (disasm.size() - MAX_DISASM_SIZE) << " chars for token budget] ...\n"
      << disasm.substr(disasm.size() - half) << "\n";
  }
  
  return p.str();
}