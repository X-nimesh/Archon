import type { SVGProps } from 'react';

/**
 * The icon set from the Archon UI reference.
 *
 * Stroke-based, 16px grid, 1.4 stroke width. Icons are decorative by default
 * (`aria-hidden`); pass a `title` to expose one to assistive tech.
 */
export type IconName =
  | 'folder'
  | 'plus'
  | 'dash'
  | 'graph'
  | 'code'
  | 'check'
  | 'up'
  | 'clock'
  | 'menu'
  | 'panel'
  | 'clip'
  | 'git'
  | 'ext'
  | 'x'
  | 'sort'
  | 'arrow'
  | 'warn';

const PATHS: Record<IconName, React.ReactNode> = {
  folder: <path d="M1.5 4.5a1 1 0 0 1 1-1h3.2l1.3 1.5h6.5a1 1 0 0 1 1 1v6a1 1 0 0 1-1 1h-11a1 1 0 0 1-1-1z" />,
  plus: <path d="M8 3v10M3 8h10" />,
  dash: (
    <>
      <rect x="2" y="2" width="5" height="6" rx="1" />
      <rect x="9" y="2" width="5" height="3.5" rx="1" />
      <rect x="2" y="10" width="5" height="4" rx="1" />
      <rect x="9" y="7.5" width="5" height="6.5" rx="1" />
    </>
  ),
  graph: (
    <>
      <circle cx="8" cy="3.5" r="1.8" />
      <circle cx="3.5" cy="12" r="1.8" />
      <circle cx="12.5" cy="12" r="1.8" />
      <path d="M7 5.2l-2.6 5.2M9 5.2l2.6 5.2" />
    </>
  ),
  code: <path d="M5.5 4.5L2 8l3.5 3.5M10.5 4.5L14 8l-3.5 3.5" />,
  check: <path d="M3 8.5l3.2 3L13 4.5" />,
  up: <path d="M8 13V3M3.5 7.5L8 3l4.5 4.5" />,
  clock: (
    <>
      <circle cx="8" cy="8" r="6" />
      <path d="M8 4.5V8l2.5 1.5" />
    </>
  ),
  menu: <path d="M2.5 4h11M2.5 8h11M2.5 12h11" />,
  panel: (
    <>
      <rect x="2" y="3" width="12" height="10" rx="1.5" />
      <path d="M6 3v10" />
    </>
  ),
  clip: (
    <path d="M12.5 7.5l-4.6 4.6a2.5 2.5 0 0 1-3.5-3.5l5-5a1.7 1.7 0 0 1 2.4 2.4l-5 5a.8.8 0 0 1-1.2-1.2l4.4-4.4" />
  ),
  git: (
    <>
      <circle cx="4.5" cy="3.5" r="1.5" />
      <circle cx="4.5" cy="12.5" r="1.5" />
      <circle cx="11.5" cy="6" r="1.5" />
      <path d="M4.5 5v6M11.5 7.5c0 2.2-2.5 2.4-7 3.6" />
    </>
  ),
  ext: (
    <path d="M9 3h4v4M13 3L7.5 8.5M11 9.5V12a1 1 0 0 1-1 1H4a1 1 0 0 1-1-1V6a1 1 0 0 1 1-1h2.5" />
  ),
  x: <path d="M4 4l8 8M12 4l-8 8" />,
  sort: <path d="M5 6.5L8 3.5l3 3M5 9.5l3 3 3-3" />,
  arrow: <path d="M6 3.5L10.5 8 6 12.5" />,
  warn: (
    <>
      <path d="M8 2.5l6 10.5H2z" />
      <path d="M8 7v2.8M8 11.4v.1" />
    </>
  ),
};

export function Icon({
  name,
  ...props
}: { name: IconName } & SVGProps<SVGSVGElement>) {
  return (
    <svg
      className="ic"
      viewBox="0 0 16 16"
      aria-hidden={props['aria-label'] ? undefined : true}
      {...props}
    >
      {PATHS[name]}
    </svg>
  );
}