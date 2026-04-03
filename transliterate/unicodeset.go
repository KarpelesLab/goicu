package transliterate

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// UnicodeSet represents a set of Unicode code points, supporting ICU-style
// set notation such as [a-z], [:Latin:], set operations, and negation.
type UnicodeSet struct {
	fn func(rune) bool
}

// Contains returns true if the rune is a member of the set.
func (s *UnicodeSet) Contains(r rune) bool {
	if s == nil || s.fn == nil {
		return false
	}
	return s.fn(r)
}

// ParseUnicodeSet parses an ICU-style Unicode set expression (e.g. "[a-z]", "[:Latin:]").
func ParseUnicodeSet(expr string) (*UnicodeSet, error) {
	s, n, err := parseUnicodeSetAt(expr, 0)
	if err != nil {
		return nil, err
	}
	if n != len(expr) {
		return nil, fmt.Errorf("unicodeset: unexpected trailing text at position %d", n)
	}
	return s, nil
}

// parseUnicodeSetAt parses a unicode set starting at position pos in expr.
// Handles both regular sets [a-z] and POSIX property classes [:Latin:].
// Returns the set and the position after the closing ']'.
func parseUnicodeSetAt(expr string, pos int) (*UnicodeSet, int, error) {
	if pos >= len(expr) || expr[pos] != '[' {
		return nil, pos, fmt.Errorf("unicodeset: expected '[' at position %d", pos)
	}

	// Check for POSIX property class: [:name:] or [:^name:]
	// The '[:' is the opening delimiter, ':]' is the closing delimiter.
	if strings.HasPrefix(expr[pos:], "[:") {
		fn, newPos, err := parsePropertyClass(expr, pos)
		if err != nil {
			return nil, newPos, err
		}
		return &UnicodeSet{fn: fn}, newPos, nil
	}

	pos++ // skip '['

	// Check for negation
	negated := false
	if pos < len(expr) && expr[pos] == '^' {
		negated = true
		pos++
	}

	var items []func(rune) bool
	var ops []setOp // pending intersection/difference operations

	for pos < len(expr) {
		pos = skipSetWhitespace(expr, pos)
		if pos >= len(expr) {
			return nil, pos, fmt.Errorf("unicodeset: unterminated set")
		}

		ch := expr[pos]

		if ch == ']' {
			pos++
			result := combineItems(items)
			result = applyOps(result, ops)
			if negated {
				inner := result
				result = func(r rune) bool { return !inner(r) }
			}
			return &UnicodeSet{fn: result}, pos, nil
		}

		// Set operations: & or - followed by a set
		if (ch == '&' || ch == '-') && pos+1 < len(expr) {
			next := expr[pos+1]
			if next == '[' {
				op := setOp{kind: ch}
				op.base = combineItems(items)
				items = nil
				operand, newPos, err := parseUnicodeSetAt(expr, pos+1)
				if err != nil {
					return nil, newPos, err
				}
				op.operand = operand.fn
				ops = append(ops, op)
				pos = newPos
				continue
			}
		}

		// Nested set or property class: [...] or [:name:]
		if ch == '[' {
			nested, newPos, err := parseUnicodeSetAt(expr, pos)
			if err != nil {
				return nil, newPos, err
			}
			items = append(items, nested.fn)
			pos = newPos
			continue
		}

		// Parse a character (possibly start of a range)
		r, newPos, err := parseSetChar(expr, pos)
		if err != nil {
			return nil, newPos, err
		}
		pos = newPos

		// Check for range: char '-' char
		pos = skipSetWhitespace(expr, pos)
		if pos < len(expr) && expr[pos] == '-' && pos+1 < len(expr) && expr[pos+1] != ']' && expr[pos+1] != '[' {
			pos++ // skip '-'
			pos = skipSetWhitespace(expr, pos)
			r2, newPos2, err := parseSetChar(expr, pos)
			if err != nil {
				return nil, newPos2, err
			}
			pos = newPos2
			lo, hi := r, r2
			items = append(items, func(r rune) bool { return r >= lo && r <= hi })
		} else {
			c := r
			items = append(items, func(r rune) bool { return r == c })
		}
	}

	return nil, pos, fmt.Errorf("unicodeset: unterminated set")
}

type setOp struct {
	kind    byte // '&' or '-'
	base    func(rune) bool
	operand func(rune) bool
}

func combineItems(items []func(rune) bool) func(rune) bool {
	if len(items) == 0 {
		return func(r rune) bool { return false }
	}
	if len(items) == 1 {
		return items[0]
	}
	// Union of all items
	cp := make([]func(rune) bool, len(items))
	copy(cp, items)
	return func(r rune) bool {
		for _, fn := range cp {
			if fn(r) {
				return true
			}
		}
		return false
	}
}

func applyOps(base func(rune) bool, ops []setOp) func(rune) bool {
	if len(ops) == 0 {
		return base
	}
	result := base
	for _, op := range ops {
		// The op's base is the items accumulated before the operator
		opBase := op.base
		if opBase == nil {
			opBase = result
		}
		operand := op.operand
		switch op.kind {
		case '&':
			result = func(r rune) bool { return opBase(r) && operand(r) }
		case '-':
			result = func(r rune) bool { return opBase(r) && !operand(r) }
		}
	}
	return result
}

// parsePropertyClass parses [:name:] or [:^name:] starting at pos (which points to '[:').
func parsePropertyClass(expr string, pos int) (func(rune) bool, int, error) {
	if !strings.HasPrefix(expr[pos:], "[:") {
		return nil, pos, fmt.Errorf("unicodeset: expected '[:' at position %d", pos)
	}
	pos += 2

	negated := false
	if pos < len(expr) && expr[pos] == '^' {
		negated = true
		pos++
	}

	end := strings.Index(expr[pos:], ":]")
	if end < 0 {
		return nil, pos, fmt.Errorf("unicodeset: unterminated property class")
	}
	name := expr[pos : pos+end]
	pos += end + 2 // skip ":]"

	rt := resolveProperty(name)
	if rt == nil {
		return nil, pos, fmt.Errorf("unicodeset: unknown property %q", name)
	}

	fn := func(r rune) bool { return unicode.Is(rt, r) }
	if negated {
		fn = func(r rune) bool { return !unicode.Is(rt, r) }
	}
	return fn, pos, nil
}

// resolveProperty resolves a Unicode property name to a RangeTable.
func resolveProperty(name string) *unicode.RangeTable {
	// Try scripts first (Latin, Hiragana, Katakana, etc.)
	if rt, ok := unicode.Scripts[name]; ok {
		return rt
	}
	// Try categories (L, Lu, Mn, N, etc.)
	if rt, ok := unicode.Categories[name]; ok {
		return rt
	}
	// Try properties (ASCII_Hex_Digit, etc.)
	if rt, ok := unicode.Properties[name]; ok {
		return rt
	}
	// Try common aliases
	aliases := map[string]string{
		"Letter":           "L",
		"Lowercase_Letter": "Ll",
		"Uppercase_Letter": "Lu",
		"Titlecase_Letter": "Lt",
		"Modifier_Letter":  "Lm",
		"Other_Letter":     "Lo",
		"Mark":             "M",
		"Nonspacing_Mark":  "Mn",
		"Spacing_Mark":     "Mc",
		"Enclosing_Mark":   "Me",
		"Number":           "N",
		"Decimal_Number":   "Nd",
		"Letter_Number":    "Nl",
		"Other_Number":     "No",
		"Punctuation":      "P",
		"Separator":        "Z",
		"Symbol":           "S",
		"Other":            "C",
		"Inherited":        "Inherited",
	}
	if canon, ok := aliases[name]; ok {
		if rt, ok := unicode.Categories[canon]; ok {
			return rt
		}
		if rt, ok := unicode.Scripts[canon]; ok {
			return rt
		}
	}
	return nil
}

// parseSetChar parses a single character within a set expression.
func parseSetChar(expr string, pos int) (rune, int, error) {
	if pos >= len(expr) {
		return 0, pos, fmt.Errorf("unicodeset: unexpected end of input")
	}

	// Escaped character
	if expr[pos] == '\\' {
		return parseEscape(expr, pos)
	}

	// Quoted literal
	if expr[pos] == '\'' {
		r, newPos, err := parseQuotedChar(expr, pos)
		if err != nil {
			return 0, newPos, err
		}
		return r, newPos, nil
	}

	r, size := utf8.DecodeRuneInString(expr[pos:])
	if r == utf8.RuneError && size <= 1 {
		return 0, pos, fmt.Errorf("unicodeset: invalid UTF-8 at position %d", pos)
	}
	return r, pos + size, nil
}

// parseEscape parses \uXXXX or \c escape sequences.
func parseEscape(expr string, pos int) (rune, int, error) {
	if pos+1 >= len(expr) {
		return 0, pos, fmt.Errorf("unicodeset: trailing backslash")
	}
	pos++ // skip '\'

	if expr[pos] == 'u' || expr[pos] == 'U' {
		pos++
		digits := 4
		if expr[pos-1] == 'U' {
			digits = 8
		}
		if pos+digits > len(expr) {
			return 0, pos, fmt.Errorf("unicodeset: incomplete unicode escape")
		}
		var val rune
		for i := 0; i < digits; i++ {
			val = val << 4
			c := expr[pos+i]
			switch {
			case c >= '0' && c <= '9':
				val |= rune(c - '0')
			case c >= 'a' && c <= 'f':
				val |= rune(c-'a') + 10
			case c >= 'A' && c <= 'F':
				val |= rune(c-'A') + 10
			default:
				return 0, pos, fmt.Errorf("unicodeset: invalid hex digit %q", string(c))
			}
		}
		return val, pos + digits, nil
	}

	// \n, \t, etc.
	switch expr[pos] {
	case 'n':
		return '\n', pos + 1, nil
	case 't':
		return '\t', pos + 1, nil
	case 'r':
		return '\r', pos + 1, nil
	}

	// Any other escaped character is literal
	r, size := utf8.DecodeRuneInString(expr[pos:])
	return r, pos + size, nil
}

// parseQuotedChar parses a single character from a quoted sequence starting at pos.
func parseQuotedChar(expr string, pos int) (rune, int, error) {
	if expr[pos] != '\'' {
		return 0, pos, fmt.Errorf("unicodeset: expected quote")
	}
	pos++
	if pos >= len(expr) {
		return 0, pos, fmt.Errorf("unicodeset: unterminated quote")
	}
	// '' is an escaped single quote
	if expr[pos] == '\'' {
		if pos+1 < len(expr) && expr[pos+1] == '\'' {
			return '\'', pos + 2, nil
		}
		// Empty quotes - treat as error
		return 0, pos, fmt.Errorf("unicodeset: empty quoted string")
	}
	r, size := utf8.DecodeRuneInString(expr[pos:])
	pos += size
	// Expect closing quote
	if pos >= len(expr) || expr[pos] != '\'' {
		return 0, pos, fmt.Errorf("unicodeset: unterminated quote")
	}
	return r, pos + 1, nil
}

func skipSetWhitespace(expr string, pos int) int {
	for pos < len(expr) {
		r, size := utf8.DecodeRuneInString(expr[pos:])
		if r != ' ' && r != '\t' && r != '\n' && r != '\r' {
			break
		}
		pos += size
	}
	return pos
}
