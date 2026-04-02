package transliterate

import (
	"golang.org/x/text/transform"
	"golang.org/x/text/width"
)

func init() {
	RegisterPair(
		"Fullwidth-Halfwidth", func() transform.Transformer { return width.Narrow },
		"Halfwidth-Fullwidth", func() transform.Transformer { return width.Widen },
	)
	Register("Any-Width", func() transform.Transformer { return width.Fold })
}
