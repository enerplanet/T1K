# EnerPlanET to MEME

`config/enerplanet-to-meme.json`, the mapping embedded in the package,
converts the calculation payload EnerPlanET's backend sends to its simulation
webservice into a MEME job, the body of MEME's `POST /simulate`. This page
records what it maps, the decisions behind it and what the reverse direction
restores.

## The two documents

**Source:** the `CalculationPayload` built by
`enerplanet/backend/internal/payload/payload.go`. Its `topology` is an array
of connections; each connection has a `from` feature, and, unless the
building stands alone, a `to` feature, a `length` in km and a `pipe` kind
(`lv` building to transformer, `mv` transformer to transformer). A feature is
a GeoJSON point with `properties` (identity, usage class, annual demands) and
a `techs` object keyed by the technology catalogue (`pv_supply`,
`battery_storage`, ...) whose entries hold Calliope 0.6 constraint names in
kW, kWh and EUR/kW. The simulation engine's
`servicehub/schemas/calliope.json` describes the shape.

**Target:** MEME's canonical model (`internal/model` in the MEME repository):
`nodes`, `technologies`, `transmission` and `trade` keyed by identifiers,
quantities in MW, MWh and EUR/MW, plus an `experiment`. MEME decodes the body
with unknown fields disallowed, so the job may contain nothing MEME does not
define.

## Decisions

- **A node per feature, an arc per connection.** Every building and
  transformer becomes a node keyed by its feature `id` with the prefix `n`
  (`n1`, `ntrafo_2585`); every connection becomes a bidirectional
  transmission arc keyed by pipe kind and position (`lv_0`, `mv_2`) with the
  length as `distance`. The topology is therefore preserved exactly and the
  reverse can rebuild it. (The translator on EnerPlanET's
  `feature/heat-integration` branch instead sums buildings into one node per
  transformer area; that aggregation cannot be inverted, so T1K keeps the
  features.)
- **Identifiers that Calliope accepts.** Calliope 0.7 requires identifiers to
  match `^[^_^\d][\w]*$`: no leading digit or underscore, and only letters,
  digits and `_`. EnerPlanET numbers its buildings, so node ids get the
  prefix `n`, and every generated key joins its parts with `_`
  (`demand_1`, `pv_supply_1`, `grid_trafo_2585`, `households_1`). The
  node references copied into transmission arcs get the same prefix through
  the `affix` converter and are checked by a `match` guard, so an id that
  cannot be made valid fails the transformation rather than MEME's Calliope
  target. The reverse strips the prefix, so EnerPlanET's ids come back
  unchanged. This answers [issue #1](https://github.com/enerplanet/T1K/issues/1).
- **The transformer is the grid connection.** As in the simulation engine,
  which attaches a `transformer_supply` technology capped by the rated power,
  each transformer gets a `trade` entry importing at most its rating (kVA as
  MW) at 300 EUR/MWh, the tariff of that template technology, with unpaid
  export. `households_supply` and `non_households_supply` attached to a
  building become a trade at the building with their own tariff.
- **kW to MW, divided by 1000**, for capacities, storage sizes and the
  transformer rating; EUR/kW, EUR/kWh and EUR/kW/a are multiplied by 1000.
- **Annual demand as average power.** `demand_energy` (kWh/a) becomes a
  constant `demand_profile` in MW (divided by 8760 h and 1000), rounded to
  nine decimals; the reverse rounds back to whole kWh. A feature that
  carries `properties.demand_series.electricity` gets that series instead
  (see [Demand series](#demand-series)).
- **Run settings from the Calliope template:** mode `plan`, objective
  `min_cost`, solver `highs`, unmet demand allowed at a penalty of 1e6
  EUR/MWh (`ensure_feasibility` with `bigM: 1e6`).
- **Heat, cooling and gas as series only.** `properties.demand_series` carries
  a building's demand per step for `electricity`, `heat`, `cooling` and
  `natural_gas` (gas used outside its technologies, such as for cooking).
  The yearly `demand_heat` is carried by no rule; the reverse writes 0.
- **Gas without a network.** `techs.gas_supply` becomes a gas import at the
  building, priced like the electricity tariffs; a gas boiler converts that
  gas to heat. No gas network is modelled.
- **A carrier appears only when used.** `heat`, `cooling` and `natural_gas`
  are added to `model.carriers` by the rules that use them, so a payload
  without them yields the same job as before.

## Identifiers

MEME accepts letters, digits, `.`, `_` and `-` in identifiers, but its
Calliope target applies Calliope's stricter rule, `^[^_^\d][\w]*$`. The
mapping therefore generates every key and reference as follows:

| EnerPlanET | MEME key | MEME references |
|---|---|---|
| building id `1` | node `n1` | `node: "n1"` in its technologies and trades; `from`/`to: "n1"` in arcs |
| transformer id `trafo_2585` | node `ntrafo_2585` | `node: "ntrafo_2585"` in its trade; `from`/`to` in arcs |
| demand of building `1` | technology `demand_1` | |
| `techs.pv_supply` of building `1` | technology `pv_supply_1` | |
| `techs.households_supply` of building `1` | trade `households_1` | |
| transformer `trafo_2585` rating | trade `grid_trafo_2585` | |
| connection at position 0 with pipe `lv` | arc `lv_0` | |

The prefix comes from the key template `model.nodes{n$k}` and the templates
`n$k` of the `node` references; the references copied from the topology
(`from.id`, `to.id`) get it through the `affix` converter followed by a
`match` guard. An id that is still invalid after the prefix, for example one
containing a hyphen, fails the transformation with the rule named. The
reverse strips the prefix, so EnerPlanET's ids return unchanged; a job that
references nodes without the prefix is rejected by the reverse rather than
guessed at.

## Field mapping

`kW→MW` means `linear` with divisor 1000, `×1000` the factor 1000, and
`inf→absent` the `absent` converter for `"inf"` with `reverse_default: "inf"`.

### Payload root

| EnerPlanET | MEME | Conversion |
|---|---|---|
| `model_id` | `model.metadata.name` | copy |
| `start_date`, `end_date` | `model.time.start`, `model.time.end` | `datetime` from `2006-01-02T15:04:05.000Z` to RFC 3339 |
| `resolution` (minutes) | `model.time.resolution` | `lookup`: 15 `15min`, 30 `30min`, 60 `1H` |
| | `model.metadata.description` | constant |
| | `model.carriers.electricity` | constant `{"name": "Electricity", "unit": "MWh"}` |
| | `experiment.mode`, `.objective`, `.solver.name`, `.allow_unmet_demand` | constants `plan`, `min_cost`, `highs`, `{enabled, penalty_price 1e6}` |
| `user_id`, `session_id`, `callback_url`, `country`, `lkr` | none | reverse writes `""` |
| `pypsa` | none | reverse writes EnerPlanET's default line and transformer types |
| `custom_demand_time_series`, `parameters` | none | not carried |

### Connections

Each `topology[$i]` with a `to` feature becomes `model.transmission{${p}_$i}`
with `$p` bound to `pipe`:

| EnerPlanET | MEME | Conversion |
|---|---|---|
| `pipe` | the key prefix (`lv_`, `mv_`) | bind |
| `from.id`, `to.id` | `from`, `to` | `affix` prefix `n`, then `match` against Calliope's identifier syntax |
| `length` (km) | `distance` | `number` |
| | `carrier`, `bidirectional`, `capacity` | constants `electricity`, `true`, `{"expandable": true}` |

### Features

Each `topology[$i].from` and `topology[$i].to` becomes `model.nodes{n$k}`
with `$k` bound to the feature `id`; a transformer referenced by several
connections is written once.

| EnerPlanET | MEME | Conversion |
|---|---|---|
| `id` | the node key | bind |
| `geometry.coordinates[0]`, `[1]` | `coords.lon`, `coords.lat` | `number` |
| `properties.osm_id` | `name` | copy |
| `properties.demand_energy` (kWh/a, when > 0 and no electricity series) | `technologies{demand_$k}` with `role` `demand`, `node` `n$k`, `carrier_in` `electricity`, `demand_profile` in MW | ÷ 8 760 000, rounded |
| `properties.demand_series.<carrier>` | see [Demand series](#demand-series) | |
| `properties.rated_power` (kVA, transformers) | `trade{grid_$k}` with `node` `n$k`, `import.limit` in MW, `import.price` 300, `export` | kW→MW |
| `type`, `geometry.type`, `techs`, `custom_demand_timeseries`, `properties.id`, `session_id`, `created_at`, `modified_at`, `f_class_demands`, `demand_heat` | none | reverse constants: `Feature`, `Point`, `null`, `null`, the key, `""`, `null`, `null`, `null`, `0` |
| `properties.feature_type`, `type`, `f_class`, `demand_profile`, `area` | none | reverse: `BasePOI`, `residential`, `residential`, `residential`, `0` for a node without a `grid_*` trade; `TopologyNode`, `transformer`, `null` area for one with |

`residential` is EnerPlanET's own fallback when a building has no usage
class.

### Demand series

`properties.demand_series` maps a carrier to the building's demand per step,
in kW, one value per model timestep. Each carrier becomes a named series and
a demand technology that follows it:

| EnerPlanET | Series | Technology |
|---|---|---|
| `demand_series.electricity` | `timeseries{demand_$k}` | `technologies{demand_$k}`, `carrier_in` `electricity` |
| `demand_series.heat` | `timeseries{demand_heat_$k}` | `technologies{demand_heat_$k}`, `carrier_in` `heat` |
| `demand_series.cooling` | `timeseries{demand_cooling_$k}` | `technologies{demand_cooling_$k}`, `carrier_in` `cooling` |
| `demand_series.natural_gas` | `timeseries{demand_natural_gas_$k}` | `technologies{demand_natural_gas_$k}`, `carrier_in` `natural_gas` |

A series is `{"source": "inline", "unit": "MW", "values": [...]}`, the values
divided by 1000 and rounded to nine decimals; the reverse rounds back to six
decimals in kW. The technology has `role` `demand`, `node` `n$k` and a
`demand_profile` naming the series. An electricity series replaces the
constant load of `demand_energy`, and the electricity technology keeps its
key `demand_$k`. A key other than these four carriers is not carried.

### Technologies

`techs.<key>` of a feature becomes `technologies{<key>_$k}` (or
`trade{..._$k}` for the grid tariffs), with `node` set to `n$k`, the
feature's node key.
The shared rules, applied to every supply and conversion technology and the battery:

| EnerPlanET (kW, kWh, EUR) | MEME (MW, MWh, EUR) | Conversion |
|---|---|---|
| | `capacity.expandable` | constant `true` |
| `cont_energy_cap_max` | `capacity.max` | inf→absent, kW→MW |
| `cont_energy_cap_min` | `capacity.min` | kW→MW |
| `cont_energy_cap_max_systemwide` | `capacity.systemwide_max` | inf→absent, kW→MW |
| `cont_lifetime` | `lifetime` | `number` |
| `cost_interest_rate` | `interest_rate` | `number` |
| `cost_energy_cap` | `costs.monetary.investment_per_capacity` | ×1000 |
| `cost_om_annual` | `costs.monetary.fixed_om` | ×1000 |
| `cost_om_annual_investment_fraction` | `costs.monetary.fixed_om_fraction` | 0→absent (MEME allows `fixed_om` or the fraction, not both); reverse default 0 |
| `cost_om_prod` | `costs.monetary.variable_om` | ×1000 |
| `cost_om_con` | `costs.monetary.fuel_cost` | ×1000 |
| `cost_purchase` | `costs.monetary.purchase` | `number` |

Per technology:

| Key | MEME | Specific fields |
|---|---|---|
| `pv_supply` | supply, `performance` physics `pv` | `tilt`, `azimuth`, `losses`→`params.tilt`, `.azimuth`, `.loss`; `cont_energy_eff`→`efficiency`; `cont_parasitic_eff`→`native.calliope.flow_out_parasitic_eff` |
| `battery_storage` | storage | `cont_energy_eff`→`storage.charge_eff` and `.discharge_eff`; `cont_storage_cap_max`/`_min`→`storage.energy_capacity.max`/`.min` (kWh→MWh, inf→absent); `cont_storage_loss`→`self_discharge`; `cont_storage_initial`→`initial_soc`; `cont_storage_discharge_depth`→`depth_of_discharge`; `cost_storage_cap`→`investment_per_energy_capacity` (×1000); `energy_capacity.expandable` `true` |
| `wind_onshore` | supply, `performance` physics `wind` | `hub_height`, `rotor_diameter`, `nominal_power`, `turbine_id`→`params.*` |
| `biomass_supply`, `geothermal_supply`, `water_supply` | supply | shared rules and `efficiency` only |
| `air_source_heat_pump`, `ground_source_heat_pump`, `electric_boilers` | conversion, `carrier_in` `electricity`, `carrier_out` `heat` | `cont_energy_eff`→`efficiency` (the COP of a heat pump) |
| `gas_boiler` | conversion, `carrier_in` `natural_gas`, `carrier_out` `heat` | `cont_energy_eff`→`efficiency` |
| `air_conditioner` | conversion, `carrier_in` `electricity`, `carrier_out` `cooling` | `cont_energy_eff`→`efficiency` (the SEER) |
| `households_supply`, `non_households_supply` | `trade{households_$k}`, `trade{non_households_$k}` | `cost_om_con`→`import.price` (×1000); `cont_energy_cap_max`→`import.limit` (inf→absent, kW→MW) |
| `gas_supply` | `trade{gas_$k}`, `carrier` `natural_gas` | as the electricity tariffs |

### Not carried

PySAM inputs of `pv_supply` (`system_capacity`, `module_type`,
`inverter_type`, `optimize_orientation`, `inv_eff`, `dc_ac_ratio`) and the
process parameters of biomass and geothermal exist to generate resource
profiles outside the optimiser; MEME has no PySAM, so they have no
counterpart. `cont_resource_unit`, `cont_energy_cap_scale`, `cont_export_cap`,
`cost_export` and `cont_resource_eff` likewise have none. A PV or wind
profile must therefore be supplied to MEME separately, as a `precomputed`
series or through TentaCron's resolvents.

## MEME target compatibility

The produced job passes MEME's decoding and model validation. The per-target
gates then apply:

| Target | Result |
|---|---|
| `calliope` | accepted; warnings that `highs` is replaced by CBC and that the unmet-demand penalty is approximated by `bigM` |
| `pypsa` | rejected: PyPSA does not support `allow_unmet_demand`; remove that constant rule for this target |
| `adopt-net0` | rejected: AdOpT-NET0 has no transmission mapping in MEME yet |

## The reverse direction

`t1k -reverse` rebuilds the calculation payload from a job. Everything the
tables above mark as a copy or conversion comes back exactly (numbers to
their digits, kWh rounded to whole numbers, node ids with their prefix
stripped); the reverse constants restore the payload's required keys with the
placeholders listed. Three limits:

- A building without a connection has a node and a demand but no
  transmission arc, so the reverse has nowhere to place it in `topology`; it
  is not restored.
- `user_id`, `session_id`, `callback_url`, `country` and `lkr` are not in a
  MEME job; the reverse writes empty strings, and the caller must set them
  (the simulation engine's schema requires a non-empty `country`).
- A transformer whose rating is 0 gets no grid trade, and the reverse, which
  tells transformers from buildings by that trade, restores it as a
  building.

`examples/` holds the payload, the job and the reverse output;
`pkg/t1k/testdata/` pins both directions, and the tests check that converting the reverse output
forward again reproduces the job apart from the standalone building.

## Adapting the mapping

`t1k -print-config > my-mapping.json` writes the embedded mapping out.
Typical edits: change the tariff constants, drop the unmet-demand rule for
PyPSA, or add rules for further building properties or demand carriers.
To change the node prefix, edit it in the three places that agree on it: the
key template `model.nodes{n$k}` of the two node rules, the `n$k` templates of
the `node` references, and the `affix` prefix of the arc references.
`t1k -config my-mapping.json` runs the edited copy; the
[configuration reference](../configuration/overview.md) describes every
construct.
