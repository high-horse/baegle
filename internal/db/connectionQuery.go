package db

import (
	"database/sql"
	"db-lens/internal/engine/entities"
	"db-lens/internal/types"
	"fmt"
	"strconv"
	"strings"
)

func GetConnectionList() ([]types.Connection, error) {
	query := `
		SELECT
			c.id, c.name, c.driver, c.host, c.port, c.user, c.password, c.dbname, c.pinned, c.color, c.read_only, c.group_name, c.ssl,
			s.id, s.name, s.host, s.port, s.username, s.auth_method, s.private_key, s.passphrase, s.password
		FROM connections c
		LEFT JOIN ssh_configs s ON s.id = c.ssh_config_id
	`
	rows, err := Conn.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var connections []types.Connection
	for rows.Next() {
		var c types.Connection
		var sshID sql.NullInt64
		var sshName, sshHost, sshAuthMethod sql.NullString
		var sshPort sql.NullInt64
		var sshUsername sql.NullString

		if err := rows.Scan(
			&c.Id, &c.Name, &c.Driver, &c.Host, &c.Port, &c.User, &c.Password, &c.DBName, &c.Pinned, &c.Color, &c.ReadOnly, &c.Group, &c.SSL,
			&sshID, &sshName, &sshHost, &sshPort, &sshUsername, &sshAuthMethod,
			&c.SSHConfig.PrivateKey, &c.SSHConfig.Passphrase, &c.SSHConfig.Password,
		); err != nil {
			return nil, err
		}

		if sshID.Valid {
			c.SSHConfigId = sshID
			c.SSHConfig.Id = int(sshID.Int64)
			c.SSHConfig.Name = sshName.String
			c.SSHConfig.Host = sshHost.String
			c.SSHConfig.Port = int(sshPort.Int64)
			c.SSHConfig.Username = sshUsername.String
			c.SSHConfig.AuthMethod = sshAuthMethod.String
		}

		connections = append(connections, c)
	}
	return connections, rows.Err()
}

func StoreConnection(conn entities.ConnectionConfig) (int64, error) {
	tx, err := Conn.Begin()
	if err != nil {
		return 0, err
	}

	var sshConfigID any // nil or int64, goes straight into the query as NULL/value

	if conn.SSHConfig != nil {
		res, err := tx.Exec(
			`INSERT INTO ssh_configs (name, host, port, username, auth_method, private_key, passphrase, password)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			conn.SSHConfig.Name,
			conn.SSHConfig.Host,
			conn.SSHConfig.Port,
			conn.SSHConfig.Username,
			conn.SSHConfig.AuthMethod,
			nullable(conn.SSHConfig.PrivateKey),
			nullable(conn.SSHConfig.Passphrase),
			nullable(conn.SSHConfig.Password),
		)
		if err != nil {
			tx.Rollback()
			return 0, err
		}
		id, err := res.LastInsertId()
		if err != nil {
			tx.Rollback()
			return 0, err
		}
		sshConfigID = id
	}

	res, err := tx.Exec(
		`INSERT INTO connections (name, driver, host, port, user, password, dbname, pinned, color, ssh_config_id, read_only, group_name, ssl)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		conn.Name,
		conn.Type,
		conn.Host,
		conn.Port,
		conn.User,
		conn.Password,
		conn.Database,
		false,
		nullable(conn.Color),
		sshConfigID,
		conn.ReadOnly,
		strings.TrimSpace(conn.Group),
		conn.SSL,
	)
	if err != nil {
		tx.Rollback()
		return 0, err
	}

	newID, err := res.LastInsertId()
	if err != nil {
		tx.Rollback()
		return 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}

	return newID, nil
}

func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func DeleteConnection(id int) error {
	if id < 1 {
		return fmt.Errorf("invalid saved connection ID")
	}
	tx, err := Conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var sshID sql.NullInt64
	if err := tx.QueryRow("SELECT ssh_config_id FROM connections WHERE id = ?", id).Scan(&sshID); err != nil {
		return fmt.Errorf("saved connection not found: %w", err)
	}
	if _, err := tx.Exec("DELETE FROM connections WHERE id = ?", id); err != nil {
		return err
	}
	// SQLite foreign-key enforcement may be disabled on an existing database.
	if _, err := tx.Exec("DELETE FROM saved_consoles WHERE connection_id = ?", id); err != nil {
		return err
	}
	if sshID.Valid {
		if _, err := tx.Exec(`DELETE FROM ssh_configs WHERE id = ? AND NOT EXISTS (SELECT 1 FROM connections WHERE ssh_config_id = ?)`, sshID.Int64, sshID.Int64); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// UpdateConnection keeps the saved connection identity and query history intact.
func UpdateConnection(config entities.ConnectionConfig) error {
	id, err := strconv.ParseInt(config.ID, 10, 64)
	if err != nil || id < 1 {
		return fmt.Errorf("invalid saved connection ID")
	}
	tx, err := Conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var previousSSH sql.NullInt64
	if err := tx.QueryRow("SELECT ssh_config_id FROM connections WHERE id = ?", id).Scan(&previousSSH); err != nil {
		return fmt.Errorf("saved connection not found: %w", err)
	}
	var sshID any
	if config.SSHConfig != nil {
		// A separate record avoids changing another connection sharing this SSH config.
		ssh := config.SSHConfig
		result, err := tx.Exec(`INSERT INTO ssh_configs (name, host, port, username, auth_method, private_key, passphrase, password) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, ssh.Name, ssh.Host, ssh.Port, ssh.Username, ssh.AuthMethod, nullable(ssh.PrivateKey), nullable(ssh.Passphrase), nullable(ssh.Password))
		if err != nil {
			return err
		}
		sshID, err = result.LastInsertId()
		if err != nil {
			return err
		}
	}
	_, err = tx.Exec(`UPDATE connections SET name = ?, driver = ?, host = ?, port = ?, user = ?, password = ?, dbname = ?, color = ?, ssh_config_id = ?, read_only = ?, group_name = ?, ssl = ? WHERE id = ?`, config.Name, config.Type, config.Host, config.Port, config.User, config.Password, config.Database, nullable(config.Color), sshID, config.ReadOnly, strings.TrimSpace(config.Group), config.SSL, id)
	if err != nil {
		return err
	}
	if previousSSH.Valid {
		if _, err := tx.Exec(`DELETE FROM ssh_configs WHERE id = ? AND NOT EXISTS (SELECT 1 FROM connections WHERE ssh_config_id = ?)`, previousSSH.Int64, previousSSH.Int64); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// SetConnectionGroup changes organization without rewriting connection credentials.
func SetConnectionGroup(id int, group string) error {
	if id < 1 {
		return fmt.Errorf("invalid saved connection ID")
	}
	result, err := Conn.Exec("UPDATE connections SET group_name = ? WHERE id = ?", strings.TrimSpace(group), id)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return fmt.Errorf("saved connection not found")
	}
	return nil
}
