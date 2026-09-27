package convert

import (
	"errors"
	"fmt"
	"time"

	"github.com/enerplanet/T1K/internal/jsondoc"
)

// datetime re-formats a timestamp string between two layouts (Go reference
// layouts or one of the names in namedLayouts). Forward parses "from" and
// renders "to"; reverse does the opposite. Timestamps without zone
// information are read in "zone" (default UTC), and output is rendered in
// that zone.
type datetime struct {
	from, to string
	zone     *time.Location
}

var namedLayouts = map[string]string{
	"RFC3339":     time.RFC3339,
	"RFC3339Nano": time.RFC3339Nano,
	"RFC1123":     time.RFC1123,
	"RFC1123Z":    time.RFC1123Z,
	"DateOnly":    time.DateOnly,
	"DateTime":    time.DateTime,
	"TimeOnly":    time.TimeOnly,
}

func newDatetime(args map[string]any) (Converter, error) {
	r := newArgReader(args)
	c := &datetime{}
	var err error
	if c.from, err = r.str("from", ""); err != nil {
		return nil, err
	}
	if c.to, err = r.str("to", ""); err != nil {
		return nil, err
	}
	zone, err := r.str("zone", "UTC")
	if err != nil {
		return nil, err
	}
	if err := r.done(); err != nil {
		return nil, err
	}
	if c.from == "" || c.to == "" {
		return nil, errors.New("arguments \"from\" and \"to\" (layouts) are required")
	}
	c.from, c.to = resolveLayout(c.from), resolveLayout(c.to)
	if c.zone, err = time.LoadLocation(zone); err != nil {
		return nil, fmt.Errorf("zone %q: %w", zone, err)
	}
	return c, nil
}

// resolveLayout replaces a layout name by the Go layout it stands for.
func resolveLayout(name string) string {
	if l, ok := namedLayouts[name]; ok {
		return l
	}
	return name
}

func (c *datetime) Forward(v any, present bool) (any, bool, error) {
	return c.reformat(v, present, c.from, c.to)
}

func (c *datetime) Reverse(v any, present bool) (any, bool, error) {
	return c.reformat(v, present, c.to, c.from)
}

func (c *datetime) reformat(v any, present bool, parse, format string) (any, bool, error) {
	if !present {
		return nil, false, nil
	}
	s, ok := v.(string)
	if !ok {
		return nil, false, fmt.Errorf("datetime: expected a string, got %s", jsondoc.TypeName(v))
	}
	t, err := time.ParseInLocation(parse, s, c.zone)
	if err != nil {
		return nil, false, fmt.Errorf("datetime: %w", err)
	}
	return t.In(c.zone).Format(format), true, nil
}
