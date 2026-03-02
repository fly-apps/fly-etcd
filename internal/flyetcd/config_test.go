package flyetcd

import (
	"os"
	"path/filepath"
	"testing"

	yaml "gopkg.in/yaml.v3"
)

// setupTestDirs overrides DataDir and ConfigFilePath to use a temp directory,
// and sets the required FLY_* env vars. Returns restore func via t.Cleanup.
func setupTestDirs(t *testing.T) string {
	t.Helper()

	tmpDir := t.TempDir()
	origDataDir := DataDir
	origConfigFilePath := ConfigFilePath

	DataDir = tmpDir
	ConfigFilePath = filepath.Join(tmpDir, "etcd.yaml")

	t.Setenv("FLY_MACHINE_ID", "test-machine")
	t.Setenv("FLY_APP_NAME", "test-app")

	t.Cleanup(func() {
		DataDir = origDataDir
		ConfigFilePath = origConfigFilePath
	})

	return tmpDir
}

func TestGetEnvOrDefault(t *testing.T) {
	t.Run("int", func(t *testing.T) {
		t.Run("unset returns fallback", func(t *testing.T) {
			result := getEnvOrDefault("ETCD_TEST_UNSET_INT", 42)
			if result != 42 {
				t.Errorf("expected 42, got %d", result)
			}
		})

		t.Run("valid", func(t *testing.T) {
			t.Setenv("ETCD_TEST_INT", "100")
			result := getEnvOrDefault("ETCD_TEST_INT", 42)
			if result != 100 {
				t.Errorf("expected 100, got %d", result)
			}
		})

		t.Run("invalid returns fallback", func(t *testing.T) {
			t.Setenv("ETCD_TEST_INT_BAD", "banana")
			result := getEnvOrDefault("ETCD_TEST_INT_BAD", 42)
			if result != 42 {
				t.Errorf("expected fallback 42, got %d", result)
			}
		})
	})

	t.Run("string", func(t *testing.T) {
		t.Run("unset returns fallback", func(t *testing.T) {
			result := getEnvOrDefault("ETCD_TEST_UNSET_STR", "default")
			if result != "default" {
				t.Errorf("expected 'default', got %q", result)
			}
		})

		t.Run("valid", func(t *testing.T) {
			t.Setenv("ETCD_TEST_STR", "override")
			result := getEnvOrDefault("ETCD_TEST_STR", "default")
			if result != "override" {
				t.Errorf("expected 'override', got %q", result)
			}
		})

		t.Run("empty overrides fallback", func(t *testing.T) {
			t.Setenv("ETCD_TEST_STR_EMPTY", "")
			result := getEnvOrDefault("ETCD_TEST_STR_EMPTY", "default")
			if result != "" {
				t.Errorf("expected empty string, got %q", result)
			}
		})
	})

}

// We want to make sure that defaults are preserved when unmarshalling a partial YAML config.
// This is the mechanism resolveConfig relies on.
func TestYAMLOverlayPreservesDefaults(t *testing.T) {
	cfg := &Config{
		AutoCompactionMode:      "periodic",
		AutoCompactionRetention: "1",
		MaxSnapshots:            10,
		MaxWals:                 10,
		SnapshotCount:           10000,
		QuotaBackendBytes:       2 * 1024 * 1024 * 1024,
	}

	// Older config file that doesn't have quota-backend-bytes.
	partialYAML := []byte(`
name: test-node
initial-cluster-state: existing
max-snapshots: 5
`)

	if err := yaml.Unmarshal(partialYAML, cfg); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	// Fields present in the YAML should overwrite defaults.
	if cfg.Name != "test-node" {
		t.Errorf("expected name 'test-node', got %q", cfg.Name)
	}

	if cfg.InitialClusterState != "existing" {
		t.Errorf("expected initial-cluster-state 'existing', got %q", cfg.InitialClusterState)
	}

	if cfg.MaxSnapshots != 5 {
		t.Errorf("expected max-snapshots 5, got %d", cfg.MaxSnapshots)
	}

	// Fields missing from YAML should not be overwritten.
	if cfg.AutoCompactionMode != "periodic" {
		t.Errorf("expected auto-compaction-mode 'periodic', got %q", cfg.AutoCompactionMode)
	}

	if cfg.AutoCompactionRetention != "1" {
		t.Errorf("expected auto-compaction-retention '1', got %q", cfg.AutoCompactionRetention)
	}

	if cfg.MaxWals != 10 {
		t.Errorf("expected max-wals 10, got %d", cfg.MaxWals)
	}

	if cfg.SnapshotCount != 10000 {
		t.Errorf("expected snapshot-count 10000, got %d", cfg.SnapshotCount)
	}

	if cfg.QuotaBackendBytes != 2*1024*1024*1024 {
		t.Errorf("expected quota-backend-bytes 2GiB, got %d", cfg.QuotaBackendBytes)
	}
}

// resolveConfig is called when trying to create or load new node configuration.
// If there's no config file, it will create one with default values. If there is
// a config file, it will load it on top of default values. In both cases, env vars
// should be loaded on top of config file values.
func TestResolveConfig(t *testing.T) {
	// Sanity check
	t.Run("no config file", func(t *testing.T) {
		t.Run("defaults", func(t *testing.T) {
			setupTestDirs(t)

			cfg, err := resolveConfig()
			if err != nil {
				t.Fatalf("resolveConfig failed: %v", err)
			}

			// Should match default values.
			if cfg.Name != "test-machine" {
				t.Errorf("expected name 'test-machine', got %q", cfg.Name)
			}

			if cfg.QuotaBackendBytes != 2*1024*1024*1024 {
				t.Errorf("expected quota-backend-bytes 2GiB, got %d", cfg.QuotaBackendBytes)
			}

			if cfg.MaxSnapshots != 10 {
				t.Errorf("expected max-snapshots 10, got %d", cfg.MaxSnapshots)
			}

			if cfg.AutoCompactionMode != "periodic" {
				t.Errorf("expected auto-compaction-mode 'periodic', got %q", cfg.AutoCompactionMode)
			}
		})

		t.Run("env overrides", func(t *testing.T) {
			setupTestDirs(t)
			t.Setenv("ETCD_QUOTA_BACKEND_BYTES", "5368709120")
			t.Setenv("ETCD_MAX_SNAPSHOTS", "20")
			t.Setenv("ETCD_AUTO_COMPACTION_MODE", "revision")
			t.Setenv("ETCD_AUTO_COMPACTION_RETENTION", "1000")

			cfg, err := resolveConfig()
			if err != nil {
				t.Fatalf("resolveConfig failed: %v", err)
			}

			// Default values should be overridden by env vars.
			if cfg.QuotaBackendBytes != 5368709120 {
				t.Errorf("expected quota-backend-bytes 5GiB, got %d", cfg.QuotaBackendBytes)
			}

			if cfg.MaxSnapshots != 20 {
				t.Errorf("expected max-snapshots 20, got %d", cfg.MaxSnapshots)
			}

			if cfg.AutoCompactionMode != "revision" {
				t.Errorf("expected auto-compaction-mode 'revision', got %q", cfg.AutoCompactionMode)
			}

			if cfg.AutoCompactionRetention != "1000" {
				t.Errorf("expected auto-compaction-retention '1000', got %q", cfg.AutoCompactionRetention)
			}

			// Unset env vars should keep defaults.
			if cfg.MaxWals != 10 {
				t.Errorf("expected max-wals 10, got %d", cfg.MaxWals)
			}
		})
	})

	t.Run("config file", func(t *testing.T) {
		// Sanity check. If we can't read, we can't read.
		// Optionally, we could return default values.
		t.Run("read error", func(t *testing.T) {
			tmpDir := setupTestDirs(t)

			// Create a directory where the config file should be, so ReadFile fails.
			if err := os.Mkdir(filepath.Join(tmpDir, "etcd.yaml"), 0700); err != nil {
				t.Fatalf("failed to create dir: %v", err)
			}

			_, err := resolveConfig()

			if err == nil {
				t.Error("expected error reading config, got nil")
			}
		})

		// Sanity check. We won't parse anything but yaml.
		t.Run("parse error", func(t *testing.T) {
			tmpDir := setupTestDirs(t)

			if err := os.WriteFile(filepath.Join(tmpDir, "etcd.yaml"), []byte("{{invalid yaml"), 0644); err != nil {
				t.Fatalf("failed to write config: %v", err)
			}

			_, err := resolveConfig()
			if err == nil {
				t.Error("expected error parsing config, got nil")
			}
		})

		// Make sure that when we load an existing config file that either
		// doesn't set / doesn't include a new field, the default value is used.
		t.Run("defaults preserved", func(t *testing.T) {
			tmpDir := setupTestDirs(t)

			// Here it won't include quota-backend-bytes.
			configYAML := []byte(`
name: existing-node
initial-cluster-state: existing
max-snapshots: 5
`)
			if err := os.WriteFile(filepath.Join(tmpDir, "etcd.yaml"), configYAML, 0644); err != nil {
				t.Fatalf("failed to write config: %v", err)
			}

			cfg, err := resolveConfig()
			if err != nil {
				t.Fatalf("resolveConfig failed: %v", err)
			}

			// Existing fields should be preserved.
			if cfg.Name != "existing-node" {
				t.Errorf("expected name 'existing-node', got %q", cfg.Name)
			}

			if cfg.InitialClusterState != "existing" {
				t.Errorf("expected initial-cluster-state 'existing', got %q", cfg.InitialClusterState)
			}

			if cfg.MaxSnapshots != 5 {
				t.Errorf("expected max-snapshots 5, got %d", cfg.MaxSnapshots)
			}

			// Missing fields should get defaults from DefaultConfig.
			if cfg.QuotaBackendBytes != 2*1024*1024*1024 {
				t.Errorf("expected quota-backend-bytes 2GiB, got %d", cfg.QuotaBackendBytes)
			}

			if cfg.AutoCompactionMode != "periodic" {
				t.Errorf("expected auto-compaction-mode 'periodic', got %q", cfg.AutoCompactionMode)
			}
		})

		// Make sure env vars always take precedence.
		t.Run("env overrides non-existing field", func(t *testing.T) {
			tmpDir := setupTestDirs(t)
			t.Setenv("ETCD_QUOTA_BACKEND_BYTES", "5368709120")

			// Config file sets quota to 1GiB.
			configYAML := []byte(`
name: existing-node
max-snapshots: 5
`)

			if err := os.WriteFile(filepath.Join(tmpDir, "etcd.yaml"), configYAML, 0644); err != nil {
				t.Fatalf("failed to write config: %v", err)
			}

			cfg, err := resolveConfig()
			if err != nil {
				t.Fatalf("resolveConfig failed: %v", err)
			}

			// Env var should take precedence over default value.
			if cfg.QuotaBackendBytes != 5368709120 {
				t.Errorf("expected quota-backend-bytes 5GiB, got %d", cfg.QuotaBackendBytes)
			}

			// Other fields should be kept.
			if cfg.MaxSnapshots != 5 {
				t.Errorf("expected max-snapshots 5, got %d", cfg.MaxSnapshots)
			}
		})

		t.Run("env overrides existing field", func(t *testing.T) {
			tmpDir := setupTestDirs(t)
			t.Setenv("ETCD_QUOTA_BACKEND_BYTES", "5368709120")

			// Config file sets quota to 1GiB.
			configYAML := []byte(`
name: existing-node
quota-backend-bytes: 1073741824
max-snapshots: 5
`)

			if err := os.WriteFile(filepath.Join(tmpDir, "etcd.yaml"), configYAML, 0644); err != nil {
				t.Fatalf("failed to write config: %v", err)
			}

			cfg, err := resolveConfig()
			if err != nil {
				t.Fatalf("resolveConfig failed: %v", err)
			}

			// Env var should override config file value.
			if cfg.QuotaBackendBytes != 5368709120 {
				t.Errorf("expected quota-backend-bytes 5GiB, got %d", cfg.QuotaBackendBytes)
			}

			// Other fields should be kept.
			if cfg.MaxSnapshots != 5 {
				t.Errorf("expected max-snapshots 5, got %d", cfg.MaxSnapshots)
			}
		})

		t.Run("dynamic settings always forced", func(t *testing.T) {
			tmpDir := setupTestDirs(t)

			// Config file has stale values for DataDir and ListenMetricsUrls.
			configYAML := []byte(`
data-dir: /old-data
listen-metrics-urls: http://wrong:1234
name: existing-node
`)
			if err := os.WriteFile(filepath.Join(tmpDir, "etcd.yaml"), configYAML, 0644); err != nil {
				t.Fatalf("failed to write config: %v", err)
			}

			cfg, err := resolveConfig()
			if err != nil {
				t.Fatalf("resolveConfig failed: %v", err)
			}

			// Dynamic settings must always be forced.
			if cfg.DataDir != DataDir {
				t.Errorf("expected data-dir %q, got %q", DataDir, cfg.DataDir)
			}

			if cfg.ListenMetricsUrls != MetricsBaseURL {
				t.Errorf("expected listen-metrics-urls %q, got %q", MetricsBaseURL, cfg.ListenMetricsUrls)
			}
		})

		// Sanity check. If we have an empty file, all we should get are defaults.
		t.Run("empty file preserves defaults", func(t *testing.T) {
			tmpDir := setupTestDirs(t)

			// Empty config file.
			if err := os.WriteFile(filepath.Join(tmpDir, "etcd.yaml"), []byte(""), 0644); err != nil {
				t.Fatalf("failed to write config: %v", err)
			}

			cfg, err := resolveConfig()
			if err != nil {
				t.Fatalf("resolveConfig failed: %v", err)
			}

			// All defaults should survive.
			if cfg.QuotaBackendBytes != 2*1024*1024*1024 {
				t.Errorf("expected quota-backend-bytes 2GiB, got %d", cfg.QuotaBackendBytes)
			}

			if cfg.MaxSnapshots != 10 {
				t.Errorf("expected max-snapshots 10, got %d", cfg.MaxSnapshots)
			}

			if cfg.AutoCompactionMode != "periodic" {
				t.Errorf("expected auto-compaction-mode 'periodic', got %q", cfg.AutoCompactionMode)
			}
		})

		// If we /explicitly/ set a value in the file, even it's a zero, it should be preserved.
		t.Run("explicit zero overwrites default", func(t *testing.T) {
			tmpDir := setupTestDirs(t)

			// Config file explicitly sets max-snapshots to 0.
			configYAML := []byte(`max-snapshots: 0`)
			if err := os.WriteFile(filepath.Join(tmpDir, "etcd.yaml"), configYAML, 0644); err != nil {
				t.Fatalf("failed to write config: %v", err)
			}

			cfg, err := resolveConfig()
			if err != nil {
				t.Fatalf("resolveConfig failed: %v", err)
			}

			// Explicit zero in YAML overwrites the default — this is expected.
			if cfg.MaxSnapshots != 0 {
				t.Errorf("expected max-snapshots 0, got %d", cfg.MaxSnapshots)
			}
		})
	})

	// Simulate what happens when we boot a node.
	t.Run("write then read round-trip", func(t *testing.T) {
		setupTestDirs(t)

		// First boot. Generate config files and write them.
		cfg, err := NewConfig()
		if err != nil {
			t.Fatalf("NewConfig failed: %v", err)
		}

		if err := WriteConfig(cfg); err != nil {
			t.Fatalf("WriteConfig failed: %v", err)
		}

		// Now we reboot. Try to load them from disk.
		loaded, err := resolveConfig()
		if err != nil {
			t.Fatalf("resolveConfig failed: %v", err)
		}

		if loaded.QuotaBackendBytes != cfg.QuotaBackendBytes {
			t.Errorf("expected quota-backend-bytes %d, got %d", cfg.QuotaBackendBytes, loaded.QuotaBackendBytes)
		}

		if loaded.MaxSnapshots != cfg.MaxSnapshots {
			t.Errorf("expected max-snapshots %d, got %d", cfg.MaxSnapshots, loaded.MaxSnapshots)
		}

		if loaded.MaxWals != cfg.MaxWals {
			t.Errorf("expected max-wals %d, got %d", cfg.MaxWals, loaded.MaxWals)
		}

		if loaded.SnapshotCount != cfg.SnapshotCount {
			t.Errorf("expected snapshot-count %d, got %d", cfg.SnapshotCount, loaded.SnapshotCount)
		}

		if loaded.AutoCompactionMode != cfg.AutoCompactionMode {
			t.Errorf("expected auto-compaction-mode %q, got %q", cfg.AutoCompactionMode, loaded.AutoCompactionMode)
		}

		if loaded.AutoCompactionRetention != cfg.AutoCompactionRetention {
			t.Errorf("expected auto-compaction-retention %q, got %q", cfg.AutoCompactionRetention, loaded.AutoCompactionRetention)
		}
	})
}

// We want to make sure env vars take precedence over values loaded from a config file.
// This is important because env vars can be set at runtime, while config files are static.
func TestEnvOverridesConfigFile(t *testing.T) {
	cfg := &Config{
		MaxSnapshots:      10,
		QuotaBackendBytes: 2 * 1024 * 1024 * 1024,
	}

	// Set config file quota to 1GiB.
	configYAML := []byte(`quota-backend-bytes: 1073741824`)
	if err := yaml.Unmarshal(configYAML, cfg); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if cfg.QuotaBackendBytes != 1073741824 {
		t.Fatalf("expected config file value 1073741824, got %d", cfg.QuotaBackendBytes)
	}

	// Change quota to 5GiB via env var.
	t.Setenv("ETCD_QUOTA_BACKEND_BYTES", "5368709120")
	cfg.QuotaBackendBytes = getEnvOrDefault("ETCD_QUOTA_BACKEND_BYTES", cfg.QuotaBackendBytes)

	if cfg.QuotaBackendBytes != 5368709120 {
		t.Errorf("expected env override 5368709120, got %d", cfg.QuotaBackendBytes)
	}

	// MaxSnapshots didn't change.
	cfg.MaxSnapshots = getEnvOrDefault("ETCD_MAX_SNAPSHOTS", cfg.MaxSnapshots)
	if cfg.MaxSnapshots != 10 {
		t.Errorf("expected max-snapshots 10, got %d", cfg.MaxSnapshots)
	}
}
