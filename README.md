# goicu

Pure Go library providing [ICU](https://icu.unicode.org/)-compatible features, using Go's own unicode support (`golang.org/x/text`) where possible and implementing whatever is missing.

## Packages

### transliterate

ICU-compatible text transliteration with support for ICU-style transform IDs.

```go
import "github.com/KarpelesLab/goicu/transliterate"

// Simple transform
tr, err := transliterate.New("Fullwidth-Halfwidth")
result, err := tr.String("Ｈｅｌｌｏ") // "Hello"

// Compound transforms chained with ";"
tr, err = transliterate.New("Hiragana-Katakana;Fullwidth-Halfwidth")
result, err = tr.String("あいう") // "ｱｲｳ"

// Streaming via transform.Transformer interface
reader := transform.NewReader(input, tr)
```

#### Available Transforms

| ID | Description |
|---|---|
| `Fullwidth-Halfwidth` | Fullwidth → halfwidth (e.g. `Ｈ` → `H`, fullwidth katakana → halfwidth) |
| `Halfwidth-Fullwidth` | Halfwidth → fullwidth |
| `Hiragana-Katakana` | Hiragana → Katakana (e.g. `あ` → `ア`) |
| `Katakana-Hiragana` | Katakana → Hiragana |
| `Any-NFC`, `Any-NFD`, `Any-NFKC`, `Any-NFKD` | Unicode normalization forms |
| `Any-Lower`, `Any-Upper`, `Any-Title` | Case transforms |
| `Latin-ASCII` | Strip diacritics (e.g. `résumé` → `resume`) |
| `Any-Null` | Identity (no-op) |
| `Any-Remove` | Remove all characters |
| `Any-Width` | Fold to canonical width |

The `Any-` prefix is optional. IDs are case-insensitive. Compound IDs are supported by separating with `;`.

Custom transforms can be registered with `transliterate.Register()`.

#### Loading CLDR Transform Rules

Load transforms from [Unicode CLDR](https://cldr.unicode.org/) data files:

```go
// Load all transforms from a CLDR common/transforms directory
err := transliterate.LoadCLDR("/path/to/cldr/common/transforms")

// Load a single CLDR XML file
err := transliterate.LoadCLDRFile("/path/to/Latin-Katakana.xml")

// Loaded transforms are registered and accessible via New()
tr, err := transliterate.New("Latin-Katakana")
```

#### Custom Rules

Create transliterators from ICU rule syntax:

```go
tr, err := transliterate.NewFromRules("Custom", `
    a → x ;
    b → y ;
    ch → Z ;
`, transliterate.Forward)
result, err := tr.String("abc") // "xyc"
```

The rule engine supports bidirectional rules (`↔`), context (`before { match } after`), variables (`$name = [set]`), Unicode set notation (`[:Latin:]`), normalization directives (`:: NFD ;`), and quoted literals.

### breakiter

ICU-compatible text segmentation (break iteration) following UAX #29 and UAX #14.

```go
import "github.com/KarpelesLab/goicu/breakiter"

// Count grapheme clusters (user-perceived characters)
n := breakiter.GraphemeCount("👨‍👩‍👧‍👦") // 1

// Count words
n = breakiter.WordCount("Hello, world!") // 2

// Split into segments
words := breakiter.SplitWords("Hello, world!")
sentences := breakiter.SplitSentences("First. Second.")

// ICU-style positional iteration
bi := breakiter.NewWord()
bi.SetText("Hello, world!")
for pos := bi.First(); ; {
    pos = bi.Next()
    if pos == breakiter.Done {
        break
    }
    fmt.Println(bi.Segment())
}
```

#### Break Types

| Type | Description | Standard |
|---|---|---|
| `Grapheme` | User-perceived characters (handles combining marks, emoji ZWJ, flags) | UAX #29 |
| `Word` | Word boundaries for selection and cursor movement | UAX #29 |
| `Sentence` | Sentence boundaries | UAX #29 |
| `Line` | Line break opportunities for text wrapping | UAX #14 |

## License

See [LICENSE](LICENSE) file.
