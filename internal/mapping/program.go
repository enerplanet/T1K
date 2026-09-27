// Package mapping compiles a T1K mapping configuration into rules and
// evaluates them in either direction.
//
// A configuration is a JSON document of rules. Compile turns it into a
// Program after validating every path, converter and condition, so that a
// mistake is reported with the rule's position before any document is
// touched. Program.Run applies the rules to a document: Forward reads each
// rule's "from" side and writes its "to" side, Reverse does the opposite.
// The rule model in rule.go is what both halves share.
package mapping

import (
	"errors"
	"fmt"
)

// ErrConfig marks a configuration that failed to parse or validate.
var ErrConfig = errors.New("t1k: invalid configuration")

// ErrRule marks a rule that could not be applied to a document: a converter
// received a value it cannot handle, a "bind" path is missing from an
// element, or a key does not fit an array index.
var ErrRule = errors.New("t1k: rule failed")

// Direction selects which side of the rules is read and which is written.
type Direction int

const (
	// Forward reads "from" and writes "to": the transformation as written.
	Forward Direction = iota
	// Reverse reads "to" and writes "from": the transformation undone.
	Reverse
)

// String names the direction as it appears in error messages.
func (d Direction) String() string {
	if d == Forward {
		return "forward"
	}
	return "reverse"
}

// Program is a compiled configuration. It is immutable after compilation and
// safe to run from several goroutines at once.
type Program struct {
	Name        string
	Version     string
	Description string

	rules []*rule
}

// Compile parses and validates a configuration. Errors wrap ErrConfig and
// name the position of the offending rule, such as rules[3].rules[1].
func Compile(data []byte) (*Program, error) {
	p, err := compile(data)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrConfig, err)
	}
	return p, nil
}

// RuleCount returns the number of top-level rules, for diagnostics.
func (p *Program) RuleCount() int { return len(p.rules) }
