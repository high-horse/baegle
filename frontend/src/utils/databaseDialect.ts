import { PostgreSQL, MySQL, SQLite, MSSQL, PLSQL, StandardSQL } from '@codemirror/lang-sql';
export type DatabaseDriver = 'pgx' | 'mysql' | 'sqlite' | 'mongodb' | 'oracle' | 'sqlserver' | 'db2';
export function editorDialect(driver?: string) {
  switch (driver) {
    case 'mysql': return MySQL;
    case 'sqlite': return SQLite;
    case 'sqlserver': return MSSQL;
    case 'oracle': return PLSQL;
    case 'db2': return StandardSQL;
    default: return PostgreSQL;
  }
}
export function formatterDialect(driver: DatabaseDriver) {
  switch (driver) {
    case 'pgx': return 'postgresql';
    case 'oracle': return 'plsql';
    case 'sqlserver': return 'transactsql';
    case 'mongodb': return 'sql';
    default: return driver;
  }
}
