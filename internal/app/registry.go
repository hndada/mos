package app

import "sync"

// Factory is a constructor the registry calls to create a new app instance.
// ctx is valid for the entire lifetime of the returned Content.
type Factory func(ctx Context) Content

var (
	mu       sync.RWMutex
	registry = map[string]Factory{}
)

// Register associates appID with factory f.
// Call this from an init() function in the app's package so that the
// windowing server can instantiate it by ID at any time.
func Register(appID string, f Factory) {
	mu.Lock()
	defer mu.Unlock()
	registry[appID] = f
}

// New instantiates the app registered under appID using ctx.
// Returns nil when appID is not registered.
func New(appID string, ctx Context) Content {
	mu.RLock()
	f, ok := registry[appID]
	mu.RUnlock()
	if !ok {
		return nil
	}
	return f(ctx)
}

// IDs returns the set of all registered app IDs (order unspecified).
func IDs() []string {
	mu.RLock()
	defer mu.RUnlock()
	ids := make([]string, 0, len(registry))
	for id := range registry {
		ids = append(ids, id)
	}
	return ids
}
