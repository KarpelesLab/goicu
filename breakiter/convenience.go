package breakiter

import "github.com/rivo/uniseg"

// Segment returns the text of the current segment (between the previous and current boundaries).
func (bi *BreakIterator) Segment() string {
	start, end := bi.Bounds()
	if start < 0 || end < 0 || start >= len(bi.text) {
		return ""
	}
	return bi.text[start:end]
}

// Bounds returns the byte range [start, end) of the current segment.
func (bi *BreakIterator) Bounds() (start, end int) {
	if len(bi.boundaries) == 0 || bi.idx <= 0 {
		return 0, 0
	}
	if bi.idx >= len(bi.boundaries) {
		last := bi.boundaries[len(bi.boundaries)-1]
		return last, last
	}
	return bi.boundaries[bi.idx-1], bi.boundaries[bi.idx]
}

// Boundaries returns all boundary positions as a slice of byte offsets.
func (bi *BreakIterator) Boundaries() []int {
	result := make([]int, len(bi.boundaries))
	copy(result, bi.boundaries)
	return result
}

// Segments returns all segments as a slice of strings.
func (bi *BreakIterator) Segments() []string {
	if len(bi.boundaries) <= 1 {
		return nil
	}
	segments := make([]string, 0, len(bi.boundaries)-1)
	for i := 1; i < len(bi.boundaries); i++ {
		segments = append(segments, bi.text[bi.boundaries[i-1]:bi.boundaries[i]])
	}
	return segments
}

// IsMandatory reports whether the current boundary is a mandatory line break.
// Only meaningful for Line type iterators; always returns false for other types.
func (bi *BreakIterator) IsMandatory() bool {
	if bi.typ != Line || bi.mandatory == nil {
		return false
	}
	if bi.idx < 0 || bi.idx >= len(bi.mandatory) {
		return false
	}
	return bi.mandatory[bi.idx]
}

// GraphemeCount returns the number of grapheme clusters in s.
func GraphemeCount(s string) int {
	return uniseg.GraphemeClusterCount(s)
}

// WordCount returns the number of word-like segments in s
// (excludes whitespace and punctuation-only segments).
func WordCount(s string) int {
	count := 0
	remaining := s
	state := -1
	for len(remaining) > 0 {
		word, rest, newState := uniseg.FirstWordInString(remaining, state)
		if isWordContent(word) {
			count++
		}
		remaining = rest
		state = newState
	}
	return count
}

// SplitWords splits s into word segments, including spaces and punctuation as separate segments.
func SplitWords(s string) []string {
	bi := NewWord()
	bi.SetText(s)
	return bi.Segments()
}

// SplitSentences splits s into sentences.
func SplitSentences(s string) []string {
	bi := NewSentence()
	bi.SetText(s)
	return bi.Segments()
}

// SplitLines splits s into line segments at line break opportunities.
func SplitLines(s string) []string {
	bi := NewLine()
	bi.SetText(s)
	return bi.Segments()
}

func isWordContent(word string) bool {
	for _, r := range word {
		if r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' ||
			r >= '0' && r <= '9' || r >= 0x80 {
			return true
		}
	}
	return false
}
