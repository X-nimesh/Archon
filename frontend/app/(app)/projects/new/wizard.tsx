'use client';

import {
  useRef,
  useState,
  type FormEvent,
  type ReactNode,
} from 'react';
import { Icon } from '@/components/Icon';
import styles from './new-project.module.css';

/* ============================================================
   Option lists — mirrors the reference demo's constants.
   ============================================================ */

const REGIONS = [
  ['ap-south-1', 'Mumbai'],
  ['ap-southeast-1', 'Singapore'],
  ['ap-southeast-2', 'Sydney'],
  ['eu-central-1', 'Frankfurt'],
  ['eu-west-1', 'Ireland'],
  ['us-east-1', 'N. Virginia'],
  ['us-west-2', 'Oregon'],
] as const;

type RegionCode = (typeof REGIONS)[number][0];

const RN = Object.fromEntries(REGIONS) as Record<RegionCode, string>;

type GeoKey = 'sa' | 'sea' | 'eu' | 'na' | 'gl';
type Env = 'dev' | 'staging' | 'prod';
type Ci = 'pr' | 'push' | 'manual';
type Users =
  | 'Internal staff'
  | 'B2B customers'
  | 'Consumers (public)'
  | 'Developers (public API)';
type Slo = '99.5' | '99.9' | '99.99';
type DataClass = 'pub' | 'int' | 'pii' | 'fin';
type Enforcement = 'Notify only' | 'Require extra approval' | 'Block apply';

const ENV: Record<Env, string> = {
  dev: 'Development',
  staging: 'Staging',
  prod: 'Production',
};

const ENVHINT: Record<Env, string> = {
  dev: 'Short-lived by default. Archon applies an auto-destroy timer and relaxed redundancy.',
  staging:
    'Mirrors production shape at lower capacity. Auto-destroy timer is optional.',
  prod: 'Multi-AZ where the target needs it, deletion protection and a stricter approval policy.',
};

const GEO: Record<GeoKey, string> = {
  sa: 'South Asia',
  sea: 'Southeast Asia and Oceania',
  eu: 'Europe',
  na: 'North America',
  gl: 'global',
};

const GEOREG: Record<GeoKey, RegionCode> = {
  sa: 'ap-south-1',
  sea: 'ap-southeast-1',
  eu: 'eu-central-1',
  na: 'us-east-1',
  gl: 'us-east-1',
};

const DATA: Record<DataClass, string> = {
  pub: 'Public content',
  int: 'Internal business data',
  pii: 'Personal data (PII)',
  fin: 'Payment or financial records',
};

const SLO: Record<Slo, string> = {
  '99.5': 'About 3 h 36 min of downtime allowed per month.',
  '99.9': 'About 43 min of downtime allowed per month.',
  '99.99': 'About 4 min of downtime allowed per month.',
};

const SVC = [
  'EC2',
  'ECS Fargate',
  'Lambda',
  'EKS',
  'RDS',
  'ElastiCache',
  'S3',
  'ALB',
  'CloudFront',
  'SQS',
];

const SVCON = ['EC2', 'RDS', 'S3', 'ALB'];

const STEPS = [
  'Requirement',
  'Source',
  'Workload',
  'Cloud and limits',
  'Review',
] as const;

/* ============================================================
   State
   ============================================================ */

interface Form {
  name: string;
  owner: string;
  desc: string;
  env: Env;
  req: string;
  repo: string;
  green: boolean;
  branch: string;
  root: string;
  ci: Ci;
  users: Users;
  geo: GeoKey;
  mau: string;
  rps: number;
  slo: Slo;
  data: DataClass;
  comp: string[];
  acct: string;
  role: string;
  region: RegionCode;
  dr: RegionCode | '';
  budget: string;
  alerts: string[];
  enf: Enforcement;
  appr: number;
  ttl: number;
  svc: string[];
}

const INITIAL: Form = {
  name: '',
  owner: '',
  desc: '',
  env: 'dev',
  req: '',
  repo: '',
  green: false,
  branch: 'main',
  root: '/',
  ci: 'pr',
  users: 'Internal staff',
  geo: 'sa',
  mau: 'Under 10k',
  rps: 100,
  slo: '99.5',
  data: 'int',
  comp: [],
  acct: '',
  role: '',
  region: 'ap-south-1',
  dr: '',
  budget: '500',
  alerts: ['50', '80', '100'],
  enf: 'Require extra approval',
  appr: 1,
  ttl: 14,
  svc: [...SVCON],
};

/** Keys that can carry a validation message. */
type Errors = Partial<Record<keyof Form, string>>;

/* ============================================================
   Derivation — the live plan summary
   ============================================================ */

const usd0 = (n: number) => `$${Math.round(n).toLocaleString('en-US')}`;

function budgetOf(v: Form): number {
  const n = Number(v.budget);
  return Number.isFinite(n) ? n : 0;
}

/** Rough monthly cost band before a real architecture exists. */
function estimate(v: Form): [number, number] {
  const envWeight = { dev: 0.4, staging: 0.65, prod: 1 }[v.env];
  let c = 180 + v.rps * 1.1;
  c *= v.slo === '99.99' ? 2.1 : v.slo === '99.9' ? 1.35 : 1;
  if (v.dr) c *= 1.3;
  c *= v.data === 'fin' ? 1.12 : v.data === 'pii' ? 1.08 : 1;
  c *= envWeight;
  return [c * 0.8, c * 1.25];
}

/** The list shown under "Archon will plan for". Warn items get an amber icon. */
function derive(v: Form): Array<[string, boolean?]> {
  const out: Array<[string, boolean?]> = [];

  if (v.slo === '99.99')
    out.push(['Active redundancy across zones with health-based failover']);
  else if (v.slo === '99.9')
    out.push(['Multi-AZ deployment for the 99.9% target']);
  else out.push(['Single-AZ with automatic restarts for the 99.5% target']);

  if (v.env === 'prod')
    out.push(['Deletion protection and encrypted backups on stateful resources']);

  if (v.dr) out.push([`Disaster recovery copy in ${RN[v.dr]} (${v.dr})`]);

  if (v.data === 'pii' || v.data === 'fin')
    out.push(['Encryption at rest with customer-managed KMS keys']);

  if (v.comp.includes('PCI DSS'))
    out.push(['Isolated cardholder data segment with flow logs']);

  if (v.users === 'Consumers (public)' || v.users === 'Developers (public API)')
    out.push(['WAF and rate limiting in front of the load balancer']);

  if (v.users === 'Internal staff')
    out.push(['Private subnets with access through SSO or VPN']);

  out.push([
    `${v.appr} ${v.appr > 1 ? 'approvals' : 'approval'} required before any apply`,
  ]);

  out.push([
    `Budget rule: ${v.enf.toLowerCase()} above ${usd0(budgetOf(v))}`,
  ]);

  if (v.env !== 'prod' && v.ttl > 0) out.push([`Auto-destroy after ${v.ttl} days`]);

  out.push([`${v.svc.length} allowed AWS service${v.svc.length === 1 ? '' : 's'}`]);

  if (!v.acct)
    out.push(['AWS account not connected. Plans can be previewed, not applied.', true]);

  return out;
}

/* ============================================================
   Helpers
   ============================================================ */

function slugify(raw: string): string {
  return raw
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '');
}

/** Validate one step. Returns the messages for that step only. */
function validateStep(n: number, v: Form, hasAttachment: boolean): Errors {
  const e: Errors = {};

  if (n === 1) {
    if (!v.name.trim()) e.name = 'Give the project a name.';
    else if (v.name.length > 40) e.name = 'Keep the name to 40 characters or fewer.';
    else if (slugify(v.name).length < 3)
      e.name = 'Use at least 3 letters or numbers.';

    if (v.req.trim().length < 40 && !hasAttachment)
      e.req =
        'Describe what it should run in a sentence or two (at least 40 characters), or attach a README.';
  }

  if (n === 2) {
    if (!v.green) {
      const re = /^https:\/\/github\.com\/[\w.-]+\/[\w.-]+?(\.git)?\/?$/;
      if (!v.repo.trim()) e.repo = 'Enter a repository URL or choose greenfield.';
      else if (!re.test(v.repo.trim()))
        e.repo = 'Use the form https://github.com/owner/repository.';
    }
  }

  if (n === 4) {
    const acct = v.acct.replace(/[\s-]/g, '');
    if (acct && !/^\d{12}$/.test(acct))
      e.acct = 'An AWS account ID has 12 digits.';

    const role = v.role.trim();
    if (role && !/^arn:aws:iam::\d{12}:role\/[\w+=,.@/-]+$/.test(role))
      e.role = 'Use a role ARN such as arn:aws:iam::123456789012:role/name.';

    if (v.dr && v.dr === v.region)
      e.dr = 'Choose a different region from the primary.';

    const b = Number(v.budget);
    if (!v.budget.trim() || !Number.isFinite(b) || b <= 0)
      e.budget = 'Enter a monthly budget greater than zero.';
  }

  return e;
}

/* ============================================================
   Shared form widgets
   ============================================================ */

interface Choice<T extends string> {
  value: T;
  id: string;
  title: string;
  sub?: string;
}

function RadioGroup<T extends string>({
  name,
  legendId,
  choices,
  value,
  onChange,
  cards = false,
}: {
  name: string;
  legendId: string;
  choices: readonly Choice<T>[];
  value: T;
  onChange: (next: T) => void;
  cards?: boolean;
}) {
  return (
    <div
      className={cards ? 'cards' : 'sg'}
      role="radiogroup"
      aria-labelledby={legendId}
    >
      {choices.map((c) => (
        <div className="cd" key={c.id}>
          <input
            type="radio"
            name={name}
            id={c.id}
            value={c.value}
            checked={value === c.value}
            onChange={() => onChange(c.value)}
          />
          <label htmlFor={c.id}>
            {c.sub ? (
              <>
                <b>{c.title}</b>
                <small>{c.sub}</small>
              </>
            ) : (
              c.title
            )}
          </label>
        </div>
      ))}
    </div>
  );
}

function CheckGroup({
  legendId,
  choices,
  values,
  onToggle,
}: {
  legendId: string;
  choices: readonly Choice<string>[];
  values: readonly string[];
  onToggle: (value: string) => void;
}) {
  return (
    <div className="sg" role="group" aria-labelledby={legendId}>
      {choices.map((c) => (
        <div className="cd" key={c.id}>
          <input
            type="checkbox"
            id={c.id}
            checked={values.includes(c.value)}
            onChange={() => onToggle(c.value)}
          />
          <label htmlFor={c.id}>{c.title}</label>
        </div>
      ))}
    </div>
  );
}

/** Renders `.err` and wires it to the control via aria-describedby. */
function Error({ id, msg }: { id: string; msg?: string }) {
  return (
    <div className="err" id={id} role={msg ? 'alert' : undefined}>
      {msg ?? ''}
    </div>
  );
}

const toggle = (list: readonly string[], value: string): string[] =>
  list.includes(value)
    ? list.filter((v) => v !== value)
    : [...list, value];

/* ============================================================
   Wizard
   ============================================================ */

export function NewProjectWizard() {
  const [form, setForm] = useState<Form>(INITIAL);
  const [errors, setErrors] = useState<Errors>({});
  const [step, setStep] = useState(1);
  const [att, setAtt] = useState<{ name: string; size: number } | null>(null);
  const [ghNote, setGhNote] = useState(false);
  const [created, setCreated] = useState(false);
  const fileRef = useRef<HTMLInputElement>(null);

  function set<K extends keyof Form>(key: K, value: Form[K]) {
    setForm((prev) => ({ ...prev, [key]: value }));
    // Clear the message the moment the field becomes valid-by-construction.
    setErrors((prev) => {
      if (!prev[key]) return prev;
      const next = { ...prev };
      delete next[key];
      return next;
    });
  }

  const slug = slugify(form.name);
  const [lo, hi] = estimate(form);
  const bud = budgetOf(form);
  const scale = Math.max(bud, hi) || 1;
  const over = bud > 0 && hi > bud;

  /** Move to a step, validating every step skipped in the forward direction. */
  function goStep(target: number) {
    const next = Math.min(STEPS.length, Math.max(1, target));
    if (next > step) {
      for (let i = step; i < next; i += 1) {
        const found = validateStep(i, form, att !== null);
        if (Object.keys(found).length > 0) {
          setErrors(found);
          setStep(i);
          return;
        }
      }
    }
    setStep(next);
  }

  function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();

    if (step !== STEPS.length) {
      goStep(step + 1);
      return;
    }

    for (let i = 1; i < STEPS.length; i += 1) {
      const found = validateStep(i, form, att !== null);
      if (Object.keys(found).length > 0) {
        setErrors(found);
        setStep(i);
        return;
      }
    }

    // Persistence is not wired yet — no projects endpoint exists. This only
    // flips local state to the confirmation state.
    setCreated(true);
  }

  /* ---------- Step 1: Requirement ---------- */

  const step1 = (
    <fieldset hidden={step !== 1}>
      <legend>Requirement</legend>

      <div className="row">
        <div className="fld">
          <label htmlFor="f-name">Project name</label>
          <input
            className="in mono"
            id="f-name"
            autoComplete="off"
            maxLength={40}
            placeholder="checkout-service"
            value={form.name}
            aria-invalid={errors.name ? true : undefined}
            aria-describedby={errors.name ? 'e-name' : undefined}
            onChange={(e) => set('name', e.target.value)}
          />
          <div className="hint">
            Slug <span className="mono">{slug || '–'}</span>
          </div>
          <Error id="e-name" msg={errors.name} />
        </div>

        <div className="fld">
          <label htmlFor="f-owner">
            Owner team <span className="opt">optional</span>
          </label>
          <input
            className="in"
            id="f-owner"
            autoComplete="off"
            placeholder="Payments Platform"
            value={form.owner}
            onChange={(e) => set('owner', e.target.value)}
          />
        </div>
      </div>

      <div className="fld">
        <label htmlFor="f-desc">
          Short description <span className="opt">optional</span>
        </label>
        <input
          className="in"
          id="f-desc"
          autoComplete="off"
          maxLength={120}
          placeholder="Cart, checkout and payment capture for the storefront"
          value={form.desc}
          onChange={(e) => set('desc', e.target.value)}
        />
      </div>

      <div className="fld">
        <span className="lb" id="l-env">
          Environment
        </span>
        <RadioGroup<Env>
          name="env"
          legendId="l-env"
          value={form.env}
          onChange={(v) => set('env', v)}
          choices={[
            { value: 'dev', id: 'env-dev', title: 'Development' },
            { value: 'staging', id: 'env-staging', title: 'Staging' },
            { value: 'prod', id: 'env-prod', title: 'Production' },
          ]}
        />
        <div className="hint">{ENVHINT[form.env]}</div>
      </div>

      <div className="fld">
        <label htmlFor="f-req">What should it run?</label>
        <textarea
          className="in"
          id="f-req"
          placeholder="Describe the application and its limits in plain language. For example: a REST API with a PostgreSQL database for checkout. It peaks at 450 requests per second during sales, must survive the loss of one availability zone, and card data must stay out of our own network scope."
          value={form.req}
          aria-invalid={errors.req ? true : undefined}
          aria-describedby={errors.req ? 'e-req' : undefined}
          onChange={(e) => set('req', e.target.value)}
        />
        <div className="tools">
          <button
            className="btn sm"
            type="button"
            onClick={() => fileRef.current?.click()}
          >
            <Icon name="clip" />
            Attach README
          </button>
          <button
            className="btn sm"
            type="button"
            onClick={() => setGhNote(true)}
          >
            <Icon name="git" />
            Import from GitHub
          </button>
          <input
            ref={fileRef}
            type="file"
            accept=".md,.markdown,.txt"
            hidden
            onChange={(e) => {
              const file = e.target.files?.[0];
              setAtt(file ? { name: file.name, size: file.size } : null);
            }}
          />
          {att && (
            <span className={styles.chip}>
              {att.name} · {(att.size / 1024).toFixed(1)} KB
              <button
                type="button"
                aria-label="Remove attachment"
                onClick={() => {
                  setAtt(null);
                  if (fileRef.current) fileRef.current.value = '';
                }}
              >
                <Icon name="x" style={{ width: 12, height: 12 }} />
              </button>
            </span>
          )}
        </div>
        {ghNote && (
          <div className="hint">
            No GitHub App is connected to this workspace yet — paste the
            repository URL on the Source step.
          </div>
        )}
        <div className="hint">
          Archon reads the requirement and the README together. Each
          architecture decision is recorded as an ADR you can edit.
        </div>
        <Error id="e-req" msg={errors.req} />
      </div>
    </fieldset>
  );

  /* ---------- Step 2: Source ---------- */

  const step2 = (
    <fieldset hidden={step !== 2}>
      <legend>Source</legend>
      <p className="lede">
        Connect the repository that holds the application. Archon opens pull
        requests for infrastructure code and the CI workflow. It never pushes to
        your default branch.
      </p>

      <div className="fld">
        <label htmlFor="f-repo">GitHub repository</label>
        <input
          className="in mono"
          id="f-repo"
          autoComplete="off"
          placeholder="https://github.com/acme/checkout-service"
          value={form.repo}
          disabled={form.green}
          aria-invalid={errors.repo ? true : undefined}
          aria-describedby={errors.repo ? 'e-repo' : undefined}
          onChange={(e) => set('repo', e.target.value)}
        />
        <div className="cd" style={{ marginTop: 4 }}>
          <input
            type="checkbox"
            id="f-green"
            checked={form.green}
            onChange={(e) => set('green', e.target.checked)}
          />
          <label htmlFor="f-green" style={{ padding: '7px 12px' }}>
            <b>Greenfield, no repository yet</b>
            <small>Archon creates one when the first plan is approved</small>
          </label>
        </div>
        <Error id="e-repo" msg={errors.repo} />
      </div>

      <div className="row">
        <div className="fld">
          <label htmlFor="f-branch">Default branch</label>
          <input
            className="in mono"
            id="f-branch"
            autoComplete="off"
            value={form.branch}
            onChange={(e) => set('branch', e.target.value)}
          />
        </div>
        <div className="fld">
          <label htmlFor="f-root">Root directory</label>
          <input
            className="in mono"
            id="f-root"
            autoComplete="off"
            value={form.root}
            onChange={(e) => set('root', e.target.value)}
          />
          <div className="hint">
            Use a subfolder for monorepos, for example{' '}
            <span className="mono">services/checkout</span>.
          </div>
        </div>
      </div>

      <div className="fld">
        <span className="lb" id="l-ci">
          When should Archon plan and apply?
        </span>
        <RadioGroup<Ci>
          name="ci"
          legendId="l-ci"
          cards
          value={form.ci}
          onChange={(v) => set('ci', v)}
          choices={[
            {
              value: 'pr',
              id: 'ci-pr',
              title: 'Plan on pull request',
              sub: 'Apply only after approval',
            },
            {
              value: 'push',
              id: 'ci-push',
              title: 'Plan on merge to default',
              sub: 'Apply only after approval',
            },
            {
              value: 'manual',
              id: 'ci-man',
              title: 'Manual only',
              sub: 'Runs when you start it',
            },
          ]}
        />
      </div>
    </fieldset>
  );

  /* ---------- Step 3: Workload ---------- */

  const step3 = (
    <fieldset hidden={step !== 3}>
      <legend>Workload</legend>
      <p className="lede">
        These answers drive redundancy, scaling and security defaults. Archon
        explains each choice in the architecture ADRs.
      </p>

      <div className="fld">
        <span className="lb" id="l-users">
          Who will use it?
        </span>
        <RadioGroup<Users>
          name="users"
          legendId="l-users"
          cards
          value={form.users}
          onChange={(v) => set('users', v)}
          choices={[
            {
              value: 'Internal staff',
              id: 'u-int',
              title: 'Internal staff',
              sub: 'Behind SSO or VPN',
            },
            {
              value: 'B2B customers',
              id: 'u-b2b',
              title: 'B2B customers',
              sub: 'Partner or tenant traffic',
            },
            {
              value: 'Consumers (public)',
              id: 'u-pub',
              title: 'Consumers',
              sub: 'Public internet, bursty',
            },
            {
              value: 'Developers (public API)',
              id: 'u-api',
              title: 'Developers',
              sub: 'Public API with keys',
            },
          ]}
        />
      </div>

      <div className="row">
        <div className="fld">
          <label htmlFor="f-geo">Where are most users?</label>
          <select
            className="in"
            id="f-geo"
            value={form.geo}
            onChange={(e) => set('geo', e.target.value as GeoKey)}
          >
            <option value="sa">South Asia</option>
            <option value="sea">Southeast Asia and Oceania</option>
            <option value="eu">Europe</option>
            <option value="na">North America</option>
            <option value="gl">Global</option>
          </select>
        </div>
        <div className="fld">
          <label htmlFor="f-mau">Monthly active users</label>
          <select
            className="in"
            id="f-mau"
            value={form.mau}
            onChange={(e) => set('mau', e.target.value)}
          >
            <option>Under 10k</option>
            <option>10k to 100k</option>
            <option>100k to 1M</option>
            <option>Over 1M</option>
          </select>
        </div>
      </div>

      <div className="fld">
        <label htmlFor="f-rps">Peak traffic</label>
        <div className={styles.rng}>
          <input
            type="range"
            id="f-rps"
            min={10}
            max={5000}
            step={10}
            value={form.rps}
            onChange={(e) => set('rps', Number(e.target.value))}
          />
          <output htmlFor="f-rps">{form.rps} req/s</output>
        </div>
      </div>

      <div className="fld">
        <span className="lb" id="l-slo">
          Availability target
        </span>
        <RadioGroup<Slo>
          name="slo"
          legendId="l-slo"
          value={form.slo}
          onChange={(v) => set('slo', v)}
          choices={[
            { value: '99.5', id: 'slo1', title: '99.5%' },
            { value: '99.9', id: 'slo2', title: '99.9%' },
            { value: '99.99', id: 'slo3', title: '99.99%' },
          ]}
        />
        <div className="hint">{SLO[form.slo]}</div>
      </div>

      <div className="row">
        <div className="fld">
          <label htmlFor="f-data">Most sensitive data it stores</label>
          <select
            className="in"
            id="f-data"
            value={form.data}
            onChange={(e) => set('data', e.target.value as DataClass)}
          >
            <option value="pub">Public content</option>
            <option value="int">Internal business data</option>
            <option value="pii">Personal data (PII)</option>
            <option value="fin">Payment or financial records</option>
          </select>
        </div>
        <div className="fld">
          <span className="lb" id="l-comp">
            Compliance scope <span className="opt">optional</span>
          </span>
          <CheckGroup
            legendId="l-comp"
            values={form.comp}
            onToggle={(v) => set('comp', toggle(form.comp, v))}
            choices={[
              { value: 'SOC 2', id: 'c1', title: 'SOC 2' },
              { value: 'PCI DSS', id: 'c2', title: 'PCI DSS' },
              { value: 'HIPAA', id: 'c3', title: 'HIPAA' },
              { value: 'GDPR', id: 'c4', title: 'GDPR' },
              { value: 'ISO 27001', id: 'c5', title: 'ISO 27001' },
            ]}
          />
        </div>
      </div>
    </fieldset>
  );

  /* ---------- Step 4: Cloud and limits ---------- */

  const sug = GEOREG[form.geo];
  const sugMatches = form.region === sug;

  const step4 = (
    <fieldset hidden={step !== 4}>
      <legend>Cloud and limits</legend>
      <p className="lede">
        Archon assumes an IAM role in your account with short-lived credentials.
        It never stores access keys.
      </p>

      <div className="row">
        <div className="fld">
          <label htmlFor="f-acct">
            AWS account ID <span className="opt">optional, connect later</span>
          </label>
          <input
            className="in mono"
            id="f-acct"
            inputMode="numeric"
            autoComplete="off"
            placeholder="123456789012"
            value={form.acct}
            aria-invalid={errors.acct ? true : undefined}
            aria-describedby={errors.acct ? 'e-acct' : undefined}
            onChange={(e) => set('acct', e.target.value)}
          />
          <Error id="e-acct" msg={errors.acct} />
        </div>
        <div className="fld">
          <label htmlFor="f-role">
            IAM role ARN <span className="opt">optional</span>
          </label>
          <input
            className="in mono"
            id="f-role"
            autoComplete="off"
            placeholder="arn:aws:iam::123456789012:role/archon-deploy"
            value={form.role}
            aria-invalid={errors.role ? true : undefined}
            aria-describedby={errors.role ? 'e-role' : undefined}
            onChange={(e) => set('role', e.target.value)}
          />
          <Error id="e-role" msg={errors.role} />
        </div>
      </div>

      <div className="row">
        <div className="fld">
          <label htmlFor="f-region">Primary region</label>
          <select
            className="in"
            id="f-region"
            value={form.region}
            onChange={(e) => set('region', e.target.value as RegionCode)}
          >
            {REGIONS.map(([code, name]) => (
              <option key={code} value={code}>
                {name} · {code}
              </option>
            ))}
          </select>
          <div className={styles.sug}>
            {sugMatches ? (
              <span>Closest region for {GEO[form.geo]} users.</span>
            ) : (
              <>
                <span>
                  Closest region for {GEO[form.geo]} users is {RN[sug]} ({sug}).
                </span>
                <button
                  className="btn sm"
                  type="button"
                  onClick={() => set('region', sug)}
                >
                  Use it
                </button>
              </>
            )}
          </div>
        </div>
        <div className="fld">
          <label htmlFor="f-dr">
            Disaster recovery region <span className="opt">optional</span>
          </label>
          <select
            className="in"
            id="f-dr"
            value={form.dr}
            aria-invalid={errors.dr ? true : undefined}
            aria-describedby={errors.dr ? 'e-dr' : undefined}
            onChange={(e) => set('dr', e.target.value as RegionCode | '')}
          >
            <option value="">None</option>
            {REGIONS.map(([code, name]) => (
              <option key={code} value={code}>
                {name} · {code}
              </option>
            ))}
          </select>
          <Error id="e-dr" msg={errors.dr} />
        </div>
      </div>

      <div className="row">
        <div className="fld">
          <label htmlFor="f-budget">Monthly budget limit</label>
          <div className={styles.pre}>
            <span>USD</span>
            <input
              className="in mono"
              id="f-budget"
              inputMode="decimal"
              autoComplete="off"
              value={form.budget}
              aria-invalid={errors.budget ? true : undefined}
              aria-describedby={errors.budget ? 'e-budget' : undefined}
              onChange={(e) => set('budget', e.target.value)}
            />
          </div>
          <Error id="e-budget" msg={errors.budget} />
        </div>
        <div className="fld">
          <span className="lb" id="l-alert">
            Alert at
          </span>
          <CheckGroup
            legendId="l-alert"
            values={form.alerts}
            onToggle={(v) => set('alerts', toggle(form.alerts, v))}
            choices={[
              { value: '50', id: 'a1', title: '50%' },
              { value: '80', id: 'a2', title: '80%' },
              { value: '100', id: 'a3', title: '100%' },
            ]}
          />
        </div>
      </div>

      <div className="fld">
        <span className="lb" id="l-enf">
          When a plan would pass the limit
        </span>
        <RadioGroup<Enforcement>
          name="enf"
          legendId="l-enf"
          cards
          value={form.enf}
          onChange={(v) => set('enf', v)}
          choices={[
            {
              value: 'Notify only',
              id: 'en1',
              title: 'Notify only',
              sub: 'Show the overage, allow apply',
            },
            {
              value: 'Require extra approval',
              id: 'en2',
              title: 'Require extra approval',
              sub: 'A second approver signs off',
            },
            {
              value: 'Block apply',
              id: 'en3',
              title: 'Block apply',
              sub: 'Raise the limit to continue',
            },
          ]}
        />
      </div>

      <div className="row">
        <div className="fld">
          <label htmlFor="f-appr">Approvals required to apply</label>
          <select
            className="in"
            id="f-appr"
            value={form.appr}
            onChange={(e) => set('appr', Number(e.target.value))}
          >
            <option value={1}>1 approver</option>
            <option value={2}>2 approvers</option>
          </select>
        </div>
        <div className="fld" hidden={form.env === 'prod'}>
          <label htmlFor="f-ttl">Auto-destroy after</label>
          <select
            className="in"
            id="f-ttl"
            value={form.ttl}
            onChange={(e) => set('ttl', Number(e.target.value))}
          >
            <option value={0}>Never</option>
            <option value={7}>7 days</option>
            <option value={14}>14 days</option>
            <option value={30}>30 days</option>
          </select>
          <div className="hint">Applies to development and staging only.</div>
        </div>
      </div>

      <div className="fld">
        <span className="lb" id="l-svc">
          Allowed AWS services
        </span>
        <CheckGroup
          legendId="l-svc"
          values={form.svc}
          onToggle={(v) => set('svc', toggle(form.svc, v))}
          choices={SVC.map((svc, i) => ({
            value: svc,
            id: `sv${i}`,
            title: svc,
          }))}
        />
        <div className="hint">
          Plans that need a service outside this list are blocked until someone
          adds it. IAM users, public S3 buckets and SSH open to the internet are
          always denied.
        </div>
      </div>
    </fieldset>
  );

  /* ---------- Step 5: Review ---------- */

  const reviewRows: Array<{ label: string; value: ReactNode; step: number }> = [
    {
      label: 'Project',
      value: (
        <>
          {form.name || 'Untitled'}{' '}
          <span className="sub mono">{slug || '—'}</span>
        </>
      ),
      step: 1,
    },
    { label: 'Environment', value: ENV[form.env], step: 1 },
    {
      label: 'Requirement',
      value:
        form.req.trim().length > 0
          ? form.req.trim().length > 150
            ? `${form.req.trim().slice(0, 150)}…`
            : form.req.trim()
          : `From ${att ? att.name : 'attached README'}`,
      step: 1,
    },
    {
      label: 'Repository',
      value: form.green ? (
        'Greenfield. Archon creates one.'
      ) : (
        <>
          {form.repo || '—'}{' '}
          <span className="sub mono">
            {form.branch} · {form.root}
          </span>
        </>
      ),
      step: 2,
    },
    {
      label: 'Pipeline',
      value: {
        pr: 'Plan on pull request',
        push: 'Plan on merge to default branch',
        manual: 'Manual only',
      }[form.ci],
      step: 2,
    },
    {
      label: 'Target users',
      value: (
        <>
          {form.users} <span className="sub">{GEO[form.geo]}</span>
        </>
      ),
      step: 3,
    },
    {
      label: 'Scale',
      value: `${form.mau} MAU · ${form.rps} req/s peak`,
      step: 3,
    },
    { label: 'Availability', value: `${form.slo}%`, step: 3 },
    {
      label: 'Data and compliance',
      value: (
        <>
          {DATA[form.data]}{' '}
          {form.comp.length > 0 && (
            <span className="sub">{form.comp.join(', ')}</span>
          )}
        </>
      ),
      step: 3,
    },
    {
      label: 'Regions',
      value: (
        <>
          {form.region}{' '}
          {form.dr && <span className="sub">DR {form.dr}</span>}
        </>
      ),
      step: 4,
    },
    {
      label: 'AWS account',
      value: form.acct || 'Not connected yet',
      step: 4,
    },
    {
      label: 'Budget',
      value: (
        <>
          {usd0(bud)} per month <span className="sub">{form.enf}</span>
        </>
      ),
      step: 4,
    },
    {
      label: 'Approvals',
      value: `${form.appr} ${form.appr > 1 ? 'approvers' : 'approver'}`,
      step: 4,
    },
  ];

  const step5 = (
    <fieldset hidden={step !== 5}>
      <legend>Review</legend>

      <dl className={styles.rv}>
        {reviewRows.map((row) => (
          <div key={row.label}>
            <dt>{row.label}</dt>
            <dd>{row.value}</dd>
            <button type="button" onClick={() => goStep(row.step)}>
              Edit
            </button>
          </div>
        ))}
      </dl>

      <div className="fld">
        <span className="lb">What happens next</span>
        <ol className={styles.next}>
          <li>
            The Requirement agent turns your text and README into explicit
            constraints.
          </li>
          <li>
            The Architecture agent drafts a topology with an ADR for each
            decision.
          </li>
          <li>
            The Plan agent writes OpenTofu and shows the exact change set and
            cost.
          </li>
          <li>
            Nothing is applied until the approvals above are given on that
            plan.
          </li>
        </ol>
      </div>
    </fieldset>
  );

  /* ---------- Confirmation ---------- */

  if (created) {
    return (
      <div className="pn">
        <div className="pn-h">
          <h2>Project created</h2>
          <span className="bd ok">
            <i />
            Draft
          </span>
        </div>
        <div className="pn-b">
          <p className="lede" style={{ marginBottom: 14 }}>
            The Requirement agent is analysing <b className="mono">{slug}</b>.
            No AWS resources exist yet, and nothing will until a human approves
            a plan on this project.
          </p>
          <div className="kv">
            <div>
              <dt>Environment</dt>
              <dd>{ENV[form.env]}</dd>
            </div>
            <div>
              <dt>Region</dt>
              <dd className="mono">
                {form.region}
                {form.dr ? ` · DR ${form.dr}` : ''}
              </dd>
            </div>
            <div>
              <dt>Availability</dt>
              <dd className="mono">{form.slo}%</dd>
            </div>
            <div>
              <dt>Est. monthly</dt>
              <dd className="mono">
                {usd0(lo)} – {usd0(hi)}
              </dd>
            </div>
            <div>
              <dt>Owner</dt>
              <dd>{form.owner || 'Unassigned'}</dd>
            </div>
            <div>
              <dt>Approvals</dt>
              <dd>
                {form.appr} {form.appr > 1 ? 'approvers' : 'approver'}
              </dd>
            </div>
          </div>
          <div className="alert alertError" style={{ marginTop: 14 }}>
            Persistence is not wired yet. Nothing was sent to the backend and no
            project record exists — this page is the only place this answer is
            held.
          </div>
        </div>
      </div>
    );
  }

  /* ---------- Wizard ---------- */

  return (
    <form onSubmit={handleSubmit} noValidate>
      <div className={styles.wiz}>
        <nav className={styles.steps} aria-label="Steps">
          {STEPS.map((label, i) => {
            const n = i + 1;
            return (
              <button
                className={`${styles.stp} ${n < step ? styles.done : ''}`}
                key={label}
                type="button"
                aria-current={n === step ? 'step' : undefined}
                onClick={() => goStep(n)}
              >
                <span className={styles.num}>{n < step ? '✓' : n}</span>
                {label}
              </button>
            );
          })}
        </nav>

        <div className="pn">
          <div className={styles.pad}>
            {step1}
            {step2}
            {step3}
            {step4}
            {step5}

            <div className={styles['nav-b']}>
              <button
                className="btn"
                type="button"
                disabled={step === 1}
                onClick={() => goStep(step - 1)}
              >
                Back
              </button>
              <div className="acts">
                {step < STEPS.length && (
                  <button className="btn p" type="submit">
                    Continue
                    <Icon name="arrow" />
                  </button>
                )}
                {step === STEPS.length && (
                  <button className="btn p" type="submit">
                    Create project
                  </button>
                )}
              </div>
            </div>
          </div>
        </div>

        <aside className={`pn ${styles.sum}`} aria-label="Plan summary">
          <div className="pn-h">
            <h2>What Archon will plan for</h2>
          </div>
          <div className="pn-b">
            <div className="cap">Project</div>
            <div
              className="mono"
              style={{ margin: '2px 0 14px', overflowWrap: 'anywhere' }}
            >
              {slug || 'untitled'} · {ENV[form.env]}
            </div>

            <div className="cap">Estimated monthly cost</div>
            <div className={styles.est}>
              {usd0(lo)} – {usd0(hi)}
            </div>

            <div
              className={`${styles.cost} ${over ? styles.costOver : ''}`}
              style={{ margin: '8px 0 6px' }}
              role="img"
              aria-label={`Estimated range against a budget of ${usd0(bud)} per month`}
            >
              <div className={styles.costHigh} style={{ width: `${(hi / scale) * 100}%` }} />
              <div className={styles.costLow} style={{ width: `${(lo / scale) * 100}%` }} />
              {bud > 0 && (
                <div
                  className={styles.costTick}
                  style={{ left: `${Math.min(99, (bud / scale) * 100)}%` }}
                />
              )}
            </div>

            <div className="sub" style={{ marginBottom: 16 }}>
              {bud <= 0
                ? 'Set a budget limit to compare.'
                : over
                  ? `The high end passes your ${usd0(bud)} limit. Raise the limit or reduce redundancy.`
                  : `Fits inside your ${usd0(bud)} limit. A rough range before design; the plan prices the real architecture.`}
            </div>

            <div className="cap" style={{ marginBottom: 8 }}>
              Archon will plan for
            </div>
            <ul className={styles.will}>
              {derive(form).map(([text, warn], i) => (
                <li
                  key={`${i}-${text}`}
                  className={warn ? styles.w : undefined}
                >
                  <Icon name={warn ? 'warn' : 'check'} />
                  <span>{text}</span>
                </li>
              ))}
            </ul>
          </div>
        </aside>
      </div>
    </form>
  );
}