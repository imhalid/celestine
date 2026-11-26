// The TypeScript engine's answers for the Go port's parity test of the corrector, the sentence rules and the
// package engine (celestine #623): one JSON line per input, every "wrong" field of tinyfix-tr's bench sets
// (bench/**/*.jsonl, files in sorted order), all in one process and in order, as the morphology's caches make
// answers depend on the order of calls. Each line holds:
//   - "c": Corrector.inspect with CAREFUL, the known misspellings and the morphology (text, corrections, flags);
//   - "i": the package engine's inspect with its default options (fixes, flags);
//   - "k": the engine's check with its default options;
//   - "r" and "kr": a range over the middle word of the line, and the engine's check with that range;
//   - on every 7th line, "kb": check in the balanced mode with every kind, contractions included; and "a" and
//     "ka": the first word with Turkish letters taken as auto-marked, and check with that.
// Offsets are in code points, as the Go port counts them. TestParityCorrector (corrector_parity_test.go) runs the
// same calls in the same order and compares every answer.
//
// Run it with Node 22 or later against a checkout of tinyfix-tr and the engine's data directory (lexicon.bin,
// morph.bin, known.tsv), from the root of this repository:
//
//	node internal/spell/testdata/parity-corrector.ts <tinyfix-tr> <data> | gzip -9n > internal/spell/testdata/parity-corrector.jsonl.gz
import { readdirSync, readFileSync, statSync } from "node:fs";
import { join } from "node:path";

const [root, dataDir] = process.argv.slice(2);
if (!root || !dataDir) throw new Error("usage: node parity-corrector.ts <tinyfix-tr checkout> <data dir>");
const { Lexicon } = await import(join(root, "src/lexicon.ts"));
const { KnownMisspellings } = await import(join(root, "src/known.ts"));
const { TurkishMorphology } = await import(join(root, "src/morphology/index.ts"));
const { Corrector, CAREFUL } = await import(join(root, "src/corrector.ts"));
const { TdkaiEngine } = await import(join(root, "pkg/src/engine.ts"));

const lexicon = Lexicon.fromBinary(new Uint8Array(readFileSync(join(dataDir, "lexicon.bin"))));
const known = KnownMisspellings.fromTsv(readFileSync(join(dataDir, "known.tsv"), "utf8"));
const morphology = TurkishMorphology.fromBinary(new Uint8Array(readFileSync(join(dataDir, "morph.bin"))));
const corrector = new Corrector(lexicon, CAREFUL, known, morphology);
const engine = new TdkaiEngine({ lexicon, known, morphology }, "careful");

function files(dir: string): string[] {
  const out: string[] = [];
  for (const name of readdirSync(dir).sort()) {
    const path = join(dir, name);
    if (statSync(path).isDirectory()) out.push(...files(path));
    else if (name.endsWith(".jsonl")) out.push(path);
  }
  return out;
}

const write = (line: string) => process.stdout.write(line + "\n");
const ALL = ["turkce-karakter", "kesme", "bitisik-ayri", "yazim", "noktalama", "kisaltma"];
let n = 0;
for (const file of files(join(root, "bench"))) {
  for (const row of readFileSync(file, "utf8").split("\n")) {
    if (!row.trim()) continue;
    const text = JSON.parse(row).wrong;
    if (typeof text !== "string") continue;
    // cp turns a UTF-16 offset of text into a code point offset.
    const cp = (i: number) => [...text.slice(0, i)].length;
    const c = corrector.inspect(text);
    const i = await engine.inspect(text);
    const k = await engine.check(text);
    const words = [...text.matchAll(/\p{L}+/gu)];
    let r: [number, number] | null = null;
    let kr: unknown = null;
    if (words.length > 0) {
      const w = words[Math.floor(words.length / 2)]!;
      r = [w.index, w.index + w[0].length];
      kr = (await engine.check(text, { range: r })).map(fix);
      r = [cp(r[0]), cp(r[1])];
    }
    let kb: unknown = undefined;
    let a: [number, number] | null | undefined = undefined;
    let ka: unknown = undefined;
    if (n++ % 7 === 0) {
      kb = (await engine.check(text, { mode: "balanced", kinds: ALL })).map(fix);
      const marked = words.find((w) => /[çğıöşüÇĞİÖŞÜ]/.test(w[0]));
      a = null;
      ka = null;
      if (marked) {
        const am: [number, number] = [marked.index!, marked.index! + marked[0].length];
        ka = (await engine.check(text, { autoMarked: [am] })).map(fix);
        a = [cp(am[0]), cp(am[1])];
      }
    }
    write(JSON.stringify({
      in: text,
      c: {
        out: c.text,
        corrections: c.corrections.map((x: any) => ({ start: cp(x.start), end: cp(x.end), original: x.original, replacement: x.replacement, kind: x.kind, margin: x.margin })),
        flags: c.flags.map(flag),
      },
      i: { fixes: i.fixes.map(fix), flags: i.flags.map(flag) },
      k: k.map(fix),
      r,
      kr,
      kb,
      a,
      ka,
    }));
    function fix(x: any) {
      return { from: cp(x.from), to: cp(x.to), original: x.original, replacement: x.replacement, kind: x.kind, scope: x.scope, confidence: x.confidence };
    }
    function flag(x: any) {
      const start = x.start ?? x.from, end = x.end ?? x.to;
      return { from: cp(start), to: cp(end), original: x.original, suggestions: x.suggestions, reason: x.reason };
    }
  }
}
