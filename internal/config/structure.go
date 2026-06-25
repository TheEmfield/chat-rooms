package config

import "time"

type Config struct {
	Logger Logger `yaml:"logger"`
	HTTP   HTTP   `yaml:"server"`
}

type Logger struct {
	Level  string `yaml:"level"  env:"LOG_LEVEL"  env-default:"info"`
	Format string `yaml:"format" env:"LOG_FORMAT" env-default:"json"`
}

type HTTP struct {
	Host            string        `yaml:"host"             env:"HTTP_HOST"             env-default:"localhost"`
	Port            string        `yaml:"port"             env:"HTTP_PORT"             env-default:"8080"`
	NumberRooms     int           `yaml:"number_rooms"     env:"HTTP_NUMBER_ROOMS"     env-default:"3"`
	NumberClients   int           `yaml:"number_clients"   env:"HTTP_NUMBER_CLIENTS"   env-default:"5"`
	NumberMessages  int           `yaml:"number_messages"  env:"HTTP_NUMBER_MESSAGES"  env-default:"50"`
	ShutdownTimeout time.Duration `yaml:"shutdown_timeout" env:"HTTP_SHUTDOWN_TIMEOUT" env-default:"20s"`
}
