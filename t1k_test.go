package t1k

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

var update = flag.Bool("update", false, "rewrite the golden files in testdata from the current output")

const examplePayload = "examples/enerplanet-calculation.json"

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// golden compares got with testdata/<name>, rewriting the file with -update.
func golden(t *testing.T, name string, got []byte) {
	t.Helper()
	path := filepath.Join("testdata", name)
	if *update {
		if err := os.WriteFile(path, append(got, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want := bytes.TrimRight(readFile(t, path), "\n")
	if !bytes.Equal(got, want) {
		t.Errorf("%s differs from the golden file (run go test -update to refresh):\n%s", name, got)
	}
}

func TestDefaultMappingGolden(t *testing.T) {
	task := NewTransformTask(WithIndent("", "  "))
	meme, err := task.Transform(readFile(t, examplePayload))
	if err != nil {
		t.Fatal(err)
	}
	golden(t, "meme-job.golden.json", meme)

	back, err := task.Reverse(meme)
	if err != nil {
		t.Fatal(err)
	}
	golden(t, "enerplanet-reverse.golden.json", back)
}

// TestDefaultMappingRoundTrip pins the documented behaviour of the default
// mapping: converting the reverse output forward again reproduces the MEME
// job except for buildings without a connection (they have no transmission
// arc, so the reverse cannot place them in the topology), and the reverse is
// stable from then on.
func TestDefaultMappingRoundTrip(t *testing.T) {
	task := NewTransformTask()
	meme, err := task.Transform(readFile(t, examplePayload))
	if err != nil {
		t.Fatal(err)
	}
	back, err := task.Reverse(meme)
	if err != nil {
		t.Fatal(err)
	}
	again, err := task.Transform(back)
	if err != nil {
		t.Fatal(err)
	}
	want := mustDecode(t, string(meme)).(map[string]any)
	model := want["model"].(map[string]any)
	delete(model["nodes"].(map[string]any), "4")
	delete(model["technologies"].(map[string]any), "demand-4")
	if got := mustDecode(t, string(again)); !equalJSON(got, want) {
		t.Errorf("second forward pass differs beyond the standalone building:\n%s", again)
	}
	back2, err := task.Reverse(again)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(back, back2) {
		t.Errorf("reverse output is not stable:\n%s\n---\n%s", back, back2)
	}
}

func TestDefaultMappingContent(t *testing.T) {
	meme, err := NewTransformTask().Transform(readFile(t, examplePayload))
	if err != nil {
		t.Fatal(err)
	}
	var job struct {
		Model struct {
			Metadata     map[string]any            `json:"metadata"`
			Time         map[string]string         `json:"time"`
			Nodes        map[string]map[string]any `json:"nodes"`
			Technologies map[string]map[string]any `json:"technologies"`
			Transmission map[string]map[string]any `json:"transmission"`
			Trade        map[string]map[string]any `json:"trade"`
		} `json:"model"`
		Experiment map[string]any `json:"experiment"`
	}
	if err := json.Unmarshal(meme, &job); err != nil {
		t.Fatal(err)
	}
	m := job.Model
	if m.Metadata["name"] != "158_1790500000" || m.Time["start"] != "2018-01-01T00:00:00Z" || m.Time["resolution"] != "1H" {
		t.Errorf("metadata/time: %v %v", m.Metadata, m.Time)
	}
	if len(m.Nodes) != 6 {
		t.Errorf("nodes = %d, want 6 (four buildings, two transformers)", len(m.Nodes))
	}
	for _, key := range []string{"pv_supply-1", "battery_storage-1", "demand-1", "demand-2", "demand-3", "demand-4"} {
		if _, ok := m.Technologies[key]; !ok {
			t.Errorf("technology %q missing", key)
		}
	}
	if _, ok := m.Technologies["demand-trafo_2585"]; ok {
		t.Error("a transformer must not get a demand technology")
	}
	pv := m.Technologies["pv_supply-1"]
	if pv["node"] != "1" || pv["role"] != "supply" {
		t.Errorf("pv tech = %v", pv)
	}
	if cap := pv["capacity"].(map[string]any); cap["max"] != 0.008 || cap["expandable"] != true {
		t.Errorf("pv capacity = %v (kW must become MW)", cap)
	}
	if _, ok := m.Technologies["battery_storage-1"]["storage"]; !ok {
		t.Error("battery needs a storage block")
	}
	for _, key := range []string{"lv_0", "lv_1", "mv_2", "lv_3"} {
		if _, ok := m.Transmission[key]; !ok {
			t.Errorf("transmission %q missing", key)
		}
	}
	if len(m.Transmission) != 4 {
		t.Errorf("transmission = %d arcs, want 4 (the standalone building has none)", len(m.Transmission))
	}
	grid := m.Trade["grid-trafo_2585"]
	if grid == nil || grid["node"] != "trafo_2585" || grid["import"].(map[string]any)["limit"] != 0.16 {
		t.Errorf("transformer trade = %v", grid)
	}
	if _, ok := m.Trade["non_households-2"]; !ok {
		t.Error("the supermarket's tariff trade is missing")
	}
	if job.Experiment["mode"] != "plan" {
		t.Errorf("experiment = %v", job.Experiment)
	}
}

func TestTransformTaskOptions(t *testing.T) {
	cfg, err := LoadConfig([]byte(`{"name": "t", "rules": [{"from": "a", "to": "b"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	compact, err := NewTransformTask(WithConfig(cfg)).Transform([]byte(`{"a": [1, 2]}`))
	if err != nil || string(compact) != `{"b":[1,2]}` {
		t.Fatalf("compact = %s, %v", compact, err)
	}
	indented, err := NewTransformTask(WithConfig(cfg), WithIndent("", "  ")).Transform([]byte(`{"a": [1]}`))
	if err != nil || string(indented) != "{\n  \"b\": [\n    1\n  ]\n}" {
		t.Fatalf("indented = %q, %v", indented, err)
	}
	back, err := NewTransformTask(WithConfig(cfg)).Reverse(compact)
	if err != nil || string(back) != `{"a":[1,2]}` {
		t.Fatalf("reverse = %s, %v", back, err)
	}
	if NewTransformTask(WithConfig(nil)).cfg != DefaultConfig() {
		t.Error("WithConfig(nil) must keep the default configuration")
	}
	_, err = NewTransformTask(WithConfig(cfg)).Transform([]byte(`{"a": 1`))
	if !errors.Is(err, ErrInput) {
		t.Errorf("truncated input error = %v, want ErrInput", err)
	}
}

func TestTransformTaskConcurrency(t *testing.T) {
	input := readFile(t, examplePayload)
	want, err := NewTransformTask().Transform(input)
	if err != nil {
		t.Fatal(err)
	}
	shared := NewTransformTask()
	var wg sync.WaitGroup
	errs := make(chan error, 64)
	for i := 0; i < 32; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			got, err := shared.Transform(input)
			if err != nil {
				errs <- err
			} else if !bytes.Equal(got, want) {
				errs <- errors.New("shared task produced a different result")
			}
		}()
		go func() {
			defer wg.Done()
			got, err := NewTransformTask().Transform(input)
			if err != nil {
				errs <- err
			} else if !bytes.Equal(got, want) {
				errs <- errors.New("separate task produced a different result")
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg == nil || cfg.Name != "enerplanet-to-meme" || cfg.Rules() == 0 {
		t.Fatalf("default config = %+v", cfg)
	}
	data := DefaultConfigJSON()
	if len(data) == 0 {
		t.Fatal("DefaultConfigJSON is empty")
	}
	data[0] = 'x'
	if DefaultConfigJSON()[0] == 'x' {
		t.Error("DefaultConfigJSON must return a copy")
	}
	if _, err := LoadConfig(DefaultConfigJSON()); err != nil {
		t.Errorf("embedded configuration does not load: %v", err)
	}
	defer func() {
		if recover() == nil {
			t.Error("mustLoadConfig should panic on an invalid configuration")
		}
	}()
	mustLoadConfig([]byte(`{}`))
}
