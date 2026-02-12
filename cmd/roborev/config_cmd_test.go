package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
)

func setupConfigFile(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "config.toml")
}

func readTOML(t *testing.T, path string) map[string]interface{} {
	t.Helper()
	raw := make(map[string]interface{})
	if _, err := toml.DecodeFile(path, &raw); err != nil {
		t.Fatalf("read TOML %s: %v", path, err)
	}
	return raw
}

// getNestedValue traverses a dot-separated key path in a nested map.
func getNestedValue(t *testing.T, raw map[string]interface{}, dotKey string) interface{} {
	t.Helper()
	parts := strings.Split(dotKey, ".")
	var current interface{} = raw
	for _, part := range parts {
		m, ok := current.(map[string]interface{})
		if !ok {
			t.Fatalf("key %q: expected map at %q, got %T", dotKey, part, current)
		}
		current = m[part]
	}
	return current
}

func assertConfigValue(t *testing.T, path, dotKey string, expected interface{}) {
	t.Helper()
	raw := readTOML(t, path)
	val := getNestedValue(t, raw, dotKey)
	if val != expected {
		t.Errorf("%s = %v (%T), want %v (%T)", dotKey, val, val, expected, expected)
	}
}

func TestSetConfigKey(t *testing.T) {
	path := setupConfigFile(t)

	t.Run("String", func(t *testing.T) {
		if err := setConfigKey(path, "default_agent", "gemini", true); err != nil {
			t.Fatalf("setConfigKey: %v", err)
		}
		assertConfigValue(t, path, "default_agent", "gemini")
	})

	t.Run("Integer", func(t *testing.T) {
		if err := setConfigKey(path, "max_workers", "8", true); err != nil {
			t.Fatalf("setConfigKey: %v", err)
		}
		assertConfigValue(t, path, "max_workers", int64(8))
	})

	t.Run("Boolean", func(t *testing.T) {
		if err := setConfigKey(path, "sync.enabled", "true", true); err != nil {
			t.Fatalf("setConfigKey: %v", err)
		}
		assertConfigValue(t, path, "sync.enabled", true)
	})

	t.Run("Persistence", func(t *testing.T) {
		// Previous values should still be present after multiple sets.
		assertConfigValue(t, path, "default_agent", "gemini")
		assertConfigValue(t, path, "max_workers", int64(8))
		assertConfigValue(t, path, "sync.enabled", true)
	})
}

func TestSetConfigKeyNestedCreation(t *testing.T) {
	path := setupConfigFile(t)

	if err := setConfigKey(path, "ci.poll_interval", "10m", true); err != nil {
		t.Fatalf("setConfigKey nested: %v", err)
	}
	assertConfigValue(t, path, "ci.poll_interval", "10m")
}

func TestSetConfigKeyInvalidKey(t *testing.T) {
	path := setupConfigFile(t)

	err := setConfigKey(path, "nonexistent_key", "value", true)
	if err == nil {
		t.Fatal("expected error for invalid key")
	}
}

func TestSetConfigKeySlice(t *testing.T) {
	path := setupConfigFile(t)

	if err := setConfigKey(path, "ci.repos", "org/repo1,org/repo2", true); err != nil {
		t.Fatalf("setConfigKey slice: %v", err)
	}

	raw := readTOML(t, path)
	repos, ok := getNestedValue(t, raw, "ci.repos").([]interface{})
	if !ok {
		t.Fatalf("ci.repos is not a slice: %v (%T)", getNestedValue(t, raw, "ci.repos"), getNestedValue(t, raw, "ci.repos"))
	}
	if len(repos) != 2 {
		t.Errorf("ci.repos length = %d, want 2", len(repos))
	}
}

func TestSetConfigKeyRepoConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".roborev.toml")

	if err := setConfigKey(path, "agent", "claude-code", false); err != nil {
		t.Fatalf("setConfigKey repo: %v", err)
	}
	assertConfigValue(t, path, "agent", "claude-code")
}

func TestSetRawMapKey(t *testing.T) {
	tests := []struct {
		name string
		key  string
		val  interface{}
		path string // dot-path to check in the resulting map
		want interface{}
	}{
		{
			name: "SimpleKey",
			key:  "foo",
			val:  "bar",
			path: "foo",
			want: "bar",
		},
		{
			name: "NestedKey",
			key:  "a.b.c",
			val:  42,
			path: "a.b.c",
			want: 42,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := make(map[string]interface{})
			setRawMapKey(m, tt.key, tt.val)
			got := getNestedValue(t, m, tt.path)
			if got != tt.want {
				t.Errorf("%s = %v (%T), want %v (%T)", tt.path, got, got, tt.want, tt.want)
			}
		})
	}
}
