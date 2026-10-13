package entities

type SSHConfig struct {
	ID         int
	Name       string
	Host       string
	Port       int
	Username   string
	AuthMethod string
	PrivateKey string
	Passphrase string
	Password   string
}

type ConnectionConfig struct {
	Group string
	ID    string
	Name  string
	Type  string

	Host string
	Port int

	User     string
	Password string
	Database string

	SSL bool

	SSHConfigID *int
	SSHConfig   *SSHConfig

	InMemory bool
	ReadOnly bool
	Color    string
}
