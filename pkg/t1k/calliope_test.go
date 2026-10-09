package t1k

import (
	"encoding/json"
	"regexp"
	"testing"

	"github.com/enerplanet/T1K/internal/jsondoc"
)

// calliopeID is Calliope 0.7's identifier syntax (AttrStr in
// src/calliope/schemas/general.py at v0.7.0.dev7, the version MEME pins):
// no leading digit or underscore, then letters, digits and underscores.
var calliopeID = regexp.MustCompile(`^[^_^\d][\w]*$`)

// issue1Payload is the calculation payload of issue #1: one building with
// a numeric id on an LV line to a transformer, and an MV line between two
// transformers. Every building in a real model has such an id.
const issue1Payload = `{
  "callback_url": "http://backend:8000/api/v1/calculation/callback/999",
  "country": "NL", "end_date": "2018-01-02T00:00:00.000Z", "lkr": "Apeldoorn",
  "model_id": "el_test_1", "resolution": 60, "session_id": "1",
  "start_date": "2018-01-01T00:00:00.000Z", "user_id": "u1",
  "topology": [
    {"from": {"id": "trafo_1", "type": "Feature", "geometry": {"type": "Point", "coordinates": [6.02745, 52.10148]},
              "properties": {"id": "trafo_1", "osm_id": "Trafo_1", "feature_type": "TopologyNode", "f_class": "transformer", "demand_energy": 0, "demand_heat": 0, "rated_power": 160},
              "techs": null, "custom_demand_timeseries": null},
     "to": {"id": "trafo_2", "type": "Feature", "geometry": {"type": "Point", "coordinates": [6.03102, 52.10355]},
            "properties": {"id": "trafo_2", "osm_id": "Trafo_2", "feature_type": "TopologyNode", "f_class": "transformer", "demand_energy": 0, "demand_heat": 0, "rated_power": 250},
            "techs": null, "custom_demand_timeseries": null},
     "length": 0.18, "pipe": "mv"},
    {"from": {"id": "1", "type": "Feature", "geometry": {"type": "Point", "coordinates": [6.02745, 52.10148]},
              "properties": {"id": "1", "osm_id": "268428040", "feature_type": "BasePOI", "f_class": "house", "area": 72.53, "demand_energy": 3745, "demand_heat": 5803},
              "techs": {"pv_supply": {"cont_energy_cap_max": 8, "cont_lifetime": 25, "cost_interest_rate": 0.02, "cost_energy_cap": 575}},
              "custom_demand_timeseries": null},
     "to": {"id": "trafo_1", "type": "Feature", "geometry": {"type": "Point", "coordinates": [6.02732, 52.10132]},
            "properties": {"id": "trafo_1", "osm_id": "Trafo_1", "feature_type": "TopologyNode", "f_class": "transformer", "demand_energy": 0, "demand_heat": 0, "rated_power": 160},
            "techs": null, "custom_demand_timeseries": null},
     "length": 0.0182, "pipe": "lv"}
  ]
}`

// memeModel is the part of a MEME job the identifier checks look at.
type memeModel struct {
	Carriers     map[string]json.RawMessage `json:"carriers"`
	Nodes        map[string]json.RawMessage `json:"nodes"`
	Technologies map[string]struct {
		Node       string   `json:"node"`
		CarrierIn  []string `json:"carrier_in"`
		CarrierOut []string `json:"carrier_out"`
	} `json:"technologies"`
	Transmission map[string]struct {
		Carrier string `json:"carrier"`
		From    string `json:"from"`
		To      string `json:"to"`
	} `json:"transmission"`
	Trade map[string]struct {
		Node    string `json:"node"`
		Carrier string `json:"carrier"`
	} `json:"trade"`
}

// decodeModel reads a job; single-string carrier fields are normalised to
// lists so references can be checked uniformly.
func decodeModel(t *testing.T, job []byte) memeModel {
	t.Helper()
	var doc struct {
		Model json.RawMessage `json:"model"`
	}
	if err := json.Unmarshal(job, &doc); err != nil {
		t.Fatal(err)
	}
	normalised := regexp.MustCompile(`"carrier_(in|out)":\s*"([^"]*)"`).ReplaceAll(doc.Model, []byte(`"carrier_$1":["$2"]`))
	var m memeModel
	if err := json.Unmarshal(normalised, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

// TestMappingIdentifiersAreValidForCalliope is the regression test for issue
// #1: every generated key and every reference satisfies Calliope's
// identifier syntax, every reference resolves, and the reverse restores
// EnerPlanET's ids.
func TestMappingIdentifiersAreValidForCalliope(t *testing.T) {
	task := NewTransformTask()
	job, err := task.Transform([]byte(issue1Payload))
	if err != nil {
		t.Fatal(err)
	}
	m := decodeModel(t, job)

	check := func(where, id string) {
		t.Helper()
		if !calliopeID.MatchString(id) {
			t.Errorf("%s: %q is not a valid Calliope identifier", where, id)
		}
	}
	resolves := func(where, id string, defined map[string]json.RawMessage) {
		t.Helper()
		check(where, id)
		if _, ok := defined[id]; !ok {
			t.Errorf("%s: %q is not defined", where, id)
		}
	}
	for id := range m.Carriers {
		check("carriers", id)
	}
	for id := range m.Nodes {
		check("nodes", id)
	}
	for id, tech := range m.Technologies {
		check("technologies", id)
		resolves("technologies."+id+".node", tech.Node, m.Nodes)
		for _, c := range append(tech.CarrierIn, tech.CarrierOut...) {
			resolves("technologies."+id+".carrier", c, m.Carriers)
		}
	}
	for id, arc := range m.Transmission {
		check("transmission", id)
		resolves("transmission."+id+".from", arc.From, m.Nodes)
		resolves("transmission."+id+".to", arc.To, m.Nodes)
		resolves("transmission."+id+".carrier", arc.Carrier, m.Carriers)
	}
	for id, trade := range m.Trade {
		check("trade", id)
		resolves("trade."+id+".node", trade.Node, m.Nodes)
		resolves("trade."+id+".carrier", trade.Carrier, m.Carriers)
	}

	// The keys the issue reported, in their valid form.
	for _, want := range []string{"n1", "ntrafo_1", "ntrafo_2"} {
		if _, ok := m.Nodes[want]; !ok {
			t.Errorf("node %q missing; nodes = %v", want, keysOf(m.Nodes))
		}
	}
	if _, ok := m.Technologies["demand_1"]; !ok || m.Technologies["demand_1"].Node != "n1" {
		t.Errorf("demand technology = %v", m.Technologies["demand_1"])
	}
	if _, ok := m.Technologies["pv_supply_1"]; !ok || m.Technologies["pv_supply_1"].Node != "n1" {
		t.Errorf("pv technology = %v", m.Technologies["pv_supply_1"])
	}
	if _, ok := m.Trade["grid_trafo_1"]; !ok || m.Trade["grid_trafo_1"].Node != "ntrafo_1" {
		t.Errorf("transformer trade = %v", m.Trade["grid_trafo_1"])
	}
	if arc := m.Transmission["lv_1"]; arc.From != "n1" || arc.To != "ntrafo_1" {
		t.Errorf("lv arc = %v", arc)
	}

	// The reverse restores the EnerPlanET ids and keeps the topology intact.
	back, err := task.Reverse(job)
	if err != nil {
		t.Fatal(err)
	}
	var payload struct {
		Topology []struct {
			From, To struct {
				ID         string `json:"id"`
				Properties struct {
					ID string `json:"id"`
				} `json:"properties"`
			}
			Pipe string `json:"pipe"`
		} `json:"topology"`
	}
	if err := json.Unmarshal(back, &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Topology) != 2 {
		t.Fatalf("reverse topology has %d entries", len(payload.Topology))
	}
	got := map[string]bool{}
	for _, c := range payload.Topology {
		got[c.Pipe+":"+c.From.ID+">"+c.To.ID] = true
		for _, f := range []struct{ id, pid string }{{c.From.ID, c.From.Properties.ID}, {c.To.ID, c.To.Properties.ID}} {
			if f.id != f.pid {
				t.Errorf("feature id %q and properties.id %q differ after the reverse", f.id, f.pid)
			}
		}
	}
	for _, want := range []string{"mv:trafo_1>trafo_2", "lv:1>trafo_1"} {
		if !got[want] {
			t.Errorf("connection %s not restored; got %v", want, got)
		}
	}
	// And the round trip is exact: no standalone building limits it here.
	again, err := task.Transform(back)
	if err != nil {
		t.Fatal(err)
	}
	if !jsondoc.Equal(mustDecode(t, string(job)), mustDecode(t, string(again))) {
		t.Errorf("second forward pass differs:\n%s", again)
	}
}

func keysOf(m map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// TestMappingRejectsIdsCalliopeCannotTake pins that an id the prefix cannot
// repair fails the transformation with the rule named, instead of reaching
// MEME: a hyphen survives the prefix, so the match guard rejects it.
func TestMappingRejectsIdsCalliopeCannotTake(t *testing.T) {
	in := payload(connection(building("bld-7", 100, nil), trafo("trafo_1", 100), 1, "lv"))
	data, _ := json.Marshal(in)
	_, err := NewTransformTask().Transform(data)
	if err == nil || !regexp.MustCompile(`rules\[\d+\]\.rules\[\d+\] \(forward\): from\.id: match: "nbld-7" does not match`).MatchString(err.Error()) {
		t.Errorf("error = %v, want the match guard naming the rule and the id", err)
	}
	// Ids that only start with a digit are repaired by the prefix.
	in = payload(connection(building("7", 100, nil), trafo("1", 100), 1, "lv"))
	data, _ = json.Marshal(in)
	job, err := NewTransformTask().Transform(data)
	if err != nil {
		t.Fatalf("numeric ids must be repaired: %v", err)
	}
	m := decodeModel(t, job)
	if _, ok := m.Nodes["n7"]; !ok || m.Transmission["lv_0"].To != "n1" {
		t.Errorf("repaired ids = %v %v", keysOf(m.Nodes), m.Transmission["lv_0"])
	}
}
