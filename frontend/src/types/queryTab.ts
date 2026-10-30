export interface QueryColumn {
  Name: string;
  Type?: string;
  Nullable?: boolean;
  DefaultValue?: string | number;
}


export interface QueryResult {
  Cursor: string;
  HasMore: boolean;
  StartRow: number;
  PageSize: number;
  IsQuery: boolean;
  CanNavigate: boolean;
  CanSort?: boolean;
  Duration: number;
  Columns: QueryColumn[];
  Rows: Array<Array<unknown>>;
  Documents?: string[];
}

export type QueryTabType = "query" | "result"
export interface ScriptStatementResult {
  statement: number;
  line: number;
  preview: string;
  duration: number;
  rowsAffected: number | null;
  results: { columns: string[]; rows: unknown[][]; truncated: boolean }[];
  error: string;
}

export interface ScriptProgress {
  requestID: string;
  completed: number;
  total: number;
  current: number;
  line: number;
  duration: number;
}

export interface QueryTab {
  id: string;
  title: string;
  type: QueryTabType;
  
  sql: string;
  result: QueryResult | null;

  loading: boolean;
  runningQuery?: boolean;
  runningScript?: boolean;
  importedScript?: boolean;
  scriptResult?: { completed: number; total: number; error: string; errorLine: number; cancelled: boolean; duration: number; statements?: ScriptStatementResult[] };
  scriptProgress?: ScriptProgress;
  queryStartedAt?: number;
  stopping?: boolean;
  error: string | null;
  dirty: boolean;
  savedSql?: string;
  savingConsole?: boolean;
  closing?: boolean;

  createdAt: number;

  connectionId?: string;
  tableRef?: { name: string; schema: string; database: string };
  needsTableRestore?: boolean;
  pages?: QueryResult[];
  pageIndex?: number;
  cursor?: string;
  executedSql?: string;
  pageError?: string;
  totalRows?: number;
  countingRows?: boolean;
  stoppingCount?: boolean;
  countError?: string;
  fetchingLast?: boolean;
  streamNextPage?: number;
  sortColumn?: number;
  sortDirection?: "asc" | "desc";
  resultWhere?: string;
  resultOrder?: string;
  resultOrderSQL?: string;
}
