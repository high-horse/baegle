import { Dialog, Notify } from "quasar";
import { useTableEditsStore } from "@/stores/tableEditsStore";
import { defineStore } from "pinia";
import { computed, ref } from "vue";
import { useConnectionStore } from "@/stores/connectionStore";
import { useResultSettingsStore } from "@/stores/resultSettingsStore";
import type { QueryTab, QueryResult } from "@/types/queryTab";
import { DbService, DatabaseService } from "@bindings/db-lens/internal/app";
import { QueryExecutionType } from "@bindings/db-lens/internal/engine/entities";
import type { QueryResult as BackendResult } from "@bindings/db-lens/internal/engine/entities";

const initialTab: QueryTab = {
  id: "query-1",
  title: "console_1.sql",
  type: "query",
  
  sql: "",

  result: null,

  loading: false,

  error: null,

  dirty: false,

  createdAt: Date.now(),

  connectionId: undefined,
};

export const useQueryTabsStore = defineStore("queryTabs", () => {
  const tabs = ref<QueryTab[]>([{ ...initialTab, id: crypto.randomUUID() }]);
  const runningQueries = new Map<string, { requestID: string; stopping: boolean }>();
  const rowCounts = new Map<string, { requestID: string; cancelled: boolean; stopping: boolean }>();
  const pendingSaves = new Map<string, Promise<boolean>>();
  const restoringConsoles = ref(false);
  let restoreVersion = 0;

  const activeTabId = ref<string | null>(tabs.value[0].id);

  const activeTab = computed(() => {
    return tabs.value.find((tab) => tab.id === activeTabId.value) ?? null;
  });

  function createTab(connectionId?: string) {
    const id = crypto.randomUUID();

    const tab: QueryTab = {
      id,
      title: `console_${tabs.value.length + 1}.${useConnectionStore().activeConnection?.driver === "mongodb" ? "json" : "sql"}`,
      type: "query",
      sql: "",
      result: null,
      loading: false,
      error: null,
      dirty: false,
      createdAt: Date.now(),
      connectionId: connectionId ?? (useConnectionStore().activeConnection ? String(useConnectionStore().activeConnection!.id) : undefined),
    };

    tabs.value.push(tab);

    activeTabId.value = id;

    return tab;
  }

  function createResultTab(title: string, connectionId?: string) {
    const id = crypto.randomUUID();

    const tab: QueryTab = {
      id,
      title,
      type: "result",
      sql: "",
      result: null,
      loading: false,
      error: null,
      dirty: false,
      createdAt: Date.now(),
      connectionId,
    };

    tabs.value.push(tab);

    activeTabId.value = id;

    return tab;
  }

  function selectTab(id: string) {
    const exists = tabs.value.some((tab) => tab.id === id);

    if (!exists) {
      return;
    }

    activeTabId.value = id;
  }

  function moveTab(id: string, targetId: string, side: "before" | "after") {
    if (id === targetId) return;
    const from = tabs.value.findIndex(tab => tab.id === id);
    if (from < 0 || !tabs.value.some(tab => tab.id === targetId)) return;
    const [tab] = tabs.value.splice(from, 1);
    const target = tabs.value.findIndex(tab => tab.id === targetId);
    tabs.value.splice(target + (side === "after" ? 1 : 0), 0, tab);
  }

  async function closeTab(id: string, discard = false) {
    const edits = useTableEditsStore();
    if (edits.isSaving(id)) return;
    if (!discard && edits.hasPending(id)) {
      Dialog.create({ title: "Unsaved changes", message: "Discard the staged row changes and close this tab?", cancel: true, ok: "Discard" }).onOk(() => closeTab(id, true));
      return;
    }
    const tab = tabs.value.find(tab => tab.id === id);
    if (!tab || tab.closing) return;
    tab.closing = true;
    try {
      // Wait for an in-flight Save before deleting, so it cannot recreate a closed tab.
      const saving = pendingSaves.get(id);
      if (saving) await saving;
      if (tab.savedSql !== undefined && tab.connectionId) {
        await DatabaseService.DeleteSavedConsole(Number(tab.connectionId), id);
      }
    } catch (error) {
      tab.closing = false;
      Notify.create({ type: "negative", message: `Could not close saved console: ${String(error)}` });
      return;
    }
    const index = tabs.value.findIndex(tab => tab.id === id);
    if (index === -1) return;
    edits.clear(id);

    const wasActive = activeTabId.value === id;

    void cancelQuery(id);
    void cancelRowCount(id);

    releaseCursor(tabs.value[index].cursor);

    tabs.value.splice(index, 1);

    if (!wasActive) {
      return;
    }

    if (tabs.value.length === 0) {
      activeTabId.value = null;
      return;
    }

    const nextIndex = Math.min(index, tabs.value.length - 1);

    activeTabId.value = tabs.value[nextIndex].id;
  }

  function saveConsole(id: string): Promise<boolean> {
    const tab = tabs.value.find(tab => tab.id === id);
    if (!tab || tab.type !== "query" || tab.closing) return Promise.resolve(false);
    const pending = pendingSaves.get(id);
    if (pending) return pending.then(() => saveConsole(id));
    const connectionID = Number(tab.connectionId ?? useConnectionStore().activeConnection?.id);
    if (!Number.isSafeInteger(connectionID) || connectionID < 1) {
      Notify.create({ type: "warning", message: "Save this connection first to keep its consoles between sessions." });
      return Promise.resolve(false);
    }
    tab.connectionId = String(connectionID);
    const snapshot = { connectionId: connectionID, id: tab.id, title: tab.title, sql: tab.sql,
      position: tabs.value.indexOf(tab), createdAt: tab.createdAt };
    tab.savingConsole = true;
    const task = Promise.resolve().then(async () => {
      try {
        await DatabaseService.SaveConsole(snapshot);
        tab.savedSql = snapshot.sql;
        tab.dirty = tab.sql !== snapshot.sql;
        return true;
      } catch (error) {
        Notify.create({ type: "negative", message: `Could not save console: ${String(error)}` });
        return false;
      } finally {
        tab.savingConsole = false;
        pendingSaves.delete(id);
      }
    });
    pendingSaves.set(id, task);
    return task;
  }

  async function restoreConsoles(connectionId: string) {
    const version = ++restoreVersion;
    restoringConsoles.value = true;
    try {
      await Promise.all([...pendingSaves.values()]);
      const saved = Number(connectionId) > 0 ? await DatabaseService.GetSavedConsoles(Number(connectionId)) : [];
      if (version !== restoreVersion) return;
      // Keep any text entered while the saved tabs were loading.
      const existing = tabs.value.filter(tab => (!tab.connectionId || tab.connectionId === connectionId)
        && (tab.type !== "query" || tab.sql.length > 0 || tab.loading || tab.savedSql !== undefined));
      const restored: QueryTab[] = (saved ?? []).map(console => existing.find(tab => tab.id === console.id) ?? ({
        id: console.id, title: console.title, type: "query", sql: console.sql, savedSql: console.sql,
        connectionId, createdAt: console.createdAt, result: null, loading: false, error: null, dirty: false,
      }));
      tabs.value = [...restored, ...existing.filter(tab => !restored.some(saved => saved.id === tab.id))];
      for (const tab of tabs.value) tab.connectionId ??= connectionId;
      if (!tabs.value.length) createTab(connectionId);
      else if (!tabs.value.some(tab => tab.id === activeTabId.value)) activeTabId.value = tabs.value[0].id;
    } catch (error) {
      if (version !== restoreVersion) return;
      Notify.create({ type: "negative", message: `Could not restore saved consoles: ${String(error)}` });
      if (!tabs.value.length) createTab(connectionId);
    } finally {
      if (version === restoreVersion) restoringConsoles.value = false;
    }
  }

  function updateSql(id: string, sql: string) {
    const tab = tabs.value.find((tab) => tab.id === id);

    if (!tab) {
      return;
    }

    tab.sql = sql;
    tab.dirty = tab.savedSql === undefined ? sql.length > 0 : sql !== tab.savedSql;
  }

  function setResult(id: string, result: QueryResult) {
    const tab = tabs.value.find((tab) => tab.id === id);

    if (!tab) {
      return;
    }

    tab.result = result;
    tab.loading = false;
    tab.error = null;
    // Executing SQL does not save the console text.
  }

  function setLoading(id: string, loading: boolean) {
    const tab = tabs.value.find((tab) => tab.id === id);

    if (tab) {
      tab.loading = loading;
    }
  }

  function setError(id: string, error: string) {
    const tab = tabs.value.find((tab) => tab.id === id);

    if (!tab) {
      return;
    }

    tab.loading = false;
    tab.error = error;
  }

  function releaseCursor(cursor?: string) {
    if (!cursor) return;
    void DbService.ExecuteQuery({ query: "", cursor, type: QueryExecutionType.QueryExecutionClose, pageSize: 0, page: 0, sortColumn: 0, sortDirection: "" }).catch(() => {});
  }

  function mapResult(response: BackendResult): QueryResult {
    return {
      Duration: response.duration,
      Columns: (response.columns ?? []).map(column => ({ Name: column.name, Type: column.databaseType, Nullable: column.nullable, DefaultValue: column.defaultValue })),
      Rows: (response.rows ?? []).map(row => row ?? []),
      Documents: response.documents ?? undefined,
      Cursor: response.cursor, HasMore: response.hasMore,
      StartRow: response.startRow, PageSize: response.pageSize, IsQuery: response.isQuery, CanNavigate: response.canNavigate,
      CanSort: response.canNavigate && useConnectionStore().activeConnection?.driver !== "mongodb",
    };
  }

  async function execute(id: string, sql: string, afterSave = false, script = false) {
    if (!afterSave && (useTableEditsStore().hasPending(id) || useTableEditsStore().isSaving(id))) return;
    const tab = tabs.value.find(tab => tab.id === id);
    if (!tab || tab.loading || runningQueries.has(id) || !sql.trim()) return;
    void cancelRowCount(id);
    tab.countError = undefined;
    const operation = { requestID: crypto.randomUUID(), stopping: false };
    runningQueries.set(id, operation);
    tab.runningQuery = true;
    tab.runningScript = script;
    tab.scriptResult = undefined;
    tab.queryStartedAt = performance.now();
    tab.stopping = false;
    const previousCursor = tab.cursor ?? "";
    const refreshPage = afterSave && tab.result?.CanNavigate
      ? Math.floor((tab.result.StartRow - 1) / tab.result.PageSize) + 1 : 1;
    const defaultPageSize = useResultSettingsStore().settings.rowsPerPage;
    const pageSize = afterSave ? tab.result?.PageSize ?? defaultPageSize : defaultPageSize;
    const refreshSortColumn = afterSave ? tab.sortColumn ?? 0 : 0;
    const refreshSortDirection = afterSave ? tab.sortDirection : undefined;
    tab.loading = true;
    tab.error = null;
    tab.pageError = undefined;
    tab.totalRows = undefined;
    tab.fetchingLast = false;
    tab.sortColumn = refreshSortColumn;
    tab.sortDirection = refreshSortDirection;
    tab.streamNextPage = 2;
    if (!afterSave) tab.result = null;
    tab.pages = [];
    tab.pageIndex = 0;
    tab.cursor = "";
    tab.executedSql = script ? undefined : sql;
    try {
      if (script) {
        releaseCursor(previousCursor);
        const response = await DbService.ExecuteConsoleScript(id, operation.requestID,
          tab.connectionId ?? String(useConnectionStore().activeConnection?.id ?? ''), sql);
        if (!response) throw new Error('Script returned no response');
        if (!tabs.value.includes(tab)) return;
        tab.scriptResult = response;
        tab.error = response.error || null;
        return;
      }
      const response = await DbService.ExecuteConsoleQuery(id, operation.requestID, { query: sql, cursor: previousCursor,
        type: afterSave && tab.result?.CanNavigate ? QueryExecutionType.QueryExecutionNavigate : QueryExecutionType.QueryExecutionExecute,
        pageSize, page: refreshPage, sortColumn: refreshSortColumn, sortDirection: refreshSortDirection ?? "",
      });
      if (!response) throw new Error("Query returned no response");
      if (operation.stopping) { releaseCursor(response.cursor); throw new Error("Query cancelled"); }
      if (!tabs.value.includes(tab)) { releaseCursor(response.cursor); return; }
      const result = mapResult(response);
      tab.pages = [result];
      tab.cursor = result.Cursor;
      tab.streamNextPage = Math.floor((result.StartRow - 1) / result.PageSize) + 2;
      if (response.totalRows != null) tab.totalRows = response.totalRows;
      else if (!result.HasMore) tab.totalRows = result.StartRow - 1 + result.Rows.length;
      setResult(id, result);
    } catch (error) {
      const message = operation.stopping ? script ? "Script cancelled; earlier commits may remain" : "Query cancelled" : error instanceof Error ? error.message : String(error);
      if (script) tab.scriptResult = { completed: 0, total: 0, error: message, errorLine: 0,
        cancelled: operation.stopping, duration: Math.round(performance.now() - (tab.queryStartedAt ?? performance.now())) };
      if (afterSave && tab.result) tab.pageError = message;
      else setError(id, message);
    } finally {
      runningQueries.delete(id);
      tab.loading = false;
      tab.runningQuery = false;
      tab.runningScript = false;
      tab.queryStartedAt = undefined;
      tab.stopping = false;
    }
  }

  function executeScript(id: string) {
    const tab = tabs.value.find(tab => tab.id === id);
    if (tab?.type === 'query') return execute(id, tab.sql, false, true);
  }

  function openSQLScript(name: string, sql: string, connectionId: string) {
    const created = createTab(connectionId);
    const tab = tabs.value.find(tab => tab.id === created.id)!;
    tab.title = name;
    tab.importedScript = true;
    updateSql(tab.id, sql);
    return tab;
  }

  async function cancelQuery(id: string) {
    const operation = runningQueries.get(id);
    if (!operation || operation.stopping) return;
    const tab = tabs.value.find(tab => tab.id === id);
    operation.stopping = true;
    if (tab) tab.stopping = true;
    try {
      // Stop may arrive before the execution call registers on the backend.
      while (runningQueries.get(id) === operation) {
        if (await DbService.CancelConsoleQuery(id, operation.requestID)) return;
        await new Promise(resolve => setTimeout(resolve, 100));
      }
    } catch (error) {
      if (runningQueries.get(id) !== operation) return;
      operation.stopping = false;
      if (tab) {
        tab.stopping = false;
        tab.error = `Could not cancel query: ${error instanceof Error ? error.message : String(error)}`;
      }
    }
  }

  async function countRows(id: string): Promise<boolean> {
    const tab = tabs.value.find(tab => tab.id === id);
    if (!tab?.result?.CanSort || !tab.executedSql || tab.loading || tab.closing || rowCounts.has(id)
        || useTableEditsStore().hasPending(id) || useTableEditsStore().isSaving(id)) return false;
    const operation = { requestID: crypto.randomUUID(), cancelled: false, stopping: false };
    rowCounts.set(id, operation);
    const query = tab.executedSql;
    tab.countingRows = true;
    tab.stoppingCount = false;
    tab.countError = undefined;
    try {
      const response = await DbService.ExecuteConsoleQuery(`${id}:count`, operation.requestID, {
        query, cursor: "", type: QueryExecutionType.QueryExecutionCount, pageSize: 0, page: 0, sortColumn: 0, sortDirection: "",
      });
      if (operation.cancelled || !tabs.value.includes(tab) || tab.executedSql !== query) return false;
      if (response?.totalRows == null) throw new Error("No row count returned");
      tab.totalRows = response.totalRows;
      return true;
    } catch (error) {
      if (!operation.cancelled && tabs.value.includes(tab)) tab.countError = `Count failed: ${error instanceof Error ? error.message : String(error)}`;
      return false;
    } finally {
      if (rowCounts.get(id) === operation) rowCounts.delete(id);
      tab.countingRows = false;
      tab.stoppingCount = false;
    }
  }

  async function cancelRowCount(id: string) {
    const operation = rowCounts.get(id);
    if (!operation || operation.stopping) return;
    operation.cancelled = true;
    operation.stopping = true;
    const tab = tabs.value.find(tab => tab.id === id);
    if (tab) tab.stoppingCount = true;
    try {
      // Stop can arrive before the count registers with the backend.
      while (rowCounts.get(id) === operation) {
        if (await DbService.CancelConsoleQuery(`${id}:count`, operation.requestID)) return;
        await new Promise(resolve => setTimeout(resolve, 100));
      }
    } catch (error) {
      if (rowCounts.get(id) !== operation) return;
      operation.stopping = false;
      if (tab) {
        tab.stoppingCount = false;
        tab.countError = `Could not cancel count: ${String(error)}`;
      }
    }
  }

  async function navigate(id: string, direction: "first" | "previous" | "next" | "last") {
    const tab = tabs.value.find(tab => tab.id === id);
    if (!tab || tab.loading || !tab.pages?.length || useTableEditsStore().hasPending(id) || useTableEditsStore().isSaving(id)) return;
    if (tab.result?.CanNavigate) {
      if (direction === "last" && tab.totalRows == null) {
        if (!(await countRows(id)) || !tabs.value.includes(tab) || tab.loading) return;
      }
      const page = Math.floor((tab.result.StartRow - 1) / tab.result.PageSize) + 1;
      const lastPage = tab.totalRows == null ? undefined : Math.max(1, Math.ceil(tab.totalRows / tab.result.PageSize));
      if (direction === "last" && lastPage == null || direction === "next" && !tab.result.HasMore) return;
      const target = direction === "first" ? 1 : direction === "last" ? lastPage! : page + (direction === "next" ? 1 : -1);
      if (target < 1 || (lastPage != null && target > lastPage) || target === page) return;
      await fetchReadPage(tab, target);
      return;
    }
    const index = tab.pageIndex ?? 0;
    const target = direction === "first"
      ? tab.pages.findIndex(result => result.StartRow === 1)
      : direction === "last"
      ? tab.pages.length - (tab.cursor ? 0 : 1)
      : index + (direction === "next" ? 1 : -1);
    if (target < 0) return;
    if (target < tab.pages.length) {
      tab.pageIndex = target;
      tab.result = tab.pages[target];
      tab.pageError = undefined;
      return;
    }
    if (!tab.cursor) return;
    tab.loading = true;
    tab.fetchingLast = direction === "last";
    tab.pageError = undefined;
    try {
      do {
        const last = tab.pages[tab.pages.length - 1];
        const response = await DbService.ExecuteQuery({ query: "", cursor: tab.cursor, type: QueryExecutionType.QueryExecutionFetchPaged, pageSize: last.PageSize, page: Math.floor((last.StartRow - 1) / last.PageSize) + 2, sortColumn: 0, sortDirection: "" });
        if (!response) throw new Error("No result page returned");
        if (!tabs.value.includes(tab)) { releaseCursor(response.cursor); return; }
        const result = mapResult(response);
        tab.cursor = result.Cursor;
        if (!result.HasMore) tab.totalRows = result.StartRow - 1 + result.Rows.length;
        tab.pages.push(result);
        // Keep navigation memory bounded regardless of the database size.
        if (tab.pages.length > 10) tab.pages.shift();
        tab.pageIndex = tab.pages.length - 1;
        tab.result = result;
      } while (tab.fetchingLast && tab.cursor && tabs.value.includes(tab));
    } catch (error) {
      tab.pageError = error instanceof Error ? error.message : String(error);
    } finally { tab.loading = false; tab.fetchingLast = false; }
  }

  async function fetchReadPage(tab: QueryTab, page: number, sort?: { column: number; direction?: "asc" | "desc" }) {
    if (!tab.result || tab.loading) return;
    const size = tab.result.PageSize;
    if (!sort) {
      const cached = tab.pages?.findIndex(result => result.StartRow === (page - 1) * size + 1) ?? -1;
      if (cached >= 0 && tab.pages) {
        tab.pageIndex = cached;
        tab.result = tab.pages[cached];
        tab.pageError = undefined;
        return;
      }
    }
    const useCursor = !sort && !!tab.cursor && page === tab.streamNextPage;
    tab.loading = true;
    tab.pageError = undefined;
    try {
      const response = await DbService.ExecuteQuery({
        query: tab.executedSql ?? "", cursor: tab.cursor ?? "",
        type: useCursor ? QueryExecutionType.QueryExecutionFetchPaged : QueryExecutionType.QueryExecutionNavigate,
        pageSize: size, page,
        sortColumn: sort?.column ?? tab.sortColumn ?? 0,
        sortDirection: sort ? sort.direction ?? "" : tab.sortDirection ?? "",
      });
      if (!response) throw new Error("No result page returned");
      if (!tabs.value.includes(tab)) { releaseCursor(response.cursor); return; }
      const result = mapResult(response);
      tab.cursor = result.Cursor;
      tab.streamNextPage = Math.floor((result.StartRow - 1) / size) + 2;
      if (sort) tab.totalRows = undefined;
      if (response.totalRows != null) tab.totalRows = response.totalRows;
      if (sort) {
        tab.sortColumn = sort.column;
        tab.sortDirection = sort.direction;
        tab.pages = [];
      }
      tab.pages ??= [];
      tab.pages = tab.pages.filter(cached => cached.StartRow !== result.StartRow);
      tab.pages.push(result);
      if (tab.pages.length > 10) tab.pages.shift();
      tab.pageIndex = tab.pages.length - 1;
      tab.result = result;
    } catch (error) {
      // A random jump closes the old stream before opening the new one.
      if (!useCursor) tab.cursor = "";
      tab.pageError = error instanceof Error ? error.message : String(error);
    } finally { tab.loading = false; }
  }

  async function sortResult(id: string, column: number, direction?: "asc" | "desc") {
    const tab = tabs.value.find(tab => tab.id === id);
    if (!tab?.result?.CanNavigate || tab.loading || useTableEditsStore().hasPending(id) || useTableEditsStore().isSaving(id)) return;
    await fetchReadPage(tab, 1, { column, direction });
  }

  function stopFetching(id: string) {
    const tab = tabs.value.find(tab => tab.id === id);
    // Complete the in-flight batch, preserving a resumable cursor.
    if (tab) tab.fetchingLast = false;
  }

  function clearResults() {
    restoreVersion++;
    restoringConsoles.value = false;
    useTableEditsStore().clearAll();
    for (const tab of tabs.value) { void cancelQuery(tab.id); void cancelRowCount(tab.id); releaseCursor(tab.cursor); }
    tabs.value = [];
    activeTabId.value = null;
  }

  return {
    tabs,
    activeTabId,
    activeTab,

    createTab,
    createResultTab,
    selectTab,
    moveTab,
    closeTab,
    saveConsole,
    restoreConsoles,
    restoringConsoles,
    updateSql,
    setResult,
    setLoading,
    setError,
    execute,
    executeScript,
    openSQLScript,
    cancelQuery,
    countRows,
    cancelRowCount,
    navigate,
    stopFetching,
    sortResult,
    clearResults,
  };
});
