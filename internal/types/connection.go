package types

import "database/sql"

type SSHConfig struct {
	Id         int            `json:"id"`
	Name       string         `json:"name"`
	Host       string         `json:"host"`
	Port       int            `json:"port"`
	Username   string         `json:"username"`
	AuthMethod string         `json:"auth_method"`
	PrivateKey sql.NullString `json:"private_key"`
	Passphrase sql.NullString `json:"passphrase"`
	Password   sql.NullString `json:"password"`
}

type Connection struct {
	Group       string         `json:"group"`
	Id          int            `json:"id"`
	Name        string         `json:"name"`
	Driver      string         `json:"driver"`
	Host        string         `json:"host"`
	Port        sql.NullInt64  `json:"port"`
	User        string         `json:"user"`
	Password    string         `json:"password"`
	DBName      string         `json:"dbname"`
	ReadOnly    bool           `json:"read_only"`
	SSL         bool           `json:"ssl"`
	Pinned      bool           `json:"pinned"`
	Color       sql.NullString `json:"color"`
	SSHConfigId sql.NullInt64  `json:"ssh_config_id"`
	SSHConfig   SSHConfig      `json:"ssh_config"`
}
