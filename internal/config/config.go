package config

import (
	"os"
	"strconv"
)

type Config struct {
	Env         string
	HTTPAddr    string
	PostgresDSN string
	AuthN       AuthNConfig
	AuthZ       AuthZConfig
}

type AuthNConfig struct {
	Provider string // v0: "noop"
}

type AuthZConfig struct {
	Provider  string // v0: "noop"
	NoopAllow bool
}

func FromEnv() Config {
	return Config{
		Env:         getEnv("CONTRATO_ENV", "dev"),
		HTTPAddr:    getEnv("CONTRATO_HTTP_ADDR", ":8080"),
		PostgresDSN: getEnv("CONTRATO_PG_DSN", ""),
		AuthN:       AuthNConfig{Provider: getEnv("CONTRATO_AUTHN_PROVIDER", "noop")},
		AuthZ: AuthZConfig{
			Provider:  getEnv("CONTRATO_AUTHZ_PROVIDER", "noop"),
			NoopAllow: getEnvBool("CONTRATO_AUTHZ_NOOP_ALLOW", false),
		},
	}
}

func getEnv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func getEnvBool(k string, def bool) bool {
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
}
