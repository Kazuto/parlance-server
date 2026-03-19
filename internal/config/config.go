package config

import (
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	JWT      JWTConfig
	DeepL    DeepLConfig
	CORS     CORSConfig
}

type ServerConfig struct {
	Port string
	Env  string
}

type DatabaseConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	Name     string
	SSLMode  string
}

type JWTConfig struct {
	Secret string
	Expiry string
}

type DeepLConfig struct {
	APIKey string
	APIURL string
}

type CORSConfig struct {
	AllowedOrigins []string
}

func Load() *Config {
	// Load .env file if it exists (ignore errors in production)
	_ = godotenv.Load()

	port := getEnv("PORT", "8080")
	dbPort, _ := strconv.Atoi(getEnv("DB_PORT", "5432"))

	return &Config{
		Server: ServerConfig{
			Port: port,
			Env:  getEnv("ENV", "development"),
		},
		Database: DatabaseConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     dbPort,
			User:     getEnv("DB_USER", "parlance"),
			Password: getEnv("DB_PASSWORD", "secret"),
			Name:     getEnv("DB_NAME", "parlance"),
			SSLMode:  getEnv("DB_SSL_MODE", "disable"),
		},
		JWT: JWTConfig{
			Secret: getEnv("JWT_SECRET", "change-this-secret"),
			Expiry: getEnv("JWT_EXPIRY", "24h"),
		},
		DeepL: DeepLConfig{
			APIKey: getEnv("DEEPL_API_KEY", ""),
			APIURL: getEnv("DEEPL_API_URL", "https://api.deepl.com/v2"),
		},
		CORS: CORSConfig{
			AllowedOrigins: parseCommaSeparated(getEnv("CORS_ALLOWED_ORIGINS", "http://localhost:3000")),
		},
	}
}

func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

func parseCommaSeparated(value string) []string {
	if value == "" {
		return []string{}
	}

	var result []string
	current := ""
	for _, char := range value {
		if char == ',' {
			if current != "" {
				result = append(result, current)
				current = ""
			}
		} else if char != ' ' {
			current += string(char)
		}
	}
	if current != "" {
		result = append(result, current)
	}

	return result
}

func (c *Config) IsDevelopment() bool {
	return c.Server.Env == "development"
}

func (c *Config) IsProduction() bool {
	return c.Server.Env == "production"
}
