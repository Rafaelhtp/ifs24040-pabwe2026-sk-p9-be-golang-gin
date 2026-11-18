package config

import (
	"os"
	"path/filepath"
	"testing"
)

func assertEqual(t *testing.T, expected, actual string) {
	t.Helper()
	if expected != actual {
		t.Fatalf("expected %q, got %q", expected, actual)
	}
}

func TestLoadConfig_Defaults(t *testing.T) {
	keys := []string{
		"APP_ENV", "APP_PORT", "APP_KEY",
		"ADMIN_USERNAME", "ADMIN_PASSWORD",
		"DB_CONNECTION", "DB_HOST", "DB_PORT", "DB_DATABASE", "DB_USERNAME", "DB_PASSWORD", "DB_SSLMODE",
		"SEARCH_ENGINE_ENDPOINT", "OPENAI_ENDPOINT", "OPENAI_KEY", "OPENAI_MODEL",
	}
	for _, k := range keys {
		t.Setenv(k, "")
		_ = os.Unsetenv(k)
	}

	cfg := LoadConfig("non_existent.env")

	assertEqual(t, "development", cfg.AppEnv)
	assertEqual(t, "8081", cfg.AppPort)
	assertEqual(t, "default_secret_key", cfg.AppKey)
	assertEqual(t, "admin", cfg.AdminUsername)
	assertEqual(t, "admin", cfg.AdminPassword)
	assertEqual(t, "pgsql", cfg.DBConnection)
	assertEqual(t, "127.0.0.1", cfg.DBHost)
	assertEqual(t, "5432", cfg.DBPort)
	assertEqual(t, "pabwe_p9_blog", cfg.DBDatabase)
	assertEqual(t, "postgres", cfg.DBUsername)
	assertEqual(t, "", cfg.DBPassword)
	assertEqual(t, "disable", cfg.DBSSLMode)
	assertEqual(t, "https://delcom.org/api/v1/search", cfg.SearchEngineEndpoint)
	assertEqual(t, "https://delcom.org/api/v1", cfg.OpenAIEndpoint)
	assertEqual(t, "", cfg.OpenAIKey)
	assertEqual(t, "gpt-3.5-turbo", cfg.OpenAIModel)
}

func TestLoadConfig_CustomEnvAndDotEnvFile(t *testing.T) {
	tmpDir := t.TempDir()
	envPath := filepath.Join(tmpDir, ".env")
	envContent := "APP_ENV=production\nAPP_PORT=9090\nAPP_KEY=my_secret\nADMIN_USERNAME=superadmin\nADMIN_PASSWORD=superpass\nDB_CONNECTION=sqlite\nDB_HOST=localhost\nDB_PORT=5433\nDB_DATABASE=test_db\nDB_USERNAME=dbuser\nDB_PASSWORD=dbpass\nDB_SSLMODE=require\nSEARCH_ENGINE_ENDPOINT=http://localhost:8000/search\nOPENAI_ENDPOINT=http://localhost:8000/v1\nOPENAI_KEY=sk-test\nOPENAI_MODEL=gpt-4o\n"
	if err := os.WriteFile(envPath, []byte(envContent), 0600); err != nil {
		t.Fatalf("failed writing temp env file: %v", err)
	}

	keys := []string{
		"APP_ENV", "APP_PORT", "APP_KEY",
		"ADMIN_USERNAME", "ADMIN_PASSWORD",
		"DB_CONNECTION", "DB_HOST", "DB_PORT", "DB_DATABASE", "DB_USERNAME", "DB_PASSWORD", "DB_SSLMODE",
		"SEARCH_ENGINE_ENDPOINT", "OPENAI_ENDPOINT", "OPENAI_KEY", "OPENAI_MODEL",
	}
	for _, k := range keys {
		t.Setenv(k, "")
		_ = os.Unsetenv(k)
	}

	cfg := LoadConfig(envPath)

	assertEqual(t, "production", cfg.AppEnv)
	assertEqual(t, "9090", cfg.AppPort)
	assertEqual(t, "my_secret", cfg.AppKey)
	assertEqual(t, "superadmin", cfg.AdminUsername)
	assertEqual(t, "superpass", cfg.AdminPassword)
	assertEqual(t, "sqlite", cfg.DBConnection)
	assertEqual(t, "localhost", cfg.DBHost)
	assertEqual(t, "5433", cfg.DBPort)
	assertEqual(t, "test_db", cfg.DBDatabase)
	assertEqual(t, "dbuser", cfg.DBUsername)
	assertEqual(t, "dbpass", cfg.DBPassword)
	assertEqual(t, "require", cfg.DBSSLMode)
	assertEqual(t, "http://localhost:8000/search", cfg.SearchEngineEndpoint)
	assertEqual(t, "http://localhost:8000/v1", cfg.OpenAIEndpoint)
	assertEqual(t, "sk-test", cfg.OpenAIKey)
	assertEqual(t, "gpt-4o", cfg.OpenAIModel)
}
