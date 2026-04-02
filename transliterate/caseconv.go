package transliterate

import (
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"golang.org/x/text/transform"
)

func init() {
	Register("Any-Lower", func() transform.Transformer { return cases.Lower(language.Und) })
	Register("Any-Upper", func() transform.Transformer { return cases.Upper(language.Und) })
	Register("Any-Title", func() transform.Transformer { return cases.Title(language.Und) })
}
