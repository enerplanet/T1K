package convert

import (
	"errors"
	"fmt"
	"math"

	"github.com/enerplanet/T1K/internal/jsondoc"
)

// linear applies y = x * factor / divisor + offset; the reverse solves for x.
// round_forward and round_reverse round the result of the respective
// direction to that many decimals; unit conversions such as kWh/a to MW are
// the typical use.
type linear struct {
	factor, divisor, offset float64
	roundForward            rounding
	roundReverse            rounding
}

// rounding is an optional number of decimals to round a result to.
type rounding struct {
	decimals int
	enabled  bool
}

func (r rounding) apply(x float64) float64 {
	if !r.enabled {
		return x
	}
	p := math.Pow10(r.decimals)
	return math.Round(x*p) / p
}

func newLinear(args map[string]any) (Converter, error) {
	r := newArgReader(args)
	c := &linear{}
	var err error
	if c.factor, err = r.float("factor", 1); err != nil {
		return nil, err
	}
	if c.divisor, err = r.float("divisor", 1); err != nil {
		return nil, err
	}
	if c.offset, err = r.float("offset", 0); err != nil {
		return nil, err
	}
	if c.roundForward, err = readRounding(r, "round_forward"); err != nil {
		return nil, err
	}
	if c.roundReverse, err = readRounding(r, "round_reverse"); err != nil {
		return nil, err
	}
	if err := r.done(); err != nil {
		return nil, err
	}
	if c.factor == 0 || c.divisor == 0 {
		return nil, errors.New("factor and divisor must not be zero")
	}
	return c, nil
}

func readRounding(r *argReader, key string) (rounding, error) {
	decimals, enabled, err := r.intOpt(key)
	if err != nil {
		return rounding{}, err
	}
	if enabled && decimals < 0 {
		return rounding{}, errors.New("rounding decimals must not be negative")
	}
	return rounding{decimals: decimals, enabled: enabled}, nil
}

func (c *linear) Forward(v any, present bool) (any, bool, error) {
	return c.apply(v, present, func(x float64) float64 {
		return c.roundForward.apply(x*c.factor/c.divisor + c.offset)
	})
}

func (c *linear) Reverse(v any, present bool) (any, bool, error) {
	return c.apply(v, present, func(y float64) float64 {
		return c.roundReverse.apply((y - c.offset) * c.divisor / c.factor)
	})
}

// apply reads the input as a number, computes the result and renders it.
func (c *linear) apply(v any, present bool, f func(float64) float64) (any, bool, error) {
	if !present {
		return nil, false, nil
	}
	x, ok := jsondoc.Float(v)
	if !ok {
		return nil, false, fmt.Errorf("linear: %s %s is not numeric", jsondoc.TypeName(v), jsondoc.Describe(v))
	}
	n, err := jsondoc.FormatNumber(f(x))
	return n, true, err
}
