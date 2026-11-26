// The TypeScript engine's answers with its context model on, for the Go port's parity tests of the context model
// (celestine #623): the package engine of tinyfix-tr with context.onnx run by onnxruntime, as the npm package's
// `context: true`, over every "wrong" field of its bench sets (bench/**/*.jsonl, files in sorted order), all in one
// process and in order, as the morphology's caches make answers depend on the order of calls.
//
// Standard output gets one JSON line per input: "i", the engine's inspect with its default options (fixes, flags),
// and "k", its check. The file named third gets one JSON line for every 40th run of the model, up to 400: what it
// was given (the packed subwords and segments, their length, the features) and the scores onnxruntime gave, for
// TestContextModelOutputs to run the Go runner on the same and compare. Offsets are in code points, as the Go port
// counts them.
//
// Run it with Node 22 or later against a checkout of tinyfix-tr whose devDependencies are installed (onnxruntime-node;
// `npm install --onnxruntime-node-install-cuda=skip`) and the engine's data directory (lexicon.bin, morph.bin,
// known.tsv, context.onnx), from the root of this repository:
//
//	node internal/spell/testdata/parity-context.ts <tinyfix-tr> <data> batches.jsonl | gzip -9n > internal/spell/testdata/parity-context.jsonl.gz
//	gzip -9n < batches.jsonl > internal/spell/testdata/context-batches.jsonl.gz
import { readdirSync, readFileSync, statSync, writeFileSync } from "node:fs";
import { createRequire } from "node:module";
import { join } from "node:path";

const [root, dataDir, batchFile] = process.argv.slice(2);
if (!root || !dataDir || !batchFile) throw new Error("usage: node parity-context.ts <tinyfix-tr checkout> <data dir> <batches file>");
const ort = createRequire(join(root, "package.json"))("onnxruntime-node");
const { Lexicon } = await import(join(root, "src/lexicon.ts"));
const { KnownMisspellings } = await import(join(root, "src/known.ts"));
const { TurkishMorphology } = await import(join(root, "src/morphology/index.ts"));
const { ContextModel, FEATURE_DIM } = await import(join(root, "src/context.ts"));
const { TdkaiEngine } = await import(join(root, "pkg/src/engine.ts"));

const lexicon = Lexicon.fromBinary(new Uint8Array(readFileSync(join(dataDir, "lexicon.bin"))));
const known = KnownMisspellings.fromTsv(readFileSync(join(dataDir, "known.tsv"), "utf8"));
const morphology = TurkishMorphology.fromBinary(new Uint8Array(readFileSync(join(dataDir, "morph.bin"))));
const session = await ort.InferenceSession.create(readFileSync(join(dataDir, "context.onnx")));

// run feeds the model as pkg/src/context.ts does for a v3 model (subwords, sub_segments, features), and keeps every
// 40th run's inputs and scores.
const batches: string[] = [];
let runs = 0;
const run = async (_t: BigInt64Array, _s: BigInt64Array, n: number, features: Float32Array | undefined, sub: any) => {
  const feats = features ?? new Float32Array(n * FEATURE_DIM);
  const result = await session.run({
    subwords: new ort.Tensor("int64", sub.subwords, [n, sub.seqLen]),
    sub_segments: new ort.Tensor("int64", sub.subSegments, [n, sub.seqLen]),
    features: new ort.Tensor("float32", feats, [n, FEATURE_DIM]),
  });
  const scores = result["scores"].data as Float32Array;
  if (runs++ % 40 === 0 && batches.length < 400) {
    batches.push(JSON.stringify({
      n,
      seqLen: sub.seqLen,
      subwords: Array.from(sub.subwords, Number),
      segments: Array.from(sub.subSegments, Number),
      features: Array.from(feats),
      scores: Array.from(scores),
    }));
  }
  return scores;
};
const model = new ContextModel(run, { subwordsOnly: true });
const engine = new TdkaiEngine({ lexicon, known, morphology, contextSession: { model, dispose: async () => {} } }, "careful");

function files(dir: string): string[] {
  const out: string[] = [];
  for (const name of readdirSync(dir).sort()) {
    const path = join(dir, name);
    if (statSync(path).isDirectory()) out.push(...files(path));
    else if (name.endsWith(".jsonl")) out.push(path);
  }
  return out;
}

for (const file of files(join(root, "bench"))) {
  for (const row of readFileSync(file, "utf8").split("\n")) {
    if (!row.trim()) continue;
    const text = JSON.parse(row).wrong;
    if (typeof text !== "string") continue;
    const cp = (i: number) => [...text.slice(0, i)].length; // a UTF-16 offset of text as a code point offset
    const fix = (x: any) => ({ from: cp(x.from), to: cp(x.to), original: x.original, replacement: x.replacement, kind: x.kind, scope: x.scope, confidence: x.confidence, byContext: x.byContext ?? false });
    const flag = (x: any) => ({ from: cp(x.start ?? x.from), to: cp(x.end ?? x.to), original: x.original, suggestions: x.suggestions, reason: x.reason });
    const i = await engine.inspect(text);
    const k = await engine.check(text);
    process.stdout.write(JSON.stringify({ in: text, i: { fixes: i.fixes.map(fix), flags: i.flags.map(flag) }, k: k.map(fix) }) + "\n");
  }
}
writeFileSync(batchFile, batches.join("\n") + "\n");
