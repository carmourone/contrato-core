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
	License     LicenseConfig
}

type AuthNConfig struct {
	Provider string // v0: "noop"
}

type AuthZConfig struct {
	Provider  string // v0: "noop"
	NoopAllow bool
}

// LicenseConfig controls which product personas are active.
// Key="" enables all features (developer mode).
// A non-empty key will be validated against an embedded public key; the
// signed payload declares which personas are permitted.
type LicenseConfig struct {
	Key string
}

// Features is the resolved set of enabled personas derived from the license.
type Features struct {
	Frontier bool
	Waypoint bool
	Meridian bool
}

func (lc LicenseConfig) Features() Features {
	if lc.Key == "" {
		// developer mode: all personas enabled
		return Features{Frontier: true, Waypoint: true, Meridian: true}
	}
	// TODO: parse and verify JWT license key, extract persona claims
	return Features{Frontier: true, Waypoint: true, Meridian: true}
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
		License: LicenseConfig{
			Key: getEnv("CONTRATO_LICENSE_KEY", ""),
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
