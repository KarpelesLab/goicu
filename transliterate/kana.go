package transliterate

import (
	"unicode/utf8"

	"golang.org/x/text/transform"
)

const kanaOffset = 0x60

func init() {
	RegisterPair(
		"Hiragana-Katakana", func() transform.Transformer { return &hiraganaToKatakana{} },
		"Katakana-Hiragana", func() transform.Transformer { return &katakanaToHiragana{} },
	)
}

type hiraganaToKatakana struct{}

func (hiraganaToKatakana) Transform(dst, src []byte, atEOF bool) (nDst, nSrc int, err error) {
	for nSrc < len(src) {
		if !utf8.FullRune(src[nSrc:]) && !atEOF {
			err = transform.ErrShortSrc
			return
		}
		r, size := utf8.DecodeRune(src[nSrc:])
		mapped := mapHiraToKata(r)
		outSize := utf8.RuneLen(mapped)
		if nDst+outSize > len(dst) {
			err = transform.ErrShortDst
			return
		}
		utf8.EncodeRune(dst[nDst:], mapped)
		nDst += outSize
		nSrc += size
	}
	return
}

func (hiraganaToKatakana) Reset() {}

type katakanaToHiragana struct{}

func (katakanaToHiragana) Transform(dst, src []byte, atEOF bool) (nDst, nSrc int, err error) {
	for nSrc < len(src) {
		if !utf8.FullRune(src[nSrc:]) && !atEOF {
			err = transform.ErrShortSrc
			return
		}
		r, size := utf8.DecodeRune(src[nSrc:])
		mapped := mapKataToHira(r)
		outSize := utf8.RuneLen(mapped)
		if nDst+outSize > len(dst) {
			err = transform.ErrShortDst
			return
		}
		utf8.EncodeRune(dst[nDst:], mapped)
		nDst += outSize
		nSrc += size
	}
	return
}

func (katakanaToHiragana) Reset() {}

// mapHiraToKata maps a hiragana rune to its katakana equivalent.
// Non-hiragana runes are returned unchanged.
func mapHiraToKata(r rune) rune {
	switch {
	case r >= 0x3041 && r <= 0x3096: // main hiragana block
		return r + kanaOffset
	case r >= 0x309D && r <= 0x309E: // iteration marks ゝゞ → ヽヾ
		return r + kanaOffset
	case r == 0x309F: // digraph yori ゟ → ヿ
		return 0x30FF
	default:
		return r
	}
}

// mapKataToHira maps a katakana rune to its hiragana equivalent.
// Non-katakana runes are returned unchanged.
func mapKataToHira(r rune) rune {
	switch {
	case r >= 0x30A1 && r <= 0x30F6: // main katakana block
		return r - kanaOffset
	case r >= 0x30FD && r <= 0x30FE: // iteration marks ヽヾ → ゝゞ
		return r - kanaOffset
	case r == 0x30FF: // digraph koto ヿ → ゟ
		return 0x309F
	default:
		return r
	}
}
