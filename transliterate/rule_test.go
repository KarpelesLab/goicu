package transliterate

import (
	"os"
	"testing"
)

func TestUnicodeSet_Simple(t *testing.T) {
	tests := []struct {
		expr string
		in   rune
		want bool
	}{
		{"[a-z]", 'a', true},
		{"[a-z]", 'z', true},
		{"[a-z]", 'A', false},
		{"[a-z]", '0', false},
		{"[abc]", 'b', true},
		{"[abc]", 'd', false},
		{"[^a-z]", 'A', true},
		{"[^a-z]", 'a', false},
	}
	for _, tt := range tests {
		s, err := ParseUnicodeSet(tt.expr)
		if err != nil {
			t.Errorf("ParseUnicodeSet(%q) error: %v", tt.expr, err)
			continue
		}
		got := s.Contains(tt.in)
		if got != tt.want {
			t.Errorf("ParseUnicodeSet(%q).Contains(%q) = %v, want %v", tt.expr, string(tt.in), got, tt.want)
		}
	}
}

func TestUnicodeSet_Properties(t *testing.T) {
	tests := []struct {
		expr string
		in   rune
		want bool
	}{
		{"[:Latin:]", 'a', true},
		{"[:Latin:]", 'α', false},
		{"[:Hiragana:]", 'あ', true},
		{"[:Hiragana:]", 'ア', false},
		{"[:Katakana:]", 'ア', true},
		{"[:^Latin:]", 'α', true},
		{"[:^Latin:]", 'a', false},
	}
	for _, tt := range tests {
		s, err := ParseUnicodeSet(tt.expr)
		if err != nil {
			t.Errorf("ParseUnicodeSet(%q) error: %v", tt.expr, err)
			continue
		}
		got := s.Contains(tt.in)
		if got != tt.want {
			t.Errorf("ParseUnicodeSet(%q).Contains(%q) = %v, want %v", tt.expr, string(tt.in), got, tt.want)
		}
	}
}

func TestUnicodeSet_Escapes(t *testing.T) {
	s, err := ParseUnicodeSet(`[\u0041-\u005A]`) // A-Z
	if err != nil {
		t.Fatal(err)
	}
	if !s.Contains('A') {
		t.Error("expected A to be in [\\u0041-\\u005A]")
	}
	if !s.Contains('Z') {
		t.Error("expected Z to be in [\\u0041-\\u005A]")
	}
	if s.Contains('a') {
		t.Error("expected a to NOT be in [\\u0041-\\u005A]")
	}
}

func TestUnicodeSet_Nested(t *testing.T) {
	s, err := ParseUnicodeSet("[[:Latin:][:Hiragana:]]")
	if err != nil {
		t.Fatal(err)
	}
	if !s.Contains('a') {
		t.Error("expected 'a' in union of Latin and Hiragana")
	}
	if !s.Contains('あ') {
		t.Error("expected 'あ' in union of Latin and Hiragana")
	}
	if s.Contains('ア') {
		t.Error("expected 'ア' NOT in union of Latin and Hiragana")
	}
}

func TestUnicodeSet_Difference(t *testing.T) {
	s, err := ParseUnicodeSet("[[:Latin:]-[aeiou]]")
	if err != nil {
		t.Fatal(err)
	}
	if !s.Contains('b') {
		t.Error("expected 'b' in Latin minus vowels")
	}
	if s.Contains('a') {
		t.Error("expected 'a' NOT in Latin minus vowels")
	}
}

func TestParseRules_SimpleForward(t *testing.T) {
	rules, err := ParseRules(`a → b ;`)
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}
	if rules[0].Type != RuleConversion {
		t.Errorf("expected RuleConversion, got %d", rules[0].Type)
	}
	if rules[0].Direction != DirForward {
		t.Errorf("expected DirForward, got %d", rules[0].Direction)
	}
}

func TestParseRules_Bidirectional(t *testing.T) {
	rules, err := ParseRules(`あ ↔ ア ;`)
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}
	if rules[0].Direction != DirBoth {
		t.Errorf("expected DirBoth, got %d", rules[0].Direction)
	}
}

func TestParseRules_ASCIIOperators(t *testing.T) {
	rules, err := ParseRules(`a > b ; c < d ; e <> f ;`)
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 3 {
		t.Fatalf("expected 3 rules, got %d", len(rules))
	}
	if rules[0].Direction != DirForward {
		t.Errorf("rule 0: expected DirForward")
	}
	if rules[1].Direction != DirReverse {
		t.Errorf("rule 1: expected DirReverse")
	}
	if rules[2].Direction != DirBoth {
		t.Errorf("rule 2: expected DirBoth")
	}
}

func TestParseRules_Variable(t *testing.T) {
	rules, err := ParseRules(`$vowel = [aeiou] ; $vowel → X ;`)
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(rules))
	}
	if rules[0].Type != RuleVariable {
		t.Errorf("rule 0: expected RuleVariable")
	}
	if rules[0].VarName != "vowel" {
		t.Errorf("rule 0: expected var name 'vowel', got %q", rules[0].VarName)
	}
}

func TestParseRules_Directive(t *testing.T) {
	rules, err := ParseRules(`:: NFD ;`)
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}
	if rules[0].Type != RuleDirective {
		t.Errorf("expected RuleDirective, got %d", rules[0].Type)
	}
	if rules[0].DirectiveFwd != "NFD" {
		t.Errorf("expected directive 'NFD', got %q", rules[0].DirectiveFwd)
	}
}

func TestParseRules_DirectiveWithReverse(t *testing.T) {
	rules, err := ParseRules(`:: NFKC (NFC) ;`)
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}
	if rules[0].DirectiveFwd != "NFKC" {
		t.Errorf("forward: expected 'NFKC', got %q", rules[0].DirectiveFwd)
	}
	if rules[0].DirectiveRev != "NFC" {
		t.Errorf("reverse: expected 'NFC', got %q", rules[0].DirectiveRev)
	}
}

func TestParseRules_Comments(t *testing.T) {
	rules, err := ParseRules(`
		# This is a comment
		a → b ; # inline comment
		// Another comment style
		c → d ;
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(rules))
	}
}

func TestParseRules_Context(t *testing.T) {
	rules, err := ParseRules(`x { a } y → b ;`)
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}
	r := rules[0]
	if len(r.BeforeContext) == 0 {
		t.Error("expected non-empty before context")
	}
	if len(r.AfterContext) == 0 {
		t.Error("expected non-empty after context")
	}
}

func TestNewFromRules_SimpleMapping(t *testing.T) {
	tr, err := NewFromRules("test", `a → x ; b → y ; c → z ;`, Forward)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		in, want string
	}{
		{"abc", "xyz"},
		{"aabbcc", "xxyyzz"},
		{"def", "def"}, // unmapped chars pass through
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

func TestNewFromRules_Bidirectional(t *testing.T) {
	rules := `あ ↔ ア ; い ↔ イ ; う ↔ ウ ;`

	fwd, err := NewFromRules("test-fwd", rules, Forward)
	if err != nil {
		t.Fatal(err)
	}
	got, err := fwd.String("あいう")
	if err != nil {
		t.Fatal(err)
	}
	if got != "アイウ" {
		t.Errorf("forward: got %q, want %q", got, "アイウ")
	}

	rev, err := NewFromRules("test-rev", rules, Reverse)
	if err != nil {
		t.Fatal(err)
	}
	got, err = rev.String("アイウ")
	if err != nil {
		t.Fatal(err)
	}
	if got != "あいう" {
		t.Errorf("reverse: got %q, want %q", got, "あいう")
	}
}

func TestNewFromRules_MultiCharPattern(t *testing.T) {
	tr, err := NewFromRules("test", `ch → X ; c → Y ;`, Forward)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		in, want string
	}{
		{"ch", "X"},
		{"c", "Y"},
		{"cha", "Xa"},
		{"ca", "Ya"},
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

func TestNewFromRules_VariableExpansion(t *testing.T) {
	tr, err := NewFromRules("test", `$vowel = [aeiou] ; $vowel → X ;`, Forward)
	if err != nil {
		t.Fatal(err)
	}
	got, err := tr.String("hello")
	if err != nil {
		t.Fatal(err)
	}
	if got != "hXllX" {
		t.Errorf("got %q, want %q", got, "hXllX")
	}
}

func TestNewFromRules_EmptyReplacement(t *testing.T) {
	tr, err := NewFromRules("test", `x → ;`, Forward)
	if err != nil {
		t.Fatal(err)
	}
	got, err := tr.String("axbxc")
	if err != nil {
		t.Fatal(err)
	}
	if got != "abc" {
		t.Errorf("got %q, want %q", got, "abc")
	}
}

func TestNewFromRules_QuotedLiteral(t *testing.T) {
	tr, err := NewFromRules("test", `'.' → X ;`, Forward)
	if err != nil {
		t.Fatal(err)
	}
	got, err := tr.String("a.b")
	if err != nil {
		t.Fatal(err)
	}
	if got != "aXb" {
		t.Errorf("got %q, want %q", got, "aXb")
	}
}

func TestNewFromRules_DirectionFiltering(t *testing.T) {
	rules := `a → X ; b ← Y ;`

	// Forward: only a→X should apply, b←Y is reverse-only
	fwd, err := NewFromRules("test", rules, Forward)
	if err != nil {
		t.Fatal(err)
	}
	got, err := fwd.String("ab")
	if err != nil {
		t.Fatal(err)
	}
	if got != "Xb" {
		t.Errorf("forward: got %q, want %q", got, "Xb")
	}

	// Reverse: only b←Y should apply (Y→b), a→X is forward-only
	rev, err := NewFromRules("test", rules, Reverse)
	if err != nil {
		t.Fatal(err)
	}
	got, err = rev.String("aY")
	if err != nil {
		t.Fatal(err)
	}
	if got != "ab" {
		t.Errorf("reverse: got %q, want %q", got, "ab")
	}
}

func TestLoadCLDRFile_HiraganaKatakana(t *testing.T) {
	path := "/tmp/cldr-test/Hiragana-Katakana.xml"
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Skip("CLDR test file not available, skipping")
	}

	err := LoadCLDRFile(path)
	if err != nil {
		t.Fatal(err)
	}

	// Test the CLDR-loaded transform
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
		{"ん", "ン"},
		{"Hello あ", "Hello ア"},
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

func TestNewFromRules_ContextRule(t *testing.T) {
	tr, err := NewFromRules("test", `x { a } y → B ;`, Forward)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		in, want string
	}{
		{"xay", "xBy"},
		{"xaz", "xaz"},
		{"zay", "zay"},
		{"a", "a"},
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

func TestLoadCLDRData_SimpleXML(t *testing.T) {
	xml := `<?xml version="1.0" encoding="UTF-8"?>
<supplementalData>
  <transforms>
    <transform source="Test" target="Out" direction="forward" alias="Test-Out">
      <tRule><![CDATA[
        a → x ;
        b → y ;
      ]]></tRule>
    </transform>
  </transforms>
</supplementalData>`

	err := LoadCLDRData([]byte(xml))
	if err != nil {
		t.Fatal(err)
	}

	tr, err := New("Test-Out")
	if err != nil {
		t.Fatal(err)
	}

	got, err := tr.String("abc")
	if err != nil {
		t.Fatal(err)
	}
	if got != "xyc" {
		t.Errorf("got %q, want %q", got, "xyc")
	}
}

func TestLoadCLDRData_BidirectionalXML(t *testing.T) {
	xml := `<?xml version="1.0" encoding="UTF-8"?>
<supplementalData>
  <transforms>
    <transform source="Src" target="Tgt" direction="both"
               alias="Src-Tgt" backwardAlias="Tgt-Src">
      <tRule><![CDATA[
        a ↔ α ;
        b ↔ β ;
      ]]></tRule>
    </transform>
  </transforms>
</supplementalData>`

	err := LoadCLDRData([]byte(xml))
	if err != nil {
		t.Fatal(err)
	}

	// Forward
	fwd, err := New("Src-Tgt")
	if err != nil {
		t.Fatal(err)
	}
	got, err := fwd.String("ab")
	if err != nil {
		t.Fatal(err)
	}
	if got != "αβ" {
		t.Errorf("forward: got %q, want %q", got, "αβ")
	}

	// Reverse
	rev, err := New("Tgt-Src")
	if err != nil {
		t.Fatal(err)
	}
	got, err = rev.String("αβ")
	if err != nil {
		t.Fatal(err)
	}
	if got != "ab" {
		t.Errorf("reverse: got %q, want %q", got, "ab")
	}
}
