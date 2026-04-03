package transliterate

import (
	"fmt"
	"strings"
)

// Direction specifies the direction for rule compilation.
type Direction int

const (
	Forward Direction = iota
	Reverse
)

// RuleDirection indicates which direction(s) a parsed rule applies to.
type RuleDirection int

const (
	DirForward RuleDirection = iota
	DirReverse
	DirBoth
)

// RuleType classifies a parsed rule.
type RuleType int

const (
	RuleConversion RuleType = iota
	RuleVariable
	RuleDirective
	RuleFilter
)

// PatternElementType classifies an element within a pattern.
type PatternElementType int

const (
	PELiteral PatternElementType = iota
	PESet
	PEVarRef
)

// PatternElement represents one element of a rule pattern or replacement.
type PatternElement struct {
	Type    PatternElementType
	Literal []rune
	Set     *UnicodeSet
	VarName string
}

// ParsedRule represents a single parsed rule from ICU rule text.
type ParsedRule struct {
	Type      RuleType
	Direction RuleDirection

	// Conversion rules
	BeforeContext []PatternElement
	Match        []PatternElement
	AfterContext  []PatternElement
	Replacement  []PatternElement

	// Variable definitions
	VarName  string
	VarValue []PatternElement

	// Directives
	DirectiveFwd string     // forward directive name (e.g. "NFD")
	DirectiveRev string     // reverse directive name (e.g. "NFC")
	Filter       *UnicodeSet // for :: [set] ;
}

// ParseRules parses ICU transliteration rule text into structured rules.
func ParseRules(text string) ([]ParsedRule, error) {
	p := &ruleParser{text: []rune(text)}
	var rules []ParsedRule

	for {
		p.skipWhitespaceAndComments()
		if p.pos >= len(p.text) {
			break
		}

		rule, err := p.parseOneRule()
		if err != nil {
			return nil, err
		}
		if rule != nil {
			rules = append(rules, *rule)
		}
	}

	return rules, nil
}

type ruleParser struct {
	text []rune
	pos  int
}

func (p *ruleParser) peek() rune {
	if p.pos >= len(p.text) {
		return -1
	}
	return p.text[p.pos]
}

func (p *ruleParser) advance() rune {
	if p.pos >= len(p.text) {
		return -1
	}
	r := p.text[p.pos]
	p.pos++
	return r
}

func (p *ruleParser) skipWhitespaceAndComments() {
	for p.pos < len(p.text) {
		r := p.text[p.pos]
		// Skip whitespace
		if r == ' ' || r == '\t' || r == '\r' || r == '\n' {
			p.pos++
			continue
		}
		// Skip # comments
		if r == '#' {
			p.skipToEndOfLine()
			continue
		}
		// Skip // comments
		if r == '/' && p.pos+1 < len(p.text) && p.text[p.pos+1] == '/' {
			p.skipToEndOfLine()
			continue
		}
		break
	}
}

func (p *ruleParser) skipToEndOfLine() {
	for p.pos < len(p.text) && p.text[p.pos] != '\n' {
		p.pos++
	}
	if p.pos < len(p.text) {
		p.pos++ // skip the newline
	}
}

func (p *ruleParser) parseOneRule() (*ParsedRule, error) {
	// Check for directive: :: ...
	if p.peek() == ':' && p.pos+1 < len(p.text) && p.text[p.pos+1] == ':' {
		return p.parseDirective()
	}

	// Check for variable definition: $name = value ;
	if p.peek() == '$' {
		if p.isVariableDefinition() {
			return p.parseVariableDefinition()
		}
	}

	return p.parseConversionRule()
}

func (p *ruleParser) isVariableDefinition() bool {
	// Look ahead for '=' before ';' or operator
	saved := p.pos
	defer func() { p.pos = saved }()

	p.pos++ // skip '$'
	for p.pos < len(p.text) {
		r := p.text[p.pos]
		if r == '=' {
			return true
		}
		if r == ';' || r == '→' || r == '←' || r == '↔' || r == '>' || r == '<' {
			return false
		}
		p.pos++
	}
	return false
}

func (p *ruleParser) parseDirective() (*ParsedRule, error) {
	p.pos += 2 // skip '::'
	p.skipInlineWhitespace()

	rule := &ParsedRule{Type: RuleDirective}

	// Check for filter: :: [set] ;
	if p.peek() == '[' {
		setExpr, err := p.consumeUnicodeSetExpr()
		if err != nil {
			return nil, fmt.Errorf("directive filter: %w", err)
		}
		s, err := ParseUnicodeSet(setExpr)
		if err != nil {
			return nil, fmt.Errorf("directive filter: %w", err)
		}
		rule.Type = RuleFilter
		rule.Filter = s
		p.skipInlineWhitespace()
		p.consumeSemicolon()
		return rule, nil
	}

	// Check for reverse-only directive: :: (name) ;
	if p.peek() == '(' {
		p.advance() // skip '('
		p.skipInlineWhitespace()
		name := p.consumeDirectiveName()
		p.skipInlineWhitespace()
		if p.peek() == ')' {
			p.advance()
		}
		rule.DirectiveRev = name
		p.skipInlineWhitespace()
		p.consumeSemicolon()
		return rule, nil
	}

	// Forward directive name
	name := p.consumeDirectiveName()
	rule.DirectiveFwd = name
	p.skipInlineWhitespace()

	// Check for reverse part: (name)
	if p.peek() == '(' {
		p.advance()
		p.skipInlineWhitespace()
		revName := p.consumeDirectiveName()
		p.skipInlineWhitespace()
		if p.peek() == ')' {
			p.advance()
		}
		rule.DirectiveRev = revName
	}

	p.skipInlineWhitespace()
	p.consumeSemicolon()
	return rule, nil
}

func (p *ruleParser) consumeDirectiveName() string {
	start := p.pos
	for p.pos < len(p.text) {
		r := p.text[p.pos]
		if r == ';' || r == '(' || r == ')' || r == '\n' || r == '\r' {
			break
		}
		p.pos++
	}
	return strings.TrimSpace(string(p.text[start:p.pos]))
}

func (p *ruleParser) parseVariableDefinition() (*ParsedRule, error) {
	p.advance() // skip '$'

	// Read variable name
	start := p.pos
	for p.pos < len(p.text) {
		r := p.text[p.pos]
		if !isVarNameChar(r) {
			break
		}
		p.pos++
	}
	name := string(p.text[start:p.pos])
	if name == "" {
		return nil, fmt.Errorf("variable: empty name")
	}

	p.skipInlineWhitespace()
	if p.peek() != '=' {
		return nil, fmt.Errorf("variable %q: expected '='", name)
	}
	p.advance() // skip '='
	p.skipInlineWhitespace()

	value, err := p.parsePatternUntilSemicolon()
	if err != nil {
		return nil, fmt.Errorf("variable %q: %w", name, err)
	}
	p.consumeSemicolon()

	return &ParsedRule{
		Type:     RuleVariable,
		VarName:  name,
		VarValue: value,
	}, nil
}

func (p *ruleParser) parseConversionRule() (*ParsedRule, error) {
	// Parse the left side (potentially with context: before { match } after)
	leftElements, err := p.parsePatternUntilOperator()
	if err != nil {
		return nil, err
	}

	// Determine operator
	dir, err := p.parseOperator()
	if err != nil {
		return nil, err
	}

	p.skipInlineWhitespace()

	// Parse replacement (right side)
	replacement, err := p.parsePatternUntilSemicolon()
	if err != nil {
		return nil, err
	}
	p.consumeSemicolon()

	// Split left side into beforeContext, match, afterContext
	before, match, after := splitContext(leftElements)

	return &ParsedRule{
		Type:          RuleConversion,
		Direction:     dir,
		BeforeContext: before,
		Match:         match,
		AfterContext:  after,
		Replacement:   replacement,
	}, nil
}

// splitContext splits pattern elements into before { match } after.
func splitContext(elements []PatternElement) (before, match, after []PatternElement) {
	// Find '{' and '}' markers
	braceOpen := -1
	braceClose := -1

	for i, e := range elements {
		if e.Type == PELiteral && len(e.Literal) == 1 {
			if e.Literal[0] == '{' && braceOpen < 0 {
				braceOpen = i
			} else if e.Literal[0] == '}' && braceClose < 0 {
				braceClose = i
			}
		}
	}

	if braceOpen < 0 {
		// No context markers — entire left side is the match
		return nil, elements, nil
	}

	before = elements[:braceOpen]
	if braceClose > braceOpen {
		match = elements[braceOpen+1 : braceClose]
		after = elements[braceClose+1:]
	} else {
		match = elements[braceOpen+1:]
	}
	return before, match, after
}

func (p *ruleParser) parseOperator() (RuleDirection, error) {
	p.skipInlineWhitespace()
	if p.pos >= len(p.text) {
		return DirForward, fmt.Errorf("rule: unexpected end, expected operator")
	}

	r := p.text[p.pos]
	switch r {
	case '↔': // U+2194
		p.pos++
		return DirBoth, nil
	case '→': // U+2192
		p.pos++
		return DirForward, nil
	case '←': // U+2190
		p.pos++
		return DirReverse, nil
	case '<':
		p.pos++
		if p.pos < len(p.text) && p.text[p.pos] == '>' {
			p.pos++
			return DirBoth, nil
		}
		return DirReverse, nil
	case '>':
		p.pos++
		return DirForward, nil
	}

	return DirForward, fmt.Errorf("rule: expected operator at position %d, got %q", p.pos, string(r))
}

func (p *ruleParser) parsePatternUntilOperator() ([]PatternElement, error) {
	var elements []PatternElement
	for p.pos < len(p.text) {
		p.skipInlineWhitespace()
		r := p.peek()
		if r == -1 || r == ';' {
			break
		}
		// Stop at operators
		if r == '→' || r == '←' || r == '↔' || r == '>' {
			break
		}
		if r == '<' {
			// Could be '<' (reverse) or '<>' (both)
			break
		}

		elem, err := p.parsePatternElement()
		if err != nil {
			return nil, err
		}
		elements = append(elements, elem)
	}
	return elements, nil
}

func (p *ruleParser) parsePatternUntilSemicolon() ([]PatternElement, error) {
	var elements []PatternElement
	for p.pos < len(p.text) {
		p.skipInlineWhitespace()
		r := p.peek()
		if r == -1 || r == ';' {
			break
		}

		elem, err := p.parsePatternElement()
		if err != nil {
			return nil, err
		}
		elements = append(elements, elem)
	}
	return elements, nil
}

func (p *ruleParser) parsePatternElement() (PatternElement, error) {
	r := p.peek()

	// Unicode set
	if r == '[' {
		setExpr, err := p.consumeUnicodeSetExpr()
		if err != nil {
			return PatternElement{}, err
		}
		s, err := ParseUnicodeSet(setExpr)
		if err != nil {
			return PatternElement{}, err
		}
		return PatternElement{Type: PESet, Set: s}, nil
	}

	// Variable reference
	if r == '$' {
		p.advance()
		start := p.pos
		for p.pos < len(p.text) && isVarNameChar(p.text[p.pos]) {
			p.pos++
		}
		name := string(p.text[start:p.pos])
		if name == "" {
			return PatternElement{}, fmt.Errorf("pattern: empty variable name")
		}
		return PatternElement{Type: PEVarRef, VarName: name}, nil
	}

	// Quoted literal
	if r == '\'' {
		runes, err := p.parseQuotedLiteral()
		if err != nil {
			return PatternElement{}, err
		}
		return PatternElement{Type: PELiteral, Literal: runes}, nil
	}

	// Escape
	if r == '\\' {
		escaped, err := p.parseEscapeRune()
		if err != nil {
			return PatternElement{}, err
		}
		return PatternElement{Type: PELiteral, Literal: []rune{escaped}}, nil
	}

	// Context markers and other special chars passed through as literals
	if r == '{' || r == '}' || r == '|' {
		p.advance()
		return PatternElement{Type: PELiteral, Literal: []rune{r}}, nil
	}

	// Regular literal character
	p.advance()
	return PatternElement{Type: PELiteral, Literal: []rune{r}}, nil
}

func (p *ruleParser) parseQuotedLiteral() ([]rune, error) {
	p.advance() // skip opening '
	var runes []rune
	for p.pos < len(p.text) {
		r := p.text[p.pos]
		if r == '\'' {
			p.pos++
			// '' inside quotes is an escaped quote
			if p.pos < len(p.text) && p.text[p.pos] == '\'' {
				runes = append(runes, '\'')
				p.pos++
				continue
			}
			// End of quoted literal
			return runes, nil
		}
		runes = append(runes, r)
		p.pos++
	}
	return nil, fmt.Errorf("pattern: unterminated quoted literal")
}

func (p *ruleParser) parseEscapeRune() (rune, error) {
	p.advance() // skip '\'
	if p.pos >= len(p.text) {
		return 0, fmt.Errorf("pattern: trailing backslash")
	}

	r := p.text[p.pos]
	if r == 'u' || r == 'U' {
		p.pos++
		digits := 4
		if r == 'U' {
			digits = 8
		}
		if p.pos+digits > len(p.text) {
			return 0, fmt.Errorf("pattern: incomplete unicode escape")
		}
		var val rune
		for i := 0; i < digits; i++ {
			val = val << 4
			c := p.text[p.pos+i]
			switch {
			case c >= '0' && c <= '9':
				val |= rune(c - '0')
			case c >= 'a' && c <= 'f':
				val |= rune(c-'a') + 10
			case c >= 'A' && c <= 'F':
				val |= rune(c-'A') + 10
			default:
				return 0, fmt.Errorf("pattern: invalid hex digit %q", string(c))
			}
		}
		p.pos += digits
		return val, nil
	}

	switch r {
	case 'n':
		p.pos++
		return '\n', nil
	case 't':
		p.pos++
		return '\t', nil
	case 'r':
		p.pos++
		return '\r', nil
	}

	p.pos++
	return r, nil
}

// consumeUnicodeSetExpr consumes a complete [...] expression including nested sets,
// returning the raw string for ParseUnicodeSet.
func (p *ruleParser) consumeUnicodeSetExpr() (string, error) {
	if p.peek() != '[' {
		return "", fmt.Errorf("expected '[' at position %d", p.pos)
	}

	// We need to track bracket depth and handle escapes/quotes to find the matching ']'
	start := p.pos
	depth := 0
	for p.pos < len(p.text) {
		r := p.text[p.pos]
		if r == '\\' {
			p.pos += 2 // skip escape
			continue
		}
		if r == '\'' {
			p.pos++ // skip opening quote
			for p.pos < len(p.text) && p.text[p.pos] != '\'' {
				p.pos++
			}
			if p.pos < len(p.text) {
				p.pos++ // skip closing quote
			}
			continue
		}
		if r == '[' {
			depth++
		} else if r == ']' {
			depth--
			if depth == 0 {
				p.pos++
				return string(p.text[start:p.pos]), nil
			}
		}
		p.pos++
	}
	return "", fmt.Errorf("unterminated unicode set starting at position %d", start)
}

func (p *ruleParser) consumeSemicolon() {
	p.skipInlineWhitespace()
	if p.pos < len(p.text) && p.text[p.pos] == ';' {
		p.pos++
	}
}

func (p *ruleParser) skipInlineWhitespace() {
	for p.pos < len(p.text) {
		r := p.text[p.pos]
		if r == ' ' || r == '\t' {
			p.pos++
			continue
		}
		break
	}
}

func isVarNameChar(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
		(r >= '0' && r <= '9') || r == '_'
}

