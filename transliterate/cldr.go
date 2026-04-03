package transliterate

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/text/transform"
)

// CLDR XML structures
type cldrSupplemental struct {
	XMLName    xml.Name       `xml:"supplementalData"`
	Transforms cldrTransforms `xml:"transforms"`
}

type cldrTransforms struct {
	Items []cldrTransform `xml:"transform"`
}

type cldrTransform struct {
	Source        string `xml:"source,attr"`
	Target        string `xml:"target,attr"`
	Direction     string `xml:"direction,attr"`
	Variant       string `xml:"variant,attr"`
	Alias         string `xml:"alias,attr"`
	BackwardAlias string `xml:"backwardAlias,attr"`
	TRule         string `xml:"tRule"`
}

// LoadCLDR loads all CLDR transform XML files from dir and registers them.
func LoadCLDR(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("LoadCLDR: %w", err)
	}

	var errs []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".xml") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		if err := LoadCLDRFile(path); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", entry.Name(), err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("LoadCLDR: %d errors:\n%s", len(errs), strings.Join(errs, "\n"))
	}
	return nil
}

// LoadCLDRFile loads a single CLDR transform XML file and registers its transforms.
func LoadCLDRFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("LoadCLDRFile: %w", err)
	}

	return LoadCLDRData(data)
}

// LoadCLDRData loads CLDR transform XML data from bytes and registers transforms.
func LoadCLDRData(data []byte) error {
	var doc cldrSupplemental
	if err := xml.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("LoadCLDRData: %w", err)
	}

	for _, t := range doc.Transforms.Items {
		if err := registerCLDRTransform(t); err != nil {
			return err
		}
	}
	return nil
}

func registerCLDRTransform(t cldrTransform) error {
	rules := strings.TrimSpace(t.TRule)
	if rules == "" {
		return nil
	}

	// Determine IDs
	fwdID := t.Alias
	if fwdID == "" {
		fwdID = t.Source + "-" + t.Target
		if t.Variant != "" {
			fwdID += "/" + t.Variant
		}
	}
	// Take the first space-separated token as the ID
	if idx := strings.IndexByte(fwdID, ' '); idx >= 0 {
		fwdID = fwdID[:idx]
	}

	revID := t.BackwardAlias
	if revID == "" && t.Direction == "both" {
		revID = t.Target + "-" + t.Source
		if t.Variant != "" {
			revID += "/" + t.Variant
		}
	}
	if idx := strings.IndexByte(revID, ' '); idx >= 0 {
		revID = revID[:idx]
	}

	parsed, err := ParseRules(rules)
	if err != nil {
		return fmt.Errorf("CLDR transform %s: %w", fwdID, err)
	}

	switch t.Direction {
	case "both":
		fwdResult, err := CompileRules(parsed, Forward)
		if err != nil {
			return fmt.Errorf("CLDR transform %s (forward): %w", fwdID, err)
		}
		revResult, err := CompileRules(parsed, Reverse)
		if err != nil {
			return fmt.Errorf("CLDR transform %s (reverse): %w", revID, err)
		}
		Register(fwdID, makeFactory(fwdResult))
		if revID != "" {
			Register(revID, makeFactory(revResult))
		}

	case "forward", "":
		fwdResult, err := CompileRules(parsed, Forward)
		if err != nil {
			return fmt.Errorf("CLDR transform %s: %w", fwdID, err)
		}
		Register(fwdID, makeFactory(fwdResult))

	case "backward":
		revResult, err := CompileRules(parsed, Reverse)
		if err != nil {
			return fmt.Errorf("CLDR transform %s: %w", revID, err)
		}
		if revID != "" {
			Register(revID, makeFactory(revResult))
		}
	}

	return nil
}

func makeFactory(result *CompilationResult) TransformFactory {
	return func() transform.Transformer {
		parts := make([]transform.Transformer, 0, len(result.PreNorm)+1+len(result.PostNorm))
		for _, t := range result.PreNorm {
			parts = append(parts, t)
		}
		parts = append(parts, NewRuleTransformer(result.RuleSet))
		for _, t := range result.PostNorm {
			parts = append(parts, t)
		}
		if len(parts) == 1 {
			return parts[0]
		}
		return transform.Chain(parts...)
	}
}

// NewFromRules creates a Transliterator from raw ICU rule text.
func NewFromRules(id string, rules string, dir Direction) (*Transliterator, error) {
	parsed, err := ParseRules(rules)
	if err != nil {
		return nil, fmt.Errorf("NewFromRules: %w", err)
	}

	result, err := CompileRules(parsed, dir)
	if err != nil {
		return nil, fmt.Errorf("NewFromRules: %w", err)
	}

	factory := makeFactory(result)
	return &Transliterator{id: id, t: factory()}, nil
}
