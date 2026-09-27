package config

import (
	"encoding/json"
	"testing"
)

func TestDefaultMappingIsShipped(t *testing.T) {
	data, err := Read(Default)
	if err != nil {
		t.Fatalf("Read(%q): %v", Default, err)
	}
	if !json.Valid(data) {
		t.Fatalf("%s is not valid JSON", Default)
	}
	entries, err := Files.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) == 0 {
		t.Fatal("no mappings embedded")
	}
	if _, err := Read("missing.json"); err == nil {
		t.Fatal("reading a missing mapping should fail")
	}
}
