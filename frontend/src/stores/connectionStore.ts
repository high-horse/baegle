import { defineStore } from "pinia";
import { ref, computed } from "vue";
import { DatabaseService } from "@bindings/db-lens/internal/app";
import type { Connection } from "@bindings/db-lens/internal/types";
import { DbService } from "@bindings/db-lens/internal/app";
import type { ConnectionConfig } from "@bindings/db-lens/internal/engine/entities";
import { Notify } from "quasar";
import type { InspectColumnInfo, InspectTableInfo } from "@bindings/db-lens/internal/engine/entities";

export const useConnectionStore = defineStore("connection", () => {
  const selectedConnection = ref<Connection | null>(null);
  const activeConnection = ref<Connection | null>(null);
  const activeConnectionMetadata = ref<InspectTableInfo[] | null>(null);

  const tableColumns = ref<Record<string, InspectColumnInfo[]>>({});
  const pendingColumns = new Map<string, Promise<InspectColumnInfo[]>>();
  let metadataGeneration = 0;
  const columnKey = (table: InspectTableInfo) => JSON.stringify([table.database, table.schema || "", table.name]);
  function clearColumnMetadata() {
    metadataGeneration++;
    tableColumns.value = {};
    pendingColumns.clear();
  }
  function getCachedTableColumns(table: InspectTableInfo): InspectColumnInfo[] {
    return tableColumns.value[columnKey(table)] || [];
  }
  async function loadTableColumns(table: InspectTableInfo): Promise<InspectColumnInfo[]> {
    const connection = activeConnection.value;
    if (!connection || connection.driver === "mongodb") return [];
    const key = columnKey(table);
    if (tableColumns.value[key]) return tableColumns.value[key];
    const pending = pendingColumns.get(key);
    if (pending) return pending;
    const generation = metadataGeneration;
    const request = DbService.InspectTableColumns({ connectionId: String(connection.id),
      name: table.name, schema: table.schema || "", database: table.database }).then(columns => {
        if (generation !== metadataGeneration) return [];
        tableColumns.value[key] = columns || [];
        return tableColumns.value[key];
      }).finally(() => {
        if (pendingColumns.get(key) === request) pendingColumns.delete(key);
      });
    pendingColumns.set(key, request);
    return request;
  }

  const searchTerm = ref<string>("");
  
  const connections = ref<Connection[]>([]);
  const connectionGroups = computed(() => Array.from(new Set([
    "Live", "Prod", "Dev", ...connections.value.map(connection => connection.group).filter(Boolean),
  ])).sort((a, b) => a.localeCompare(b)));
  async function setConnectionGroup(id: number, group: string): Promise<boolean> {
    try {
      await DatabaseService.SetConnectionGroup(id, group.trim());
      const connection = connections.value.find(item => item.id === id);
      if (connection) connection.group = group.trim();
      if (selectedConnection.value?.id === id) selectedConnection.value.group = group.trim();
      if (activeConnection.value?.id === id) activeConnection.value.group = group.trim();
      return true;
    } catch (error) {
      Notify.create({ type: "negative", message: error instanceof Error ? error.message : String(error) });
      return false;
    }
  }
  const showNewConnectionDialog = ref(false);
  const loadingStates = ref<Record<string, boolean>>({
    getConnections: false,
    saveConnection: false,
    connecting: false,
    setActiveConnectionMetadata: false,
  });
  const connectingConnectionId = ref<number | null>(null);
  const connectionStage = ref("Checking connection…");
  const connectionError = ref<{ connectionId: number; name: string; message: string } | null>(null);

  const isConnected = computed(() => activeConnection.value !== null);

  function setSelectedSession(session: Connection) {
    selectedConnection.value = session;
  }
  function clearSelectedSession() {
    selectedConnection.value = null;
  }
  function setActiveSession(session: Connection) {
    const previous = activeConnection.value;
    if (!previous || previous.id !== session.id || previous.driver !== session.driver || previous.dbname !== session.dbname) {
      activeConnectionMetadata.value = null;
      clearColumnMetadata();
    }
    activeConnection.value = session;
  }

  function getActiveSession() {
    return activeConnection.value;
  }
  
  function clearActiveSession() {
    activeConnection.value = null;
    activeConnectionMetadata.value = null;
    clearColumnMetadata();
  }

  async function getConnections() {
    try {
      loadingStates.value.getConnections = true;
      connections.value = (await DatabaseService.GetConnections()) || [];
    } catch (error) {
      console.error(error);
    } finally {
      loadingStates.value.getConnections = false;
    }
  }

  async function deleteConnection(id: number): Promise<boolean> {
    try {
      await DatabaseService.DeleteConnection(id);
      connections.value = connections.value.filter(connection => connection.id !== id);
      if (selectedConnection.value?.id === id) clearSelectedSession();
      Notify.create({ type: "positive", message: "Connection deleted" });
      return true;
    } catch (error) {
      Notify.create({ type: "negative", message: error instanceof Error ? error.message : String(error) });
      return false;
    }
  }

  // Shared mapper — used by both ping and connect so they can never drift apart.
  function toConfig(connection: Connection): ConnectionConfig {
    return {
      ID: String(connection.id),
      Name: connection.name,
      Group: connection.group,
      Host: connection.host,
      Port: Number(connection.port.Int64),
      User: connection.user,
      Password: connection.password,
      Database: connection.dbname,
      Type: connection.driver,
      SSL: false,
      SSHConfigID: connection.ssh_config_id?.Valid
        ? Number(connection.ssh_config_id.Int64)
        : null,
      SSHConfig: connection.ssh_config?.id
        ? {
            ID: connection.ssh_config.id,
            Name: connection.ssh_config.name,
            Host: connection.ssh_config.host,
            Port: Number(connection.ssh_config.port),
            Username: connection.ssh_config.username,
            AuthMethod: connection.ssh_config.auth_method,
            PrivateKey: connection.ssh_config.private_key.Valid
              ? connection.ssh_config.private_key.String
              : "",
            Passphrase: connection.ssh_config.passphrase.Valid
              ? connection.ssh_config.passphrase.String
              : "",
            Password: connection.ssh_config.password.Valid
              ? connection.ssh_config.password.String
              : "",
          }
        : null,
      InMemory: false,
      ReadOnly: connection.read_only,
      Color: connection.color.Valid ? connection.color.String : "",
    };
  }

  async function pingConnection(connection: Connection): Promise<boolean> {
    try {
      return await DbService.PingConfig(toConfig(connection));
    } catch (error) {
      Notify.create({
        type: "negative",
        message: error instanceof Error ? error.message : String(error),
      });
      console.error(error);
      return false;
    }
  }

  // Keep backend errors intact so the portal can show a useful failure dialog.
  async function connectToSession(connection: Connection): Promise<boolean> {
    if (loadingStates.value.connecting) return false;
    loadingStates.value.connecting = true;
    connectingConnectionId.value = connection.id;
    connectionStage.value = "Checking connection…";
    connectionError.value = null;
    try {
      const config = toConfig(connection);
      if (!(await DbService.PingConfig(config))) {
        throw new Error("The database connection check failed. Check the host, port, credentials, and SSH settings.");
      }
      connectionStage.value = "Connecting…";
      if (!(await DbService.Connect(config))) {
        throw new Error("The database could not establish a connection. Check your saved connection settings and try again.");
      }
      setActiveSession(connection);
      Notify.create({ type: "positive", message: `Connected to ${connection.name}` });
      return true;
    } catch (error) {
      const message = error && typeof error === "object" && "message" in error
        ? String(error.message)
        : String(error || "The connection failed. Check your saved settings and try again.");
      connectionError.value = { connectionId: connection.id, name: connection.name, message };
      return false;
    } finally {
      connectingConnectionId.value = null;
      loadingStates.value.connecting = false;
    }
  }

  async function setActiveConnectionMetadata() {
    try {
      loadingStates.value.setActiveConnectionMetadata = true;
      if (!activeConnection.value) return;
      clearColumnMetadata();
      activeConnectionMetadata.value = await DbService.InspectDatabase();
    } catch (error) {
      console.error(error);
    } finally {
      loadingStates.value.setActiveConnectionMetadata = false;
    }
  }

  function resetStore() {
    clearColumnMetadata();
    connectionError.value = null;
    connectingConnectionId.value = null;
    selectedConnection.value = null;
    activeConnection.value = null;
    activeConnectionMetadata.value = null;
    clearColumnMetadata();
    
    searchTerm.value = "";
    
    connections.value = [];
    
    showNewConnectionDialog.value = false;
    
    loadingStates.value = {
    getConnections: false,
    saveConnection: false,
    connecting: false,
    setActiveConnectionMetadata: false,
    };
  }
  
  return {
    selectedConnection,
    activeConnection,
    isConnected,
    connections,
    loadingStates,
    connectingConnectionId,
    connectionStage,
    connectionError,
    showNewConnectionDialog,
    activeConnectionMetadata,
    tableColumns,
    getCachedTableColumns,
    loadTableColumns,
    
    setSelectedSession,
    clearSelectedSession,
    setActiveSession,
    getActiveSession,
    clearActiveSession,
    getConnections,
    deleteConnection,
    connectionGroups,
    setConnectionGroup,
    pingConnection,
    connectToSession,
    setActiveConnectionMetadata,
    searchTerm,
    resetStore,
  };
});
