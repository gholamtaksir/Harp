import type { Rule } from "./api";
import { countryName, randomId } from "./format";

export type Action = "warp" | "direct" | "block";

export type RuleType =
  | "suffix"
  | "domain"
  | "keyword"
  | "regex"
  | "ip"
  | "app"
  | "path"
  | "path_regex"
  | "port"
  | "protocol"
  | "network"
  | "geoip"
  | "geosite"
  | "ruleset"
  | "private";

export type RuleGroup = "Websites" | "Addresses" | "Applications" | "Traffic" | "Lists";

export interface RuleTypeInfo {
  type: RuleType;
  label: string;
  group: RuleGroup;
  description: string;
  placeholder: string;
  /** Whether the value may hold several comma separated entries. */
  multiple: boolean;
}

export const RULE_TYPES: RuleTypeInfo[] = [
  {
    type: "suffix",
    label: "Domain & subdomains",
    group: "Websites",
    description: "Matches the domain and every subdomain, e.g. example.com also matches www.example.com.",
    placeholder: "example.com, ir",
    multiple: true,
  },
  {
    type: "domain",
    label: "Exact domain",
    group: "Websites",
    description: "Matches exactly this host name and nothing else.",
    placeholder: "chat.example.com",
    multiple: true,
  },
  {
    type: "keyword",
    label: "Domain keyword",
    group: "Websites",
    description: "Matches any domain that contains this text.",
    placeholder: "google",
    multiple: true,
  },
  {
    type: "regex",
    label: "Domain regex",
    group: "Websites",
    description: "Matches domains with a regular expression (Go RE2 syntax).",
    placeholder: "^cdn[0-9]+\\.example\\.com$",
    multiple: false,
  },
  {
    type: "geosite",
    label: "Site category",
    group: "Lists",
    description: "A curated list of domains, such as all Google services or known ad servers.",
    placeholder: "category-ads-all",
    multiple: true,
  },
  {
    type: "ip",
    label: "IP address / range",
    group: "Addresses",
    description: "Matches destination IPv4/IPv6 addresses or CIDR ranges. Domains are resolved first.",
    placeholder: "10.0.0.0/8, 1.1.1.1, 2001:db8::/32",
    multiple: true,
  },
  {
    type: "geoip",
    label: "Country",
    group: "Lists",
    description: "Matches IP addresses located in a country.",
    placeholder: "ir",
    multiple: true,
  },
  {
    type: "private",
    label: "Local network",
    group: "Addresses",
    description: "Private and LAN addresses such as 192.168.x.x, 10.x.x.x and localhost.",
    placeholder: "",
    multiple: false,
  },
  {
    type: "app",
    label: "Application",
    group: "Applications",
    description: "Matches traffic from a program by its executable name.",
    placeholder: "chrome.exe",
    multiple: true,
  },
  {
    type: "path",
    label: "Application path",
    group: "Applications",
    description: "Matches traffic from a program by its full executable path.",
    placeholder: "C:\\Program Files\\App\\app.exe",
    multiple: false,
  },
  {
    type: "path_regex",
    label: "Application path regex",
    group: "Applications",
    description: "Matches executable paths with a regular expression.",
    placeholder: "^/usr/bin/.+",
    multiple: false,
  },
  {
    type: "port",
    label: "Port",
    group: "Traffic",
    description: "Matches a destination port or a range such as 6000-7000.",
    placeholder: "443, 6000-7000",
    multiple: true,
  },
  {
    type: "protocol",
    label: "Protocol",
    group: "Traffic",
    description: "Matches the detected protocol of the connection.",
    placeholder: "quic",
    multiple: true,
  },
  {
    type: "network",
    label: "Network",
    group: "Traffic",
    description: "Matches all TCP or all UDP traffic.",
    placeholder: "udp",
    multiple: false,
  },
  {
    type: "ruleset",
    label: "Rule set file",
    group: "Lists",
    description: "A sing-box rule set: a URL or local path to a .srs (binary) or .json (source) file.",
    placeholder: "https://example.com/list.srs",
    multiple: false,
  },
];

export const RULE_TYPE_MAP = Object.fromEntries(RULE_TYPES.map((info) => [info.type, info])) as Record<
  RuleType,
  RuleTypeInfo
>;

export function ruleTypeInfo(type: string): RuleTypeInfo {
  return RULE_TYPE_MAP[type as RuleType] ?? RULE_TYPE_MAP.suffix;
}

export const ACTIONS: { value: Action; label: string; short: string; description: string }[] = [
  { value: "warp", label: "Through WARP", short: "WARP", description: "Encrypted and sent through Cloudflare WARP." },
  { value: "direct", label: "Bypass WARP", short: "Direct", description: "Sent directly over your normal connection." },
  { value: "block", label: "Block", short: "Block", description: "Connections are refused." },
];

export const PROTOCOLS = ["http", "tls", "quic", "dns", "stun", "bittorrent", "dtls", "ssh", "rdp", "ntp"];

export const GEOSITE_SUGGESTIONS: { value: string; label: string }[] = [
  { value: "category-ads-all", label: "Ads & trackers" },
  { value: "category-ir", label: "Iranian websites" },
  { value: "cn", label: "Chinese websites" },
  { value: "geolocation-!cn", label: "Non-Chinese websites" },
  { value: "google", label: "Google" },
  { value: "youtube", label: "YouTube" },
  { value: "openai", label: "OpenAI / ChatGPT" },
  { value: "anthropic", label: "Anthropic / Claude" },
  { value: "github", label: "GitHub" },
  { value: "microsoft", label: "Microsoft" },
  { value: "apple", label: "Apple" },
  { value: "amazon", label: "Amazon" },
  { value: "netflix", label: "Netflix" },
  { value: "spotify", label: "Spotify" },
  { value: "telegram", label: "Telegram" },
  { value: "twitter", label: "X / Twitter" },
  { value: "facebook", label: "Facebook" },
  { value: "instagram", label: "Instagram" },
  { value: "whatsapp", label: "WhatsApp" },
  { value: "discord", label: "Discord" },
  { value: "tiktok", label: "TikTok" },
  { value: "steam", label: "Steam" },
  { value: "category-games", label: "Games" },
  { value: "category-porn", label: "Adult content" },
];

const COUNTRY_CODES =
  "AD AE AF AG AI AL AM AO AQ AR AS AT AU AW AX AZ BA BB BD BE BF BG BH BI BJ BL BM BN BO BQ BR BS BT BV BW BY BZ CA CC CD CF CG CH CI CK CL CM CN CO CR CU CV CW CX CY CZ DE DJ DK DM DO DZ EC EE EG EH ER ES ET FI FJ FK FM FO FR GA GB GD GE GF GG GH GI GL GM GN GP GQ GR GS GT GU GW GY HK HM HN HR HT HU ID IE IL IM IN IO IQ IR IS IT JE JM JO JP KE KG KH KI KM KN KP KR KW KY KZ LA LB LC LI LK LR LS LT LU LV LY MA MC MD ME MF MG MH MK ML MM MN MO MP MQ MR MS MT MU MV MW MX MY MZ NA NC NE NF NG NI NL NO NP NR NU NZ OM PA PE PF PG PH PK PL PM PN PR PS PT PW PY QA RE RO RS RU RW SA SB SC SD SE SG SH SI SJ SK SL SM SN SO SR SS ST SV SX SY SZ TC TD TF TG TH TJ TK TL TM TN TO TR TT TV TW TZ UA UG UM US UY UZ VA VC VE VG VI VN VU WF WS YE YT ZA ZM ZW";

export const COUNTRIES: { code: string; name: string }[] = COUNTRY_CODES.split(" ")
  .map((code) => ({ code: code.toLowerCase(), name: countryName(code) }))
  .sort((a, b) => a.name.localeCompare(b.name));

export interface Preset {
  id: string;
  title: string;
  description: string;
  rules: { action: Action; type: RuleType; value: string; note: string }[];
}

export const PRESETS: Preset[] = [
  {
    id: "iran-direct",
    title: "Iranian sites bypass WARP",
    description: "Local websites and Iranian IP ranges use your normal connection.",
    rules: [
      { action: "direct", type: "geosite", value: "category-ir", note: "Iranian websites" },
      { action: "direct", type: "geoip", value: "ir", note: "Iranian IP ranges" },
    ],
  },
  {
    id: "china-direct",
    title: "Chinese sites bypass WARP",
    description: "Mainland China websites and IP ranges use your normal connection.",
    rules: [
      { action: "direct", type: "geosite", value: "cn", note: "Chinese websites" },
      { action: "direct", type: "geoip", value: "cn", note: "Chinese IP ranges" },
    ],
  },
  {
    id: "block-ads",
    title: "Block ads & trackers",
    description: "Refuses connections to known advertising and tracking domains.",
    rules: [{ action: "block", type: "geosite", value: "category-ads-all", note: "Ads & trackers" }],
  },
  {
    id: "block-torrent",
    title: "Block BitTorrent",
    description: "Prevents peer-to-peer file sharing traffic from using WARP.",
    rules: [{ action: "block", type: "protocol", value: "bittorrent", note: "BitTorrent" }],
  },
  {
    id: "ai-warp",
    title: "AI services through WARP",
    description: "Always route ChatGPT, Claude and Gemini through WARP.",
    rules: [
      { action: "warp", type: "geosite", value: "openai", note: "OpenAI / ChatGPT" },
      { action: "warp", type: "geosite", value: "anthropic", note: "Anthropic / Claude" },
      { action: "warp", type: "suffix", value: "gemini.google.com", note: "Google Gemini" },
    ],
  },
  {
    id: "streaming-warp",
    title: "Streaming through WARP",
    description: "Always route YouTube, Netflix and Spotify through WARP.",
    rules: [
      { action: "warp", type: "geosite", value: "youtube", note: "YouTube" },
      { action: "warp", type: "geosite", value: "netflix", note: "Netflix" },
      { action: "warp", type: "geosite", value: "spotify", note: "Spotify" },
    ],
  },
];

/** Creates rules for a value that may contain several comma separated entries. */
export function makeRules(action: Action, type: RuleType, value: string, note: string, enabled = true): Rule[] {
  const info = RULE_TYPE_MAP[type];
  if (type === "private") {
    return [{ id: randomId(), action, type, value: "", enabled, note }];
  }
  const values = info.multiple
    ? value
        .split(/[,\n]/)
        .map((v) => v.trim())
        .filter(Boolean)
    : [value.trim()].filter(Boolean);
  return values.map((v) => ({ id: randomId(), action, type, value: v, enabled, note }));
}

export function sameRule(a: Pick<Rule, "action" | "type" | "value">, b: Pick<Rule, "action" | "type" | "value">) {
  return a.action === b.action && a.type === b.type && a.value.toLowerCase() === b.value.toLowerCase();
}

/** Human readable value of a rule. */
export function describeValue(rule: Rule): string {
  switch (rule.type) {
    case "private":
      return "Private & LAN addresses";
    case "geoip":
      return `${countryName(rule.value)} (${rule.value.toUpperCase()})`;
    case "geosite": {
      const suggestion = GEOSITE_SUGGESTIONS.find((s) => s.value === rule.value);
      return suggestion ? `${suggestion.label} (${rule.value})` : rule.value;
    }
    default:
      return rule.value;
  }
}

/** Evaluation stage of a rule type, mirroring the engine. */
export function needsResolve(type: string): boolean {
  return type === "ip" || type === "geoip" || type === "ruleset" || type === "private";
}
