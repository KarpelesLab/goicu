// Package breakiter provides ICU-compatible text segmentation (break iteration).
//
// It supports four types of boundary detection following the Unicode standards:
//   - Grapheme: user-perceived character boundaries (UAX #29)
//   - Word: word boundaries for selection and cursor movement (UAX #29)
//   - Sentence: sentence boundaries (UAX #29)
//   - Line: line break opportunities for text wrapping (UAX #14)
package breakiter

import (
	"sort"

	"github.com/rivo/uniseg"
)

// Type selects the kind of boundary detection.
type Type int

const (
	Grapheme Type = iota // User-perceived characters (UAX #29)
	Word                 // Word boundaries (UAX #29)
	Sentence             // Sentence boundaries (UAX #29)
	Line                 // Line break opportunities (UAX #14)
)

// Done is returned by positional methods when no further boundary exists.
const Done = -1

// BreakIterator iterates over text boundaries of a given Type.
// Not safe for concurrent use.
type BreakIterator struct {
	typ        Type
	text       string
	boundaries []int  // sorted byte offsets of all boundaries (includes 0 and len(text))
	mandatory  []bool // for Line type: whether each boundary is a mandatory break
	idx        int    // current index into boundaries
}

// New creates a BreakIterator of the given type.
func New(t Type) *BreakIterator {
	return &BreakIterator{typ: t}
}

// NewGrapheme creates an iterator for grapheme cluster boundaries.
func NewGrapheme() *BreakIterator { return New(Grapheme) }

// NewWord creates an iterator for word boundaries.
func NewWord() *BreakIterator { return New(Word) }

// NewSentence creates an iterator for sentence boundaries.
func NewSentence() *BreakIterator { return New(Sentence) }

// NewLine creates an iterator for line break opportunities.
func NewLine() *BreakIterator { return New(Line) }

// SetText sets the text to iterate over and resets the iterator to the first boundary.
func (bi *BreakIterator) SetText(s string) {
	bi.text = s
	bi.idx = 0
	bi.boundaries = nil
	bi.mandatory = nil
	bi.computeBoundaries()
}

// Text returns the text being iterated.
func (bi *BreakIterator) Text() string {
	return bi.text
}

// First moves the iterator to the first boundary (position 0) and returns it.
func (bi *BreakIterator) First() int {
	bi.idx = 0
	if len(bi.boundaries) == 0 {
		return 0
	}
	return bi.boundaries[0]
}

// Last moves the iterator to the last boundary and returns it.
func (bi *BreakIterator) Last() int {
	if len(bi.boundaries) == 0 {
		return 0
	}
	bi.idx = len(bi.boundaries) - 1
	return bi.boundaries[bi.idx]
}

// Next advances to the next boundary and returns it, or Done if exhausted.
func (bi *BreakIterator) Next() int {
	if bi.idx+1 >= len(bi.boundaries) {
		return Done
	}
	bi.idx++
	return bi.boundaries[bi.idx]
}

// Previous moves to the previous boundary and returns it, or Done if at the beginning.
func (bi *BreakIterator) Previous() int {
	if bi.idx <= 0 {
		return Done
	}
	bi.idx--
	return bi.boundaries[bi.idx]
}

// Pos returns the current boundary position.
func (bi *BreakIterator) Pos() int {
	if len(bi.boundaries) == 0 {
		return 0
	}
	if bi.idx < 0 {
		return 0
	}
	if bi.idx >= len(bi.boundaries) {
		return bi.boundaries[len(bi.boundaries)-1]
	}
	return bi.boundaries[bi.idx]
}

// Following returns the first boundary strictly after offset, or Done.
func (bi *BreakIterator) Following(offset int) int {
	i := sort.SearchInts(bi.boundaries, offset+1)
	if i >= len(bi.boundaries) {
		return Done
	}
	bi.idx = i
	return bi.boundaries[i]
}

// Preceding returns the first boundary strictly before offset, or Done.
func (bi *BreakIterator) Preceding(offset int) int {
	i := sort.SearchInts(bi.boundaries, offset)
	i-- // we want strictly less than offset
	if i < 0 {
		return Done
	}
	bi.idx = i
	return bi.boundaries[i]
}

// IsBoundary reports whether offset is a boundary position.
func (bi *BreakIterator) IsBoundary(offset int) bool {
	i := sort.SearchInts(bi.boundaries, offset)
	return i < len(bi.boundaries) && bi.boundaries[i] == offset
}

func (bi *BreakIterator) computeBoundaries() {
	if bi.text == "" {
		bi.boundaries = []int{0}
		return
	}

	switch bi.typ {
	case Grapheme:
		bi.computeGrapheme()
	case Word:
		bi.computeWord()
	case Sentence:
		bi.computeSentence()
	case Line:
		bi.computeLine()
	}
}

func (bi *BreakIterator) computeGrapheme() {
	bi.boundaries = []int{0}
	remaining := bi.text
	offset := 0
	state := -1
	for len(remaining) > 0 {
		cluster, rest, _, newState := uniseg.FirstGraphemeClusterInString(remaining, state)
		offset += len(cluster)
		bi.boundaries = append(bi.boundaries, offset)
		remaining = rest
		state = newState
	}
}

func (bi *BreakIterator) computeWord() {
	bi.boundaries = []int{0}
	remaining := bi.text
	offset := 0
	state := -1
	for len(remaining) > 0 {
		word, rest, newState := uniseg.FirstWordInString(remaining, state)
		offset += len(word)
		bi.boundaries = append(bi.boundaries, offset)
		remaining = rest
		state = newState
	}
}

func (bi *BreakIterator) computeSentence() {
	bi.boundaries = []int{0}
	remaining := bi.text
	offset := 0
	state := -1
	for len(remaining) > 0 {
		sentence, rest, newState := uniseg.FirstSentenceInString(remaining, state)
		offset += len(sentence)
		bi.boundaries = append(bi.boundaries, offset)
		remaining = rest
		state = newState
	}
}

func (bi *BreakIterator) computeLine() {
	bi.boundaries = []int{0}
	bi.mandatory = []bool{false}
	remaining := bi.text
	offset := 0
	state := -1
	for len(remaining) > 0 {
		segment, rest, mustBreak, newState := uniseg.FirstLineSegmentInString(remaining, state)
		offset += len(segment)
		bi.boundaries = append(bi.boundaries, offset)
		bi.mandatory = append(bi.mandatory, mustBreak)
		remaining = rest
		state = newState
	}
}
