import 'server-only';
import { D2, type CompileOptions } from '@terrastruct/d2';

/**
 * Compile a D2 document to SVG, server-side.
 *
 * D2 (https://d2lang.com) is a text-to-diagram language. `@terrastruct/d2` is a
 * WASM wrapper around the real compiler that runs it in a worker thread and
 * hands back an SVG string. This module is the only place that touches it, so
 * the rest of the app deals in plain SVG and never has to know the compiler
 * exists.
 *
 * Everything here is server-only on purpose: the WASM binary is ~22MB and is
 * loaded into the Node process, never shipped to a browser.
 */

/* ------------------------------------------------------------------ errors */

/** A D2 document that could not be compiled, or output we refused to render. */
export class D2RenderError extends Error {
  /** D2's own diagnostics, flattened into one human-readable line. */
  readonly detail: string;

  constructor(message: string, detail = '') {
    super(message);
    this.name = 'D2RenderError';
    this.detail = detail;
  }
}

/* ------------------------------------------------------------------ options */

export type D2Theme = 'dark' | 'light';

export interface RenderD2Options {
  /** Which D2 palette to compile against. The app is dark-first. */
  theme?: D2Theme;
}

/**
 * D2 theme ids. Only two are used: `0` is the light neutral palette, `200` is
 * its dark counterpart.
 *
 * The theme is deliberately not the last word on colour — `applyDesignTokens`
 * rewrites the rendered palette onto the app's CSS variables afterwards — but
 * the two palettes are not equivalent, so the right one still has to be chosen
 * at compile time.
 */
const THEME_ID: Record<D2Theme, number> = { dark: 200, light: 0 };

/**
 * The compiler is not fast enough to hang a render on indefinitely. A stuck
 * compile is reported as a normal diagram error rather than a stuck request.
 */
const RENDER_TIMEOUT_MS = 20_000;

/* ------------------------------------------------------------------- engine */

/**
 * One compiler per process.
 *
 * Each `D2` instance boots its own worker thread and loads a 22MB WASM binary,
 * and `D2` exposes no way to shut a worker down — so creating one per request
 * would leak a thread every render. The instance is kept alive for the life of
 * the process and created lazily so importing this module costs nothing.
 */
let engine: Promise<D2> | null = null;

function getEngine(): Promise<D2> {
  if (!engine) {
    engine = (async () => new D2())();

    // A failed boot poisons the cached promise forever. Drop it so the next
    // request can try again rather than replaying the same failure.
    engine.catch(() => {
      engine = null;
    });
  }

  return engine;
}

/**
 * Serialises every call onto the shared instance.
 *
 * `D2` keeps the pending request's resolve callback in a single instance field,
 * so two overlapping calls overwrite each other and the first one never settles
 * — the request hangs forever. One call at a time is the only way to drive this
 * API safely.
 */
let queue: Promise<unknown> = Promise.resolve();

function enqueue<T>(task: () => Promise<T>): Promise<T> {
  const run = queue.then(task, task);

  // Keep the chain alive even when a task rejects, and keep waiting on the
  // *underlying* work rather than the timed-out wrapper so ordering holds.
  queue = run.then(
    () => undefined,
    () => undefined,
  );

  return run;
}

/* ----------------------------------------------------------------- compiling */

/**
 * Renders a D2 document to a sanitised, design-system-coloured SVG string.
 *
 * Throws {@link D2RenderError} for a malformed document, a compiler failure, or
 * output that fails sanitisation. It never throws anything else and never
 * returns partial output — callers can treat a rejection as "show an empty
 * state" without a `try/catch` around every other error.
 */
export async function renderD2(source: string, opts: RenderD2Options = {}): Promise<string> {
  const theme = opts.theme ?? 'dark';

  if (typeof source !== 'string' || source.trim() === '') {
    throw new D2RenderError('The architecture document is empty.');
  }

  // The timeout sits outside the queue: a render that gives up must not reorder
  // the next compile, and the abandoned work keeps holding the queue until it
  // finishes on its own.
  const rendered = await withTimeout(
    enqueue(async () => {
      const d2 = await getEngine();

      // `noXMLTag` drops the `<?xml ...?>` prologue, which is meaningless
      // inside an HTML body. `pad` is deliberately left at its default:
      // non-default values make this build emit a viewBox that does not match
      // the drawing.
      const options: CompileOptions = {
        layout: 'elk',
        themeID: THEME_ID[theme],
        darkThemeID: THEME_ID[theme],
        noXMLTag: true,
        // Stable suffix on generated ids, so two diagrams on one page cannot
        // collide on element ids.
        salt: 'archon',
      };

      try {
        // The `fs` form is used rather than the `(source, options)` shorthand:
        // the published types declare that overload's second argument as a
        // partial `CompileRequest` — which has no `options` field at all and so
        // cannot describe compile options. The `fs` overload is the one the
        // types get right.
        const compiled = await d2.compile({
          fs: { index: source },
          inputPath: 'index',
          options,
        });

        return await d2.render(compiled.diagram, compiled.renderOptions);
      } catch (error) {
        const { message, detail } = describeCompileError(error);
        throw new D2RenderError(message, detail);
      }
    }),
    RENDER_TIMEOUT_MS,
    'rendering the diagram',
  );

  // Recolour before validating, so the string that is actually injected is the
  // one that was checked. `applyDesignTokens` only substitutes from the fixed
  // SLOT table, so it cannot introduce anything the sanitiser rejects.
  return sanitizeSvg(applyDesignTokens(rendered));
}

async function withTimeout<T>(work: Promise<T>, ms: number, what: string): Promise<T> {
  let timer: ReturnType<typeof setTimeout> | undefined;

  try {
    return await Promise.race([
      work,
      new Promise<never>((_, reject) => {
        timer = setTimeout(
          () => reject(new D2RenderError(`Gave up after ${ms / 1000}s while ${what}.`)),
          ms,
        );
      }),
    ]);
  } finally {
    clearTimeout(timer);
  }
}

/* ------------------------------------------------------------ error messages */

/**
 * The compiler reports failures by throwing an `Error` whose *message* is a
 * JSON array of `{ range, errmsg }` records. That is unreadable in a UI, so it
 * is unpacked here into a single line. Anything that is not in that shape is
 * passed through as-is.
 */
function describeCompileError(error: unknown): { message: string; detail: string } {
  const raw = error instanceof Error ? error.message : String(error);

  try {
    const parsed: unknown = JSON.parse(raw);

    if (Array.isArray(parsed)) {
      const messages = parsed
        .map((entry) => (entry as { errmsg?: unknown }).errmsg)
        .filter((errmsg): errmsg is string => typeof errmsg === 'string');

      if (messages.length > 0) {
        return { message: 'The D2 document could not be compiled.', detail: messages.join(' · ') };
      }
    }
  } catch {
    // Not JSON — fall through to the raw message.
  }

  return { message: 'The D2 compiler failed.', detail: raw };
}

/* --------------------------------------------------------------- sanitising */

/**
 * Constructs that must never reach `dangerouslySetInnerHTML`.
 *
 * The SVG is third-party output: D2 source can set shape links, tooltips and
 * icon URLs, and the SVG document format is itself a scripting context — an
 * `<svg>` carrying `<script>` or an `onload` attribute executes as soon as the
 * markup is parsed. Inlining it therefore has to be treated as untrusted input,
 * the same as any other `dangerouslySetInnerHTML` call. There is no sanitiser
 * option in the library, so this is a conservative allow-list check: it does not
 * try to repair the document, it refuses anything it does not recognise.
 */
const FORBIDDEN: ReadonlyArray<{ pattern: RegExp; label: string }> = [
  { pattern: /<\s*script/i, label: 'a <script> element' },
  { pattern: /<\s*(foreignObject|iframe|object|embed)\b/i, label: 'an embedded document' },
  { pattern: /<\s*use\b/i, label: 'a <use> reference' },
  { pattern: /<\s*(image|audio|video)\b/i, label: 'an external media reference' },
  { pattern: /<\s*a\s/i, label: 'a hyperlink' },
  { pattern: /\son[a-z]+\s*=\s*["']/i, label: 'an inline event handler' },
  { pattern: /javascript\s*:/i, label: 'a javascript: URL' },
  { pattern: /data\s*:\s*text\/html/i, label: 'an inline HTML document' },
];

/**
 * Checks that `input` is a single, well-formed `<svg>` document containing
 * nothing that could execute, and returns it trimmed.
 *
 * The library offers no sanitisation of its own, so this is deliberately
 * strict: an unrecognised structure is an error, not something to pass through.
 */
export function sanitizeSvg(input: string): string {
  const svg = input.trim();

  if (!svg.startsWith('<svg')) {
    throw new D2RenderError('The diagram renderer did not return an <svg> element.');
  }

  if (!svg.endsWith('</svg>')) {
    throw new D2RenderError('The rendered diagram is truncated.');
  }

  // D2 nests one <svg> inside the outer document element, so counting them is
  // a cheap structural check that the output was not cut short.
  const opens = svg.match(/<svg\b/gi)?.length ?? 0;
  const closes = svg.match(/<\/svg>/gi)?.length ?? 0;

  if (opens === 0 || opens !== closes) {
    throw new D2RenderError('The rendered diagram is not a well-formed <svg> element.');
  }

  for (const { pattern, label } of FORBIDDEN) {
    if (pattern.test(svg)) {
      throw new D2RenderError('The rendered diagram was rejected because it contains ' + label + '.');
    }
  }

  return svg;
}

/* ----------------------------------------------------------- design tokens */

/**
 * D2 addresses its palette by *slot*, not by colour: every shape, connector and
 * label is painted by a rule such as `.d2-1234 .fill-N7{fill:#FFFFFF}`, and the
 * literal colour is decided by the theme chosen at compile time.
 *
 * Rewriting those rules by slot — rather than by hex value — means the diagram
 * can point at the app's CSS custom properties instead of carrying a palette
 * baked in at build time. `var(--fg)` and friends resolve inside the SVG because
 * custom properties inherit into inline SVG from the document, so the diagram
 * follows the colour scheme on its own: dark-first, and correct in light mode,
 * with no duplicated palette and no hardcoded hex anywhere in the stylesheet.
 *
 * The returned string is derived from a fixed table, so this step cannot
 * introduce anything the sanitiser above would have rejected.
 */

/** Palette slot -> the custom property that should paint it. */
const SLOT: Record<string, string> = {
  N1: '--d2-ink', // primary label text
  N2: '--d2-ink-2', // secondary text, edge labels
  N3: '--d2-ink-3', // faint text
  N4: '--d2-line', // hairlines
  N5: '--d2-line',
  N6: '--d2-surface', // subtle canvas, group fills
  N7: '--d2-canvas', // the diagram background
  B1: '--d2-edge', // shape borders and connectors
  B2: '--d2-accent', // deliberate accent
  B3: '--d2-accent-bg',
  B4: '--d2-accent-bg',
  B5: '--d2-surface',
  B6: '--d2-surface', // default shape fill
  AA2: '--d2-accent-2',
  AA4: '--d2-accent-2-bg',
  AA5: '--d2-accent-2-bg',
  AB4: '--d2-accent-3-bg',
  AB5: '--d2-accent-3-bg',
};

/**
 * Matches one palette rule, keeping the theme-generated `.d2-<id>` scope
 * selector so the rules stay scoped to this diagram.
 */
const PALETTE_RULE = /\.d2-(\d+)\s+\.(fill|stroke)-([A-Z]{1,2}\d)\{(?:fill|stroke):#[0-9a-fA-F]{6};\}/g;

/**
 * D2 inlines a dozen Source Sans Pro webfonts as base64. The diagram is
 * painted in the UI font instead (see `architect.module.css`), which makes
 * roughly two thirds of the output dead weight — about 18KB on a diagram this
 * size, paid on every render. Base64 is `[A-Za-z0-9+/=]`, so the blocks are
 * unambiguous and safe to cut without a real CSS parser.
 */
const EMBEDDED_FONTS = /@font-face\s*\{[^}]*\}/g;

function applyDesignTokens(svg: string): string {
  return svg.replace(EMBEDDED_FONTS, '').replace(
    PALETTE_RULE,
    (match: string, scope: string, property: string, slot: string) => {
      const token = SLOT[slot];

      // An unrecognised slot keeps its compiled colour rather than being
      // silently dropped — a slightly off palette beats a missing shape.
      if (!token) return match;

      return `.d2-${scope} .${property}-${slot}{${property}:var(${token});}`;
    },
  );
}