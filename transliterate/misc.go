package transliterate

import (
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

func init() {
	Register("Any-Null", func() transform.Transformer { return transform.Nop })

	Register("Any-Remove", func() transform.Transformer {
		return runes.Remove(runes.Predicate(func(r rune) bool { return true }))
	})

	// Latin-ASCII strips diacritics: NFD decompose → remove combining marks → NFC recompose
	Register("Latin-ASCII", func() transform.Transformer {
		return transform.Chain(
			norm.NFD,
			runes.Remove(runes.In(unicode.Mn)),
			norm.NFC,
		)
	})
}
