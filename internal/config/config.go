package config

import (
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	AppEnv               string
	AppPort              string
	AppKey               string
	AdminUsername        string
	AdminPassword        string
	DBConnection         string
	DBHost               string
	DBPort               string
	DBDatabase           string
	DBUsername           string
	DBPassword           string
	DBSSLMode            string
	SearchEngineEndpoint string
	OpenAIEndpoint       string
	OpenAIKey            string
	OpenAIModel          string
}

func LoadConfig(envFiles ...string) *Config {
	_ = godotenv.Load(envFiles...)

	return &Config{
		AppEnv:               getEnv("APP_ENV", "development"),
		AppPort:              getEnv("APP_PORT", "8081"),
		AppKey:               getEnv("APP_KEY", "default_secret_key"),
		AdminUsername:        getEnv("ADMIN_USERNAME", "admin"),
		AdminPassword:        getEnv("ADMIN_PASSWORD", "admin"),
		DBConnection:         getEnv("DB_CONNECTION", "pgsql"),
		DBHost:               getEnv("DB_HOST", "127.0.0.1"),
		DBPort:               getEnv("DB_PORT", "5432"),
		DBDatabase:           getEnv("DB_DATABASE", "pabwe_p9_blog"),
		DBUsername:           getEnv("DB_USERNAME", "postgres"),
		DBPassword:           getEnv("DB_PASSWORD", ""),
		DBSSLMode:            getEnv("DB_SSLMODE", "disable"),
		SearchEngineEndpoint: getEnv("SEARCH_ENGINE_ENDPOINT", "https://delcom.org/api/v1/search"),
		OpenAIEndpoint:       getEnv("OPENAI_ENDPOINT", "https://delcom.org/api/v1"),
		OpenAIKey:            getEnv("OPENAI_KEY", ""),
		OpenAIModel:          getEnv("OPENAI_MODEL", "gpt-3.5-turbo"),
	}
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists && value != "" {
		return value
	}
	return fallback
}
