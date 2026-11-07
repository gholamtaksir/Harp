import { useEffect, useId, useMemo, useRef, useState } from "react";
import type { Stats } from "../lib/api";
import { formatSpeed, formatTime } from "../lib/format";

const WINDOW = 120; // seconds shown
const PADDING = { top: 12, right: 12, bottom: 22, left: 64 };

function niceMax(value: number): number {
  if (value <= 0) return 1024;
  const exponent = Math.floor(Math.log(value) / Math.log(1024));
  const unit = 1024 ** exponent;
  const scaled = value / unit;
  const steps = [1, 2, 4, 5, 8, 10, 20, 40, 50, 80, 100, 200, 400, 500, 800, 1000];
  const step = steps.find((s) => s >= scaled * 1.1) ?? 1024;
  return step * unit;
}

/** Smooth path through points using a monotone cubic interpolation. */
function smoothPath(points: [number, number][]): string {
  if (points.length === 0) return "";
  if (points.length === 1) return `M${points[0][0]},${points[0][1]}`;
  let path = `M${points[0][0]},${points[0][1]}`;
  for (let i = 0; i < points.length - 1; i++) {
    const [x0, y0] = points[i];
    const [x1, y1] = points[i + 1];
    const mx = (x0 + x1) / 2;
    path += ` C${mx},${y0} ${mx},${y1} ${x1},${y1}`;
  }
  return path;
}

export function TrafficChart({ stats, height = 210 }: { stats: Stats[]; height?: number }) {
  const id = useId();
  const container = useRef<HTMLDivElement>(null);
  const [width, setWidth] = useState(600);
  const [hover, setHover] = useState<number | null>(null);

  useEffect(() => {
    const element = container.current;
    if (!element) return;
    const observer = new ResizeObserver(([entry]) => setWidth(Math.max(200, entry.contentRect.width)));
    observer.observe(element);
    return () => observer.disconnect();
  }, []);

  const data = useMemo(() => {
    const recent = stats.slice(-WINDOW);
    const padding = Array.from({ length: Math.max(0, WINDOW - recent.length) }, () => null);
    return [...padding, ...recent];
  }, [stats]);

  const max = niceMax(Math.max(0, ...data.map((s) => (s ? Math.max(s.up, s.down) : 0))));
  const innerWidth = width - PADDING.left - PADDING.right;
  const innerHeight = height - PADDING.top - PADDING.bottom;
  const x = (index: number) => PADDING.left + (index / (WINDOW - 1)) * innerWidth;
  const y = (value: number) => PADDING.top + innerHeight - (value / max) * innerHeight;

  const series = (key: "up" | "down") => {
    const points: [number, number][] = [];
    data.forEach((s, i) => {
      if (s) points.push([x(i), y(s[key])]);
    });
    return points;
  };
  const down = series("down");
  const up = series("up");
  const area = (points: [number, number][]) =>
    points.length > 1
      ? `${smoothPath(points)} L${points[points.length - 1][0]},${y(0)} L${points[0][0]},${y(0)} Z`
      : "";

  const ticks = [0, 0.5, 1].map((f) => f * max);
  const hovered = hover !== null ? data[hover] : null;

  return (
    <div ref={container} className="relative w-full select-none" style={{ height }}>
      <svg
        width={width}
        height={height}
        className="block"
        onMouseLeave={() => setHover(null)}
        onMouseMove={(event) => {
          const rect = event.currentTarget.getBoundingClientRect();
          const index = Math.round(((event.clientX - rect.left - PADDING.left) / innerWidth) * (WINDOW - 1));
          setHover(index >= 0 && index < WINDOW && data[index] ? index : null);
        }}
      >
        <defs>
          <linearGradient id={`${id}-down`} x1="0" y1="0" x2="0" y2="1">
            <stop offset="0" stopColor="var(--accent)" stopOpacity="0.35" />
            <stop offset="1" stopColor="var(--accent)" stopOpacity="0" />
          </linearGradient>
          <linearGradient id={`${id}-up`} x1="0" y1="0" x2="0" y2="1">
            <stop offset="0" stopColor="var(--cyan)" stopOpacity="0.28" />
            <stop offset="1" stopColor="var(--cyan)" stopOpacity="0" />
          </linearGradient>
        </defs>

        {ticks.map((tick) => (
          <g key={tick}>
            <line x1={PADDING.left} x2={width - PADDING.right} y1={y(tick)} y2={y(tick)} stroke="var(--border)" strokeDasharray={tick === 0 ? undefined : "3 4"} />
            <text x={PADDING.left - 10} y={y(tick) + 4} textAnchor="end" fontSize="10.5" fill="var(--faint)" className="tabular font-mono">
              {formatSpeed(tick)}
            </text>
          </g>
        ))}
        {["-2m", "-1m", "now"].map((label, i) => (
          <text key={label} x={x((i * (WINDOW - 1)) / 2)} y={height - 5} textAnchor={i === 0 ? "start" : i === 2 ? "end" : "middle"} fontSize="10.5" fill="var(--faint)">
            {label}
          </text>
        ))}

        <path d={area(down)} fill={`url(#${id}-down)`} />
        <path d={area(up)} fill={`url(#${id}-up)`} />
        <path d={smoothPath(down)} fill="none" stroke="var(--accent)" strokeWidth="2" strokeLinejoin="round" />
        <path d={smoothPath(up)} fill="none" stroke="var(--cyan)" strokeWidth="2" strokeLinejoin="round" />

        {hovered && hover !== null && (
          <g>
            <line x1={x(hover)} x2={x(hover)} y1={PADDING.top} y2={y(0)} stroke="var(--border-strong)" />
            <circle cx={x(hover)} cy={y(hovered.down)} r="3.5" fill="var(--accent)" stroke="var(--panel)" strokeWidth="2" />
            <circle cx={x(hover)} cy={y(hovered.up)} r="3.5" fill="var(--cyan)" stroke="var(--panel)" strokeWidth="2" />
          </g>
        )}
      </svg>

      {hovered && hover !== null && (
        <div
          className="card pointer-events-none absolute top-2 z-10 min-w-36 px-3 py-2 text-[12px]"
          style={{ left: Math.min(Math.max(x(hover) + 12, PADDING.left), width - 160) }}
        >
          <div className="mb-1 text-[11px] text-faint">{formatTime(hovered.time)}</div>
          <div className="flex items-center justify-between gap-4">
            <span className="flex items-center gap-1.5 text-muted">
              <span className="size-2 rounded-full bg-accent" /> Download
            </span>
            <span className="tabular font-mono">{formatSpeed(hovered.down)}</span>
          </div>
          <div className="flex items-center justify-between gap-4">
            <span className="flex items-center gap-1.5 text-muted">
              <span className="size-2 rounded-full bg-cyan" /> Upload
            </span>
            <span className="tabular font-mono">{formatSpeed(hovered.up)}</span>
          </div>
        </div>
      )}
    </div>
  );
}
