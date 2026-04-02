package transliterate

import (
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

func init() {
	Register("Any-NFC", func() transform.Transformer { return norm.NFC })
	Register("NFC", func() transform.Transformer { return norm.NFC })
	Register("Any-NFD", func() transform.Transformer { return norm.NFD })
	Register("NFD", func() transform.Transformer { return norm.NFD })
	Register("Any-NFKC", func() transform.Transformer { return norm.NFKC })
	Register("NFKC", func() transform.Transformer { return norm.NFKC })
	Register("Any-NFKD", func() transform.Transformer { return norm.NFKD })
	Register("NFKD", func() transform.Transformer { return norm.NFKD })
}
