import { useId } from "react";

/** Points of the harp glyph in a 100×100 box, matching the app icon. */
function harpPoints() {
  const pillarX = 29;
  const top = 22;
  const bottom = 80;
  const neckEnd = [75, 33];
  const frame: [number, number][] = [
    [pillarX, bottom],
    [pillarX, top],
  ];
  const neck: [number, number][] = [];
  for (let i = 1; i <= 30; i++) {
    const t = i / 30;
    neck.push([
      pillarX + (neckEnd[0] - pillarX) * t,
      top + (neckEnd[1] - top) * t + 5 * Math.sin(2 * Math.PI * t) * (1 - t * 0.3),
    ]);
  }
  const sound: [number, number][] = [];
  for (let i = 1; i <= 20; i++) {
    const t = i / 20;
    sound.push([
      neckEnd[0] + (pillarX - neckEnd[0]) * t,
      neckEnd[1] + (bottom - neckEnd[1]) * t + 3.5 * Math.sin(Math.PI * t),
    ]);
  }
  frame.push(...neck, ...sound);
  const nearest = (curve: [number, number][], x: number) =>
    curve.reduce((best, p) => (Math.abs(p[0] - x) < Math.abs(best[0] - x) ? p : best))[1];
  const strings = [38.5, 47.5, 56.5, 65.5]
    .map((x) => [x, nearest(neck, x) + 4, nearest(sound, x) - 4] as const)
    .filter(([, y1, y2]) => y2 - y1 > 4);
  return { frame: frame.map(([x, y]) => `${x.toFixed(2)},${y.toFixed(2)}`).join(" "), strings };
}

const HARP = harpPoints();

export function HarpGlyph({ className, strokeWidth = 7.5 }: { className?: string; strokeWidth?: number }) {
  return (
    <svg viewBox="0 0 100 100" className={className} fill="none" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round">
      <polyline points={HARP.frame} strokeWidth={strokeWidth} />
      {HARP.strings.map(([x, y1, y2]) => (
        <line key={x} x1={x} y1={y1} x2={x} y2={y2} strokeWidth={strokeWidth * 0.47} />
      ))}
    </svg>
  );
}

export function Logo({ size = 32, muted = false }: { size?: number; muted?: boolean }) {
  const id = useId();
  return (
    <svg width={size} height={size} viewBox="0 0 100 100" aria-hidden="true">
      <defs>
        <linearGradient id={`${id}-bg`} x1="0" y1="1" x2="1" y2="0">
          <stop offset="0" stopColor={muted ? "#4b5563" : "#7c3aed"} />
          <stop offset="1" stopColor={muted ? "#94a3b8" : "#06b6d4"} />
        </linearGradient>
      </defs>
      <rect width="100" height="100" rx="23.5" fill={`url(#${id}-bg)`} />
      <g color="white">
        <polyline
          points={HARP.frame}
          fill="none"
          stroke="currentColor"
          strokeWidth={7.5}
          strokeLinecap="round"
          strokeLinejoin="round"
        />
        {HARP.strings.map(([x, y1, y2]) => (
          <line key={x} x1={x} y1={y1} x2={x} y2={y2} stroke="currentColor" strokeWidth={3.5} strokeLinecap="round" />
        ))}
      </g>
    </svg>
  );
}
