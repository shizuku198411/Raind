package utils

import "os"

const (
	PublicCertPath    = "/etc/raind/cert/raind.crt"
	PrivateKeyPath    = "/etc/raind/cert/raind.key"
	ClientCertPath    = "/etc/raind/cert/raindClient.crt"
	ClientKeyPath     = "/etc/raind/cert/raindClient.key"
	CliCACertPath     = "/etc/raind/cli/ca.crt"
	CliCertPath       = "/etc/raind/cli/client.crt"
	CliKeyPath        = "/etc/raind/cli/client.key"
	EnvCACertPath     = "RAIND_CA_CERT"
	EnvClientCertPath = "RAIND_CLIENT_CERT"
	EnvClientKeyPath  = "RAIND_CLIENT_KEY"
)

type ClientCertPaths struct {
	CA     string
	Cert   string
	Key    string
	Legacy bool
}

func ResolveClientCertPaths() ClientCertPaths {
	if ca := os.Getenv(EnvCACertPath); ca != "" {
		return ClientCertPaths{
			CA:   ca,
			Cert: getenvDefault(EnvClientCertPath, CliCertPath),
			Key:  getenvDefault(EnvClientKeyPath, CliKeyPath),
		}
	}
	if cert := os.Getenv(EnvClientCertPath); cert != "" {
		return ClientCertPaths{
			CA:   CliCACertPath,
			Cert: cert,
			Key:  getenvDefault(EnvClientKeyPath, CliKeyPath),
		}
	}
	if key := os.Getenv(EnvClientKeyPath); key != "" {
		return ClientCertPaths{
			CA:   CliCACertPath,
			Cert: CliCertPath,
			Key:  key,
		}
	}

	if fileExists(CliCertPath) || fileExists(CliKeyPath) || fileExists(CliCACertPath) {
		return ClientCertPaths{CA: CliCACertPath, Cert: CliCertPath, Key: CliKeyPath}
	}

	return ClientCertPaths{CA: PublicCertPath, Cert: ClientCertPath, Key: ClientKeyPath, Legacy: true}
}

func getenvDefault(key string, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil || os.IsPermission(err)
}
