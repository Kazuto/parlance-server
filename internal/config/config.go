package config

import (
	"log"
	"net/netip"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	S3       S3Config
	JWT      JWTConfig
	Secrets  SecretsConfig
	CORS     CORSConfig
}

type ServerConfig struct {
	Port string
	Env  string
	// TrustedProxies are the proxies whose X-Forwarded-For header tells where a
	// request came from.
	TrustedProxies []netip.Prefix
}

type DatabaseConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	Name     string
	SSLMode  string
}

type S3Config struct {
	AccessKey string
	SecretKey string
	Endpoint  string
	Region    string
	Bucket    string
}

type JWTConfig struct {
	Secret string
	Expiry string
}

// SecretsConfig holds the key that encrypts stored secrets, such as the API keys
// of translation servers. Without it, such secrets can't be saved.
type SecretsConfig struct {
	EncryptionKey string
}

type CORSConfig struct {
	AllowedOrigins []string
}

func Load() *Config {
	// Load .env file if it exists (ignore errors in production)
	_ = godotenv.Load()

	port := getEnv("PORT", "8080")
	dbPort := 5432

	if dbPortStr := getEnv("DB_PORT", "5432"); dbPortStr != "" {
		if parsed, err := strconv.Atoi(dbPortStr); err == nil {
			dbPort = parsed
		}
	}

	return &Config{
		Server: ServerConfig{
			Port:           port,
			Env:            getEnv("ENV", "development"),
			TrustedProxies: parsePrefixes("TRUSTED_PROXIES", getEnv("TRUSTED_PROXIES", "")),
		},
		Database: DatabaseConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     dbPort,
			User:     getEnv("DB_USER", "parlance"),
			Password: getEnv("DB_PASSWORD", "secret"),
			Name:     getEnv("DB_NAME", "parlance"),
			SSLMode:  getEnv("DB_SSL_MODE", "disable"),
		},
		S3: S3Config{
			AccessKey: getEnv("S3_ACCESS_KEY", ""),
			SecretKey: getEnv("S3_SECRET_KEY", ""),
			Endpoint:  getEnv("S3_ENDPOINT", ""),
			Region:    getEnv("S3_REGION", ""),
			Bucket:    getEnv("S3_BUCKET", ""),
		},
		JWT: JWTConfig{
			Secret: getEnv("JWT_SECRET", "change-this-secret"),
			Expiry: getEnv("JWT_EXPIRY", "24h"),
		},
		Secrets: SecretsConfig{
			EncryptionKey: getEnv("ENCRYPTION_KEY", ""),
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

// parsePrefixes reads comma-separated addresses and CIDR ranges, skipping the ones
// it can't read.
func parsePrefixes(name, value string) []netip.Prefix {
	var prefixes []netip.Prefix

	for _, item := range parseCommaSeparated(value) {
		prefix, err := parsePrefix(item)
		if err != nil {
			log.Printf("config: %s: skipping %q, which is no address or CIDR range", name, item)

			continue
		}

		prefixes = append(prefixes, prefix.Masked())
	}

	return prefixes
}

// parsePrefix reads a CIDR range, or an address as the range of just itself.
func parsePrefix(value string) (netip.Prefix, error) {
	if strings.Contains(value, "/") {
		return netip.ParsePrefix(value)
	}

	addr, err := netip.ParseAddr(value)
	if err != nil {
		return netip.Prefix{}, err
	}

	addr = addr.Unmap()

	return netip.PrefixFrom(addr, addr.BitLen()), nil
}

func (c *Config) IsDevelopment() bool {
	return c.Server.Env == "development"
}

func (c *Config) IsProduction() bool {
	return c.Server.Env == "production"
}
