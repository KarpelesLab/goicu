package transliterate

import (
	"fmt"

	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

// CompiledElement is a single element in a compiled pattern.
type CompiledElement struct {
	kind    int // elemLiteral or elemSet
	literal []rune
	set     *UnicodeSet
}

const (
	elemLiteral = iota
	elemSet
)

// CompiledPattern is a sequence of compiled elements forming a match or context pattern.
type CompiledPattern struct {
	elements []CompiledElement
	minLen   int // minimum runes this pattern can match
	maxLen   int // maximum runes this pattern can match (-1 if unbounded)
}

// CompiledRule is a single conversion rule ready for execution.
type CompiledRule struct {
	beforeCtx   CompiledPattern
	match       CompiledPattern
	afterCtx    CompiledPattern
	replacement []rune
}

// CompiledRuleSet holds all compiled rules plus metadata for execution.
type CompiledRuleSet struct {
	rules       []*CompiledRule
	maxMatchLen int
	maxBeforeLen int
	filter      *UnicodeSet
}

// CompilationResult holds the compiled rules plus any pre/post transforms.
type CompilationResult struct {
	RuleSet  *CompiledRuleSet
	PreNorm  []transform.Transformer
	PostNorm []transform.Transformer
}

// CompileRules compiles parsed rules for a specific direction.
func CompileRules(parsed []ParsedRule, dir Direction) (*CompilationResult, error) {
	// First pass: collect variables
	vars := make(map[string][]PatternElement)
	for _, r := range parsed {
		if r.Type == RuleVariable {
			vars[r.VarName] = r.VarValue
		}
	}

	result := &CompilationResult{
		RuleSet: &CompiledRuleSet{},
	}

	// Track whether we've seen conversion rules (for pre/post norm placement)
	seenConversion := false

	for _, r := range parsed {
		switch r.Type {
		case RuleVariable:
			// Already collected above
			continue

		case RuleFilter:
			result.RuleSet.filter = r.Filter

		case RuleDirective:
			t := resolveDirectiveTransform(r, dir)
			if t != nil {
				if seenConversion {
					result.PostNorm = append(result.PostNorm, t)
				} else {
					result.PreNorm = append(result.PreNorm, t)
				}
			}

		case RuleConversion:
			seenConversion = true
			compiled, err := compileConversionRule(r, dir, vars)
			if err != nil {
				return nil, err
			}
			if compiled != nil {
				result.RuleSet.rules = append(result.RuleSet.rules, compiled)
			}
		}
	}

	// Compute max match length
	for _, r := range result.RuleSet.rules {
		if r.match.maxLen > result.RuleSet.maxMatchLen {
			result.RuleSet.maxMatchLen = r.match.maxLen
		}
		if r.beforeCtx.maxLen > result.RuleSet.maxBeforeLen {
			result.RuleSet.maxBeforeLen = r.beforeCtx.maxLen
		}
	}

	return result, nil
}

func resolveDirectiveTransform(r ParsedRule, dir Direction) transform.Transformer {
	var name string
	if dir == Forward {
		name = r.DirectiveFwd
	} else {
		name = r.DirectiveRev
	}
	if name == "" {
		return nil
	}

	switch name {
	case "NFD":
		return norm.NFD
	case "NFC":
		return norm.NFC
	case "NFKC":
		return norm.NFKC
	case "NFKD":
		return norm.NFKD
	default:
		// Try to resolve as a registered transform
		factory, err := lookup(name)
		if err == nil {
			return factory()
		}
		return nil
	}
}

func compileConversionRule(r ParsedRule, dir Direction, vars map[string][]PatternElement) (*CompiledRule, error) {
	// Skip rules that don't apply to this direction
	switch r.Direction {
	case DirForward:
		if dir == Reverse {
			return nil, nil
		}
	case DirReverse:
		if dir == Forward {
			return nil, nil
		}
	case DirBoth:
		// Applies to both directions
	}

	var matchElems, replElems, beforeElems, afterElems []PatternElement

	if dir == Reverse && (r.Direction == DirBoth || r.Direction == DirReverse) {
		// For bidirectional and reverse-only rules compiled in reverse:
		// swap match and replacement.
		// "a ← b" means b→a in reverse; "a ↔ b" means b→a in reverse.
		//
		// For ← rules, context markers {/} may be in the Replacement (right side),
		// which becomes the match after swapping. Extract context from it.
		if r.Direction == DirReverse {
			// Reassemble the right side and extract context
			rawRight := r.Replacement
			beforeElems, matchElems, afterElems = splitContext(rawRight)
			// The left side (r.Match) becomes the replacement.
			// Reassemble it including any original context that was extracted at parse time.
			replElems = reassemblePattern(r.BeforeContext, r.Match, r.AfterContext)
		} else {
			// DirBoth: simple swap, context stays with the original left side
			matchElems = r.Replacement
			replElems = r.Match
			beforeElems = nil
			afterElems = nil
		}
	} else {
		matchElems = r.Match
		replElems = r.Replacement
		beforeElems = r.BeforeContext
		afterElems = r.AfterContext
	}

	match, err := compilePattern(matchElems, vars)
	if err != nil {
		return nil, fmt.Errorf("match pattern: %w", err)
	}

	before, err := compilePattern(beforeElems, vars)
	if err != nil {
		return nil, fmt.Errorf("before context: %w", err)
	}

	after, err := compilePattern(afterElems, vars)
	if err != nil {
		return nil, fmt.Errorf("after context: %w", err)
	}

	repl, err := expandReplacement(replElems, vars)
	if err != nil {
		return nil, fmt.Errorf("replacement: %w", err)
	}

	return &CompiledRule{
		beforeCtx:   before,
		match:       match,
		afterCtx:    after,
		replacement: repl,
	}, nil
}

func compilePattern(elements []PatternElement, vars map[string][]PatternElement) (CompiledPattern, error) {
	var compiled []CompiledElement
	minLen := 0
	maxLen := 0

	for _, e := range elements {
		switch e.Type {
		case PELiteral:
			// Skip context markers in compiled output
			runes := filterContextMarkers(e.Literal)
			if len(runes) > 0 {
				compiled = append(compiled, CompiledElement{kind: elemLiteral, literal: runes})
				minLen += len(runes)
				maxLen += len(runes)
			}

		case PESet:
			compiled = append(compiled, CompiledElement{kind: elemSet, set: e.Set})
			minLen++
			maxLen++

		case PEVarRef:
			// Expand variable
			val, ok := vars[e.VarName]
			if !ok {
				return CompiledPattern{}, fmt.Errorf("undefined variable $%s", e.VarName)
			}
			// If the variable is a single set element, use it as a set
			if len(val) == 1 && val[0].Type == PESet {
				compiled = append(compiled, CompiledElement{kind: elemSet, set: val[0].Set})
				minLen++
				maxLen++
			} else {
				// Expand variable value as literals
				sub, err := compilePattern(val, vars)
				if err != nil {
					return CompiledPattern{}, fmt.Errorf("variable $%s: %w", e.VarName, err)
				}
				compiled = append(compiled, sub.elements...)
				minLen += sub.minLen
				maxLen += sub.maxLen
			}
		}
	}

	return CompiledPattern{elements: compiled, minLen: minLen, maxLen: maxLen}, nil
}

func expandReplacement(elements []PatternElement, vars map[string][]PatternElement) ([]rune, error) {
	var runes []rune
	for _, e := range elements {
		switch e.Type {
		case PELiteral:
			// Skip cursor markers in replacement
			for _, r := range e.Literal {
				if r != '|' {
					runes = append(runes, r)
				}
			}
		case PESet:
			// Sets in replacement aren't meaningful; skip
		case PEVarRef:
			val, ok := vars[e.VarName]
			if !ok {
				return nil, fmt.Errorf("undefined variable $%s", e.VarName)
			}
			sub, err := expandReplacement(val, vars)
			if err != nil {
				return nil, err
			}
			runes = append(runes, sub...)
		}
	}
	return runes, nil
}

func filterContextMarkers(runes []rune) []rune {
	var out []rune
	for _, r := range runes {
		if r != '{' && r != '}' && r != '|' {
			out = append(out, r)
		}
	}
	return out
}

// reassemblePattern combines before + match + after back into a flat slice,
// without context markers, for use as a replacement.
func reassemblePattern(before, match, after []PatternElement) []PatternElement {
	var result []PatternElement
	result = append(result, before...)
	result = append(result, match...)
	result = append(result, after...)
	return result
}
