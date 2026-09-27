package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/enerplanet/T1K/pkg/t1k"
)

const payload = "../../examples/enerplanet-calculation.json"

func runCLI(t *testing.T, stdin string, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errBuf bytes.Buffer
	code = run(args, strings.NewReader(stdin), &out, &errBuf)
	return code, out.String(), errBuf.String()
}

func TestVersionAndConfig(t *testing.T) {
	code, out, _ := runCLI(t, "", "-version")
	if code != exitOK || !strings.HasPrefix(out, "t1k dev") {
		t.Errorf("-version: %d %q", code, out)
	}
	code, out, _ = runCLI(t, "", "-print-config")
	if code != exitOK || !strings.Contains(out, `"name": "enerplanet-to-meme"`) {
		t.Errorf("-print-config: %d %q", code, out[:min(len(out), 80)])
	}
}

func TestUsageErrors(t *testing.T) {
	if code, _, stderr := runCLI(t, "", "-bogus"); code != exitUsage || !strings.Contains(stderr, "Usage") {
		t.Errorf("unknown flag: %d %q", code, stderr)
	}
	if code, _, stderr := runCLI(t, "", "extra"); code != exitUsage || !strings.Contains(stderr, "unexpected argument") {
		t.Errorf("extra argument: %d %q", code, stderr)
	}
	if code, _, _ := runCLI(t, "", "-h"); code != exitOK {
		t.Errorf("-h should exit 0, got %d", code)
	}
}

func TestStdinToStdout(t *testing.T) {
	input, err := os.ReadFile(payload)
	if err != nil {
		t.Fatal(err)
	}
	code, out, stderr := runCLI(t, string(input))
	if code != exitOK || stderr != "" {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
	if !strings.Contains(out, "\n  \"model\": {") || !strings.HasSuffix(out, "}\n") {
		t.Errorf("expected indented output ending in a newline, got %q...", out[:min(len(out), 60)])
	}
	code, compact, _ := runCLI(t, string(input), "-compact")
	if code != exitOK || strings.Count(compact, "\n") != 1 || !strings.HasPrefix(compact, `{"experiment":`) {
		t.Errorf("-compact output: %d %q...", code, compact[:min(len(compact), 60)])
	}
}

func TestFilesAndReverse(t *testing.T) {
	dir := t.TempDir()
	memePath := filepath.Join(dir, "meme.json")
	if code, _, stderr := runCLI(t, "", "-in", payload, "-out", memePath); code != exitOK {
		t.Fatalf("forward: %d %s", code, stderr)
	}
	backPath := filepath.Join(dir, "back.json")
	if code, _, stderr := runCLI(t, "", "-reverse", "-in", memePath, "-out", backPath); code != exitOK {
		t.Fatalf("reverse: %d %s", code, stderr)
	}
	back, err := os.ReadFile(backPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(back), `"model_id": "158_1790500000"`) || !strings.Contains(string(back), `"pipe": "lv"`) {
		t.Errorf("reverse output lacks expected fields: %s", back[:min(len(back), 200)])
	}
}

func TestCustomConfig(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "swap.json")
	if err := os.WriteFile(cfgPath, []byte(`{"name": "swap", "rules": [{"from": "a", "to": "b"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	code, out, _ := runCLI(t, `{"a": 1}`, "-config", cfgPath, "-compact")
	if code != exitOK || out != "{\"b\":1}\n" {
		t.Errorf("custom config: %d %q", code, out)
	}
	code, out, _ = runCLI(t, `{"b": 2}`, "-config", cfgPath, "-compact", "-reverse")
	if code != exitOK || out != "{\"a\":2}\n" {
		t.Errorf("custom config reverse: %d %q", code, out)
	}
}

func TestFailures(t *testing.T) {
	if code, _, stderr := runCLI(t, "", "-config", "/nonexistent/t1k.json"); code != exitError || !strings.Contains(stderr, "invalid configuration") {
		t.Errorf("missing config: %d %q", code, stderr)
	}
	if code, _, stderr := runCLI(t, "", "-in", "/nonexistent/in.json"); code != exitError || !strings.Contains(stderr, "read input") {
		t.Errorf("missing input: %d %q", code, stderr)
	}
	if code, _, stderr := runCLI(t, "not json"); code != exitError || !strings.Contains(stderr, "invalid input") {
		t.Errorf("invalid input: %d %q", code, stderr)
	}
	if code, _, stderr := runCLI(t, `{"topology": [{"to": {}, "length": "far", "pipe": "lv"}]}`); code != exitError || !strings.Contains(stderr, "rule failed") {
		t.Errorf("rule failure: %d %q", code, stderr)
	}
	if code, _, stderr := runCLI(t, `{}`, "-out", filepath.Join(t.TempDir(), "missing-dir", "out.json")); code != exitError || !strings.Contains(stderr, "write output") {
		t.Errorf("unwritable output: %d %q", code, stderr)
	}
}

func TestPrintConfigMatchesTheEmbeddedMapping(t *testing.T) {
	code, out, _ := runCLI(t, "", "-print-config")
	if code != exitOK || out != string(t1k.DefaultConfigJSON())+"\n" {
		t.Errorf("-print-config differs from DefaultConfigJSON (exit %d)", code)
	}
}

func TestStdinReadError(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := run(nil, iotest.ErrReader(errors.New("broken pipe")), &out, &errBuf)
	if code != exitError || !strings.Contains(errBuf.String(), "read standard input: broken pipe") {
		t.Errorf("stdin error: %d %q", code, errBuf.String())
	}
}

func TestInvalidConfigFile(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(cfgPath, []byte(`{"name": "bad", "rules": [{"from": "a[$i]", "to": "b"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	code, _, stderr := runCLI(t, `{}`, "-config", cfgPath)
	if code != exitError || !strings.Contains(stderr, "bad.json") || !strings.Contains(stderr, "rules[0]") {
		t.Errorf("invalid config: %d %q", code, stderr)
	}
}

func TestReverseCompactAndFlagsTogether(t *testing.T) {
	code, out, _ := runCLI(t, `{"model": {"metadata": {"name": "m"}}}`, "-reverse", "-compact")
	if code != exitOK || !strings.HasPrefix(out, `{"callback_url":"","country":"","lkr":"","model_id":"m"`) {
		t.Errorf("-reverse -compact: %d %q", code, out)
	}
	// -in takes precedence over standard input.
	code, out, _ = runCLI(t, `not json`, "-in", payload, "-compact")
	if code != exitOK || !strings.HasPrefix(out, `{"experiment":`) {
		t.Errorf("-in with junk on stdin: %d %q", code, out[:min(len(out), 40)])
	}
}
