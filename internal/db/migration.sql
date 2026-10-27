CREATE TABLE IF NOT EXISTS ssh_configs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,

    host TEXT NOT NULL,
    port INTEGER NOT NULL DEFAULT 22,
    username TEXT NOT NULL,

    auth_method TEXT NOT NULL DEFAULT 'private_key',

    private_key TEXT,
    passphrase TEXT,
    password TEXT
);


CREATE TABLE IF NOT EXISTS connections (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT,
    driver TEXT,
    host TEXT,
    port INTEGER,
    user TEXT,
    password TEXT,
    dbname TEXT,
    pinned BOOLEAN DEFAULT false,
    read_only BOOLEAN NOT NULL DEFAULT false,
    ssl BOOLEAN NOT NULL DEFAULT false,
    color TEXT,
    group_name TEXT NOT NULL DEFAULT '',

    ssh_config_id INTEGER,
    
    FOREIGN KEY (ssh_config_id) REFERENCES ssh_configs(id) ON DELETE SET NULL
);

CREATE TABLE IF NOT EXISTS users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT,
    password TEXT
);

CREATE TABLE IF NOT EXISTS query_history (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    connection_id TEXT NOT NULL,
    database_name TEXT NOT NULL,
    query_text TEXT NOT NULL,
    executed_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    duration_ms INTEGER NOT NULL,
    status TEXT NOT NULL -- 'SUCCESS' or 'ERROR'
);

CREATE TABLE IF NOT EXISTS app_preferences (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS saved_consoles (
    connection_id INTEGER NOT NULL,
    id TEXT NOT NULL,
    title TEXT NOT NULL,
    sql_text TEXT NOT NULL,
    position INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL,
    tab_state TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (connection_id, id),
    FOREIGN KEY (connection_id) REFERENCES connections(id) ON DELETE CASCADE
);


-- INSERT INTO connections (
--     name, driver, host, port, user, password, dbname
-- )
-- VALUES
-- (
--     'Local SQLite', 'sqlite', '', NULL, '', '','/home/camel/Desktop/go/chaarm/db-viewer/static/app.db'
-- ),
-- (
    --     'dvdrental', 'pgx', 'localhost', 5432, 'postgres', 'your_password', 'dvdrental'
-- ),
-- (
--     'Local MySQL', 'mysql', '127.0.0.1', 3306, 'app_user', 'strong_password', 'app_database'
-- );

-- INSERT INTO connections (name, driver, host, port, user, password, dbname)
-- VALUES
--     ('Local SQLite', 'sqlite', '', NULL, '', '', 'app.db'),
--     ('Local PostgreSQL', 'postgres', 'localhost', 5432, 'postgres', 'password123', 'myapp'),
--     ('Development MySQL', 'mysql', '127.0.0.1', 3306, 'root', 'root123', 'dev_db'),
--     ('Production MariaDB', 'mariadb', '192.168.1.100', 3306, 'admin', 'securepass', 'production'),
--     ('SQL Server', 'sqlserver', 'db-server.local', 1433, 'sa', 'StrongPass123!', 'SalesDB'),
--     ('Oracle XE', 'oracle', 'localhost', 1521, 'system', 'oracle123', 'XE'),
--     ('Local MariaDB', 'mariadb', 'localhost', 3306, 'app_user', 'strong_password', 'app_database');
