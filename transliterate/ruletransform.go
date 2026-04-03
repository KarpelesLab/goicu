package transliterate

import (
	"unicode/utf8"

	"golang.org/x/text/transform"
)

// RuleTransformer implements transform.Transformer using compiled transliteration rules.
type RuleTransformer struct {
	ruleSet   *CompiledRuleSet
	beforeBuf []rune // sliding window of recent output for before-context matching
	maxBefore int
}

// NewRuleTransformer creates a new RuleTransformer from a compiled rule set.
func NewRuleTransformer(rs *CompiledRuleSet) *RuleTransformer {
	return &RuleTransformer{
		ruleSet:   rs,
		maxBefore: rs.maxBeforeLen,
	}
}

func (rt *RuleTransformer) Transform(dst, src []byte, atEOF bool) (nDst, nSrc int, err error) {
	// Decode all complete runes from src
	runes := make([]rune, 0, len(src)/2+1)
	byteOffsets := make([]int, 0, len(src)/2+1)
	bytePos := 0

	for bytePos < len(src) {
		if !utf8.FullRune(src[bytePos:]) {
			break
		}
		r, size := utf8.DecodeRune(src[bytePos:])
		runes = append(runes, r)
		byteOffsets = append(byteOffsets, bytePos)
		bytePos += size
	}

	// If there are trailing incomplete bytes and not at EOF, we'll handle them
	trailingIncomplete := bytePos < len(src)

	runePos := 0
	for runePos < len(runes) {
		r := runes[runePos]

		// Check global filter — pass through unfiltered chars
		if rt.ruleSet.filter != nil && !rt.ruleSet.filter.Contains(r) {
			outSize := utf8.RuneLen(r)
			if nDst+outSize > len(dst) {
				err = transform.ErrShortDst
				break
			}
			utf8.EncodeRune(dst[nDst:], r)
			nDst += outSize
			rt.pushBefore(r)
			runePos++
			continue
		}

		remaining := len(runes) - runePos
		matched := false
		needMore := false

		for _, rule := range rt.ruleSet.rules {
			// Check before-context
			if !rt.matchBeforeCtx(rule.beforeCtx) {
				continue
			}

			// Try main match
			consumed := matchPattern(rule.match, runes, runePos)
			if consumed < 0 {
				// Check if we might match with more input
				if !atEOF && remaining < rule.match.maxLen {
					if couldMatchMore(rule.match, runes, runePos) {
						needMore = true
						break
					}
				}
				continue
			}

			// Check after-context
			if len(rule.afterCtx.elements) > 0 {
				afterStart := runePos + consumed
				if matchPattern(rule.afterCtx, runes, afterStart) < 0 {
					// After-context failed; might succeed with more input
					if !atEOF && afterStart >= len(runes) && rule.afterCtx.minLen > 0 {
						needMore = true
						break
					}
					continue
				}
			}

			// Rule matched — write replacement
			replBytes := encodeRunes(rule.replacement)
			if nDst+len(replBytes) > len(dst) {
				err = transform.ErrShortDst
				goto done
			}
			copy(dst[nDst:], replBytes)
			nDst += len(replBytes)

			for _, rr := range rule.replacement {
				rt.pushBefore(rr)
			}

			runePos += consumed
			matched = true
			break
		}

		if needMore {
			err = transform.ErrShortSrc
			break
		}

		if !matched {
			// Pass through
			outSize := utf8.RuneLen(r)
			if nDst+outSize > len(dst) {
				err = transform.ErrShortDst
				break
			}
			utf8.EncodeRune(dst[nDst:], r)
			nDst += outSize
			rt.pushBefore(r)
			runePos++
		}
	}

done:
	// Calculate nSrc: how many bytes from src were consumed
	if runePos >= len(runes) {
		nSrc = bytePos // all decoded bytes consumed
		// If there are trailing incomplete bytes and not at EOF, signal need for more
		if trailingIncomplete && !atEOF && err == nil {
			err = transform.ErrShortSrc
		}
	} else {
		nSrc = byteOffsets[runePos]
	}

	return
}

func (rt *RuleTransformer) Reset() {
	rt.beforeBuf = rt.beforeBuf[:0]
}

func (rt *RuleTransformer) pushBefore(r rune) {
	if rt.maxBefore == 0 {
		return
	}
	if len(rt.beforeBuf) >= rt.maxBefore {
		copy(rt.beforeBuf, rt.beforeBuf[1:])
		rt.beforeBuf = rt.beforeBuf[:rt.maxBefore-1]
	}
	rt.beforeBuf = append(rt.beforeBuf, r)
}

func (rt *RuleTransformer) matchBeforeCtx(ctx CompiledPattern) bool {
	if len(ctx.elements) == 0 {
		return true
	}
	if len(rt.beforeBuf) < ctx.minLen {
		return false
	}
	// Try matching the context against the tail of beforeBuf
	start := len(rt.beforeBuf) - ctx.minLen
	return matchPattern(ctx, rt.beforeBuf, start) >= 0
}

// matchPattern tries to match pattern against runes starting at pos.
// Returns the number of runes consumed, or -1 if no match.
func matchPattern(pat CompiledPattern, runes []rune, pos int) int {
	if len(pat.elements) == 0 {
		return 0
	}
	consumed := 0
	runeIdx := pos
	for _, elem := range pat.elements {
		switch elem.kind {
		case elemLiteral:
			for _, lr := range elem.literal {
				if runeIdx >= len(runes) || runes[runeIdx] != lr {
					return -1
				}
				runeIdx++
				consumed++
			}
		case elemSet:
			if runeIdx >= len(runes) || !elem.set.Contains(runes[runeIdx]) {
				return -1
			}
			runeIdx++
			consumed++
		}
	}
	return consumed
}

// couldMatchMore checks if a pattern could potentially match if more runes were available.
func couldMatchMore(pat CompiledPattern, runes []rune, pos int) bool {
	runeIdx := pos
	for _, elem := range pat.elements {
		switch elem.kind {
		case elemLiteral:
			for _, lr := range elem.literal {
				if runeIdx >= len(runes) {
					return true // ran out of runes, could match with more
				}
				if runes[runeIdx] != lr {
					return false
				}
				runeIdx++
			}
		case elemSet:
			if runeIdx >= len(runes) {
				return true
			}
			if !elem.set.Contains(runes[runeIdx]) {
				return false
			}
			runeIdx++
		}
	}
	return false // fully matched, doesn't need more
}

func encodeRunes(runes []rune) []byte {
	buf := make([]byte, 0, len(runes)*3)
	tmp := make([]byte, utf8.UTFMax)
	for _, r := range runes {
		n := utf8.EncodeRune(tmp, r)
		buf = append(buf, tmp[:n]...)
	}
	return buf
}
