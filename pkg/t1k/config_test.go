package t1k

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	cfg, err := LoadConfig([]byte(`{"name": "demo", "version": "1", "description": "d", "rules": [{"from": "a", "to": "b"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Name != "demo" || cfg.Version != "1" || cfg.Description != "d" || cfg.Rules() != 1 {
		t.Errorf("metadata = %q %q %q, %d rules", cfg.Name, cfg.Version, cfg.Description, cfg.Rules())
	}
	_, err = LoadConfig([]byte(`{"name": "x", "rules": [{"from": "a[$i]", "to": "b"}]}`))
	if !errors.Is(err, ErrConfig) || !strings.Contains(err.Error(), "rules[0]") {
		t.Errorf("invalid configuration error = %v, want ErrConfig naming the rule", err)
	}
}

func TestLoadConfigFile(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "good.json")
	if err := os.WriteFile(good, []byte(`{"name": "file", "rules": [{"from": "a", "to": "b"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfigFile(good)
	if err != nil || cfg.Name != "file" {
		t.Fatalf("LoadConfigFile = %v, %v", cfg, err)
	}
	if _, err := LoadConfigFile(filepath.Join(dir, "missing.json")); !errors.Is(err, ErrConfig) {
		t.Errorf("missing file error = %v, want ErrConfig", err)
	}
	bad := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(bad, []byte(`{"name": "x"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = LoadConfigFile(bad)
	if !errors.Is(err, ErrConfig) || !strings.Contains(err.Error(), "bad.json") {
		t.Errorf("bad file error = %v", err)
	}
}

func TestErrorsAreClassified(t *testing.T) {
	cfg, err := LoadConfig([]byte(`{"name": "t", "rules": [{"from": "a", "to": "b", "convert": "number"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	task := NewTransformTask(WithConfig(cfg))
	if _, err := task.Transform([]byte(`{"a": true}`)); !errors.Is(err, ErrRule) {
		t.Errorf("rule failure = %v, want ErrRule", err)
	}
	if _, err := task.Reverse([]byte(`{"b": "x"}`)); !errors.Is(err, ErrRule) {
		t.Errorf("reverse rule failure = %v, want ErrRule", err)
	}
	if _, err := task.Transform([]byte(`{"a": `)); !errors.Is(err, ErrInput) {
		t.Errorf("invalid input = %v, want ErrInput", err)
	}
}
