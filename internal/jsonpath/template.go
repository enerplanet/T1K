package jsonpath

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Template is the text between the braces of a path segment, or the template
// of a constant: literal characters and variables. A template renders
// bindings into a key and parses a key back into bindings.
//
// Variables are written "$name" or "${name}", "$$" is a literal dollar sign,
// and "*" alone is an anonymous variable. Two variables need literal text
// between them, because a key is parsed by matching that text: earlier
// variables take the shortest possible match, the last one takes the rest.
type Template struct {
	text  string
	parts []part
	vars  []string
	re    *regexp.Regexp // nil for a template without variables
}

// part is one piece of a template: either literal text or a variable.
type part struct {
	literal  string
	variable string
}

// ParseTemplate parses template text as it appears in a constant rule.
func ParseTemplate(text string) (*Template, error) {
	counter := 0
	return parseTemplate(text, &counter)
}

// parseTemplate parses template text, naming an anonymous "*" from the
// shared counter of the enclosing path.
func parseTemplate(text string, anonymous *int) (*Template, error) {
	if !utf8.ValidString(text) {
		return nil, fmt.Errorf("key template {%s}: invalid UTF-8", text)
	}
	if text == "*" {
		*anonymous++
		return anonymousTemplate(text, "_"+strconv.Itoa(*anonymous)), nil
	}
	tp := &templateParser{template: &Template{text: text}}
	if err := tp.parse(text); err != nil {
		return nil, fmt.Errorf("key template {%s}: %w", text, err)
	}
	return tp.finish()
}

func anonymousTemplate(text, variable string) *Template {
	t := &Template{text: text, parts: []part{{variable: variable}}, vars: []string{variable}}
	// A single variable compiles to a pattern that cannot fail.
	_ = t.compile()
	return t
}

// Vars returns the template's variables in order of appearance.
func (t *Template) Vars() []string { return append([]string(nil), t.vars...) }

// Render substitutes the bound variables into the template.
func (t *Template) Render(b Bindings) (string, error) {
	var out strings.Builder
	for _, p := range t.parts {
		if p.variable == "" {
			out.WriteString(p.literal)
			continue
		}
		val, ok := b[p.variable]
		if !ok {
			return "", fmt.Errorf("variable $%s is not bound", p.variable)
		}
		out.WriteString(val)
	}
	return out.String(), nil
}

// match parses key against the template. Variables already bound in b must
// agree with the parsed value; the returned bindings add the new ones.
func (t *Template) match(key string, b Bindings) (Bindings, bool) {
	if t.re == nil {
		return b, key == t.parts[0].literal
	}
	m := t.re.FindStringSubmatch(key)
	if m == nil {
		return nil, false
	}
	out := b.Clone()
	for i, v := range t.vars {
		if cur, ok := out[v]; ok && cur != m[i+1] {
			return nil, false
		}
		out[v] = m[i+1]
	}
	return out, true
}

func (t *Template) unbound(b Bindings) []string { return unbound(t.vars, b) }

// compile builds the regular expression that parses keys: every variable is
// a non-greedy group, so earlier variables take the shortest match. The
// pattern runs in single-line mode because an object key may contain a
// newline, which "." would otherwise refuse to match.
func (t *Template) compile() error {
	if len(t.vars) == 0 {
		return nil
	}
	var b strings.Builder
	b.WriteString("(?s)^")
	for _, p := range t.parts {
		if p.variable != "" {
			b.WriteString("(.+?)")
		} else {
			b.WriteString(regexp.QuoteMeta(p.literal))
		}
	}
	b.WriteString("$")
	re, err := regexp.Compile(b.String())
	if err != nil {
		return fmt.Errorf("key template {%s}: %w", t.text, err)
	}
	t.re = re
	return nil
}

// templateParser accumulates the parts of a template while scanning its text.
type templateParser struct {
	template *Template
	literal  strings.Builder
}

func (tp *templateParser) parse(text string) error {
	for i := 0; i < len(text); i++ {
		if text[i] != '$' {
			tp.literal.WriteByte(text[i])
			continue
		}
		if i+1 < len(text) && text[i+1] == '$' {
			tp.literal.WriteByte('$')
			i++
			continue
		}
		name, next, err := readVarName(text, i+1)
		if err != nil {
			return err
		}
		if err := tp.addVariable(name); err != nil {
			return err
		}
		i = next - 1
	}
	tp.flushLiteral()
	return nil
}

func (tp *templateParser) flushLiteral() {
	if tp.literal.Len() > 0 {
		tp.template.parts = append(tp.template.parts, part{literal: tp.literal.String()})
		tp.literal.Reset()
	}
}

// addVariable appends a variable part, rejecting two variables in a row.
func (tp *templateParser) addVariable(name string) error {
	tp.flushLiteral()
	if last, ok := tp.lastVariable(); ok {
		return fmt.Errorf("variables $%s and $%s need literal text between them", last, name)
	}
	tp.template.parts = append(tp.template.parts, part{variable: name})
	tp.template.vars = append(tp.template.vars, name)
	return nil
}

// lastVariable returns the variable of the last part, if the last part is
// one.
func (tp *templateParser) lastVariable() (string, bool) {
	parts := tp.template.parts
	if n := len(parts); n > 0 && parts[n-1].variable != "" {
		return parts[n-1].variable, true
	}
	return "", false
}

// finish validates the assembled template and compiles its matcher.
func (tp *templateParser) finish() (*Template, error) {
	t := tp.template
	if len(t.parts) == 0 {
		return nil, fmt.Errorf("key template {%s}: empty", t.text)
	}
	for i, v := range t.vars {
		for _, w := range t.vars[:i] {
			if v == w {
				return nil, fmt.Errorf("key template {%s}: variable $%s appears twice", t.text, v)
			}
		}
	}
	if err := t.compile(); err != nil {
		return nil, err
	}
	return t, nil
}

// readVarName reads a variable name starting at s[i], just after the "$":
// either "{name}" or the longest run of name characters.
func readVarName(s string, i int) (name string, next int, err error) {
	if i < len(s) && s[i] == '{' {
		end := strings.IndexByte(s[i:], '}')
		if end < 0 {
			return "", 0, errors.New("unterminated ${ variable")
		}
		return checkVarName(s[i+1:i+end], i+end+1)
	}
	j := i
	for j < len(s) && isNameByte(s[j]) {
		j++
	}
	return checkVarName(s[i:j], j)
}

func checkVarName(name string, next int) (string, int, error) {
	if !ValidVarName(name) {
		return "", 0, fmt.Errorf("invalid variable name %q", "$"+name)
	}
	return name, next, nil
}

func isNameByte(c byte) bool {
	return c == '_' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}
