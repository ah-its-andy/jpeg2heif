package converter

import (
	"fmt"
	"sync"
)

var (
	registry = make(map[string]Converter)
	mu       sync.RWMutex
	disabled = make(map[string]bool)
)

// Register registers a converter in the global registry
func Register(c Converter) {
	mu.Lock()
	defer mu.Unlock()
	registry[c.Name()] = c
}

// Get retrieves a converter by name
func Get(name string) (Converter, bool) {
	mu.RLock()
	defer mu.RUnlock()
	c, ok := registry[name]
	return c, ok
}

// List returns all registered converters
func List() []Converter {
	mu.RLock()
	defer mu.RUnlock()
	converters := make([]Converter, 0, len(registry))
	for _, c := range registry {
		converters = append(converters, c)
	}
	return converters
}

// ListInfo returns information about all registered converters
func ListInfo() []ConverterInfo {
	mu.RLock()
	defer mu.RUnlock()
	infos := make([]ConverterInfo, 0, len(registry))
	for name, c := range registry {
		infos = append(infos, ConverterInfo{
			Name:         name,
			TargetFormat: c.TargetFormat(),
			Enabled:      !disabled[name],
		})
	}
	return infos
}

// FindConverter finds the first enabled converter that can handle the given file
func FindConverter(srcPath string, srcMime string) (Converter, error) {
	mu.RLock()
	defer mu.RUnlock()

	for name, c := range registry {
		if disabled[name] {
			continue
		}
		if c.CanConvert(srcPath, srcMime) {
			return c, nil
		}
	}

	return nil, fmt.Errorf("no converter found for file: %s (mime: %s)", srcPath, srcMime)
}

// Enable enables a converter by name
func Enable(name string) error {
	mu.Lock()
	defer mu.Unlock()

	if _, ok := registry[name]; !ok {
		return fmt.Errorf("converter not found: %s", name)
	}

	delete(disabled, name)
	return nil
}

// Disable disables a converter by name
func Disable(name string) error {
	mu.Lock()
	defer mu.Unlock()

	if _, ok := registry[name]; !ok {
		return fmt.Errorf("converter not found: %s", name)
	}

	disabled[name] = true
	return nil
}

// IsEnabled checks if a converter is enabled
func IsEnabled(name string) bool {
	mu.RLock()
	defer mu.RUnlock()
	return !disabled[name]
}
