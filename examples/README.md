# Examples

| File | Content |
|---|---|
| `enerplanet-calculation.json` | a calculation payload as EnerPlanET's backend builds it: two transformer areas, three buildings connected to their transformers (one with PV and a battery, one with a non-household grid tariff), an MV line between the transformers, and one building without a connection |
| `meme-job.json` | the MEME job `t1k -in enerplanet-calculation.json` produces |
| `enerplanet-calculation.reverse.json` | the payload `t1k -reverse -in meme-job.json` rebuilds from it |

Regenerate both outputs with `make example`. The
[mapping page](https://enerplanet.github.io/T1K/mappings/enerplanet-to-meme/)
explains every field and what the reverse restores; the `testdata/` golden
files pin the same conversions for the test suite.
