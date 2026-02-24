package config

import (
	"os"
	"strconv"
)

type Config struct {
	GRPCHost    string
	GRPCPort    int
	LevelDBPath string
}

func Load() Config {
	host := os.Getenv("SIGN_GRPC_HOST")
	if host == "" {
		host = "0.0.0.0"
	}

	port := 9091
	if p := os.Getenv("SIGN_GRPC_PORT"); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil {
			port = parsed
		}
	}

	levelPath := os.Getenv("SIGN_LEVELDB_PATH")
	if levelPath == "" {
		levelPath = "./data/sign-leveldb"
	}

	return Config{
		GRPCHost:    host,
		GRPCPort:    port,
		LevelDBPath: levelPath,
	}
}
