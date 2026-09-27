package jsonpath

import "regexp"

// Bindings maps variable names (without the "$") to the strings they are
// bound to: array indices as decimal digits, object keys as they are.
type Bindings map[string]string

// Clone returns a copy that can be extended without affecting the original.
func (b Bindings) Clone() Bindings {
	c := make(Bindings, len(b)+1)
	for k, v := range b {
		c[k] = v
	}
	return c
}

var varNameRE = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// ValidVarName reports whether name (without the "$") is a valid variable
// name: a letter or underscore followed by letters, digits and underscores.
func ValidVarName(name string) bool { return varNameRE.MatchString(name) }

// unbound lists the variables in vars that b does not bind, in order.
func unbound(vars []string, b Bindings) []string {
	var out []string
	for _, v := range vars {
		if _, ok := b[v]; !ok {
			out = append(out, v)
		}
	}
	return out
}
