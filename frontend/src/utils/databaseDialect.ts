import { PostgreSQL, MySQL, SQLite, MSSQL, PLSQL, StandardSQL, Cassandra } from '@codemirror/lang-sql';
export type DatabaseDriver = 'pgx' | 'mysql' | 'sqlite' | 'mongodb' | 'oracle' | 'sqlserver' | 'db2' | 'cassandra' | 'couchdb' | 'redis';
export const isNoSQL = (driver?: string) => ['mongodb', 'cassandra', 'couchdb', 'redis'].includes(driver ?? '');
export const isJSONDriver = (driver?: string) => ['mongodb', 'couchdb', 'redis'].includes(driver ?? '');
export const consoleExtension = (driver?: string) => isJSONDriver(driver) ? 'json' : driver === 'cassandra' ? 'cql' : 'sql';
export function editorDialect(driver?: string) {
  switch (driver) {
    case 'mysql': return MySQL;
    case 'sqlite': return SQLite;
    case 'sqlserver': return MSSQL;
    case 'oracle': return PLSQL;
    case 'db2': return StandardSQL;
    case 'cassandra': return Cassandra;
    default: return PostgreSQL;
  }
}

// Use the SQL lexer so semicolons in strings, quoted identifiers, and comments
// do not turn a single query into a script. The backend handles script splitting
// and dialect-specific batches on one connection.
export function hasMultipleSQLStatements(text: string, driver?: string): boolean {
  if (isNoSQL(driver)) return false;
  let count = 0;
  for (const statement of editorDialect(driver).language.parser.parse(text).topNode.getChildren('Statement')) {
    let hasSQL = false;
    for (let child = statement.firstChild; child; child = child.nextSibling) {
      if (![';', 'LineComment', 'BlockComment'].includes(child.name)) {
        hasSQL = true;
        break;
      }
    }
    if (hasSQL && ++count > 1) return true;
  }
  return false;
}
export function formatterDialect(driver: DatabaseDriver) {
  switch (driver) {
    case 'pgx': return 'postgresql';
    case 'oracle': return 'plsql';
    case 'sqlserver': return 'transactsql';
    case 'mongodb': case 'cassandra': case 'couchdb': case 'redis': return 'sql';
    default: return driver;
  }
}
