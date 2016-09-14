package server

import "flag"

var (
	fAddr = flag.String("addr", "localhost:8080", "host:port for MadSquid")
)

type Config struct {
	Addr string
}

func GetConfig() *Config {
	flag.Parse()
	return &Config{
		Addr: *fAddr,
	}
}
