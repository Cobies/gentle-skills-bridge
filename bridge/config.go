package bridge

import (
	"fmt"
	"strings"
)

// Validate checks whether the configuration can run safely.
func (cfg *Config) Validate() error {
	if cfg == nil {
		return fmt.Errorf("config is required")
	}

	if len(cfg.Sources) == 0 {
		return fmt.Errorf("at least one source directory is required")
	}

	for i, source := range cfg.Sources {
		if strings.TrimSpace(source) == "" {
			return fmt.Errorf("source directory at index %d is empty", i)
		}
	}

	if !cfg.AutoDiscoverAgents && len(cfg.Targets) == 0 {
		return fmt.Errorf("at least one target directory is required when auto_discover_agents is false")
	}

	for i, target := range cfg.Targets {
		if strings.TrimSpace(target) == "" {
			return fmt.Errorf("target directory at index %d is empty", i)
		}
	}

	if cfg.SyncToEngram && strings.TrimSpace(cfg.EngramProject) == "" {
		return fmt.Errorf("engram_project is required when sync_to_engram is true")
	}

	if cfg.WatchIntervalMS < 0 {
		return fmt.Errorf("watch_interval_ms must be greater than or equal to 0")
	}

	return nil
}
