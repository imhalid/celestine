// The context model's subword tokenizer's answers, for the Go port's parity test (celestine #623): for every third
// "wrong" field of tinyfix-tr's bench sets (bench/**/*.jsonl, files in sorted order) and a few texts of the cases a
// tokenizer gets wrong (apostrophes, capitals, numbers, letters of other languages, a character outside the BMP), one
// JSON line with the text and its encodeSubwords. TestParitySubwords (contextmodel_test.go) encodes the same and
// compares.
//
//	node internal/spell/testdata/parity-subwords.ts <tinyfix-tr> | gzip -9n > internal/spell/testdata/parity-subwords.jsonl.gz
import { readdirSync, readFileSync, statSync } from "node:fs";
import { join } from "node:path";

const [root] = process.argv.slice(2);
if (!root) throw new Error("usage: node parity-subwords.ts <tinyfix-tr checkout>");
const { encodeSubwords } = await import(join(root, "src/context.ts"));

function files(dir: string): string[] {
  const out: string[] = [];
  for (const name of readdirSync(dir).sort()) {
    const path = join(dir, name);
    if (statSync(path).isDirectory()) out.push(...files(path));
    else if (name.endsWith(".jsonl")) out.push(path);
  }
  return out;
}

const out = (text: string) => process.stdout.write(JSON.stringify({ t: text, s: encodeSubwords(text) }) + "\n");
for (const text of [
  "Ankara'ya giderken GOES-15 verilerini inceledik.",
  "İSTANBUL'DA ıslak İğne; ÇOK güzel.",
  "Bir café'de 3,14 ve 2026'da, ’Ali’nin’ kitabı…",
  "Jürgen Müller und Ångström: naïve résumé.",
  "Emoji 😀 ve 𝔸 harfi, tab\there, nbsp\u00a0burada.",
  "x'İ 'a ' ' 12abc abc12 A1B2",
]) out(text);
let n = 0;
for (const file of files(join(root, "bench"))) {
  for (const row of readFileSync(file, "utf8").split("\n")) {
    if (!row.trim()) continue;
    const text = JSON.parse(row).wrong;
    if (typeof text === "string" && n++ % 3 === 0) out(text);
  }
}
