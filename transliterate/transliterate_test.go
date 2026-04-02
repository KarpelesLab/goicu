package transliterate

import (
	"io"
	"strings"
	"testing"

	"golang.org/x/text/transform"
)

func TestNew_ValidIDs(t *testing.T) {
	for _, id := range IDs() {
		tr, err := New(id)
		if err != nil {
			t.Errorf("New(%q) returned error: %v", id, err)
			continue
		}
		if tr.ID() != id {
			t.Errorf("New(%q).ID() = %q", id, tr.ID())
		}
	}
}

func TestNew_CaseInsensitive(t *testing.T) {
	tr, err := New("fullwidth-halfwidth")
	if err != nil {
		t.Fatal(err)
	}
	if tr == nil {
		t.Fatal("expected non-nil transliterator")
	}
}

func TestNew_InvalidID(t *testing.T) {
	_, err := New("Bogus-Transform")
	if err == nil {
		t.Fatal("expected error for unknown ID")
	}
}

func TestNew_EmptyID(t *testing.T) {
	_, err := New("")
	if err == nil {
		t.Fatal("expected error for empty ID")
	}
}

func TestNew_CompoundID(t *testing.T) {
	tr, err := New("Hiragana-Katakana;Fullwidth-Halfwidth")
	if err != nil {
		t.Fatal(err)
	}
	// ひらがな → カタカナ → halfwidth katakana
	got, err := tr.String("あいう")
	if err != nil {
		t.Fatal(err)
	}
	want := "ｱｲｳ"
	if got != want {
		t.Errorf("compound transform: got %q, want %q", got, want)
	}
}

func TestNew_AnyPrefix(t *testing.T) {
	// "NFC" without "Any-" prefix should work via the "any-" fallback
	tr, err := New("Lower")
	if err != nil {
		t.Fatal(err)
	}
	got, err := tr.String("HELLO")
	if err != nil {
		t.Fatal(err)
	}
	if got != "hello" {
		t.Errorf("got %q, want %q", got, "hello")
	}
}

func TestFullwidthToHalfwidth(t *testing.T) {
	tr, err := New("Fullwidth-Halfwidth")
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		in, want string
	}{
		{"Ｈｅｌｌｏ", "Hello"},
		{"１２３", "123"},
		{"Hello", "Hello"},
		{"", ""},
	}

	for _, tt := range tests {
		got, err := tr.String(tt.in)
		if err != nil {
			t.Errorf("String(%q) error: %v", tt.in, err)
			continue
		}
		if got != tt.want {
			t.Errorf("String(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestHalfwidthToFullwidth(t *testing.T) {
	tr, err := New("Halfwidth-Fullwidth")
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		in, want string
	}{
		{"Hello", "Ｈｅｌｌｏ"},
		{"123", "１２３"},
		{"", ""},
	}

	for _, tt := range tests {
		got, err := tr.String(tt.in)
		if err != nil {
			t.Errorf("String(%q) error: %v", tt.in, err)
			continue
		}
		if got != tt.want {
			t.Errorf("String(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestHiraganaToKatakana(t *testing.T) {
	tr, err := New("Hiragana-Katakana")
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		in, want string
	}{
		{"あいうえお", "アイウエオ"},
		{"がぎぐげご", "ガギグゲゴ"},
		{"ぱぴぷぺぽ", "パピプペポ"},
		{"ゝゞ", "ヽヾ"},
		{"Hello あ", "Hello ア"},
		{"アイウ", "アイウ"}, // katakana unchanged
		{"", ""},
	}

	for _, tt := range tests {
		got, err := tr.String(tt.in)
		if err != nil {
			t.Errorf("String(%q) error: %v", tt.in, err)
			continue
		}
		if got != tt.want {
			t.Errorf("String(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestKatakanaToHiragana(t *testing.T) {
	tr, err := New("Katakana-Hiragana")
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		in, want string
	}{
		{"アイウエオ", "あいうえお"},
		{"ガギグゲゴ", "がぎぐげご"},
		{"ヽヾ", "ゝゞ"},
		{"Hello ア", "Hello あ"},
		{"あいう", "あいう"}, // hiragana unchanged
	}

	for _, tt := range tests {
		got, err := tr.String(tt.in)
		if err != nil {
			t.Errorf("String(%q) error: %v", tt.in, err)
			continue
		}
		if got != tt.want {
			t.Errorf("String(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestNormalization(t *testing.T) {
	tr, err := New("Any-NFC")
	if err != nil {
		t.Fatal(err)
	}
	// NFD form of é is e + combining acute (U+0065 U+0301)
	// NFC should compose it to é (U+00E9)
	in := "e\u0301"
	got, err := tr.String(in)
	if err != nil {
		t.Fatal(err)
	}
	want := "\u00E9"
	if got != want {
		t.Errorf("NFC(%q) = %q, want %q", in, got, want)
	}
}

func TestCaseTransforms(t *testing.T) {
	tests := []struct {
		id, in, want string
	}{
		{"Any-Upper", "hello", "HELLO"},
		{"Any-Lower", "HELLO", "hello"},
		{"Any-Title", "hello world", "Hello World"},
	}
	for _, tt := range tests {
		tr, err := New(tt.id)
		if err != nil {
			t.Fatalf("New(%q) error: %v", tt.id, err)
		}
		got, err := tr.String(tt.in)
		if err != nil {
			t.Errorf("%s.String(%q) error: %v", tt.id, tt.in, err)
			continue
		}
		if got != tt.want {
			t.Errorf("%s.String(%q) = %q, want %q", tt.id, tt.in, got, tt.want)
		}
	}
}

func TestLatinASCII(t *testing.T) {
	tr, err := New("Latin-ASCII")
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		in, want string
	}{
		{"résumé", "resume"},
		{"café", "cafe"},
		{"naïve", "naive"},
		{"Hello", "Hello"},
	}
	for _, tt := range tests {
		got, err := tr.String(tt.in)
		if err != nil {
			t.Errorf("String(%q) error: %v", tt.in, err)
			continue
		}
		if got != tt.want {
			t.Errorf("String(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestNullTransform(t *testing.T) {
	tr, err := New("Any-Null")
	if err != nil {
		t.Fatal(err)
	}
	in := "Hello 世界"
	got, err := tr.String(in)
	if err != nil {
		t.Fatal(err)
	}
	if got != in {
		t.Errorf("Null: got %q, want %q", got, in)
	}
}

func TestRemoveTransform(t *testing.T) {
	tr, err := New("Any-Remove")
	if err != nil {
		t.Fatal(err)
	}
	got, err := tr.String("Hello 世界")
	if err != nil {
		t.Fatal(err)
	}
	if got != "" {
		t.Errorf("Remove: got %q, want empty", got)
	}
}

func TestTransformerInterface(t *testing.T) {
	// Verify Transliterator satisfies transform.Transformer
	var _ transform.Transformer = (*Transliterator)(nil)
}

func TestStreaming(t *testing.T) {
	tr, err := New("Hiragana-Katakana")
	if err != nil {
		t.Fatal(err)
	}

	reader := transform.NewReader(strings.NewReader("あいうえお"), tr)
	buf, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	got := string(buf)
	want := "アイウエオ"
	if got != want {
		t.Errorf("streaming: got %q, want %q", got, want)
	}
}

func TestIDs(t *testing.T) {
	ids := IDs()
	if len(ids) == 0 {
		t.Fatal("IDs() returned empty list")
	}
	// Check some expected IDs are present
	expected := []string{"Fullwidth-Halfwidth", "Halfwidth-Fullwidth", "Hiragana-Katakana", "Katakana-Hiragana"}
	for _, want := range expected {
		found := false
		for _, id := range ids {
			if id == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("IDs() missing %q", want)
		}
	}
}
