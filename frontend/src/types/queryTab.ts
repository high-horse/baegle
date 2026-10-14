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
  scriptResult?: { completed: number; total: number; error: string; errorLine: number; cancelled: boolean; duration: number };
  queryStartedAt?: number;
  stopping?: boolean;
  error: string | null;
  dirty: boolean;
  savedSql?: string;
  savingConsole?: boolean;
  closing?: boolean;

  createdAt: number;

  connectionId?: string;
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
}
