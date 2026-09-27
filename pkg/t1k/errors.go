package t1k

import (
	"errors"

	"github.com/enerplanet/T1K/internal/mapping"
)

// Every error the package returns wraps one of these sentinels, so callers
// can classify failures with errors.Is.
var (
	// ErrConfig marks a configuration that failed to parse or validate; the
	// message names the position of the offending rule.
	ErrConfig = mapping.ErrConfig

	// ErrInput marks a document that is not valid JSON.
	ErrInput = errors.New("t1k: invalid input")

	// ErrRule marks a rule that could not be applied to a document: a
	// converter received a value it cannot handle, a "bind" path is missing
	// from an element, or a key does not fit an array index. The message
	// names the rule and the direction.
	ErrRule = mapping.ErrRule
)
