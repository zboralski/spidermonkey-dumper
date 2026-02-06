package decompile

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"text/template"
	"time"
)

// Backend names for LLM decompilation.
const (
	BackendClaude = "claude-code"
	BackendCodex  = "codex"
)

// Config holds settings for LLM decompilation.
type Config struct {
	Backend string // claude-code, codex
	Model   string // model name (backend-specific)
	Timeout time.Duration
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() Config {
	return Config{
		Backend: BackendClaude,
		Timeout: 5 * time.Minute,
	}
}

// promptTmpl is the decompilation prompt template.
var promptTmpl = template.Must(template.New("prompt").Parse(`Decompile this SpiderMonkey 33 bytecode into idiomatic JavaScript.

OUTPUT FORMAT - respond with ONLY this structure:
/*
 * {{.Name}}
 *
 * [Concise analysis: what the function does, its inputs/outputs,
 *  any notable patterns (event handlers, initialization, state
 *  machines, etc.)]
 */
function {{.Name}}() {
    // idiomatic JavaScript here
}

RULES:
- Output ONLY the comment block + function. No prose outside the code.
- Write idiomatic JS: use const/let, modern patterns, meaningful names.
- The comment block IS the analysis. Keep it concise (3-6 lines).
- Reconstruct control flow naturally. No mechanical 1:1 opcode translation.

Bytecode:
{{.Disasm}}
`))

// buildPrompt constructs the decompilation prompt from disassembly text.
func buildPrompt(disasm string, functionName string) (string, error) {
	var b strings.Builder
	if err := promptTmpl.Execute(&b, struct {
		Name   string
		Disasm string
	}{functionName, disasm}); err != nil {
		return "", fmt.Errorf("build prompt: %w", err)
	}
	return b.String(), nil
}

// Decompile sends disassembly to an LLM backend and returns JavaScript.
func Decompile(ctx context.Context, cfg Config, disasm string, funcName string) (string, error) {
	prompt, err := buildPrompt(disasm, funcName)
	if err != nil {
		return "", err
	}

	var raw string

	switch cfg.Backend {
	case BackendClaude:
		raw, err = decompileClaude(ctx, cfg, prompt)
	case BackendCodex:
		raw, err = decompileCodex(ctx, cfg, prompt)
	default:
		return "", fmt.Errorf("unknown backend %q", cfg.Backend)
	}
	if err != nil {
		return "", err
	}

	return StripMarkdownFences(raw), nil
}

// decompileClaude uses `claude -p` for non-interactive output.
func decompileClaude(ctx context.Context, cfg Config, prompt string) (string, error) {
	args := []string{"-p", "--no-session-persistence"}
	if cfg.Model != "" {
		args = append(args, "--model", cfg.Model)
	}

	ctx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "claude", args...)
	cmd.Stdin = strings.NewReader(prompt)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("claude: %w: %s", err, stderr.String())
	}
	return stdout.String(), nil
}

// decompileCodex uses `codex exec -` for non-interactive output.
func decompileCodex(ctx context.Context, cfg Config, prompt string) (string, error) {
	args := []string{"exec"}
	if cfg.Model != "" {
		args = append(args, "-m", cfg.Model)
	}
	args = append(args, "-") // read prompt from stdin

	ctx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "codex", args...)
	cmd.Stdin = strings.NewReader(prompt)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("codex: %w: %s", err, stderr.String())
	}
	return stdout.String(), nil
}

// StripMarkdownFences removes ```javascript ... ``` wrappers from LLM output.
func StripMarkdownFences(s string) string {
	s = strings.TrimSpace(s)

	// Remove opening fence
	for _, prefix := range []string{"```javascript", "```js", "```"} {
		if strings.HasPrefix(s, prefix) {
			s = strings.TrimPrefix(s, prefix)
			s = strings.TrimLeft(s, "\n")
			break
		}
	}

	// Remove closing fence
	if strings.HasSuffix(s, "```") {
		s = strings.TrimSuffix(s, "```")
		s = strings.TrimRight(s, "\n")
	}

	return s
}
