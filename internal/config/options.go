package config

import (
	"log"
	"os"

	"go.yaml.in/yaml/v3"
)

const (
	VERSION     = "0.1.0"
	CONFIG_FILE = "/etc/go-video-daemon/config.yaml"
)

type LibraryConfig struct {
	Database string
	Movies   string
	Tvshows  string
}

type PlayerConfig struct {
	Socket string
}

type VideoDaemonServerConfig struct {
	Listen  string
	Player  PlayerConfig
	Library LibraryConfig
}

var (
	ServerConfig   = VideoDaemonServerConfig{}
	INITIAL_CONFIG = `
listen: "localhost:10123"
player:
  socket: /tmp/mpv-ipc-socket
library:
  database: "db/go-video-daemon.db"
  movies: "/Users/mroyer/Movies/Films"
  tvshows: "/Users/mroyer/Movies/Series"
`
)

func InitServerConfig() {
	err := yaml.Unmarshal([]byte(INITIAL_CONFIG), &ServerConfig)
	if err != nil {
		log.Fatalf("Unable to initialize server config: %v", err)
	}
}

func CreateServerConfig(config string) error {
	return os.WriteFile(config, []byte(INITIAL_CONFIG), 0644)
}

func ParseServerConfig(config string) error {
	data, err := os.ReadFile(config)
	if err != nil {
		return err
	}

	return yaml.Unmarshal(data, &ServerConfig)
}
