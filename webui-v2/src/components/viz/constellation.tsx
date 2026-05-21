/* ============================================================
   Constellation — deterministic mini star-field that hints at a
   group's graph structure. Pure presentational; seeded so the same
   group always renders the same dots. Tokens only (pastel scale).

   Lives in the component lib (viz layer) so any screen that wants a
   decorative graph thumbnail can reuse it. No business logic.
   ============================================================ */

const PASTELS = [
  "--pastel-1",
  "--pastel-2",
  "--pastel-3",
  "--pastel-5",
  "--pastel-7",
  "--pastel-9",
];

/** Deterministic LCG so a seed always yields the same field. */
function seededRand(seed: number): () => number {
  let s = seed || 1;
  return () => {
    s = (s * 9301 + 49297) % 233280;
    return s / 233280;
  };
}

interface Node {
  x: number;
  y: number;
  r: number;
  color: string;
}

function generate(seed: number, paletteSize: number, count = 38) {
  const rng = seededRand(seed);
  const clusters = Math.max(1, paletteSize);
  const nodes: Node[] = [];
  for (let c = 0; c < clusters; c++) {
    const cx = 0.18 + rng() * 0.64;
    const cy = 0.18 + rng() * 0.64;
    const nPer = Math.floor(count / clusters) + (c === 0 ? count % clusters : 0);
    for (let i = 0; i < nPer; i++) {
      const ang = rng() * Math.PI * 2;
      const rad = rng() ** 1.5 * 0.13;
      nodes.push({
        x: cx + Math.cos(ang) * rad,
        y: cy + Math.sin(ang) * rad,
        r: 1.5 + rng() * 2,
        color: PASTELS[c % PASTELS.length],
      });
    }
  }
  const edges: Array<[number, number]> = [];
  for (let i = 0; i < nodes.length; i++) {
    if (rng() < 0.55) {
      const j = Math.floor(rng() * nodes.length);
      if (j !== i) {
        const dx = nodes[i].x - nodes[j].x;
        const dy = nodes[i].y - nodes[j].y;
        if (Math.sqrt(dx * dx + dy * dy) < 0.18) edges.push([i, j]);
      }
    }
  }
  return { nodes, edges };
}

export interface ConstellationProps {
  /** Stable seed — same seed → same field. */
  seed: number;
  /** Number of pastel clusters (1–6). */
  clusters?: number;
  /** Render the "nothing indexed yet" dashed-circle variant. */
  empty?: boolean;
  className?: string;
}

export function Constellation({ seed, clusters = 4, empty, className }: ConstellationProps) {
  if (empty) {
    return (
      <svg
        viewBox="0 0 100 100"
        preserveAspectRatio="none"
        className={className}
        aria-hidden
      >
        <g stroke="var(--text-4)" strokeDasharray="2 3" fill="none" opacity="0.5">
          <circle cx="50" cy="50" r="22" />
        </g>
        <circle cx="50" cy="50" r="2.4" fill="var(--text-4)" />
      </svg>
    );
  }

  const { nodes, edges } = generate(seed, clusters);
  return (
    <svg viewBox="0 0 100 100" preserveAspectRatio="none" className={className} aria-hidden>
      <g stroke="var(--border-strong)" strokeWidth="0.3" fill="none" opacity="0.6">
        {edges.map(([i, j], k) => (
          <line
            key={k}
            x1={nodes[i].x * 100}
            y1={nodes[i].y * 100}
            x2={nodes[j].x * 100}
            y2={nodes[j].y * 100}
          />
        ))}
      </g>
      <g>
        {nodes.map((n, k) => (
          <circle key={k} cx={n.x * 100} cy={n.y * 100} r={n.r * 0.7} fill={`var(${n.color})`} />
        ))}
      </g>
    </svg>
  );
}

/** Stable seed from a group slug (so backend needn't return a seed field). */
export function seedFromString(s: string): number {
  let h = 0;
  for (let i = 0; i < s.length; i++) h = (h * 31 + s.charCodeAt(i)) % 233280;
  return h || 7;
}
