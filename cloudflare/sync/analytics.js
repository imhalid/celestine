// The figures of the admin panel's dashboard (the GÖSTERGE tab, admin/dash.js): Cloudflare's own, read from its
// GraphQL Analytics API, and the app's, read from the database. Only the administrator asks for them (auth.js
// guards both paths with ADMIN_TOKEN).
//
// Cloudflare's figures need two secrets: CF_ANALYTICS_TOKEN, an API token that can only read analytics (Account
// Analytics: Read, and Zone Analytics: Read for halid.dev), and CF_ACCOUNT_ID. The token stays here: the panel gets
// the figures, never the token. Without them the dashboard says what to set and shows the app's figures alone.
//
// Every figure is an aggregate Cloudflare keeps anyway (by Worker, bucket, status, country, data centre); nothing
// here is stored, and nothing is per person.

/** The Workers, the buckets and the host names that are Celestine's, out of all the account has. */
export const SCRIPTS = ["celestine-sync", "celestine-updates", "celestine-site"];
export const BUCKETS = ["celestine-sync", "celestine-releases"];
export const HOSTS = ["celestine.halid.dev", "dl.celestine.halid.dev"];
/** The D1 database `celestine` (wrangler.toml, deploy.sh). */
const D1_ID = "d5ad1d9a-3bed-4344-a7b9-0576f5848d82";

/**
 * RANGES are the spans the panel offers: how far back, and the step of the series. A day is in hours; a month in
 * days. Cloudflare keeps these datasets 32 days at most, so 30 days is the longest.
 */
export const RANGES = {
  "24h": { hours: 24, step: "datetimeHour" },
  "7d": { hours: 24 * 7, step: "datetimeHour" },
  "30d": { hours: 24 * 30, step: "date" },
};

const HOUR = 3600 * 1000;

/** span is where a range starts and ends: from the first step's start up to now, with every step's start, oldest first. */
export function span(range, now = Date.now()) {
  const r = RANGES[range];
  const stepMs = r.step === "date" ? 24 * HOUR : HOUR;
  const last = Math.floor(now / stepMs) * stepMs;
  const n = r.hours / (stepMs / HOUR);
  const steps = Array.from({ length: n }, (_, i) => last - (n - 1 - i) * stepMs);
  const key = (t) => (r.step === "date" ? new Date(t).toISOString().slice(0, 10) : new Date(t).toISOString().replace(".000Z", "Z"));
  return { start: new Date(steps[0]).toISOString(), end: new Date(now).toISOString(), steps: steps.map(key), step: r.step };
}

/**
 * R2's operations by class, as Cloudflare bills them: class A writes and lists, class B reads; the rest (deletes,
 * aborts) are free. https://developers.cloudflare.com/r2/pricing/#class-a-operations
 */
const CLASS_A = new Set([
  "ListBuckets", "PutBucket", "ListObjects", "PutObject", "CopyObject", "CompleteMultipartUpload", "CreateMultipartUpload",
  "LifecycleStorageTierTransition", "ListMultipartUploads", "UploadPart", "UploadPartCopy", "ListParts",
  "PutBucketEncryption", "PutBucketCors", "PutBucketLifecycleConfiguration",
]);
const CLASS_B = new Set([
  "HeadBucket", "HeadObject", "GetObject", "UsageSummary", "GetBucketEncryption", "GetBucketLocation", "GetBucketCors",
  "GetBucketLifecycleConfiguration",
]);
export const r2Class = (action) => (CLASS_A.has(action) ? "A" : CLASS_B.has(action) ? "B" : "free");

/** query is the one request the dashboard makes of Cloudflare: every dataset by an alias of its own. */
function query(step) {
  const q = JSON.stringify;
  const t = "datetime_geq: $s, datetime_leq: $e";
  const w = `{${t}, scriptName_in: ${q(SCRIPTS)}}`;
  const h = `{${t}, clientRequestHTTPHost_in: ${q(HOSTS)}}`;
  const d = `{${t}, databaseId: ${q(D1_ID)}}`;
  const b = `{${t}, bucketName_in: ${q(BUCKETS)}}`;
  return `query($a: String!, $s: Time!, $e: Time!) { viewer { accounts(filter: {accountTag: $a}) {
  workers: workersInvocationsAdaptive(limit: 10, filter: ${w}) {
    sum { requests errors subrequests cpuTimeUs responseBodySize }
    quantiles { cpuTimeP50 cpuTimeP99 wallTimeP50 wallTimeP99 }
    dimensions { scriptName } }
  workerSeries: workersInvocationsAdaptive(limit: 5000, filter: ${w}, orderBy: [${step}_ASC]) {
    sum { requests errors } dimensions { scriptName ${step} } }
  workerStatus: workersInvocationsAdaptive(limit: 300, filter: ${w}, orderBy: [sum_requests_DESC]) {
    sum { requests } dimensions { scriptName httpResponseStatus } }
  colos: workersInvocationsAdaptive(limit: 12, filter: ${w}, orderBy: [sum_requests_DESC]) {
    sum { requests } dimensions { coloCode } }
  countries: httpRequestsAdaptiveGroups(limit: 15, filter: ${h}, orderBy: [count_DESC]) {
    count sum { edgeResponseBytes } dimensions { clientCountryName } }
  edgeStatus: httpRequestsAdaptiveGroups(limit: 40, filter: ${h}, orderBy: [count_DESC]) {
    count dimensions { clientRequestHTTPHost edgeResponseStatus } }
  hosts: httpRequestsAdaptiveGroups(limit: 10, filter: ${h}) {
    count sum { edgeResponseBytes visits } ratio { status4xx status5xx } dimensions { clientRequestHTTPHost } }
  d1: d1AnalyticsAdaptiveGroups(limit: 1, filter: ${d}) {
    sum { readQueries writeQueries rowsRead rowsWritten queryBatchResponseBytes }
    quantiles { queryBatchTimeMsP50 queryBatchTimeMsP99 } }
  d1Series: d1AnalyticsAdaptiveGroups(limit: 1000, filter: ${d}, orderBy: [${step}_ASC]) {
    sum { readQueries writeQueries rowsRead rowsWritten } dimensions { ${step} } }
  d1Size: d1StorageAdaptiveGroups(limit: 1, filter: ${d}, orderBy: [datetime_DESC]) {
    max { databaseSizeBytes } dimensions { datetime } }
  r2Ops: r2OperationsAdaptiveGroups(limit: 300, filter: ${b}) {
    sum { requests responseObjectSize } dimensions { bucketName actionType } }
  r2Size: r2StorageAdaptiveGroups(limit: 100, filter: ${b}, orderBy: [datetime_DESC]) {
    max { objectCount payloadSize metadataSize uploadCount } dimensions { bucketName datetime } }
} } }`;
}

/** shape turns what Cloudflare sent into what the panel draws: totals, series aligned to the range's steps, and lists. */
export function shape(acc, sp) {
  const at = new Map(sp.steps.map((k, i) => [k, i]));
  const zeros = () => sp.steps.map(() => 0);
  const workers = Object.fromEntries(
    SCRIPTS.map((name) => [name, { requests: 0, errors: 0, subrequests: 0, cpuTimeUs: 0, bytes: 0, cpuP50: 0, cpuP99: 0, wallP50: 0, wallP99: 0, series: zeros(), errorSeries: zeros(), status: [] }]),
  );
  for (const g of acc.workers ?? []) {
    const w = workers[g.dimensions.scriptName];
    if (!w) continue;
    Object.assign(w, {
      requests: g.sum.requests, errors: g.sum.errors, subrequests: g.sum.subrequests, cpuTimeUs: g.sum.cpuTimeUs, bytes: g.sum.responseBodySize,
      cpuP50: g.quantiles.cpuTimeP50, cpuP99: g.quantiles.cpuTimeP99, wallP50: g.quantiles.wallTimeP50, wallP99: g.quantiles.wallTimeP99,
    });
  }
  for (const g of acc.workerSeries ?? []) {
    const w = workers[g.dimensions.scriptName];
    const i = at.get(g.dimensions[sp.step]);
    if (!w || i == null) continue;
    w.series[i] += g.sum.requests;
    w.errorSeries[i] += g.sum.errors;
  }
  for (const g of acc.workerStatus ?? []) {
    const w = workers[g.dimensions.scriptName];
    if (w) w.status.push({ status: g.dimensions.httpResponseStatus, requests: g.sum.requests });
  }
  const d1 = acc.d1?.[0];
  const d1Series = { reads: zeros(), writes: zeros(), rowsRead: zeros(), rowsWritten: zeros() };
  for (const g of acc.d1Series ?? []) {
    const i = at.get(g.dimensions[sp.step]);
    if (i == null) continue;
    d1Series.reads[i] += g.sum.readQueries;
    d1Series.writes[i] += g.sum.writeQueries;
    d1Series.rowsRead[i] += g.sum.rowsRead;
    d1Series.rowsWritten[i] += g.sum.rowsWritten;
  }
  const buckets = Object.fromEntries(BUCKETS.map((name) => [name, { objects: 0, bytes: 0, uploads: 0, classA: 0, classB: 0, free: 0, egress: 0, ops: [] }]));
  for (const g of acc.r2Ops ?? []) {
    const b = buckets[g.dimensions.bucketName];
    if (!b) continue;
    const cls = r2Class(g.dimensions.actionType);
    b[cls === "A" ? "classA" : cls === "B" ? "classB" : "free"] += g.sum.requests;
    b.egress += g.sum.responseObjectSize;
    b.ops.push({ action: g.dimensions.actionType, cls, requests: g.sum.requests });
  }
  for (const b of Object.values(buckets)) b.ops.sort((x, y) => y.requests - x.requests);
  const seen = new Set();
  for (const g of acc.r2Size ?? []) {
    const name = g.dimensions.bucketName;
    if (seen.has(name) || !buckets[name]) continue; // newest first: the first row of a bucket is its size now
    seen.add(name);
    Object.assign(buckets[name], { objects: g.max.objectCount, bytes: g.max.payloadSize + g.max.metadataSize, uploads: g.max.uploadCount });
  }
  return {
    steps: sp.steps,
    step: sp.step,
    workers,
    colos: (acc.colos ?? []).map((g) => ({ colo: g.dimensions.coloCode, requests: g.sum.requests })),
    countries: (acc.countries ?? []).map((g) => ({ country: g.dimensions.clientCountryName, requests: g.count, bytes: g.sum.edgeResponseBytes })),
    edgeStatus: (acc.edgeStatus ?? []).map((g) => ({ host: g.dimensions.clientRequestHTTPHost, status: g.dimensions.edgeResponseStatus, requests: g.count })),
    hosts: (acc.hosts ?? []).map((g) => ({ host: g.dimensions.clientRequestHTTPHost, requests: g.count, bytes: g.sum.edgeResponseBytes, visits: g.sum.visits, r4xx: g.ratio.status4xx, r5xx: g.ratio.status5xx })),
    d1: {
      reads: d1?.sum.readQueries ?? 0, writes: d1?.sum.writeQueries ?? 0, rowsRead: d1?.sum.rowsRead ?? 0, rowsWritten: d1?.sum.rowsWritten ?? 0,
      responseBytes: d1?.sum.queryBatchResponseBytes ?? 0, timeP50: d1?.quantiles.queryBatchTimeMsP50 ?? 0, timeP99: d1?.quantiles.queryBatchTimeMsP99 ?? 0,
      size: acc.d1Size?.[0]?.max.databaseSizeBytes ?? 0, series: d1Series,
    },
    buckets,
  };
}

// A minute's memory of each range, per isolate: the panel reads again when a tab is opened, and Cloudflare's
// GraphQL API allows some 300 queries in five minutes.
const memo = new Map();

/**
 * cloudflareFigures reads Cloudflare's figures for a range (a key of RANGES). Without the secrets it says so
 * ({configured: false}); a dataset Cloudflare refused leaves its figures at zero and its error in `errors`.
 */
export async function cloudflareFigures(env, range, fresh = false) {
  if (!env.CF_ANALYTICS_TOKEN || !env.CF_ACCOUNT_ID) return { configured: false };
  const hit = memo.get(range);
  if (!fresh && hit && Date.now() - hit.at < 60_000) return hit.body;
  const sp = span(range);
  const r = await fetch("https://api.cloudflare.com/client/v4/graphql", {
    method: "POST",
    headers: { authorization: "Bearer " + env.CF_ANALYTICS_TOKEN, "content-type": "application/json" },
    body: JSON.stringify({ query: query(sp.step), variables: { a: env.CF_ACCOUNT_ID, s: sp.start, e: sp.end } }),
  });
  const j = await r.json().catch(() => null);
  if (!r.ok || !j) throw new Error(`cloudflare ${r.status}`);
  const acc = j.data?.viewer?.accounts?.[0];
  const errors = (j.errors ?? []).map((e) => (e.path ? e.path.at(-1) + ": " : "") + e.message);
  if (!acc) throw new Error(errors[0] || "cloudflare: no account");
  const body = { configured: true, range, at: Math.floor(Date.now() / 1000), errors, ...shape(acc, sp) };
  memo.set(range, { at: Date.now(), body });
  return body;
}

/** appFigures are the app's own, from its database: accounts, the waitlist, devices and bytes, and what is new in the range. */
export async function appFigures(env, range) {
  const since = Math.floor(Date.now() / 1000) - RANGES[range].hours * 3600;
  const t = Math.floor(Date.now() / 1000);
  const [users, waitlist, devices, invites] = await env.DB.batch([
    env.DB.prepare(
      `SELECT COUNT(*) AS n, SUM(status='active') AS active, SUM(created_at>=?1) AS new, COALESCE(SUM(bytes),0) AS bytes,
              COALESCE(MAX(bytes),0) AS maxBytes, COALESCE(SUM(quota),0) AS quota, COALESCE(SUM(changes),0) AS changes FROM users`,
    ).bind(since),
    env.DB.prepare("SELECT status, COUNT(*) AS n, SUM(created_at>=?1) AS new FROM waitlist GROUP BY status").bind(since),
    env.DB.prepare("SELECT COUNT(*) AS n, SUM(last_seen>=?1) AS seen, COUNT(DISTINCT CASE WHEN last_seen>=?1 THEN user_id END) AS users FROM devices").bind(since),
    env.DB.prepare("SELECT SUM(used_at IS NULL AND expires_at>?2) AS open, SUM(used_at>=?1) AS used FROM invites").bind(since, t),
  ]);
  const u = users.results[0];
  const wl = Object.fromEntries(waitlist.results.map((r) => [r.status, { n: r.n, new: r.new ?? 0 }]));
  const d = devices.results[0];
  const i = invites.results[0];
  return {
    range,
    users: { total: u.n, active: u.active ?? 0, new: u.new ?? 0, bytes: u.bytes, maxBytes: u.maxBytes, avgBytes: u.n ? Math.round(u.bytes / u.n) : 0, quota: u.quota, changes: u.changes },
    waitlist: { pending: wl.pending ?? { n: 0, new: 0 }, approved: wl.approved ?? { n: 0, new: 0 }, rejected: wl.rejected ?? { n: 0, new: 0 } },
    devices: { total: d.n, seen: d.seen ?? 0, activeUsers: d.users ?? 0 },
    invites: { open: i.open ?? 0, used: i.used ?? 0 },
  };
}
