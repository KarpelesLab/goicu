// Package transliterate provides ICU-compatible text transliteration.
//
// It supports ICU-style transliterator IDs such as "Fullwidth-Halfwidth",
// "Hiragana-Katakana", or compound IDs like "Hiragana-Katakana;Fullwidth-Halfwidth".
//
// Each Transliterator implements transform.Transformer from golang.org/x/text,
// allowing it to be used with transform.NewReader, transform.NewWriter, and
// transform.Chain.
package transliterate

import (
	"errors"
	"fmt"
	"strings"

	"golang.org/x/text/transform"
)

// ErrUnknownTransliterator is returned when a transliterator ID is not found.
var ErrUnknownTransliterator = errors.New("transliterate: unknown transliterator")

// Transliterator performs text transliteration using ICU-style transform IDs.
// It implements transform.Transformer for streaming use.
type Transliterator struct {
	id string
	t  transform.Transformer
}

// New creates a Transliterator from an ICU-style transliterator ID.
// Supports compound IDs separated by ";" (e.g. "Hiragana-Katakana;Fullwidth-Halfwidth").
func New(id string) (*Transliterator, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("transliterate: empty ID")
	}

	components := splitCompound(id)
	if len(components) == 0 {
		return nil, fmt.Errorf("transliterate: empty ID")
	}

	transformers := make([]transform.Transformer, 0, len(components))
	for _, comp := range components {
		factory, err := lookup(comp)
		if err != nil {
			return nil, fmt.Errorf("transliterate: unknown transform %q in ID %q", comp, id)
		}
		transformers = append(transformers, factory())
	}

	var t transform.Transformer
	if len(transformers) == 1 {
		t = transformers[0]
	} else {
		t = transform.Chain(transformers...)
	}

	return &Transliterator{id: id, t: t}, nil
}

// String applies the transliteration to the input string.
func (tr *Transliterator) String(s string) (string, error) {
	result, _, err := transform.String(tr.t, s)
	return result, err
}

// Transform implements transform.Transformer.
func (tr *Transliterator) Transform(dst, src []byte, atEOF bool) (nDst, nSrc int, err error) {
	return tr.t.Transform(dst, src, atEOF)
}

// Reset implements transform.Transformer.
func (tr *Transliterator) Reset() {
	tr.t.Reset()
}

// ID returns the transliterator's ICU-style identifier.
func (tr *Transliterator) ID() string {
	return tr.id
}
