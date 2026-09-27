package mapping

import (
	"github.com/enerplanet/T1K/internal/convert"
	"github.com/enerplanet/T1K/internal/jsonpath"
)

// The compiled rule model. A configuration's rules are compiled into a tree
// of rule values: copy rules and constants are leaves, "each" rules carry the
// nested rules they apply to every element.

type ruleKind int

const (
	ruleCopy     ruleKind = iota // from <-> to, optionally converted
	ruleConstant                 // a fixed value written on one side only
	ruleScope                    // "each": iterate and apply nested rules
)

// rule is one compiled rule of any kind; the fields a kind does not use stay
// zero.
type rule struct {
	kind ruleKind
	loc  string // position in the configuration, for error messages
	desc string

	from, to *jsonpath.Path

	// Copy rules.
	conv   convert.Converter
	def    optional // written by Forward when "from" is absent
	revDef optional // written by Reverse when "to" is absent

	// Constant rules: exactly one of value and template is set.
	value    any
	template *jsonpath.Template

	// Scope rules.
	each  *eachSpec
	rules []*rule

	when *whenSpec
}

// optional is a JSON value that may be absent, such as a rule's default.
type optional struct {
	value any
	set   bool
}

// eachSpec is the iteration of a scope rule: the paths of the elements on
// both sides and the variables bound from values inside the source element.
type eachSpec struct {
	from, to *jsonpath.Path
	bind     []binding // in configuration order
	// join is set when from has variables to cannot supply, so the reverse
	// direction must join onto the elements earlier rules created.
	join bool
}

// binding reads a variable's value from a path inside the source element.
type binding struct {
	variable string
	path     *jsonpath.Path
}

// whenSpec gates a rule per direction; a direction without conditions
// always applies.
type whenSpec struct {
	forward, reverse []condition
}

// forDirection returns the conditions the given direction must satisfy.
func (w *whenSpec) forDirection(dir Direction) []condition {
	if dir == Forward {
		return w.forward
	}
	return w.reverse
}

// condition tests the value at a path with one or more operators, all of
// which must hold.
type condition struct {
	path *jsonpath.Path
	ops  []operator
}

// operator is one test of a condition: exists, equals, not_equals, in, gt,
// gte, lt or lte with its operand.
type operator struct {
	name  string
	value any
}
