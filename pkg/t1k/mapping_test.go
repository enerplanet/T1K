package t1k

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/enerplanet/T1K/internal/jsondoc"
)

// The tests in this file pin the behaviour of the default mapping,
// config/enerplanet-to-meme.json, beyond the golden example: every
// technology kind, the edge cases of the payload and what the reverse does
// with jobs T1K did not produce.

// payload builds a calculation payload around the given topology entries.
func payload(topology ...map[string]any) map[string]any {
	return map[string]any{
		"user_id": "u", "model_id": "42_1", "session_id": "1", "country": "DE", "lkr": "x",
		"callback_url": "http://backend/cb/42",
		"start_date":   "2018-01-01T00:00:00.000Z", "end_date": "2018-01-02T00:00:00.000Z",
		"resolution": 60, "topology": topology,
	}
}

func building(id string, demand float64, techs map[string]any) map[string]any {
	return map[string]any{
		"type": "Feature", "id": id,
		"geometry": map[string]any{"type": "Point", "coordinates": []any{6.0, 52.0}},
		"properties": map[string]any{
			"id": id, "feature_type": "BasePOI", "type": "house", "f_class": "house", "osm_id": "osm-" + id,
			"area": 100.0, "demand_profile": "house", "demand_energy": demand, "demand_heat": 0,
			"f_class_demands": nil, "created_at": nil, "modified_at": nil, "session_id": "1",
		},
		"techs": techs, "custom_demand_timeseries": nil,
	}
}

func trafo(id string, ratedPower float64) map[string]any {
	return map[string]any{
		"type": "Feature", "id": id,
		"geometry": map[string]any{"type": "Point", "coordinates": []any{6.1, 52.1}},
		"properties": map[string]any{
			"id": id, "feature_type": "TopologyNode", "f_class": "transformer", "osm_id": "Trafo_" + id,
			"area": nil, "demand_energy": 0, "demand_heat": 0, "rated_power": ratedPower,
			"f_class_demands": nil, "created_at": nil, "modified_at": nil, "session_id": "1",
		},
		"techs": nil, "custom_demand_timeseries": nil,
	}
}

func connection(from, to map[string]any, length float64, pipe string) map[string]any {
	return map[string]any{"from": from, "to": to, "length": length, "pipe": pipe}
}

// transform runs the default mapping and decodes the result for inspection.
func transform(t *testing.T, doc any) map[string]any {
	t.Helper()
	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	out, err := NewTransformTask().Transform(data)
	if err != nil {
		t.Fatalf("Transform: %v", err)
	}
	return mustDecode(t, string(out)).(map[string]any)
}

func lookup(t *testing.T, doc map[string]any, path string) any {
	t.Helper()
	var cur any = doc
	for _, key := range strings.Split(path, ".") {
		obj, ok := cur.(map[string]any)
		if !ok {
			t.Fatalf("%s: %q is not an object", path, key)
		}
		if cur, ok = obj[key]; !ok {
			t.Fatalf("%s: key %q missing", path, key)
		}
	}
	return cur
}

func expectNumber(t *testing.T, doc map[string]any, path string, want float64) {
	t.Helper()
	if got, ok := jsondoc.Number(lookup(t, doc, path)); !ok || got != want {
		t.Errorf("%s = %v, want %v", path, lookup(t, doc, path), want)
	}
}

func expectAbsent(t *testing.T, doc map[string]any, path string) {
	t.Helper()
	var cur any = doc
	keys := strings.Split(path, ".")
	for i, key := range keys {
		obj, ok := cur.(map[string]any)
		if !ok {
			return
		}
		cur, ok = obj[key]
		if !ok {
			return
		}
		if i == len(keys)-1 {
			t.Errorf("%s should be absent, got %v", path, cur)
		}
	}
}

func TestMappingTechnologyKinds(t *testing.T) {
	techs := map[string]any{
		"pv_supply":             map[string]any{"cont_energy_cap_max": 8, "cont_energy_eff": 0.9, "cont_lifetime": 25, "cost_interest_rate": 0.02, "cost_energy_cap": 575, "cost_om_annual": 8, "tilt": 35, "azimuth": 180, "losses": 0.14, "cont_parasitic_eff": 1, "system_capacity": 8},
		"battery_storage":       map[string]any{"cont_energy_cap_max": 8715, "cont_storage_cap_max": 8889.3, "cont_storage_cap_min": 10, "cont_storage_loss": 0.01, "cont_storage_initial": 0.5, "cont_storage_discharge_depth": 0.1, "cont_energy_eff": 0.97, "cont_lifetime": 11, "cost_energy_cap": 1028, "cost_storage_cap": 1007.93, "cost_om_annual": 25.19, "cost_om_annual_investment_fraction": 0, "cost_interest_rate": 0.02},
		"wind_onshore":          map[string]any{"cont_energy_cap_max": 12000, "cont_energy_eff": 0.9, "cont_lifetime": 25, "turbine_id": 61, "nominal_power": 2000, "hub_height": 80, "rotor_diameter": 100, "cost_energy_cap": 1600, "cost_interest_rate": 0.05, "cost_om_annual": 35},
		"biomass_supply":        map[string]any{"cont_energy_cap_max": 5000, "cont_energy_eff": 0.25, "cont_lifetime": 20, "cost_energy_cap": 6500, "cost_interest_rate": 0.07, "cost_om_annual": 80, "cost_om_prod": 0.096, "feedstock_type": "forest"},
		"geothermal_supply":     map[string]any{"cont_energy_cap_max": 30000, "cont_energy_eff": 0.9, "cont_lifetime": 50, "cost_energy_cap": 5000, "cost_interest_rate": 0.08, "cost_om_annual": 100, "num_wells": 2},
		"water_supply":          map[string]any{"cont_energy_cap_max": 4700, "cost_om_annual": 52.5, "cost_om_prod": -0.0616},
		"households_supply":     map[string]any{"cont_energy_cap_max": "INF", "cont_resource_eff": 1, "cost_interest_rate": 0.02, "cost_om_con": 0.3943},
		"non_households_supply": map[string]any{"cont_energy_cap_max": 500, "cost_om_con": 0.2359},
	}
	out := transform(t, payload(connection(building("1", 3745, techs), trafo("trafo_9", 160), 0.05, "lv")))
	tech := func(name string) map[string]any {
		return lookup(t, out, "model.technologies."+name+"_1").(map[string]any)
	}

	pv := tech("pv_supply")
	if pv["role"] != "supply" || pv["node"] != "n1" || pv["carrier_out"] != "electricity" {
		t.Errorf("pv essentials = %v", pv)
	}
	expectNumber(t, out, "model.technologies.pv_supply_1.capacity.max", 0.008)
	expectNumber(t, out, "model.technologies.pv_supply_1.costs.monetary.investment_per_capacity", 575000)
	expectNumber(t, out, "model.technologies.pv_supply_1.costs.monetary.fixed_om", 8000)
	expectNumber(t, out, "model.technologies.pv_supply_1.performance.params.tilt", 35)
	expectNumber(t, out, "model.technologies.pv_supply_1.performance.params.loss", 0.14)
	expectNumber(t, out, "model.technologies.pv_supply_1.native.calliope.flow_out_parasitic_eff", 1)
	if lookup(t, out, "model.technologies.pv_supply_1.performance.model") != "pv" {
		t.Error("pv performance model")
	}
	expectAbsent(t, out, "model.technologies.pv_supply_1.performance.params.system_capacity")

	battery := tech("battery_storage")
	if battery["role"] != "storage" || battery["carrier_in"] != "electricity" {
		t.Errorf("battery essentials = %v", battery)
	}
	expectNumber(t, out, "model.technologies.battery_storage_1.storage.energy_capacity.max", 8.8893)
	expectNumber(t, out, "model.technologies.battery_storage_1.storage.energy_capacity.min", 0.01)
	expectNumber(t, out, "model.technologies.battery_storage_1.storage.charge_eff", 0.97)
	expectNumber(t, out, "model.technologies.battery_storage_1.storage.discharge_eff", 0.97)
	expectNumber(t, out, "model.technologies.battery_storage_1.storage.self_discharge", 0.01)
	expectNumber(t, out, "model.technologies.battery_storage_1.storage.initial_soc", 0.5)
	expectNumber(t, out, "model.technologies.battery_storage_1.storage.depth_of_discharge", 0.1)
	expectNumber(t, out, "model.technologies.battery_storage_1.costs.monetary.investment_per_energy_capacity", 1007930)
	expectAbsent(t, out, "model.technologies.battery_storage_1.costs.monetary.fixed_om_fraction")
	expectAbsent(t, out, "model.technologies.battery_storage_1.efficiency")

	wind := tech("wind_onshore")
	if lookup(t, out, "model.technologies.wind_onshore_1.performance.model") != "wind" {
		t.Error("wind performance model")
	}
	expectNumber(t, out, "model.technologies.wind_onshore_1.performance.params.hub_height", 80)
	expectNumber(t, out, "model.technologies.wind_onshore_1.performance.params.turbine_id", 61)
	expectNumber(t, out, "model.technologies.wind_onshore_1.capacity.max", 12)
	if wind["efficiency"] == nil {
		t.Error("wind efficiency")
	}

	for _, name := range []string{"biomass_supply", "geothermal_supply", "water_supply"} {
		if tech(name)["role"] != "supply" || tech(name)["node"] != "n1" {
			t.Errorf("%s essentials = %v", name, tech(name))
		}
		expectAbsent(t, out, "model.technologies."+name+"-1.performance")
	}
	expectNumber(t, out, "model.technologies.biomass_supply_1.costs.monetary.variable_om", 96)
	expectAbsent(t, out, "model.technologies.biomass_supply_1.feedstock_type")
	expectNumber(t, out, "model.technologies.water_supply_1.costs.monetary.variable_om", -61.6)
	expectAbsent(t, out, "model.technologies.water_supply_1.lifetime")

	expectNumber(t, out, "model.trade.households_1.import.price", 394.3)
	expectAbsent(t, out, "model.trade.households_1.import.limit")
	expectNumber(t, out, "model.trade.non_households_1.import.limit", 0.5)
	expectNumber(t, out, "model.trade.non_households_1.import.price", 235.9)
	if lookup(t, out, "model.trade.households_1.node") != "n1" || lookup(t, out, "model.trade.households_1.carrier") != "electricity" {
		t.Error("household trade essentials")
	}
	expectNumber(t, out, "model.trade.grid_trafo_9.import.limit", 0.16)
	expectNumber(t, out, "model.trade.grid_trafo_9.import.price", 300)
	if _, ok := lookup(t, out, "model.trade.grid_trafo_9.export").(map[string]any); !ok {
		t.Error("transformer export block")
	}
}

// withSeries gives a building hourly demand per carrier, in kW per step.
func withSeries(b map[string]any, series map[string]any) map[string]any {
	b["properties"].(map[string]any)["demand_series"] = series
	return b
}

func TestMappingDemandSeries(t *testing.T) {
	b := withSeries(building("1", 3745, nil), map[string]any{
		"electricity": []any{0.38, 0.41, 0.37},
		"heat":        []any{3.36, 3.86, 4.02},
		"cooling":     []any{0, 1.5, 1.2},
		"natural_gas": []any{0, 0, 0.21},
	})
	out := transform(t, payload(connection(b, trafo("trafo_9", 160), 0.05, "lv")))

	elec := lookup(t, out, "model.technologies.demand_1").(map[string]any)
	if elec["carrier_in"] != "electricity" || elec["demand_profile"] != "demand_1" || elec["node"] != "n1" {
		t.Errorf("electricity demand = %v", elec)
	}
	heat := lookup(t, out, "model.technologies.demand_heat_1").(map[string]any)
	if heat["role"] != "demand" || heat["carrier_in"] != "heat" || heat["demand_profile"] != "demand_heat_1" || heat["node"] != "n1" {
		t.Errorf("heat demand = %v", heat)
	}
	for carrier, name := range map[string]string{"cooling": "demand_cooling_1", "natural_gas": "demand_natural_gas_1"} {
		tech := lookup(t, out, "model.technologies."+name).(map[string]any)
		if tech["role"] != "demand" || tech["carrier_in"] != carrier || tech["demand_profile"] != name || tech["node"] != "n1" {
			t.Errorf("%s demand = %v", carrier, tech)
		}
	}
	for name, want := range map[string][]float64{
		"demand_1": {0.00038, 0.00041, 0.00037}, "demand_heat_1": {0.00336, 0.00386, 0.00402},
		"demand_cooling_1": {0, 0.0015, 0.0012}, "demand_natural_gas_1": {0, 0, 0.00021},
	} {
		ts := lookup(t, out, "model.timeseries."+name).(map[string]any)
		if ts["source"] != "inline" || ts["unit"] != "MW" {
			t.Errorf("%s = %v", name, ts)
		}
		values := ts["values"].([]any)
		if len(values) != len(want) {
			t.Fatalf("%s has %d values, want %d", name, len(values), len(want))
		}
		for i, v := range values {
			if got, _ := jsondoc.Number(v); got != want[i] {
				t.Errorf("%s[%d] = %v, want %v (kW to MW)", name, i, v, want[i])
			}
		}
	}
	for _, carrier := range []string{"heat", "cooling", "natural_gas"} {
		if lookup(t, out, "model.carriers."+carrier+".unit") != "MWh" {
			t.Errorf("a %s demand needs the %s carrier", carrier, carrier)
		}
	}

	// A series replaces the constant load even when the yearly number is 0.
	zero := withSeries(building("2", 0, nil), map[string]any{"electricity": []any{0.5}})
	out = transform(t, payload(connection(zero, trafo("trafo_9", 160), 0.05, "lv")))
	if lookup(t, out, "model.technologies.demand_2.demand_profile") != "demand_2" {
		t.Error("a series must not be gated by demand_energy")
	}
	for _, carrier := range []string{"heat", "cooling", "natural_gas"} {
		expectAbsent(t, out, "model.carriers."+carrier)
	}

	// Carriers without a rule are not mapped.
	steam := withSeries(building("3", 1000, nil), map[string]any{"steam": []any{1.0}})
	out = transform(t, payload(connection(steam, trafo("trafo_9", 160), 0.05, "lv")))
	expectNumber(t, out, "model.technologies.demand_3.demand_profile", 0.000114155)
	expectAbsent(t, out, "model.timeseries")
}

func TestMappingHeatTechnologies(t *testing.T) {
	techs := map[string]any{
		"air_source_heat_pump":    map[string]any{"cont_energy_cap_max": "INF", "cont_energy_eff": 3.33, "cont_lifetime": 18, "cost_energy_cap": 900, "cost_interest_rate": 0.1},
		"ground_source_heat_pump": map[string]any{"cont_energy_eff": 4.55, "cont_lifetime": 25, "cost_energy_cap": 1800, "cost_interest_rate": 0.1},
		"electric_boilers":        map[string]any{"cont_energy_eff": 0.99, "cont_lifetime": 20, "cost_energy_cap": 150, "cost_interest_rate": 0.1},
		"gas_boiler":              map[string]any{"cont_energy_eff": 0.9, "cont_lifetime": 18, "cost_energy_cap": 100, "cost_interest_rate": 0.1},
		"air_conditioner":         map[string]any{"cont_energy_eff": 4.6, "cont_lifetime": 12, "cost_energy_cap": 450, "cost_interest_rate": 0.1},
		"gas_supply":              map[string]any{"cont_energy_cap_max": "INF", "cost_om_con": 0.04},
	}
	out := transform(t, payload(connection(building("1", 3745, techs), trafo("trafo_9", 160), 0.05, "lv")))

	for name, eff := range map[string]float64{"air_source_heat_pump": 3.33, "ground_source_heat_pump": 4.55, "electric_boilers": 0.99} {
		tech := lookup(t, out, "model.technologies."+name+"_1").(map[string]any)
		if tech["role"] != "conversion" || tech["node"] != "n1" || tech["carrier_in"] != "electricity" || tech["carrier_out"] != "heat" {
			t.Errorf("%s essentials = %v", name, tech)
		}
		expectNumber(t, out, "model.technologies."+name+"_1.efficiency", eff)
	}
	expectNumber(t, out, "model.technologies.air_source_heat_pump_1.costs.monetary.investment_per_capacity", 900000)
	expectNumber(t, out, "model.technologies.air_source_heat_pump_1.lifetime", 18)
	expectAbsent(t, out, "model.technologies.air_source_heat_pump_1.capacity.max")

	boiler := lookup(t, out, "model.technologies.gas_boiler_1").(map[string]any)
	if boiler["role"] != "conversion" || boiler["carrier_in"] != "natural_gas" || boiler["carrier_out"] != "heat" || boiler["node"] != "n1" {
		t.Errorf("gas boiler essentials = %v", boiler)
	}
	expectNumber(t, out, "model.technologies.gas_boiler_1.efficiency", 0.9)

	ac := lookup(t, out, "model.technologies.air_conditioner_1").(map[string]any)
	if ac["role"] != "conversion" || ac["carrier_in"] != "electricity" || ac["carrier_out"] != "cooling" || ac["node"] != "n1" {
		t.Errorf("air conditioner essentials = %v", ac)
	}
	expectNumber(t, out, "model.technologies.air_conditioner_1.efficiency", 4.6)
	expectNumber(t, out, "model.technologies.air_conditioner_1.costs.monetary.investment_per_capacity", 450000)

	gas := lookup(t, out, "model.trade.gas_1").(map[string]any)
	if gas["node"] != "n1" || gas["carrier"] != "natural_gas" {
		t.Errorf("gas supply = %v", gas)
	}
	expectNumber(t, out, "model.trade.gas_1.import.price", 40)
	expectAbsent(t, out, "model.trade.gas_1.import.limit")

	for _, carrier := range []string{"heat", "cooling", "natural_gas"} {
		if lookup(t, out, "model.carriers."+carrier+".unit") != "MWh" {
			t.Errorf("the technologies need the %s carrier", carrier)
		}
	}
}

func TestMappingPVArrays(t *testing.T) {
	array := func(surface string, tilt, azimuth, capMax float64, profile ...any) map[string]any {
		return map[string]any{
			"surface_id": surface, "tilt": tilt, "azimuth": azimuth, "losses": 14,
			"cont_energy_cap_max": capMax, "cont_lifetime": 25, "cost_energy_cap": 1100, "cost_interest_rate": 0.1,
			"profile": profile,
		}
	}
	techs := map[string]any{"pv_arrays": []any{
		array("roof-a", 35, 180, 12.4, 0, 0.42, 0.61),
		array("roof-b", 35, 0, 8.0, 0, 0.11, 0.2),
	}}
	out := transform(t, payload(connection(building("1", 3745, techs), trafo("trafo_9", 160), 0.05, "lv")))

	for j, want := range map[string]struct {
		azimuth, capMax float64
		profile         []float64
	}{"0": {180, 0.0124, []float64{0, 0.42, 0.61}}, "1": {0, 0.008, []float64{0, 0.11, 0.2}}} {
		name := "pv_supply_1_" + j
		tech := lookup(t, out, "model.technologies."+name).(map[string]any)
		if tech["role"] != "supply" || tech["node"] != "n1" || tech["carrier_out"] != "electricity" {
			t.Errorf("%s essentials = %v", name, tech)
		}
		if lookup(t, out, "model.technologies."+name+".performance.precomputed") != "pv_1_"+j {
			t.Errorf("%s must name its own series", name)
		}
		expectNumber(t, out, "model.technologies."+name+".performance.params.azimuth", want.azimuth)
		expectNumber(t, out, "model.technologies."+name+".capacity.max", want.capMax)
		expectNumber(t, out, "model.technologies."+name+".costs.monetary.investment_per_capacity", 1100000)
		expectAbsent(t, out, "model.technologies."+name+".surface_id")

		ts := lookup(t, out, "model.timeseries.pv_1_"+j).(map[string]any)
		if ts["source"] != "inline" || ts["unit"] != "fraction" {
			t.Errorf("pv_1_%s = %v", j, ts)
		}
		values := ts["values"].([]any)
		if len(values) != len(want.profile) {
			t.Fatalf("pv_1_%s has %d values, want %d", j, len(values), len(want.profile))
		}
		for i, v := range values {
			if got, _ := jsondoc.Number(v); got != want.profile[i] {
				t.Errorf("pv_1_%s[%d] = %v, want %v", j, i, v, want.profile[i])
			}
		}
	}
	expectAbsent(t, out, "model.technologies.pv_supply_1")

	back := reverseOf(t, out)
	conn := lookup(t, back, "topology").([]any)[0].(map[string]any)
	arrays := conn["from"].(map[string]any)["techs"].(map[string]any)["pv_arrays"].([]any)
	if len(arrays) != 2 {
		t.Fatalf("reverse restored %d arrays, want 2", len(arrays))
	}
	first := arrays[0].(map[string]any)
	if got, _ := jsondoc.Number(first["azimuth"]); got != 180 {
		t.Errorf("reverse array 0 azimuth = %v", first["azimuth"])
	}
	if got := first["profile"].([]any); len(got) != 3 {
		t.Errorf("reverse array 0 profile = %v", got)
	}
}

func TestMappingOperateMode(t *testing.T) {
	b := building("1", 3745, map[string]any{"pv_supply": map[string]any{"cont_energy_cap_min": 8, "cont_energy_cap_max": 8}})
	in := payload(connection(b, trafo("trafo_9", 160), 0.05, "lv"), connection(trafo("trafo_9", 160), trafo("trafo_10", 250), 1.2, "mv"))
	if got := lookup(t, transform(t, in), "experiment.mode"); got != "plan" {
		t.Errorf("mode without a payload mode = %v, want plan", got)
	}

	in["mode"] = "operate"
	out := transform(t, in)
	if got := lookup(t, out, "experiment.mode"); got != "operate" {
		t.Errorf("experiment.mode = %v, want operate", got)
	}
	expectNumber(t, out, "model.technologies.pv_supply_1.capacity.min", 0.008)
	expectNumber(t, out, "model.technologies.pv_supply_1.capacity.max", 0.008)
	if got := lookup(t, reverseOf(t, out), "mode"); got != "operate" {
		t.Errorf("reverse mode = %v, want operate", got)
	}
}

func TestMappingInfinityIsCaseInsensitiveAndNormalised(t *testing.T) {
	techs := map[string]any{"pv_supply": map[string]any{"cont_energy_cap_max": "Inf", "cont_energy_cap_max_systemwide": "INF", "cost_om_annual_investment_fraction": 0.02, "cost_om_annual": 8}}
	in := payload(connection(building("1", 100, techs), trafo("t", 100), 1, "lv"))
	out := transform(t, in)
	expectAbsent(t, out, "model.technologies.pv_supply_1.capacity.max")
	expectAbsent(t, out, "model.technologies.pv_supply_1.capacity.systemwide_max")
	// A non-zero fraction is carried even next to fixed_om; MEME, not T1K,
	// rejects the combination.
	expectNumber(t, out, "model.technologies.pv_supply_1.costs.monetary.fixed_om_fraction", 0.02)
	expectNumber(t, out, "model.technologies.pv_supply_1.costs.monetary.fixed_om", 8000)

	back := reverseOf(t, out)
	pv := lookup(t, back, "topology").([]any)[0].(map[string]any)["from"].(map[string]any)["techs"].(map[string]any)["pv_supply"].(map[string]any)
	if pv["cont_energy_cap_max"] != "inf" || pv["cont_energy_cap_max_systemwide"] != "inf" {
		t.Errorf("infinity placeholders are restored in lower case: %v", pv)
	}
	if got, _ := jsondoc.Number(pv["cost_om_annual_investment_fraction"]); got != 0.02 {
		t.Errorf("fraction restored = %v", pv["cost_om_annual_investment_fraction"])
	}
}

func reverseOf(t *testing.T, job map[string]any) map[string]any {
	t.Helper()
	data, err := json.Marshal(job)
	if err != nil {
		t.Fatal(err)
	}
	back, err := NewTransformTask().Reverse(data)
	if err != nil {
		t.Fatalf("Reverse: %v", err)
	}
	return mustDecode(t, string(back)).(map[string]any)
}

func TestMappingPayloadEdgeCases(t *testing.T) {
	t.Run("no topology", func(t *testing.T) {
		in := payload()
		delete(in, "topology")
		out := transform(t, in)
		for _, key := range []string{"nodes", "technologies", "transmission", "trade"} {
			expectAbsent(t, out, "model."+key)
		}
		if lookup(t, out, "model.metadata.name") != "42_1" || lookup(t, out, "experiment.mode") != "plan" {
			t.Error("metadata and experiment must not depend on the topology")
		}
		back := reverseOf(t, out)
		if _, ok := back["topology"]; ok {
			t.Error("the reverse of a job without nodes has no topology")
		}
	})
	t.Run("standalone building only", func(t *testing.T) {
		out := transform(t, payload(map[string]any{"from": building("7", 500, nil)}))
		expectAbsent(t, out, "model.transmission")
		expectAbsent(t, out, "model.trade")
		if _, ok := lookup(t, out, "model.nodes.n7").(map[string]any); !ok {
			t.Error("standalone building node")
		}
		expectNumber(t, out, "model.technologies.demand_7.demand_profile", 0.000057078)
		if _, ok := reverseOf(t, out)["topology"]; ok {
			t.Error("a standalone building cannot be placed back into the topology")
		}
	})
	t.Run("zero demand and zero rating", func(t *testing.T) {
		out := transform(t, payload(connection(building("1", 0, nil), trafo("t0", 0), 0.1, "lv")))
		expectAbsent(t, out, "model.technologies")
		expectAbsent(t, out, "model.trade")
		back := reverseOf(t, out)
		conn := lookup(t, back, "topology").([]any)[0].(map[string]any)
		if got, _ := jsondoc.Number(conn["from"].(map[string]any)["properties"].(map[string]any)["demand_energy"]); got != 0 {
			t.Error("demand_energy is restored as 0")
		}
		// Without a grid trade the reverse cannot tell a transformer apart
		// from a building: a known limitation, pinned here.
		if conn["to"].(map[string]any)["properties"].(map[string]any)["feature_type"] != "BasePOI" {
			t.Error("a transformer without a rating reverses as a building")
		}
	})
	t.Run("resolutions", func(t *testing.T) {
		for minutes, want := range map[float64]string{15: "15min", 30: "30min", 60: "1H"} {
			in := payload()
			in["resolution"] = minutes
			if got := lookup(t, transform(t, in), "model.time.resolution"); got != want {
				t.Errorf("resolution %v = %v, want %v", minutes, got, want)
			}
		}
	})
	t.Run("empty techs and null geometry parts", func(t *testing.T) {
		b := building("1", 100, map[string]any{})
		out := transform(t, payload(connection(b, trafo("t", 100), 1, "lv")))
		for name := range out["model"].(map[string]any)["technologies"].(map[string]any) {
			if !strings.HasPrefix(name, "demand_") {
				t.Errorf("unexpected technology %s from empty techs", name)
			}
		}
	})
}

func TestMappingRejectsUnconvertibleValues(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(in map[string]any)
		want   string
	}{
		{"unknown resolution", func(in map[string]any) { in["resolution"] = 45 }, "rules[4] (forward): resolution: lookup: no pair for number 45"},
		{"date in another format", func(in map[string]any) { in["start_date"] = "01.01.2018" }, "rules[2] (forward): start_date: datetime:"},
		{"non-numeric length", func(in map[string]any) {
			in["topology"].([]any)[0].(map[string]any)["length"] = "far"
		}, "(forward): length: number:"},
		{"non-numeric capacity", func(in map[string]any) {
			in["topology"].([]any)[0].(map[string]any)["from"].(map[string]any)["techs"] = map[string]any{"pv_supply": map[string]any{"cont_energy_cap_max": "lots"}}
		}, "cont_energy_cap_max: linear:"},
		{"unknown mode", func(in map[string]any) { in["mode"] = "dispatch" }, "mode: lookup: no pair"},
		{"feature without an id", func(in map[string]any) {
			delete(in["topology"].([]any)[0].(map[string]any)["from"].(map[string]any), "id")
		}, `bind $k: "id" not found`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			in := payload(connection(building("1", 100, nil), trafo("t", 100), 1, "lv"))
			in["topology"] = []any{in["topology"].([]map[string]any)[0]}
			tc.mutate(in)
			data, _ := json.Marshal(in)
			_, err := NewTransformTask().Transform(data)
			if err == nil || !errors.Is(err, ErrRule) || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want ErrRule mentioning %q", err, tc.want)
			}
		})
	}
}

func TestMappingReverseOfForeignJobs(t *testing.T) {
	job := func(transmission map[string]any) map[string]any {
		return map[string]any{
			"model": map[string]any{
				"metadata": map[string]any{"name": "m"}, "time": map[string]any{"start": "2018-01-01T00:00:00Z", "end": "2018-01-02T00:00:00Z", "resolution": "1H"},
				"carriers":     map[string]any{"electricity": map[string]any{}},
				"nodes":        map[string]any{"na": map[string]any{"name": "A", "coords": map[string]any{"lat": 1, "lon": 2}}},
				"transmission": transmission,
			},
			"experiment": map[string]any{"mode": "plan", "solver": map[string]any{"name": "highs"}},
		}
	}
	t.Run("keys that do not fit the template are skipped", func(t *testing.T) {
		back := reverseOf(t, job(map[string]any{"line-a": map[string]any{"from": "na", "to": "nb"}, "lv_0": map[string]any{"from": "na", "to": "nb", "distance": 1}}))
		topo := lookup(t, back, "topology").([]any)
		if len(topo) != 1 {
			t.Fatalf("topology = %v", topo)
		}
		conn := topo[0].(map[string]any)
		if conn["pipe"] != "lv" || conn["from"].(map[string]any)["properties"].(map[string]any)["osm_id"] != "A" {
			t.Errorf("connection = %v", conn)
		}
		// The node "nb" is referenced but not defined: the element keeps only
		// the id the transmission gave it, with the prefix stripped.
		if to := conn["to"].(map[string]any); to["id"] != "b" || to["properties"] != nil {
			t.Errorf("undefined node reverses to its id alone: %v", to)
		}
	})
	t.Run("a key with a non-numeric position is an error", func(t *testing.T) {
		data, _ := json.Marshal(job(map[string]any{"lv_x": map[string]any{"from": "na", "to": "nb"}}))
		_, err := NewTransformTask().Reverse(data)
		if err == nil || !errors.Is(err, ErrRule) || !strings.Contains(err.Error(), "not an array index") {
			t.Errorf("error = %v", err)
		}
	})
	t.Run("a node reference without the prefix is an error", func(t *testing.T) {
		// A job T1K did not produce may reference nodes without the prefix;
		// such an id cannot be mapped back to EnerPlanET, and the reverse
		// says so instead of guessing.
		data, _ := json.Marshal(job(map[string]any{"lv_0": map[string]any{"from": "a", "to": "nb"}}))
		_, err := NewTransformTask().Reverse(data)
		if err == nil || !errors.Is(err, ErrRule) || !strings.Contains(err.Error(), `from: affix: "a" does not carry the prefix "n"`) {
			t.Errorf("error = %v", err)
		}
	})
	t.Run("placeholders and defaults", func(t *testing.T) {
		back := reverseOf(t, job(map[string]any{}))
		for _, key := range []string{"user_id", "session_id", "callback_url", "country", "lkr"} {
			if back[key] != "" {
				t.Errorf("%s = %v, want the empty placeholder", key, back[key])
			}
		}
		if got, _ := jsondoc.Number(back["resolution"]); got != 60 || back["start_date"] != "2018-01-01T00:00:00.000Z" {
			t.Errorf("time fields = %v %v", back["resolution"], back["start_date"])
		}
		if back["pypsa"].(map[string]any)["line_type_lv"] != "NAYY 4x150 SE" {
			t.Error("pypsa defaults")
		}
	})
}

// TestMappingRoundTripAllKinds is the round-trip property on a payload with
// every technology kind and both pipe kinds, where no standalone building
// limits the reverse: converting the reverse output forward again reproduces
// the job exactly.
func TestMappingRoundTripAllKinds(t *testing.T) {
	techs := map[string]any{
		"pv_supply":             map[string]any{"cont_energy_cap_max": 8, "cont_lifetime": 25, "cost_interest_rate": 0.02, "cost_energy_cap": 575, "tilt": 35, "azimuth": 180},
		"battery_storage":       map[string]any{"cont_energy_cap_max": 8715, "cont_storage_cap_max": 8889.3, "cont_energy_eff": 0.97, "cont_lifetime": 11, "cost_interest_rate": 0.02},
		"wind_onshore":          map[string]any{"cont_energy_cap_max": 12000, "hub_height": 80},
		"biomass_supply":        map[string]any{"cont_energy_cap_max": 5000, "cost_om_prod": 0.096},
		"geothermal_supply":     map[string]any{"cont_energy_cap_max": 30000},
		"water_supply":          map[string]any{"cont_energy_cap_max": 4700},
		"households_supply":     map[string]any{"cont_energy_cap_max": "inf", "cost_om_con": 0.3943},
		"non_households_supply": map[string]any{"cont_energy_cap_max": 500, "cost_om_con": 0.2359},
	}
	t1, t2 := trafo("trafo_1", 160), trafo("trafo_2", 250)
	in := payload(
		connection(building("1", 3745, techs), t1, 0.05, "lv"),
		connection(building("2", 128000, nil), t1, 0.06, "lv"),
		connection(t1, t2, 0.4, "mv"),
		connection(building("3", 21000, nil), t2, 0.02, "lv"),
		connection(withSeries(building("4", 4200, map[string]any{
			"air_source_heat_pump": map[string]any{"cont_energy_eff": 3.33, "cont_lifetime": 18, "cost_energy_cap": 900},
			"gas_boiler":           map[string]any{"cont_energy_eff": 0.9},
			"air_conditioner":      map[string]any{"cont_energy_eff": 4.6, "cost_energy_cap": 450},
			"gas_supply":           map[string]any{"cont_energy_cap_max": 12, "cost_om_con": 0.04},
		}), map[string]any{
			"electricity": []any{0.381234, 0.4, 0}, "heat": []any{3.36, 3.86, 4.02},
			"cooling": []any{0, 1.5, 0}, "natural_gas": []any{0, 0, 0.21},
		}), t2, 0.03, "lv"),
	)
	data, _ := json.Marshal(in)
	task := NewTransformTask()
	job, err := task.Transform(data)
	if err != nil {
		t.Fatal(err)
	}
	back, err := task.Reverse(job)
	if err != nil {
		t.Fatal(err)
	}
	again, err := task.Transform(back)
	if err != nil {
		t.Fatal(err)
	}
	if string(job) != string(again) {
		t.Errorf("round trip changed the job:\n%s\n---\n%s", job, again)
	}
	back2, err := task.Reverse(again)
	if err != nil || string(back) != string(back2) {
		t.Errorf("reverse is not stable: %v", err)
	}
}

func TestTransformOfDegenerateInputs(t *testing.T) {
	task := NewTransformTask()
	for _, in := range []string{`null`, `[]`, `{}`, `"text"`, `5`} {
		out, err := task.Transform([]byte(in))
		if err != nil {
			t.Errorf("Transform(%s): %v", in, err)
			continue
		}
		doc := mustDecode(t, string(out)).(map[string]any)
		if lookup(t, doc, "experiment.mode") != "plan" || lookup(t, doc, "model.carriers.electricity.unit") != "MWh" {
			t.Errorf("Transform(%s) lacks the constants: %s", in, out)
		}
		if _, err := task.Reverse([]byte(in)); err != nil {
			t.Errorf("Reverse(%s): %v", in, err)
		}
	}
	if _, err := task.Transform([]byte("")); !errors.Is(err, ErrInput) {
		t.Errorf("empty input = %v, want ErrInput", err)
	}
}

func TestDefaultConfigMetadata(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Name != "enerplanet-to-meme" || cfg.Version == "" || cfg.Description == "" || cfg.Rules() == 0 {
		t.Errorf("default config metadata = %q %q %d", cfg.Name, cfg.Version, cfg.Rules())
	}
}

// FuzzTransform checks that no input can panic the default mapping in either
// direction and that failures are classified.
func FuzzTransform(f *testing.F) {
	f.Add(readFile(&testing.T{}, examplePayload))
	for _, seed := range []string{`null`, `{"topology": [{"from": {"id": "a"}, "to": {"id": "b"}, "pipe": "lv", "length": 1}]}`, `{"model": {"transmission": {"lv_0": {"from": "a", "to": "b"}}, "nodes": {"a": {}}}}`, `{"resolution": 45}`, `[`} {
		f.Add([]byte(seed))
	}
	task := NewTransformTask()
	f.Fuzz(func(t *testing.T, data []byte) {
		for _, run := range []func([]byte) ([]byte, error){task.Transform, task.Reverse} {
			out, err := run(data)
			if err != nil {
				if !errors.Is(err, ErrInput) && !errors.Is(err, ErrRule) {
					t.Fatalf("unclassified error: %v", err)
				}
				continue
			}
			if !json.Valid(out) {
				t.Fatalf("output is not JSON: %s", out)
			}
		}
	})
}

func BenchmarkTransform(b *testing.B) {
	data := readFile(&testing.T{}, examplePayload)
	task := NewTransformTask()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := task.Transform(data); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkReverse(b *testing.B) {
	data := readFile(&testing.T{}, examplePayload)
	task := NewTransformTask()
	job, err := task.Transform(data)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := task.Reverse(job); err != nil {
			b.Fatal(err)
		}
	}
}
