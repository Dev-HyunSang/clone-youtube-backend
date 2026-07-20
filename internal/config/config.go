package config

import (
	"os"

	"github.com/joho/godotenv"
)

func GetEnv(key string) string {
	// ponytail: .env.dev is the only env file today; falls back to real OS env vars if missing (e.g. in prod)
	godotenv.Load(".env.dev")

	return os.Getenv(key)
}
