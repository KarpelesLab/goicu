package breakiter

import (
	"testing"
)

func TestGrapheme_Basic(t *testing.T) {
	bi := NewGrapheme()
	bi.SetText("Hello")

	segs := bi.Segments()
	if len(segs) != 5 {
		t.Errorf("expected 5 graphemes, got %d: %v", len(segs), segs)
	}
}

func TestGrapheme_CombiningMark(t *testing.T) {
	// e + combining acute = 1 grapheme
	bi := NewGrapheme()
	bi.SetText("e\u0301")
	segs := bi.Segments()
	if len(segs) != 1 {
		t.Errorf("expected 1 grapheme for combining mark, got %d: %v", len(segs), segs)
	}
}

func TestGrapheme_Emoji(t *testing.T) {
	bi := NewGrapheme()
	bi.SetText("👨‍👩‍👧‍👦")
	segs := bi.Segments()
	if len(segs) != 1 {
		t.Errorf("expected 1 grapheme for family emoji, got %d: %v", len(segs), segs)
	}
}

func TestGrapheme_Flag(t *testing.T) {
	bi := NewGrapheme()
	bi.SetText("🇯🇵")
	segs := bi.Segments()
	if len(segs) != 1 {
		t.Errorf("expected 1 grapheme for flag, got %d: %v", len(segs), segs)
	}
}

func TestGraphemeCount(t *testing.T) {
	tests := []struct {
		in   string
		want int
	}{
		{"Hello", 5},
		{"e\u0301", 1},
		{"👨‍👩‍👧‍👦", 1},
		{"🇯🇵", 1},
		{"", 0},
		{"日本語", 3},
	}
	for _, tt := range tests {
		got := GraphemeCount(tt.in)
		if got != tt.want {
			t.Errorf("GraphemeCount(%q) = %d, want %d", tt.in, got, tt.want)
		}
	}
}

func TestWord_English(t *testing.T) {
	bi := NewWord()
	bi.SetText("Hello, world!")
	segs := bi.Segments()
	// "Hello" ", " "world" "!"
	if len(segs) < 4 {
		t.Errorf("expected at least 4 word segments, got %d: %v", len(segs), segs)
	}
}

func TestWordCount(t *testing.T) {
	tests := []struct {
		in   string
		want int
	}{
		{"Hello world", 2},
		{"Hello, world!", 2},
		{"one two three", 3},
		{"", 0},
		{"  spaces  ", 1},
	}
	for _, tt := range tests {
		got := WordCount(tt.in)
		if got != tt.want {
			t.Errorf("WordCount(%q) = %d, want %d", tt.in, got, tt.want)
		}
	}
}

func TestSplitWords(t *testing.T) {
	segs := SplitWords("Hello world")
	found := false
	for _, s := range segs {
		if s == "Hello" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'Hello' in segments: %v", segs)
	}
}

func TestSentence(t *testing.T) {
	bi := NewSentence()
	bi.SetText("Hello. World. Goodbye.")
	segs := bi.Segments()
	if len(segs) != 3 {
		t.Errorf("expected 3 sentences, got %d: %v", len(segs), segs)
	}
}

func TestSplitSentences(t *testing.T) {
	segs := SplitSentences("First. Second. Third.")
	if len(segs) != 3 {
		t.Errorf("expected 3 sentences, got %d: %v", len(segs), segs)
	}
}

func TestLine(t *testing.T) {
	bi := NewLine()
	bi.SetText("Hello world")
	segs := bi.Segments()
	if len(segs) < 2 {
		t.Errorf("expected at least 2 line segments, got %d: %v", len(segs), segs)
	}
}

func TestLine_MandatoryBreak(t *testing.T) {
	bi := NewLine()
	bi.SetText("Line1\nLine2")
	bi.First()
	bi.Next()
	if !bi.IsMandatory() {
		t.Error("expected mandatory break at newline")
	}
}

func TestPositional_Next(t *testing.T) {
	bi := NewWord()
	bi.SetText("Hello world")

	pos := bi.First()
	if pos != 0 {
		t.Errorf("First() = %d, want 0", pos)
	}

	positions := []int{pos}
	for {
		pos = bi.Next()
		if pos == Done {
			break
		}
		positions = append(positions, pos)
	}

	// Should end at len("Hello world") = 11
	if positions[len(positions)-1] != 11 {
		t.Errorf("last boundary = %d, want 11", positions[len(positions)-1])
	}
}

func TestPositional_Previous(t *testing.T) {
	bi := NewGrapheme()
	bi.SetText("abc")

	bi.Last() // move to end (3)
	pos := bi.Previous()
	if pos != 2 {
		t.Errorf("Previous from last = %d, want 2", pos)
	}
	pos = bi.Previous()
	if pos != 1 {
		t.Errorf("Previous again = %d, want 1", pos)
	}
	pos = bi.Previous()
	if pos != 0 {
		t.Errorf("Previous again = %d, want 0", pos)
	}
	pos = bi.Previous()
	if pos != Done {
		t.Errorf("Previous at start = %d, want Done", pos)
	}
}

func TestPositional_Following(t *testing.T) {
	bi := NewGrapheme()
	bi.SetText("abcde")

	pos := bi.Following(2)
	if pos != 3 {
		t.Errorf("Following(2) = %d, want 3", pos)
	}
}

func TestPositional_Preceding(t *testing.T) {
	bi := NewGrapheme()
	bi.SetText("abcde")

	pos := bi.Preceding(3)
	if pos != 2 {
		t.Errorf("Preceding(3) = %d, want 2", pos)
	}
}

func TestIsBoundary(t *testing.T) {
	bi := NewGrapheme()
	bi.SetText("abc")

	// Boundaries should be at 0, 1, 2, 3
	for i := 0; i <= 3; i++ {
		if !bi.IsBoundary(i) {
			t.Errorf("IsBoundary(%d) = false, want true", i)
		}
	}
	if bi.IsBoundary(4) {
		t.Error("IsBoundary(4) = true for 3-byte string")
	}
}

func TestIsBoundary_CombiningMark(t *testing.T) {
	bi := NewGrapheme()
	bi.SetText("e\u0301") // e + combining acute
	// Should have boundaries at 0 and 3 (end), NOT at 1 (between e and combining)
	if !bi.IsBoundary(0) {
		t.Error("IsBoundary(0) = false")
	}
	if bi.IsBoundary(1) {
		t.Error("IsBoundary(1) = true, should be false (inside grapheme cluster)")
	}
}

func TestBounds(t *testing.T) {
	bi := NewWord()
	bi.SetText("Hello world")
	bi.First()
	bi.Next()
	start, end := bi.Bounds()
	segment := bi.Segment()
	if segment != "Hello" {
		t.Errorf("first segment = %q, want %q (bounds: %d-%d)", segment, "Hello", start, end)
	}
}

func TestEmpty(t *testing.T) {
	bi := NewGrapheme()
	bi.SetText("")
	if bi.First() != 0 {
		t.Error("First() on empty should be 0")
	}
	if bi.Next() != Done {
		t.Error("Next() on empty should be Done")
	}
	if GraphemeCount("") != 0 {
		t.Error("GraphemeCount of empty should be 0")
	}
}

func TestSingleChar(t *testing.T) {
	bi := NewGrapheme()
	bi.SetText("x")
	segs := bi.Segments()
	if len(segs) != 1 || segs[0] != "x" {
		t.Errorf("single char segments = %v, want [x]", segs)
	}
}

func TestBoundaries(t *testing.T) {
	bi := NewGrapheme()
	bi.SetText("abc")
	bounds := bi.Boundaries()
	want := []int{0, 1, 2, 3}
	if len(bounds) != len(want) {
		t.Fatalf("boundaries = %v, want %v", bounds, want)
	}
	for i, b := range bounds {
		if b != want[i] {
			t.Errorf("boundary[%d] = %d, want %d", i, b, want[i])
		}
	}
}

func TestJapanese(t *testing.T) {
	// Each Japanese character should be its own grapheme cluster
	bi := NewGrapheme()
	bi.SetText("日本語")
	segs := bi.Segments()
	if len(segs) != 3 {
		t.Errorf("expected 3 graphemes for Japanese, got %d: %v", len(segs), segs)
	}
}
