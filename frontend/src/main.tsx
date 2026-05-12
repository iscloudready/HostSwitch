import React, { useEffect, useMemo, useState } from "react";
import { createRoot } from "react-dom/client";
import { Boxes, DatabaseBackup, Download, Globe2, KeyRound, Network, RefreshCw, ServerCog, ShieldCheck } from "lucide-react";
import "./styles.css";

type Environment = { id: number; name: string; is_active: boolean };
type Group = { id: number; environment_id: number; name: string; is_enabled: boolean };
type HostEntry = { id: number; group_id: number; group_name: string; domain: string; ip_address: string; status: string; source: string; tags: string };
type Backup = { id: number; file_path: string; created_at: string; trigger_reason: string };
type DockerSuggestion = { name: string; suggested_domain: string; source: string };
type Tab = "Dashboard" | "Environments" | "Docker Discovery" | "Backups" | "Settings" | "API";

const apiBase = import.meta.env.VITE_API_BASE ?? "http://localhost:18080";

function App() {
  const [environments, setEnvironments] = useState<Environment[]>([]);
  const [groups, setGroups] = useState<Group[]>([]);
  const [hosts, setHosts] = useState<HostEntry[]>([]);
  const [backups, setBackups] = useState<Backup[]>([]);
  const [dockerSuggestions, setDockerSuggestions] = useState<DockerSuggestion[]>([]);
  const [activeTab, setActiveTab] = useState<Tab>("Dashboard");
  const activeEnvironment = useMemo(() => environments.find((item) => item.is_active) ?? environments[0], [environments]);

  async function load() {
    const envs = (await get<Environment[]>("/api/environments")) ?? [];
    setEnvironments(envs);
    const active = envs.find((item) => item.is_active) ?? envs[0];
    if (active) {
      setGroups((await get<Group[]>(`/api/groups?environment_id=${active.id}`)) ?? []);
      setHosts((await get<HostEntry[]>(`/api/hosts?environment_id=${active.id}`)) ?? []);
    } else {
      setGroups([]);
      setHosts([]);
    }
  }

  async function loadBackups() {
    setBackups((await get<Backup[]>("/api/backups")) ?? []);
  }

  async function loadDockerSuggestions() {
    setDockerSuggestions((await get<DockerSuggestion[]>("/api/docker/discover")) ?? []);
  }

  useEffect(() => {
    load().catch(console.error);
  }, []);

  useEffect(() => {
    if (activeTab === "Backups") loadBackups().catch(console.error);
    if (activeTab === "Docker Discovery") loadDockerSuggestions().catch(console.error);
  }, [activeTab]);

  async function switchEnvironment(environment: Environment) {
    await post("/api/environments/switch", { environment: environment.name });
    await load();
  }

  async function toggleGroup(group: Group) {
    await patch(`/api/groups/${group.id}`, { is_enabled: !group.is_enabled });
    await load();
  }

  async function applyHosts() {
    await post("/api/apply", {});
    await load();
  }

  async function importSystemHosts(scope: "active" | "all") {
    await post("/api/hosts/import-system", { scope });
    await load();
  }

  async function createBackup() {
    await post("/api/backups", {});
    await loadBackups();
  }

  return (
    <main className="app-shell">
      <aside className="sidebar">
        <div className="brand">
          <Globe2 size={24} />
          <span>HostSwitch</span>
        </div>
        {([
          ["Dashboard", Network],
          ["Environments", ServerCog],
          ["Docker Discovery", Boxes],
          ["Backups", DatabaseBackup],
          ["Settings", ShieldCheck],
          ["API", KeyRound],
        ] as const).map(([label, Icon]) => (
          <button className={activeTab === label ? "nav-item active" : "nav-item"} key={label} onClick={() => setActiveTab(label)}>
            <Icon size={18} />
            <span>{label}</span>
          </button>
        ))}
      </aside>

      <section className="workspace">
        <header className="topbar">
          <div>
            <p className="eyebrow">Local Domain Control Center</p>
            <h1>{activeTab}</h1>
          </div>
          <div className="top-actions">
            <button className="secondary" onClick={() => importSystemHosts("active")}>
              <Download size={18} />
              Import Current
            </button>
            <button className="secondary" onClick={() => importSystemHosts("all")}>
              <Download size={18} />
              Import All Envs
            </button>
            <button className="primary" onClick={applyHosts}>
              <ShieldCheck size={18} />
              Apply Hosts
            </button>
          </div>
        </header>

        {activeTab === "Dashboard" && (
          <DashboardView
            environments={environments}
            activeEnvironment={activeEnvironment}
            groups={groups}
            hosts={hosts}
            onSwitchEnvironment={switchEnvironment}
            onToggleGroup={toggleGroup}
          />
        )}
        {activeTab === "Environments" && (
          <EnvironmentsView environments={environments} activeEnvironment={activeEnvironment} hosts={hosts} groups={groups} onSwitchEnvironment={switchEnvironment} />
        )}
        {activeTab === "Docker Discovery" && <DockerDiscoveryView suggestions={dockerSuggestions} onRefresh={loadDockerSuggestions} />}
        {activeTab === "Backups" && <BackupsView backups={backups} onCreateBackup={createBackup} onRefresh={loadBackups} />}
        {activeTab === "Settings" && <SettingsView activeEnvironment={activeEnvironment} />}
        {activeTab === "API" && <APIView />}
      </section>
    </main>
  );
}

function DashboardView({
  environments,
  activeEnvironment,
  groups,
  hosts,
  onSwitchEnvironment,
  onToggleGroup,
}: {
  environments: Environment[];
  activeEnvironment?: Environment;
  groups: Group[];
  hosts: HostEntry[];
  onSwitchEnvironment: (environment: Environment) => void;
  onToggleGroup: (group: Group) => void;
}) {
  return (
    <>
      <EnvironmentToolbar environments={environments} activeEnvironment={activeEnvironment} onSwitchEnvironment={onSwitchEnvironment} />
      <section className="summary-grid">
        <Metric label="Groups" value={groups.length} />
        <Metric label="Enabled Hosts" value={hosts.filter((host) => host.status === "enabled").length} />
        <Metric label="Docker Sources" value={hosts.filter((host) => host.source === "docker").length} />
      </section>
      <section className="content-grid">
        <GroupsPanel activeEnvironment={activeEnvironment} groups={groups} onToggleGroup={onToggleGroup} />
        <HostsPanel hosts={hosts} />
      </section>
    </>
  );
}

function EnvironmentsView({
  environments,
  activeEnvironment,
  hosts,
  groups,
  onSwitchEnvironment,
}: {
  environments: Environment[];
  activeEnvironment?: Environment;
  hosts: HostEntry[];
  groups: Group[];
  onSwitchEnvironment: (environment: Environment) => void;
}) {
  return (
    <>
      <EnvironmentToolbar environments={environments} activeEnvironment={activeEnvironment} onSwitchEnvironment={onSwitchEnvironment} />
      <section className="env-grid">
        {environments.map((environment) => (
          <button className={environment.is_active ? "env-tile active" : "env-tile"} key={environment.id} onClick={() => onSwitchEnvironment(environment)}>
            <span>{environment.name}</span>
            <strong>{environment.is_active ? "Active" : "Inactive"}</strong>
          </button>
        ))}
      </section>
      <section className="summary-grid">
        <Metric label="Active Groups" value={groups.filter((group) => group.is_enabled).length} />
        <Metric label="Hosts In View" value={hosts.length} />
        <Metric label="Disabled Groups" value={groups.filter((group) => !group.is_enabled).length} />
      </section>
    </>
  );
}

function DockerDiscoveryView({ suggestions, onRefresh }: { suggestions: DockerSuggestion[]; onRefresh: () => void }) {
  return (
    <section className="panel">
      <div className="panel-title">
        <h2>Discovered Services</h2>
        <button className="mini-button" onClick={onRefresh}>Refresh</button>
      </div>
      <table>
        <thead>
          <tr>
            <th>Container</th>
            <th>Suggested Domain</th>
            <th>Source</th>
          </tr>
        </thead>
        <tbody>
          {suggestions.map((item) => (
            <tr key={item.name}>
              <td>{item.name}</td>
              <td>{item.suggested_domain}</td>
              <td>{item.source}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </section>
  );
}

function BackupsView({ backups, onCreateBackup, onRefresh }: { backups: Backup[]; onCreateBackup: () => void; onRefresh: () => void }) {
  return (
    <section className="panel">
      <div className="panel-title">
        <h2>Backup History</h2>
        <div className="panel-actions">
          <button className="mini-button" onClick={onRefresh}>Refresh</button>
          <button className="mini-button" onClick={onCreateBackup}>Manual Backup</button>
        </div>
      </div>
      <table>
        <thead>
          <tr>
            <th>Created</th>
            <th>Reason</th>
            <th>Path</th>
          </tr>
        </thead>
        <tbody>
          {backups.map((backup) => (
            <tr key={backup.id}>
              <td>{new Date(backup.created_at).toLocaleString()}</td>
              <td>{backup.trigger_reason}</td>
              <td>{backup.file_path}</td>
            </tr>
          ))}
          {backups.length === 0 && (
            <tr>
              <td colSpan={3} className="empty">No backups yet. Apply, import, or create a manual backup.</td>
            </tr>
          )}
        </tbody>
      </table>
    </section>
  );
}

function SettingsView({ activeEnvironment }: { activeEnvironment?: Environment }) {
  return (
    <section className="settings-grid">
      <InfoPanel title="Hosts File Safety" rows={[["Write mode", "Managed block only"], ["Current environment", activeEnvironment?.name ?? "None"], ["Validation", "Domain and IP checks enabled"]]} />
      <InfoPanel title="Runtime Defaults" rows={[["Backend API", apiBase], ["Host source", "Configured hosts preview"], ["Backup policy", "Before import, apply, and restore"]]} />
    </section>
  );
}

function APIView() {
  const endpoints = [
    "GET /api/environments",
    "POST /api/environments/switch",
    "GET /api/groups?environment_id=1",
    "GET /api/hosts?environment_id=1",
    "POST /api/hosts/import-system",
    "POST /api/apply",
    "GET /api/backups",
  ];
  return (
    <section className="panel">
      <div className="panel-title">
        <h2>REST API</h2>
        <span>{apiBase}</span>
      </div>
      <div className="endpoint-list">
        {endpoints.map((endpoint) => (
          <code key={endpoint}>{endpoint}</code>
        ))}
      </div>
    </section>
  );
}

function EnvironmentToolbar({
  environments,
  activeEnvironment,
  onSwitchEnvironment,
}: {
  environments: Environment[];
  activeEnvironment?: Environment;
  onSwitchEnvironment: (environment: Environment) => void;
}) {
  return (
    <section className="toolbar">
      <span>Environment</span>
      {environments.map((environment) => (
        <button className={environment.id === activeEnvironment?.id ? "segment selected" : "segment"} key={environment.id} onClick={() => onSwitchEnvironment(environment)}>
          {environment.name}
        </button>
      ))}
    </section>
  );
}

function GroupsPanel({ activeEnvironment, groups, onToggleGroup }: { activeEnvironment?: Environment; groups: Group[]; onToggleGroup: (group: Group) => void }) {
  return (
    <div className="panel">
      <div className="panel-title">
        <h2>Groups</h2>
        <span>{activeEnvironment?.name ?? "No environment"}</span>
      </div>
      <div className="group-list">
        {groups.map((group) => (
          <button className="group-row" key={group.id} onClick={() => onToggleGroup(group)}>
            <span>{group.name}</span>
            <span className={group.is_enabled ? "status enabled" : "status"}>{group.is_enabled ? "Enabled" : "Disabled"}</span>
          </button>
        ))}
      </div>
    </div>
  );
}

function HostsPanel({ hosts }: { hosts: HostEntry[] }) {
  return (
    <div className="panel table-panel">
      <div className="panel-title">
        <h2>Hosts</h2>
        <span>{hosts.length} entries</span>
      </div>
      <table>
        <thead>
          <tr>
            <th>Domain</th>
            <th>IP</th>
            <th>Group</th>
            <th>Tags</th>
            <th>Status</th>
            <th>Source</th>
          </tr>
        </thead>
        <tbody>
          {hosts.map((host) => (
            <tr key={host.id}>
              <td>{host.domain}</td>
              <td>{host.ip_address}</td>
              <td>{host.group_name}</td>
              <td><TagList tags={host.tags} /></td>
              <td><span className={host.status === "enabled" ? "status enabled" : "status"}>{host.status}</span></td>
              <td>{host.source}</td>
            </tr>
          ))}
          {hosts.length === 0 && (
            <tr>
              <td colSpan={6} className="empty">No hosts yet. Use Import Current to load entries into this environment.</td>
            </tr>
          )}
        </tbody>
      </table>
    </div>
  );
}

function TagList({ tags }: { tags: string }) {
  const values = tags.split(",").map((tag) => tag.trim()).filter(Boolean);
  if (values.length === 0) {
    return <span className="muted">None</span>;
  }
  return (
    <div className="tag-list">
      {values.map((tag) => (
        <span className="tag" key={tag}>{tag}</span>
      ))}
    </div>
  );
}

function InfoPanel({ title, rows }: { title: string; rows: Array<[string, string]> }) {
  return (
    <section className="panel">
      <div className="panel-title">
        <h2>{title}</h2>
      </div>
      <div className="info-list">
        {rows.map(([label, value]) => (
          <div className="info-row" key={label}>
            <span>{label}</span>
            <strong>{value}</strong>
          </div>
        ))}
      </div>
    </section>
  );
}

function Metric({ label, value }: { label: string; value: number }) {
  return (
    <div className="metric">
      <span>{label}</span>
      <strong>{value}</strong>
    </div>
  );
}

async function get<T>(path: string): Promise<T> {
  const response = await fetch(apiBase + path);
  if (!response.ok) throw new Error(await response.text());
  return response.json();
}

async function post(path: string, body: unknown) {
  const response = await fetch(apiBase + path, { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(body) });
  if (!response.ok) throw new Error(await response.text());
  return response.json();
}

async function patch(path: string, body: unknown) {
  const response = await fetch(apiBase + path, { method: "PATCH", headers: { "Content-Type": "application/json" }, body: JSON.stringify(body) });
  if (!response.ok) throw new Error(await response.text());
  return response.json();
}

createRoot(document.getElementById("root")!).render(<App />);
