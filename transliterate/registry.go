package transliterate

import (
	"sort"
	"strings"
	"sync"

	"golang.org/x/text/transform"
)

// TransformFactory creates a new transform.Transformer instance.
// Each call must return a fresh instance since transformers may be stateful.
type TransformFactory func() transform.Transformer

type entry struct {
	canonicalID string
	factory     TransformFactory
}

var (
	mu       sync.RWMutex
	registry = make(map[string]entry)
)

// Register adds a transliterator to the global registry.
// The ID should be in canonical form (e.g. "Fullwidth-Halfwidth").
func Register(id string, factory TransformFactory) {
	mu.Lock()
	defer mu.Unlock()
	registry[strings.ToLower(id)] = entry{canonicalID: id, factory: factory}
}

// RegisterPair registers a forward and reverse transliterator pair.
func RegisterPair(forwardID string, forwardFactory TransformFactory, reverseID string, reverseFactory TransformFactory) {
	Register(forwardID, forwardFactory)
	Register(reverseID, reverseFactory)
}

// lookup finds a factory for the given raw ID (not yet normalized).
// It tries several fallback strategies for matching.
func lookup(id string) (TransformFactory, error) {
	mu.RLock()
	defer mu.RUnlock()

	key := strings.ToLower(strings.TrimSpace(id))

	// Direct lookup
	if e, ok := registry[key]; ok {
		return e.factory, nil
	}

	// Try stripping "any-" prefix
	if strings.HasPrefix(key, "any-") {
		stripped := key[4:]
		if e, ok := registry[stripped]; ok {
			return e.factory, nil
		}
	}

	// Try adding "any-" prefix
	withAny := "any-" + key
	if e, ok := registry[withAny]; ok {
		return e.factory, nil
	}

	return nil, ErrUnknownTransliterator
}

// IDs returns all registered transliterator IDs in sorted order.
func IDs() []string {
	mu.RLock()
	defer mu.RUnlock()
	ids := make([]string, 0, len(registry))
	for _, e := range registry {
		ids = append(ids, e.canonicalID)
	}
	sort.Strings(ids)
	return ids
}
