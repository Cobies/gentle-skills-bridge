package bridge

import (
	"strings"
	"testing"
)

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *Config
		wantErr string
	}{
		{
			name: "valid explicit target",
			cfg: &Config{
				Sources:         []string{"skills"},
				Targets:         []string{"target"},
				WatchIntervalMS: 1000,
			},
		},
		{
			name: "valid auto discovery",
			cfg: &Config{
				Sources:            []string{"skills"},
				AutoDiscoverAgents: true,
				WatchIntervalMS:    1000,
			},
		},
		{
			name:    "nil config",
			cfg:     nil,
			wantErr: "config is required",
		},
		{
			name:    "missing sources",
			cfg:     &Config{Targets: []string{"target"}},
			wantErr: "at least one source directory is required",
		},
		{
			name: "empty source",
			cfg: &Config{
				Sources: []string{" "},
				Targets: []string{"target"},
			},
			wantErr: "source directory at index 0 is empty",
		},
		{
			name: "missing target without auto discovery",
			cfg: &Config{
				Sources: []string{"skills"},
			},
			wantErr: "at least one target directory is required",
		},
		{
			name: "empty target",
			cfg: &Config{
				Sources: []string{"skills"},
				Targets: []string{""},
			},
			wantErr: "target directory at index 0 is empty",
		},
		{
			name: "engram project required",
			cfg: &Config{
				Sources:      []string{"skills"},
				Targets:      []string{"target"},
				SyncToEngram: true,
			},
			wantErr: "engram_project is required",
		},
		{
			name: "negative watch interval",
			cfg: &Config{
				Sources:         []string{"skills"},
				Targets:         []string{"target"},
				WatchIntervalMS: -1,
			},
			wantErr: "watch_interval_ms must be greater than or equal to 0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("Validate() error = %v, want nil", err)
				}
				return
			}

			if err == nil {
				t.Fatalf("Validate() error = nil, want %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Validate() error = %q, want to contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}
