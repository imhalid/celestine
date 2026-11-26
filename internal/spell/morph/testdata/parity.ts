// The TypeScript morphology's answers for the Go port's parity test (celestine #623): one JSON line per word of
// the word list, with isWord, analyze, priorCount, fuzzySearch(w, 1, 15) and fuzzySearch(w, 2, 40), called in
// that order on one morphology, as TestParity calls them.
//
// Run it with Node 22 or newer against a tinyfix-tr checkout and its data (the pinned Hugging Face revision):
//
//	node parity.ts <tinyfix-tr checkout> <data dir> < words.txt | gzip -9 > parity.jsonl.gz
//
// The words are the golden's own: zcat parity.jsonl.gz | node -e 'for await (const l of require("readline")
// .createInterface({input: process.stdin})) console.log(JSON.parse(l).w)' gives them back.
import { readFileSync } from "node:fs";
import { createInterface } from "node:readline";
import { pathToFileURL } from "node:url";

const [checkout, dir] = process.argv.slice(2);
if (!checkout || !dir) throw new Error("usage: node parity.ts <tinyfix-tr checkout> <data dir> < words.txt");
const { TurkishMorphology } = await import(pathToFileURL(checkout + "/src/morphology/index.ts").href);
const morph = TurkishMorphology.fromBinary(new Uint8Array(readFileSync(dir + "/morph.bin")));
for await (const w of createInterface({ input: process.stdin })) {
  const line = {
    w,
    isWord: morph.isWord(w),
    analyze: morph.analyze(w),
    prior: morph.priorCount(w),
    f1: morph.fuzzySearch(w, 1, 15),
    f2: morph.fuzzySearch(w, 2, 40),
  };
  console.log(JSON.stringify(line));
}
