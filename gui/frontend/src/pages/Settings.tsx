import { Code2, ExternalLink, Info, Monitor, Moon, Palette, Power, RotateCcw, ScrollText, Sun, Wand2 } from "lucide-react";
import { useEffect, useState } from "react";
import { Logo } from "../components/Logo";
import { Button, Card, CardHeader, Field, Input, PageHeader, Segmented, Select, SettingList, SettingRow, Switch, useFeedback } from "../components/ui";
import { api, errorMessage } from "../lib/api";
import { randomPassword } from "../lib/format";
import { useDraft, useStore } from "../lib/store";

const REPOSITORY = "https://github.com/gholamtaksir/Harp";

export function Settings() {
  const { draft, update, errors } = useDraft();
  const { info, replaceDraft } = useStore();
  const { toast, confirm } = useFeedback();
  const [autostart, setAutostart] = useState<boolean | null>(null);
  const app = draft.app;
  const setApp = (patch: Partial<typeof app>) => update((d) => ({ ...d, app: { ...d.app, ...patch } }));

  useEffect(() => {
    api
      .GetAutostart()
      .then(setAutostart)
      .catch(() => setAutostart(null));
  }, []);

  const toggleAutostart = async (enabled: boolean) => {
    try {
      await api.SetAutostart(enabled);
      setAutostart(enabled);
    } catch (error) {
      toast("danger", "Could not change the startup setting", errorMessage(error));
    }
  };

  const reset = async () => {
    if (!(await confirm({ title: "Reset all settings?", description: "Rules, listeners and every other setting return to their defaults. Your WARP account is kept. You can still discard before saving.", confirmLabel: "Reset", danger: true }))) return;
    replaceDraft(await api.GetDefaultSettings());
  };

  return (
    <div className="space-y-5 pb-20">
      <PageHeader title="Settings" description="Application behavior, appearance and advanced options." />

      <Card>
        <CardHeader icon={<Power className="size-4" />} title="Startup & window" />
        <SettingList>
          <SettingRow title="Launch at login" description="Start Harp automatically when you sign in. Applies immediately.">
            <Switch checked={!!autostart} disabled={autostart === null} onChange={toggleAutostart} />
          </SettingRow>
          <SettingRow title="Start minimized" description="Open in the system tray instead of showing the window.">
            <Switch checked={app.startMinimized} onChange={(startMinimized) => setApp({ startMinimized })} />
          </SettingRow>
          <SettingRow title="Connect on launch" description="Connect automatically when Harp starts.">
            <Switch checked={app.connectOnLaunch} onChange={(connectOnLaunch) => setApp({ connectOnLaunch })} />
          </SettingRow>
          <SettingRow title="Keep running in the tray" description="Closing the window hides Harp instead of quitting, so the connection stays up.">
            <Switch checked={app.closeToTray} onChange={(closeToTray) => setApp({ closeToTray })} />
          </SettingRow>
        </SettingList>
      </Card>

      <Card>
        <CardHeader icon={<Palette className="size-4" />} title="Appearance" />
        <SettingList>
          <SettingRow title="Theme">
            <Segmented
              value={app.theme}
              onChange={(theme) => setApp({ theme })}
              options={[
                { value: "system", label: "System", icon: <Monitor className="size-3.5" /> },
                { value: "light", label: "Light", icon: <Sun className="size-3.5" /> },
                { value: "dark", label: "Dark", icon: <Moon className="size-3.5" /> },
              ]}
            />
          </SettingRow>
        </SettingList>
      </Card>

      <Card>
        <CardHeader icon={<ScrollText className="size-4" />} title="Logging" />
        <SettingList>
          <SettingRow title="Log level" description="Debug and trace show every connection and are useful for troubleshooting.">
            <Select className="w-36" value={app.logLevel} onChange={(event) => setApp({ logLevel: event.target.value })}>
              {["trace", "debug", "info", "warn", "error"].map((level) => (
                <option key={level} value={level}>
                  {level[0].toUpperCase() + level.slice(1)}
                </option>
              ))}
            </Select>
          </SettingRow>
        </SettingList>
      </Card>

      <Card>
        <CardHeader icon={<Code2 className="size-4" />} title="Advanced" />
        <SettingList>
          <SettingRow title="Clash API" description="Expose a Clash compatible control API for dashboards such as metacubexd or yacd." error={errors["app.clashApiAddress"]}>
            <Switch checked={app.clashApi} onChange={(clashApi) => setApp({ clashApi, clashApiSecret: clashApi && !app.clashApiSecret ? randomPassword(24) : app.clashApiSecret })} />
          </SettingRow>
          {app.clashApi && (
            <div className="grid grid-cols-2 gap-4 px-5 py-3.5">
              <Field label="Address">
                <Input mono value={app.clashApiAddress} onChange={(event) => setApp({ clashApiAddress: event.target.value.trim() })} />
              </Field>
              <Field label="Secret">
                <div className="flex gap-2">
                  <Input mono className="flex-1" value={app.clashApiSecret} onChange={(event) => setApp({ clashApiSecret: event.target.value })} />
                  <Button variant="secondary" icon={<Wand2 className="size-4" />} onClick={() => setApp({ clashApiSecret: randomPassword(24) })}>
                    Generate
                  </Button>
                </div>
              </Field>
            </div>
          )}
          <div className="grid grid-cols-2 gap-4 px-5 py-3.5">
            <Field label="GeoIP rule-set URL" description="%s is replaced by the country code." error={errors["routing.geoipUrl"]}>
              <Input mono value={draft.routing.geoipUrl} onChange={(event) => update((d) => ({ ...d, routing: { ...d.routing, geoipUrl: event.target.value } }))} />
            </Field>
            <Field label="GeoSite rule-set URL" description="%s is replaced by the category name." error={errors["routing.geositeUrl"]}>
              <Input mono value={draft.routing.geositeUrl} onChange={(event) => update((d) => ({ ...d, routing: { ...d.routing, geositeUrl: event.target.value } }))} />
            </Field>
          </div>
          <SettingRow title="Reset settings" description="Restore every setting to its default value.">
            <Button variant="danger" size="sm" icon={<RotateCcw className="size-3.5" />} onClick={reset}>
              Reset…
            </Button>
          </SettingRow>
        </SettingList>
      </Card>

      <Card>
        <CardHeader icon={<Info className="size-4" />} title="About" />
        <div className="flex items-start gap-5 border-t border-line px-5 py-5">
          <Logo size={56} />
          <div className="min-w-0 flex-1">
            <div className="text-[16px] font-semibold">Harp {info?.version}</div>
            <div className="text-[12.5px] text-muted">An unofficial Cloudflare WARP client. Not affiliated with Cloudflare.</div>
            <dl className="mt-4 grid grid-cols-[140px_1fr] gap-y-1.5 text-[12.5px]">
              <dt className="text-faint">Network engine</dt>
              <dd>sing-box {info?.singBoxVersion}</dd>
              <dt className="text-faint">Runtime</dt>
              <dd>
                {info?.goVersion} · {info?.os}/{info?.arch}
              </dd>
              <dt className="text-faint">Privileges</dt>
              <dd>{info?.elevated ? "Administrator" : "Standard user"}</dd>
              <dt className="text-faint">Data folder</dt>
              <dd className="selectable truncate font-mono text-[12px]">{info?.dataDir}</dd>
            </dl>
            <div className="mt-4 flex gap-2">
              <Button size="sm" variant="secondary" icon={<ExternalLink className="size-3.5" />} onClick={() => api.OpenURL(REPOSITORY)}>
                Project page
              </Button>
              <Button size="sm" variant="ghost" onClick={() => api.OpenURL(`${REPOSITORY}/blob/master/LICENSE`)}>
                License (GPL-3.0)
              </Button>
            </div>
            <p className="mt-4 text-[11.5px] leading-relaxed text-faint">
              Based on wgcf by ViRb3 (MIT). Cloudflare and WARP are trademarks of Cloudflare, Inc.
            </p>
          </div>
        </div>
      </Card>
    </div>
  );
}
