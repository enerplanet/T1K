# Quickstart

This page converts the shipped example in both directions, then writes a
small mapping of its own, and ends with the same conversion from Go.

## Convert the example payload

The repository ships an EnerPlanET calculation payload under `examples/`.
Convert it into a MEME job with the embedded mapping:

```bash
t1k -in examples/enerplanet-calculation.json -out meme-job.json
```

`meme-job.json` is the body of MEME's `POST /simulate`. The building and
transformer features have become `model.nodes` (`n1`, `ntrafo_2585`, ...),
the connections `model.transmission` arcs (`lv_0`, `mv_2`, ...), the annual
demands `demand_*` technologies, the attached PV and battery `pv_supply_1`
and `battery_storage_1`, and the transformers `model.trade` entries that
import from the grid. The [mapping page](../mappings/enerplanet-to-meme.md)
explains every field and the identifier scheme.

Convert the job back:

```bash
t1k -reverse -in meme-job.json -out calculation.json
```

`calculation.json` has the payload's structure again, with EnerPlanET's own
ids (`1`, `trafo_2585`). Fields a MEME job cannot carry (`user_id`,
`callback_url`, ...) come back as the placeholders the mapping declares for
them; the mapping page lists them.

## Write your own mapping

Save the following as `orders.json`:

```json
{
  "name": "orders-to-invoices",
  "rules": [
    {"from": "order.number", "to": "invoice.reference"},
    {"from": "order.total_cents", "to": "invoice.total", "convert": {"linear": {"divisor": 100}}},
    {"from": "order.placed", "to": "invoice.date", "convert": {"datetime": {"from": "RFC3339", "to": "DateOnly"}}},
    {"to": "invoice.currency", "value": "EUR"},
    {
      "each": {"from": "order.lines[$i]", "to": "invoice.items{line_$i}"},
      "rules": [
        {"from": "sku", "to": "article"},
        {"from": "qty", "to": "quantity", "convert": "number"}
      ]
    }
  ]
}
```

Run it on an order:

```bash
echo '{"order": {"number": "A-17", "total_cents": 1250, "placed": "2026-09-27T10:15:00Z",
       "lines": [{"sku": "bolt", "qty": "40"}, {"sku": "nut", "qty": 40}]}}' \
  | t1k -config orders.json
```

```json
{
  "invoice": {
    "currency": "EUR",
    "date": "2026-09-27",
    "items": {
      "line_0": {
        "article": "bolt",
        "quantity": 40
      },
      "line_1": {
        "article": "nut",
        "quantity": 40
      }
    },
    "reference": "A-17",
    "total": 12.5
  }
}
```

The total was divided by 100, the timestamp reduced to a date, the quantity
given as text normalised to a number, and the lines keyed by their position.
Object keys are written in sorted order; `-compact` writes the document on
one line.

Pipe that output through `t1k -config orders.json -reverse` and the order
comes back:

```json
{
  "order": {
    "lines": [
      {
        "qty": 40,
        "sku": "bolt"
      },
      {
        "qty": 40,
        "sku": "nut"
      }
    ],
    "number": "A-17",
    "placed": "2026-09-27T00:00:00Z",
    "total_cents": 1250
  }
}
```

The total is in cents again and the items are an array. The date comes back
at midnight UTC, because the invoice does not carry the time of day; such
losses are the documented exceptions of the [reverse direction](../configuration/reverse.md).
`currency` was a constant for the invoice side only, so the reverse does not
write it.

## Use it from Go

```go
package main

import (
	"fmt"
	"os"

	"github.com/enerplanet/T1K/pkg/t1k"
)

func main() {
	payload, err := os.ReadFile("examples/enerplanet-calculation.json")
	if err != nil {
		panic(err)
	}
	task := t1k.NewTransformTask(t1k.WithIndent("", "  "))
	job, err := task.Transform(payload)
	if err != nil {
		panic(err)
	}
	fmt.Println(string(job))
}
```

See [Go package](library.md) for custom configurations, options and error
handling.
