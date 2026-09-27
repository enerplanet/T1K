package convert

import (
	"errors"
	"fmt"

	"github.com/enerplanet/T1K/internal/jsondoc"
)

// lookup translates between two vocabularies through a list of pairs
// [[left, right], ...]. Forward maps left to right, reverse maps right to
// left; both columns must be free of duplicates so the table is invertible.
// A value without a pair is an error unless strict is false, in which case it
// passes through unchanged.
type lookup struct {
	pairs  [][2]any
	strict bool
}

func newLookup(args map[string]any) (Converter, error) {
	r := newArgReader(args)
	raw, ok := r.get("pairs")
	if !ok {
		return nil, errors.New("argument \"pairs\" is required")
	}
	strict, err := r.boolean("strict", true)
	if err != nil {
		return nil, err
	}
	if err := r.done(); err != nil {
		return nil, err
	}
	pairs, err := parsePairs(raw)
	if err != nil {
		return nil, err
	}
	return &lookup{pairs: pairs, strict: strict}, nil
}

// parsePairs validates the table: a non-empty list of two-element pairs with
// no value repeated in either column.
func parsePairs(raw any) ([][2]any, error) {
	list, ok := raw.([]any)
	if !ok || len(list) == 0 {
		return nil, errors.New("\"pairs\" must be a non-empty array of [left, right] pairs")
	}
	pairs := make([][2]any, 0, len(list))
	for i, p := range list {
		pair, ok := p.([]any)
		if !ok || len(pair) != 2 {
			return nil, fmt.Errorf("pairs[%d] must be a [left, right] pair", i)
		}
		if err := checkUnique(pairs, i, pair[0], pair[1]); err != nil {
			return nil, err
		}
		pairs = append(pairs, [2]any{pair[0], pair[1]})
	}
	return pairs, nil
}

func checkUnique(pairs [][2]any, i int, left, right any) error {
	for _, prev := range pairs {
		if jsondoc.Equal(prev[0], left) {
			return fmt.Errorf("pairs[%d]: left value %s appears twice", i, jsondoc.Describe(left))
		}
		if jsondoc.Equal(prev[1], right) {
			return fmt.Errorf("pairs[%d]: right value %s appears twice", i, jsondoc.Describe(right))
		}
	}
	return nil
}

func (c *lookup) Forward(v any, present bool) (any, bool, error) {
	return c.translate(v, present, 0, 1)
}
func (c *lookup) Reverse(v any, present bool) (any, bool, error) {
	return c.translate(v, present, 1, 0)
}

// translate maps v from one column of the table to the other.
func (c *lookup) translate(v any, present bool, from, to int) (any, bool, error) {
	if !present {
		return nil, false, nil
	}
	for _, p := range c.pairs {
		if jsondoc.Equal(p[from], v) {
			return jsondoc.DeepCopy(p[to]), true, nil
		}
	}
	if c.strict {
		return nil, false, fmt.Errorf("lookup: no pair for %s %s", jsondoc.TypeName(v), jsondoc.Describe(v))
	}
	return v, true, nil
}
