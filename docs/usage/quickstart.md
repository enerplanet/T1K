# Quickstart

## Convert the example payload

The repository ships an EnerPlanET calculation payload under `examples/`.
Convert it into a MEME job with the embedded mapping:

```bash
t1k -in examples/enerplanet-calculation.json -out meme-job.json
```

`meme-job.json` is the body of MEME's `POST /simulate`: the building and
transformer features have become `model.nodes`, the connections
`model.transmission`, the annual demands `demand-*` technologies, the attached
PV and battery `pv_supply-1` and `battery_storage-1`, and the transformers
`model.trade` entries that import from the grid. The
[mapping page](../mappings/enerplanet-to-meme.md) explains every field.

Convert it back:

```bash
t1k -reverse -in meme-job.json -out calculation.json
```

`calculation.json` has the payload's structure again. Fields a MEME job cannot
carry (`user_id`, `callback_url`, ...) come back as the placeholders the
mapping declares for them; the page linked above lists them.

## Write your own mapping

Save this as `orders.json`:

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

and run it on an order:

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
      "line_0": {"article": "bolt", "quantity": 40},
      "line_1": {"article": "nut", "quantity": 40}
    },
    "reference": "A-17",
    "total": 12.5
  }
}
```

Pipe that output through `t1k -config orders.json -reverse` and the order
comes back: the total in cents, the date in RFC 3339 (at midnight UTC, the
time of day is not in the invoice), the items as an array. `currency` was a
constant for the invoice side only, so the reverse does not write it.

## Use it from Go

```go
package main

import (
	"fmt"
	"os"

	t1k "github.com/enerplanet/T1K"
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

See [the Go package](library.md) for custom configurations and error handling.
