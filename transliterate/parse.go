package transliterate

import "strings"

// splitCompound splits a compound transliterator ID on ";" separators.
func splitCompound(id string) []string {
	parts := strings.Split(id, ";")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}
