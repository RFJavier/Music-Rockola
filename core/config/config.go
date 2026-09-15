// Package config carga la configuración de la aplicación desde
// variables de entorno, con valores por defecto sensatos para el MVP.
package config

import "os"

// Config agrupa la configuración del Core.
type Config struct {
	// DBPath es la ruta del archivo SQLite. Env: ROCKOLA_DB_PATH.
	DBPath string
	// LogLevel: "debug" | "info" | "warn" | "error". Env: ROCKOLA_LOG_LEVEL.
	LogLevel string
	// HTTPAddr es la dirección local de la API. Env: ROCKOLA_HTTP_ADDR.
	HTTPAddr string
}

// Load lee la configuración desde variables de entorno.
func Load() Config {
	return Config{
		DBPath:   getEnv("ROCKOLA_DB_PATH", "rockola.db"),
		LogLevel: getEnv("ROCKOLA_LOG_LEVEL", "info"),
		HTTPAddr: getEnv("ROCKOLA_HTTP_ADDR", "127.0.0.1:8080"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
