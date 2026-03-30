package xmlparser

import (
	"fmt"
	"os"
)

// InjectBackendListener is intentionally a no-op.
// The uploaded JMX is the source of truth for any BackendListener configuration.
func InjectBackendListener(jmxPath string) error {
	if _, err := os.Stat(jmxPath); err != nil {
		return fmt.Errorf("failed to access JMX file: %w", err)
	}

	return nil
}
