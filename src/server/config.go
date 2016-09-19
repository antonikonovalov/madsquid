package server

import "flag"

var (
	fTLS  = flag.Bool("tls", true, "Use TLS")
	fAddr = flag.String("addr", ":10443", "host:port for MadSquid")
	fCert = flag.String("cert", "server.crt", "Certificate file")
	fKey  = flag.String("keyfile", "server.key", "Key file")
)

type Config struct {
	TLS  bool
	Addr string
	Cert string
	Key  string
}

func GetConfig() *Config {
	flag.Parse()
	return &Config{
		TLS:  *fTLS,
		Addr: *fAddr,
		Cert: *fCert,
		Key:  *fKey,
	}
}
