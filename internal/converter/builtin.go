package converter

import (
	"fmt"
	"os"
	"strings"
)

// RegisterBuiltinConverters registers builtin converters based on environment variable
// Environment variable BUILTIN_CONVERTERS controls which converters to register
// Format: comma-separated list of converter names (e.g., "jpeg2heic,png2avif")
// If empty or not set, no builtin converters are registered
func RegisterBuiltinConverters() {
	convertersEnv := os.Getenv("BUILTIN_CONVERTERS")

	// If environment variable is empty, don't register any converters
	if convertersEnv == "" {
		fmt.Println("BUILTIN_CONVERTERS not set or empty - no builtin converters registered")
		return
	}

	// Parse comma-separated list
	converterNames := strings.Split(convertersEnv, ",")
	registeredCount := 0

	for _, name := range converterNames {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}

		switch strings.ToLower(name) {
		case "jpeg2heic":
			Register(NewJPEG2HEICConverter())
			fmt.Printf("Registered builtin converter: jpeg2heic\n")
			registeredCount++
		default:
			fmt.Printf("Warning: unknown builtin converter '%s'\n", name)
		}
	}

	fmt.Printf("Registered %d builtin converters\n", registeredCount)
}

// ListAvailableBuiltinConverters returns a list of all available builtin converter names
func ListAvailableBuiltinConverters() []string {
	return []string{
		"jpeg2heic",
		// Add more builtin converters here in the future
	}
}
