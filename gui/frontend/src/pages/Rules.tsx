import {
  AppWindow,
  ArrowLeftRight,
  Ban,
  Database,
  Download,
  File,
  FileCode,
  Flag,
  FolderOpen,
  Globe,
  Hash,
  House,
  Library,
  Link,
  ListTree,
  Network,
  Pencil,
  Plus,
  Radio,
  Regex,
  Route,
  Search,
  ShieldCheck,
  Tags,
  Trash2,
  Type,
  Upload,
  type LucideIcon,
} from "lucide-react";
import { useEffect, useMemo, useState, type ReactNode } from "react";
import {
  Badge,
  Button,
  Callout,
  Card,
  CardHeader,
  EmptyState,
  Field,
  IconButton,
  Input,
  Modal,
  PageHeader,
  Segmented,
  SettingList,
  SettingRow,
  Spinner,
  Switch,
  TextArea,
  cn,
  useFeedback,
} from "../components/ui";
import { api, errorMessage, isCancelled, type Process, type Rule } from "../lib/api";
import {
  ACTIONS,
  COUNTRIES,
  GEOSITE_SUGGESTIONS,
  PRESETS,
  PROTOCOLS,
  RULE_TYPES,
  describeValue,
  makeRules,
  ruleTypeInfo,
  sameRule,
  type Action,
  type RuleGroup,
  type RuleType,
} from "../lib/rules";
import { useDraft } from "../lib/store";

const TYPE_ICONS: Record<RuleType, LucideIcon> = {
  suffix: Globe,
  domain: Link,
  keyword: Type,
  regex: Regex,
  geosite: Tags,
  ip: Network,
  geoip: Flag,
  private: House,
  app: AppWindow,
  path: File,
  path_regex: FileCode,
  port: Hash,
  protocol: Radio,
  network: ArrowLeftRight,
  ruleset: Database,
};

const ACTION_STYLE: Record<Action, { tone: "accent" | "success" | "danger"; icon: LucideIcon; title: string; description: string }> = {
  block: { tone: "danger", icon: Ban, title: "Blocked", description: "Evaluated first. Matching connections are refused." },
  warp: { tone: "accent", icon: ShieldCheck, title: "Through WARP", description: "Always sent through WARP, even if a bypass rule also matches." },
  direct: { tone: "success", icon: Route, title: "Bypass WARP", description: "Sent over your normal connection." },
};

export function Rules() {
  const { draft, update, errors } = useDraft();
  const { toast, confirm } = useFeedback();
  const [editing, setEditing] = useState<Rule | null | undefined>(undefined);
  const [presetsOpen, setPresetsOpen] = useState(false);
  const rules = draft.routing.rules ?? [];

  const setRules = (next: Rule[]) => update((d) => ({ ...d, routing: { ...d.routing, rules: next } }));

  const addRules = (added: Rule[]) => {
    const fresh = added.filter((rule) => !rules.some((existing) => sameRule(existing, rule)));
    setRules([...rules, ...fresh]);
    return fresh.length;
  };

  const saveEdited = (original: Rule | null, result: Rule[]) => {
    if (original) {
      const index = rules.findIndex((r) => r.id === original.id);
      const next = rules.slice();
      next.splice(index, 1, ...result);
      setRules(next);
    } else {
      const count = addRules(result);
      if (count < result.length) toast("accent", `${result.length - count} duplicate rule(s) skipped`);
    }
    setEditing(undefined);
  };

  const importRules = async () => {
    try {
      const imported = await api.ImportRules();
      if (!imported || imported.length === 0) return;
      const count = addRules(imported);
      toast("success", `Imported ${count} rule${count === 1 ? "" : "s"}`, "Save your changes to apply them.");
    } catch (error) {
      if (!isCancelled(error)) toast("danger", "Import failed", errorMessage(error));
    }
  };

  const exportRules = async () => {
    try {
      const path = await api.ExportRules(rules);
      if (path) toast("success", "Rules exported", path);
    } catch (error) {
      if (!isCancelled(error)) toast("danger", "Export failed", errorMessage(error));
    }
  };

  const removeAll = async () => {
    if (await confirm({ title: "Remove all rules?", description: "You can discard the change before saving.", confirmLabel: "Remove all", danger: true })) {
      setRules([]);
    }
  };

  const final = draft.routing.final as Action;

  return (
    <div className="space-y-5 pb-20">
      <PageHeader
        title="Routing rules"
        description="Decide which websites, IP ranges and applications go through WARP, bypass it, or are blocked. Rules apply in both proxy and TUN mode."
        actions={
          <>
            <Button variant="secondary" icon={<Library className="size-4" />} onClick={() => setPresetsOpen(true)}>
              Presets
            </Button>
            <Button variant="secondary" icon={<Upload className="size-4" />} onClick={importRules}>
              Import
            </Button>
            <Button variant="secondary" icon={<Download className="size-4" />} onClick={exportRules} disabled={rules.length === 0}>
              Export
            </Button>
            <Button variant="primary" icon={<Plus className="size-4" />} onClick={() => setEditing(null)}>
              Add rule
            </Button>
          </>
        }
      />

      <Card>
        <CardHeader title="Default route" description="What happens to traffic that matches none of your rules." icon={<Route className="size-4" />} />
        <SettingList>
          <SettingRow
            title="Unmatched traffic"
            description={
              final === "direct"
                ? "Whitelist mode: only traffic matching a WARP rule goes through WARP."
                : final === "block"
                  ? "Strict mode: only traffic matching a rule is allowed."
                  : "Everything goes through WARP unless a rule says otherwise."
            }
          >
            <Segmented<Action>
              value={final}
              onChange={(value) => update((d) => ({ ...d, routing: { ...d.routing, final: value } }))}
              options={ACTIONS.map((a) => ({ value: a.value, label: a.short }))}
            />
          </SettingRow>
          <SettingRow title="Bypass local network" description="Keep printers, routers and other LAN devices reachable directly.">
            <Switch checked={draft.routing.bypassLan} onChange={(value) => update((d) => ({ ...d, routing: { ...d.routing, bypassLan: value } }))} />
          </SettingRow>
        </SettingList>
      </Card>

      {rules.length === 0 ? (
        <Card>
          <EmptyState
            icon={<ListTree className="size-5" />}
            title="No rules yet"
            description="Without rules, all traffic follows the default route. Start from a preset or add your own rule."
            action={
              <div className="flex gap-2">
                <Button variant="secondary" icon={<Library className="size-4" />} onClick={() => setPresetsOpen(true)}>
                  Browse presets
                </Button>
                <Button variant="primary" icon={<Plus className="size-4" />} onClick={() => setEditing(null)}>
                  Add rule
                </Button>
              </div>
            }
          />
        </Card>
      ) : (
        <>
          {(["block", "warp", "direct"] as Action[]).map((action) => {
            const group = rules.filter((r) => r.action === action);
            if (group.length === 0) return null;
            const style = ACTION_STYLE[action];
            const Icon = style.icon;
            return (
              <Card key={action}>
                <CardHeader
                  icon={<Icon className="size-4" />}
                  title={
                    <span className="flex items-center gap-2">
                      {style.title} <Badge tone={style.tone}>{group.length}</Badge>
                    </span>
                  }
                  description={style.description}
                />
                <div className="divide-y divide-line border-t border-line">
                  {group.map((rule) => (
                    <RuleRow
                      key={rule.id}
                      rule={rule}
                      error={errors[`routing.rules.${rule.id}`]}
                      onToggle={(enabled) => setRules(rules.map((r) => (r.id === rule.id ? { ...r, enabled } : r)))}
                      onEdit={() => setEditing(rule)}
                      onDelete={() => setRules(rules.filter((r) => r.id !== rule.id))}
                    />
                  ))}
                </div>
              </Card>
            );
          })}
          <div className="flex items-start justify-between gap-6">
            <Callout tone="accent" title="How rules are evaluated" className="flex-1">
              Block rules come first, then WARP rules, then bypass rules. Rules on domains, apps, ports and protocols are checked
              before rules on IP addresses and countries, which need the domain to be resolved. The first match wins.
            </Callout>
            <Button variant="ghost" icon={<Trash2 className="size-4" />} onClick={removeAll}>
              Remove all
            </Button>
          </div>
        </>
      )}

      <RuleEditor key={editing === undefined ? "closed" : (editing?.id ?? "new")} open={editing !== undefined} rule={editing ?? null} onClose={() => setEditing(undefined)} onSave={saveEdited} />
      <PresetsModal
        open={presetsOpen}
        onClose={() => setPresetsOpen(false)}
        rules={rules}
        onApply={(preset) => {
          const count = addRules(preset.rules.flatMap((r) => makeRules(r.action, r.type, r.value, r.note)));
          toast(count ? "success" : "accent", count ? `Added ${preset.title.toLowerCase()}` : "Preset already added", count ? "Save your changes to apply them." : undefined);
        }}
      />
    </div>
  );
}

function RuleRow({
  rule,
  error,
  onToggle,
  onEdit,
  onDelete,
}: {
  rule: Rule;
  error?: string;
  onToggle: (enabled: boolean) => void;
  onEdit: () => void;
  onDelete: () => void;
}) {
  const info = ruleTypeInfo(rule.type);
  const Icon = TYPE_ICONS[info.type];
  return (
    <div className={cn("group flex items-center gap-3 px-5 py-2.5", !rule.enabled && "opacity-55")}>
      <div className="flex size-8 shrink-0 items-center justify-center rounded-lg border border-line bg-panel-2 text-muted">
        <Icon className="size-4" />
      </div>
      <div className="min-w-0 flex-1 cursor-pointer" onDoubleClick={onEdit}>
        <div className="selectable truncate font-mono text-[12.5px] text-fg">{describeValue(rule)}</div>
        <div className="truncate text-[11.5px] text-faint">
          {info.label}
          {rule.note && ` · ${rule.note}`}
        </div>
        {error && <div className="text-[11.5px] text-danger">{error}</div>}
      </div>
      <div className="flex items-center gap-1 opacity-0 transition-opacity group-hover:opacity-100">
        <IconButton label="Edit rule" onClick={onEdit}>
          <Pencil className="size-3.5" />
        </IconButton>
        <IconButton label="Delete rule" tone="danger" onClick={onDelete}>
          <Trash2 className="size-3.5" />
        </IconButton>
      </div>
      <Switch checked={rule.enabled} onChange={onToggle} label="Enabled" />
    </div>
  );
}

const GROUPS: RuleGroup[] = ["Websites", "Addresses", "Applications", "Traffic", "Lists"];

function RuleEditor({
  open,
  rule,
  onClose,
  onSave,
}: {
  open: boolean;
  rule: Rule | null;
  onClose: () => void;
  onSave: (original: Rule | null, rules: Rule[]) => void;
}) {
  const [action, setAction] = useState<Action>((rule?.action as Action) ?? "direct");
  const [type, setType] = useState<RuleType>((rule?.type as RuleType) ?? "suffix");
  const [value, setValue] = useState(rule?.value ?? "");
  const [note, setNote] = useState(rule?.note ?? "");
  const info = ruleTypeInfo(type);

  const valid = type === "private" || value.trim().length > 0;

  const submit = () => {
    const created = makeRules(action, type, value, note.trim(), rule?.enabled ?? true);
    if (rule && created.length > 0) created[0] = { ...created[0], id: rule.id };
    onSave(rule, created);
  };

  return (
    <Modal
      open={open}
      onClose={onClose}
      width="lg"
      title={rule ? "Edit rule" : "Add rule"}
      description="Choose what to match and where matching traffic should go."
      footer={
        <>
          <Button variant="ghost" onClick={onClose}>
            Cancel
          </Button>
          <Button variant="primary" onClick={submit} disabled={!valid}>
            {rule ? "Update rule" : "Add rule"}
          </Button>
        </>
      }
    >
      <div className="space-y-5">
        <Field label="Action">
          <div className="grid grid-cols-3 gap-2">
            {ACTIONS.map((a) => {
              const style = ACTION_STYLE[a.value];
              const Icon = style.icon;
              const selected = action === a.value;
              return (
                <button
                  key={a.value}
                  type="button"
                  onClick={() => setAction(a.value)}
                  className={cn(
                    "flex items-start gap-2.5 rounded-xl border p-3 text-left transition-colors",
                    selected ? "border-accent bg-accent-soft" : "border-line bg-panel-2 hover:border-line-strong",
                  )}
                >
                  <Icon className={cn("mt-0.5 size-4 shrink-0", selected ? "text-accent" : "text-faint")} />
                  <div>
                    <div className="text-[13px] font-medium">{a.label}</div>
                    <div className="text-[11.5px] leading-snug text-muted">{a.description}</div>
                  </div>
                </button>
              );
            })}
          </div>
        </Field>

        <Field label="Match">
          <div className="grid grid-cols-5 gap-3">
            {GROUPS.map((group) => (
              <div key={group} className="space-y-1">
                <div className="px-1 text-[10.5px] font-semibold tracking-wider text-faint uppercase">{group}</div>
                {RULE_TYPES.filter((t) => t.group === group).map((t) => {
                  const Icon = TYPE_ICONS[t.type];
                  return (
                    <button
                      key={t.type}
                      type="button"
                      onClick={() => {
                        setType(t.type);
                        if (t.type !== type) setValue(t.type === "network" ? "udp" : "");
                      }}
                      className={cn(
                        "flex w-full items-start gap-2 rounded-lg px-2 py-1.5 text-left text-[12px] leading-tight transition-colors",
                        type === t.type ? "bg-accent-soft font-medium text-accent" : "text-muted hover:bg-hover hover:text-fg",
                      )}
                    >
                      <Icon className="mt-px size-3.5 shrink-0" />
                      <span>{t.label}</span>
                    </button>
                  );
                })}
              </div>
            ))}
          </div>
        </Field>

        <div className="rounded-xl border border-line bg-panel-2/50 p-4">
          <div className="mb-3 text-[12.5px] leading-relaxed text-muted">{info.description}</div>
          <ValueInput type={type} value={value} onChange={setValue} />
        </div>

        <Field label="Note" description="Optional, for your own reference.">
          <Input value={note} onChange={(event) => setNote(event.target.value)} placeholder="e.g. Work VPN, Banking" />
        </Field>
      </div>
    </Modal>
  );
}

function toggleValue(list: string, item: string): string {
  const values = list
    .split(",")
    .map((v) => v.trim())
    .filter(Boolean);
  const exists = values.some((v) => v.toLowerCase() === item.toLowerCase());
  return (exists ? values.filter((v) => v.toLowerCase() !== item.toLowerCase()) : [...values, item]).join(", ");
}

function hasValue(list: string, item: string) {
  return list
    .split(",")
    .map((v) => v.trim().toLowerCase())
    .includes(item.toLowerCase());
}

function ValueInput({ type, value, onChange }: { type: RuleType; value: string; onChange: (value: string) => void }) {
  const info = ruleTypeInfo(type);
  switch (type) {
    case "private":
      return <div className="text-[12.5px] text-fg">Matches 10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16, link-local and loopback addresses.</div>;
    case "network":
      return (
        <Segmented
          value={value || "udp"}
          onChange={onChange}
          options={[
            { value: "tcp", label: "TCP" },
            { value: "udp", label: "UDP" },
          ]}
        />
      );
    case "protocol":
      return (
        <div className="flex flex-wrap gap-2">
          {PROTOCOLS.map((protocol) => (
            <Chip key={protocol} selected={hasValue(value, protocol)} onClick={() => onChange(toggleValue(value, protocol))}>
              {protocol.toUpperCase()}
            </Chip>
          ))}
        </div>
      );
    case "geoip":
      return <CountryPicker value={value} onChange={onChange} />;
    case "geosite":
      return (
        <div className="space-y-3">
          <Input mono value={value} onChange={(event) => onChange(event.target.value)} placeholder={info.placeholder} autoFocus />
          <div className="flex flex-wrap gap-1.5">
            {GEOSITE_SUGGESTIONS.map((s) => (
              <Chip key={s.value} selected={hasValue(value, s.value)} onClick={() => onChange(toggleValue(value, s.value))}>
                {s.label}
              </Chip>
            ))}
          </div>
          <p className="text-[11.5px] text-faint">Any category from the sing-geosite rule-set branch can be used.</p>
        </div>
      );
    case "app":
    case "path":
      return <AppPicker type={type} value={value} onChange={onChange} />;
    case "regex":
    case "path_regex":
    case "ruleset":
      return <Input mono value={value} onChange={(event) => onChange(event.target.value)} placeholder={info.placeholder} autoFocus />;
    default:
      return (
        <div className="space-y-1.5">
          <TextArea mono rows={3} value={value} onChange={(event) => onChange(event.target.value)} placeholder={info.placeholder} autoFocus />
          <p className="text-[11.5px] text-faint">Separate several values with commas or new lines; each becomes its own rule.</p>
        </div>
      );
  }
}

function Chip({ selected, onClick, children }: { selected: boolean; onClick: () => void; children: ReactNode }) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={cn(
        "h-7 rounded-lg border px-2.5 text-[12px] font-medium transition-colors",
        selected ? "border-accent bg-accent-soft text-accent" : "border-line bg-panel text-muted hover:border-line-strong hover:text-fg",
      )}
    >
      {children}
    </button>
  );
}

function CountryPicker({ value, onChange }: { value: string; onChange: (value: string) => void }) {
  const [query, setQuery] = useState("");
  const selected = value
    .split(",")
    .map((v) => v.trim().toLowerCase())
    .filter(Boolean);
  const filtered = COUNTRIES.filter((c) => !query || c.name.toLowerCase().includes(query.toLowerCase()) || c.code === query.toLowerCase());
  return (
    <div className="space-y-2.5">
      {selected.length > 0 && (
        <div className="flex flex-wrap gap-1.5">
          {selected.map((code) => (
            <Chip key={code} selected onClick={() => onChange(toggleValue(value, code))}>
              {COUNTRIES.find((c) => c.code === code)?.name ?? code.toUpperCase()} ✕
            </Chip>
          ))}
        </div>
      )}
      <Input leading={<Search className="size-4" />} placeholder="Search countries" value={query} onChange={(event) => setQuery(event.target.value)} autoFocus />
      <div className="grid max-h-52 grid-cols-3 gap-1 overflow-y-auto rounded-lg border border-line bg-panel p-1.5">
        {filtered.map((country) => (
          <button
            key={country.code}
            type="button"
            onClick={() => onChange(toggleValue(value, country.code))}
            className={cn(
              "flex items-center justify-between gap-2 rounded-md px-2 py-1.5 text-left text-[12px]",
              selected.includes(country.code) ? "bg-accent-soft text-accent" : "text-muted hover:bg-hover hover:text-fg",
            )}
          >
            <span className="truncate">{country.name}</span>
            <span className="font-mono text-[10.5px] text-faint uppercase">{country.code}</span>
          </button>
        ))}
      </div>
    </div>
  );
}

function baseName(path: string) {
  return path.split(/[\\/]/).pop() ?? path;
}

function AppPicker({ type, value, onChange }: { type: "app" | "path"; value: string; onChange: (value: string) => void }) {
  const { toast } = useFeedback();
  const [processes, setProcesses] = useState<Process[] | null>(null);
  const [query, setQuery] = useState("");

  useEffect(() => {
    api
      .ListProcesses()
      .then((list) => setProcesses(list ?? []))
      .catch(() => setProcesses([]));
  }, []);

  const filtered = useMemo(
    () => (processes ?? []).filter((p) => !query || p.name.toLowerCase().includes(query.toLowerCase()) || p.path.toLowerCase().includes(query.toLowerCase())),
    [processes, query],
  );

  const browse = async () => {
    try {
      const path = await api.BrowseApplication();
      if (!path) return;
      onChange(type === "app" ? (value ? toggleValue(value, baseName(path)) : baseName(path)) : path);
    } catch (error) {
      if (!isCancelled(error)) toast("danger", "Could not open the file", errorMessage(error));
    }
  };

  const pick = (process: Process) => {
    if (type === "app") onChange(toggleValue(value, process.name));
    else if (process.path) onChange(process.path);
  };

  return (
    <div className="space-y-2.5">
      <div className="flex gap-2">
        <Input mono className="flex-1" value={value} onChange={(event) => onChange(event.target.value)} placeholder={ruleTypeInfo(type).placeholder} />
        <Button variant="secondary" icon={<FolderOpen className="size-4" />} onClick={browse}>
          Browse…
        </Button>
      </div>
      <div className="rounded-lg border border-line bg-panel">
        <div className="border-b border-line p-1.5">
          <Input leading={<Search className="size-4" />} placeholder="Search running applications" value={query} onChange={(event) => setQuery(event.target.value)} />
        </div>
        <div className="max-h-48 overflow-y-auto p-1">
          {processes === null ? (
            <div className="flex items-center gap-2 p-3 text-[12px] text-muted">
              <Spinner /> Loading applications…
            </div>
          ) : filtered.length === 0 ? (
            <div className="p-3 text-[12px] text-faint">No running applications match.</div>
          ) : (
            filtered.map((process) => {
              const selected = type === "app" ? hasValue(value, process.name) : value === process.path;
              return (
                <button
                  key={process.name + process.path}
                  type="button"
                  disabled={type === "path" && !process.path}
                  onClick={() => pick(process)}
                  className={cn(
                    "flex w-full items-center gap-2.5 rounded-md px-2 py-1.5 text-left disabled:opacity-40",
                    selected ? "bg-accent-soft" : "hover:bg-hover",
                  )}
                >
                  <AppWindow className={cn("size-3.5 shrink-0", selected ? "text-accent" : "text-faint")} />
                  <span className={cn("w-44 shrink-0 truncate text-[12.5px]", selected ? "font-medium text-accent" : "text-fg")}>{process.name}</span>
                  <span className="truncate font-mono text-[11px] text-faint">{process.path}</span>
                </button>
              );
            })
          )}
        </div>
      </div>
      {type === "app" && <p className="text-[11.5px] text-faint">Select several applications to create one rule for each.</p>}
    </div>
  );
}

function PresetsModal({
  open,
  onClose,
  rules,
  onApply,
}: {
  open: boolean;
  onClose: () => void;
  rules: Rule[];
  onApply: (preset: (typeof PRESETS)[number]) => void;
}) {
  return (
    <Modal open={open} onClose={onClose} width="lg" title="Rule presets" description="Common setups you can add with one click. They are added to your existing rules.">
      <div className="grid grid-cols-2 gap-3">
        {PRESETS.map((preset) => {
          const added = preset.rules.every((r) => rules.some((existing) => sameRule(existing, r)));
          return (
            <div key={preset.id} className="flex flex-col rounded-xl border border-line bg-panel-2/50 p-4">
              <div className="text-[13px] font-semibold">{preset.title}</div>
              <div className="mt-1 flex-1 text-[12px] leading-relaxed text-muted">{preset.description}</div>
              <div className="mt-3 flex flex-wrap gap-1">
                {preset.rules.map((r) => (
                  <span key={r.type + r.value} className="rounded bg-panel px-1.5 py-0.5 font-mono text-[10.5px] text-faint">
                    {r.action} {r.type}:{r.value}
                  </span>
                ))}
              </div>
              <div className="mt-3">
                <Button size="sm" variant={added ? "ghost" : "subtle"} disabled={added} icon={added ? undefined : <Plus className="size-3.5" />} onClick={() => onApply(preset)}>
                  {added ? "Added" : "Add preset"}
                </Button>
              </div>
            </div>
          );
        })}
      </div>
    </Modal>
  );
}
