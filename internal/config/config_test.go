package config

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestSaveConfigEncryptsSensitiveFieldsAndLoadDecrypts(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	configPath, err := GetProfileConfigPath("dev")
	if err != nil {
		t.Fatalf("get profile config path: %v", err)
	}
	cfg := &Config{
		Host:          "market.hiclaw.io",
		Port:          80,
		AuthType:      "nacos",
		Username:      "alice@example.com",
		Password:      "password-value-for-test",
		AccessKey:     "access-key-value-for-test",
		SecretKey:     "secret-key-value-for-test",
		SecurityToken: "security-token-value-for-test",
		Namespace:     "test-ns",
	}

	if err := cfg.SaveConfig(configPath); err != nil {
		t.Fatalf("save config: %v", err)
	}

	if cfg.Username != "alice@example.com" || cfg.Password != "password-value-for-test" {
		t.Fatalf("SaveConfig mutated in-memory config: %+v", cfg)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read saved config: %v", err)
	}
	var raw Config
	if err := yaml.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal raw config: %v", err)
	}
	for name, value := range map[string]string{
		"username":      raw.Username,
		"password":      raw.Password,
		"accessKey":     raw.AccessKey,
		"secretKey":     raw.SecretKey,
		"securityToken": raw.SecurityToken,
	} {
		if !isEncryptedValue(value) {
			t.Fatalf("%s was not encrypted: %q", name, value)
		}
	}

	keyPath, err := getEncryptionKeyPath()
	if err != nil {
		t.Fatalf("get encryption key path: %v", err)
	}
	keyInfo, err := os.Stat(keyPath)
	if err != nil {
		t.Fatalf("stat encryption key: %v", err)
	}
	if keyInfo.Mode().Perm() != 0600 {
		t.Fatalf("encryption key mode = %v, want 0600", keyInfo.Mode().Perm())
	}

	loaded, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if loaded.Username != cfg.Username ||
		loaded.Password != cfg.Password ||
		loaded.AccessKey != cfg.AccessKey ||
		loaded.SecretKey != cfg.SecretKey ||
		loaded.SecurityToken != cfg.SecurityToken {
		t.Fatalf("loaded credentials mismatch: got %+v want %+v", loaded, cfg)
	}
}

func TestLoadConfigKeepsPlaintextLegacyConfigCompatible(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	configPath := filepath.Join(homeDir, "legacy.conf")
	if err := os.WriteFile(configPath, []byte(`host: 127.0.0.1
port: 8848
authType: nacos
username: legacy-user
password: legacy-password
namespace: legacy-ns
`), 0600); err != nil {
		t.Fatalf("write legacy config: %v", err)
	}

	loaded, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("load legacy config: %v", err)
	}
	if loaded.Username != "legacy-user" || loaded.Password != "legacy-password" {
		t.Fatalf("legacy credentials mismatch: %+v", loaded)
	}

	keyPath, err := getEncryptionKeyPath()
	if err != nil {
		t.Fatalf("get encryption key path: %v", err)
	}
	if _, err := os.Stat(keyPath); !os.IsNotExist(err) {
		t.Fatalf("legacy plaintext load should not create encryption key, stat err=%v", err)
	}
}

func TestLoadOrCreateConfigMigratesPlaintextProfile(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	configPath, err := GetProfileConfigPath("default")
	if err != nil {
		t.Fatalf("get profile config path: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(configPath), 0700); err != nil {
		t.Fatalf("create config dir: %v", err)
	}
	if err := os.WriteFile(configPath, []byte(`host: 127.0.0.1
port: 8848
authType: nacos
username: migration-user
password: migration-password
namespace: migration-ns
`), 0600); err != nil {
		t.Fatalf("write plaintext profile: %v", err)
	}

	loaded, _, err := LoadOrCreateConfig("default")
	if err != nil {
		t.Fatalf("load or create config: %v", err)
	}
	if loaded.Username != "migration-user" || loaded.Password != "migration-password" {
		t.Fatalf("loaded migrated credentials mismatch: %+v", loaded)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read migrated config: %v", err)
	}
	var raw Config
	if err := yaml.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal migrated config: %v", err)
	}
	if !isEncryptedValue(raw.Username) || !isEncryptedValue(raw.Password) {
		t.Fatalf("plaintext profile was not migrated: %+v", raw)
	}
}

func TestLoadEncryptedConfigRequiresExistingKey(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	configPath, err := GetProfileConfigPath("dev")
	if err != nil {
		t.Fatalf("get profile config path: %v", err)
	}
	cfg := &Config{
		Host:     "market.hiclaw.io",
		Port:     80,
		AuthType: "nacos",
		Username: "encrypted-user",
		Password: "encrypted-password",
	}
	if err := cfg.SaveConfig(configPath); err != nil {
		t.Fatalf("save config: %v", err)
	}
	keyPath, err := getEncryptionKeyPath()
	if err != nil {
		t.Fatalf("get encryption key path: %v", err)
	}
	if err := os.Remove(keyPath); err != nil {
		t.Fatalf("remove encryption key: %v", err)
	}

	if _, err := LoadConfig(configPath); err == nil {
		t.Fatalf("LoadConfig succeeded without encryption key")
	}
	if _, err := os.Stat(keyPath); !os.IsNotExist(err) {
		t.Fatalf("encrypted load without key should not create a new key, stat err=%v", err)
	}
}
