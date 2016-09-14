package server

import "flag"

var (
	fAddr = flag.String("addr", "localhost:10443", "host:port for MadSquid")
	fCert = flag.String("cert", "server.crt", "Certificate file")
	fKey  = flag.String("keyfile", "server.key", "Key file")
)

type Config struct {
	Addr string
	Cert string
	Key  string
}

func GetConfig() *Config {
	flag.Parse()
	return &Config{
		Addr: *fAddr,
		Cert: *fCert,
		Key:  *fKey,
	}
}
