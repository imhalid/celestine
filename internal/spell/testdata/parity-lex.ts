// The TypeScript engine's answers for the Go port's parity test of the lexicon, the known misspellings and the
// rules (celestine #623): one JSON line per probe, {"f": function, "a": arguments, "r": result}. The probes are
// words, word pairs, and stems and suffixes cut from real words, taken from tinyfix-tr's bench inputs, with
// typo-like variants of them. TestParityLexicon (parity_test.go) gives the same arguments to the Go functions.
//
// Run it with Node 22 or later against a checkout of tinyfix-tr and the engine's data directory (lexicon.bin,
// known.tsv), from the root of this repository:
//
//	node internal/spell/testdata/parity-lex.ts <tinyfix-tr> <data> | gzip -9n > internal/spell/testdata/parity-lex.jsonl.gz
import { createHash } from "node:crypto";
import { readdirSync, readFileSync, statSync } from "node:fs";
import { join } from "node:path";

const [root, dataDir] = process.argv.slice(2);
if (!root || !dataDir) throw new Error("usage: node parity-lex.ts <tinyfix-tr checkout> <data dir>");
const { Lexicon } = await import(join(root, "src/lexicon.ts"));
const { KnownMisspellings } = await import(join(root, "src/known.ts"));
const R = await import(join(root, "src/rules/turkish-rules.ts"));
const P = await import(join(root, "src/rules/proper-names.ts"));
const T = await import(join(root, "src/turkish.ts"));

const lexicon = Lexicon.fromBinary(new Uint8Array(readFileSync(join(dataDir, "lexicon.bin"))));
const knownText = readFileSync(join(dataDir, "known.tsv"), "utf8");
const known = KnownMisspellings.fromTsv(knownText);

const out: string[] = [];
function probe(f: string, a: unknown[], r: unknown): void {
  out.push(JSON.stringify({ f, a, r: r === undefined ? null : r }));
}

// The bench sentences, in a fixed order.
function files(dir: string): string[] {
  const found: string[] = [];
  for (const name of readdirSync(dir).sort()) {
    const path = join(dir, name);
    if (statSync(path).isDirectory()) found.push(...files(path));
    else if (name.endsWith(".jsonl")) found.push(path);
  }
  return found;
}
const sentences: string[] = [];
for (const file of files(join(root, "bench"))) {
  for (const line of readFileSync(file, "utf8").split("\n")) {
    if (!line.trim()) continue;
    const row = JSON.parse(line);
    for (const key of ["wrong", "right", "text"]) if (typeof row[key] === "string") sentences.push(row[key]);
  }
}

// A deterministic choice: a token is taken when its hash falls in the share asked for.
function hash(text: string): number {
  return parseInt(createHash("sha1").update(text).digest("hex").slice(0, 8), 16);
}
function sample<T>(items: T[], key: (item: T) => string, count: number): T[] {
  if (items.length <= count) return items;
  return items
    .map((item) => [hash(key(item)), item] as const)
    .sort((x, y) => x[0] - y[0])
    .slice(0, count)
    .map(([, item]) => item);
}
function unique<T>(items: T[], key: (item: T) => string = String as any): T[] {
  const seen = new Set<string>();
  return items.filter((item) => {
    const k = key(item);
    if (seen.has(k)) return false;
    seen.add(k);
    return true;
  });
}

const TOKEN = /[A-Za-zÇĞİÖŞÜçğıöşüÂÎÛâîû0-9'’]+/g;
const rawTokens: string[] = [];
const rawPairs: [string, string][] = [];
for (const s of sentences) {
  const toks = s.match(TOKEN) ?? [];
  rawTokens.push(...toks);
  for (let i = 0; i + 1 < toks.length; i++) rawPairs.push([toks[i]!, toks[i + 1]!]);
}
const cased = sample(unique(rawTokens), (t) => t, 3000);
const lowers = unique(rawTokens.map((t) => T.lowerTr(t)));
const words = sample(lowers, (t) => t, 4000);

// Typo-like variants: a letter dropped, two swapped, a letter doubled, the marks taken off or put on.
function variants(word: string): string[] {
  const vs: string[] = [];
  const h = hash(word);
  const n = word.length;
  if (n > 2) {
    const i = h % n;
    vs.push(word.slice(0, i) + word.slice(i + 1));
    const j = h % (n - 1);
    vs.push(word.slice(0, j) + word[j + 1] + word[j] + word.slice(j + 2));
    vs.push(word.slice(0, i) + word[i] + word.slice(i));
  }
  vs.push(P.foldTurkish(word));
  const marks = T.markVariants(word, 4);
  if (marks.length) vs.push(marks[h % marks.length]!);
  return vs;
}
const typos = unique(sample(words, (t) => "v" + t, 1500).flatMap(variants)).filter((w) => !words.includes(w));

// The words of the closed lists with suffixes, so the rules that split them meet real cases.
const SUFFIXES = ["", "a", "e", "ı", "i", "u", "ü", "ya", "ye", "yı", "yi", "nın", "nin", "nun", "nün", "in", "ın", "un", "ün",
  "da", "de", "ta", "te", "dan", "den", "tan", "ten", "daki", "deki", "na", "ne", "nı", "ni", "nda", "nde", "ndan", "nden",
  "yla", "yle", "la", "le", "lar", "ler", "ları", "leri", "m", "mı", "mi", "mız", "miz", "sı", "si", "sına", "sinde", "dır", "dir",
  "tır", "tir", "ydı", "ydi", "ymış", "ymiş", "ki", "nki", "nin", "lık", "lik", "ca", "ce", "cı", "ci", "sız", "siz", "sal", "sel"];
const listWords: string[] = unique([
  ...R.PLACE_APOSTROPHE_HEADS, ...R.HONORIFIC_TITLES, ...P.MONTHS, ...P.WEEKDAYS, ...P.INSTITUTION_HEADS,
  ...P.OFFICER_TITLE_HEADS, ...P.GEO_HEADS, ...P.STATE_HEADS, ...P.EVENT_HEADS, ...P.SAYIN_TITLES,
  ...P.SAYIN_COMPOUND_TAIL, ...P.POST_HONORIFICS, ...P.POST_TITLES_WITH_APOS, ...P.KINSHIP_STEMS,
  ...P.MEDIA_TYPE_STEMS, ...P.LANGUAGE_NAMES, ...P.OPTIONAL_APOS_ISM_IYET, ...P.PLURAL_PLACE_KEEP_APOS,
  ...P.DERIV_LOOKING_KEEP_APOS,
] as string[]);
const listForms = unique(
  sample(listWords.flatMap((w) => SUFFIXES.map((s) => [w, s] as const)), ([w, s]) => w + "|" + s, 2000).flatMap(([w, s]) =>
    s ? [w + s, w + "'" + s, P.foldTurkish(w + s), w.slice(0, -1) + s] : [w],
  ),
);

// The known misspellings with suffixes, for the rules that reach into suffixed forms.
const knownWrongs: string[] = [];
for (const line of knownText.split("\n")) {
  const [wrong, , , , section] = line.split("\t");
  if (!wrong || line.startsWith("#")) continue;
  const sec = section?.trim();
  if (sec && !["MISSPELLINGS", "MINED", "CONTRACTION", "JOINED_ALWAYS", "SEPARATE_ALWAYS"].includes(sec)) continue;
  knownWrongs.push(wrong);
}
const knownForms = unique(
  knownWrongs.flatMap((w) => {
    const first = w.split(" ")[0]!;
    return [w, first, ...["in", "de", "ler", "ı", "mı", "sın", "ydı", "miş", "ce", "dan"].map((s) => (w.includes(" ") ? w : w + s))];
  }),
);

const allWords = unique([...words, ...typos, ...listForms]);

// Word pairs: adjacent words of the sentences, and the known two-word rules typed plain or suffixed.
const pairs: [string, string][] = sample(
  unique(rawPairs.map(([x, y]) => [T.lowerTr(x), T.lowerTr(y)] as [string, string]), (p) => p.join(" ")),
  (p) => p.join(" "),
  6000,
);
const knownPairs: [string, string][] = unique(
  knownWrongs
    .filter((w) => w.split(" ").length === 2)
    .flatMap((w) => {
      const [x, y] = w.split(" ") as [string, string];
      return [[x, y], [x, y + "lar"], [x, y + "sın"], [x, y + "dı"], [P.foldTurkish(x), P.foldTurkish(y)]] as [string, string][];
    }),
  (p) => p.join(" "),
);

// Stems and suffixes cut from real words: at an apostrophe, and at each of the last six letters.
const cuts: [string, string][] = [];
for (const w of [...words, ...listForms]) {
  const q = w.indexOf("'");
  if (q > 0) cuts.push([w.slice(0, q), w.slice(q + 1)]);
  for (let k = 1; k <= 6 && k < w.length - 1; k++) cuts.push([w.slice(0, -k), w.slice(-k)]);
}
const stemSuffix = sample(unique(cuts, (c) => c.join("|")), (c) => c.join("|"), 10000);

// The whole lexicon, by digest: every word, count and pair the binary holds, as TSV.
const sha = (s: string) => createHash("sha256").update(s).digest("hex");
const tsv = lexicon.toTsv();
probe("lexicon.size", [], lexicon.size);
probe("lexicon.pairCount", [], lexicon.pairCount);
probe("lexicon.toTsv", [], [sha(tsv.unigrams), sha(tsv.bigrams), tsv.unigrams.length, tsv.bigrams.length]);

// A lexicon read from TSV: the binary's own TSV, read back.
const tsvLexicon = Lexicon.fromTsv(tsv.unigrams, tsv.bigrams, 3, 2);
probe("tsvLexicon.size", [], tsvLexicon.size);
probe("tsvLexicon.pairCount", [], tsvLexicon.pairCount);
const tsv2 = tsvLexicon.toTsv();
probe("tsvLexicon.toTsv", [], [sha(tsv2.unigrams), sha(tsv2.bigrams)]);

// The known misspellings' totals.
probe("known.size", [], known.size);
probe("known.hasProperData", [], known.hasProperData);

// The closed lists, whole: a list's order is checked where it can reach a result.
for (const [mod, M] of [["R", R], ["P", P]] as const) {
  for (const [name, value] of Object.entries(M)) {
    if (value instanceof Set) probe("set", [mod + "." + name], [...value]);
  }
}
probe("COMPOUND_VERB_JOINS", [], [...R.COMPOUND_VERB_JOINS]);
probe("CANONICAL_PHRASES", [], P.CANONICAL_PHRASES);

// Probes on one input, grouped in one line so the input is written once: {"k": kind, "a": arguments, "r": {name:
// result}}.
function group(k: string, a: unknown[], r: Record<string, unknown>): void {
  for (const key of Object.keys(r)) if (r[key] === undefined) r[key] = null;
  out.push(JSON.stringify({ k, a, r }));
}

const REGEXES: Record<string, RegExp> = {
  QUESTION: R.QUESTION, ALSO: R.ALSO, NO_LOCATIVE_AFTER: R.NO_LOCATIVE_AFTER, STRONG_NO_LOCATIVE_AFTER: R.STRONG_NO_LOCATIVE_AFTER,
  FINITE: R.FINITE, NAME_SUFFIX: R.NAME_SUFFIX, PLACE_P3SG_SUFFIX: R.PLACE_P3SG_SUFFIX,
  NON_APOSTROPHE_SUFFIX_RE: P.NON_APOSTROPHE_SUFFIX_RE, PROPER_INFLECTIONAL_SUFFIX_RE: P.PROPER_INFLECTIONAL_SUFFIX_RE,
  HEAD_SUFFIX_RE: P.HEAD_SUFFIX_RE, NO_APOS_PROPER_SUFFIX_RE: P.NO_APOS_PROPER_SUFFIX_RE,
};
const HEAD_SETS = ["INSTITUTION_HEADS", "GEO_HEADS", "STATE_HEADS", "EVENT_HEADS", "OFFICER_TITLE_HEADS", "SAYIN_TITLES", "SAYIN_COMPOUND_TAIL"];
const VOWELS = ["a", "e", "ı", "i", "o", "ö", "u", "ü", "â", "î", "û", "x"];
const suffixes = unique([...stemSuffix.map(([, s]) => s), ...SUFFIXES]);
const tsvWords = new Set(sample(allWords, (t) => "tsv" + t, 1500));

// Single words: the lexicon, the known names, the rules of one word, the regular expressions.
for (const w of unique([...allWords, ...knownForms, ...suffixes])) {
  const folded = P.foldTurkish(w);
  const vowel = VOWELS[hash(w) % VOWELS.length]!;
  const r: Record<string, unknown> = {
    count: lexicon.count(w),
    exactCount: lexicon.exactCount(w),
    capitalShare: lexicon.capitalShare(w),
    nameFor50: known.nameFor(w, 50),
    nameEntry: known.nameEntry(w),
    asciiNameEntry: known.asciiNameEntry(folded),
    acronymFor: known.acronymFor(w),
    asciiAcronymFor: known.asciiAcronymFor(folded),
    givenNameFor: known.givenNameFor(w),
    surnameFor: known.surnameFor(w),
    ambigNameFor: known.ambigNameFor(w),
    phrases: known.phrasesStartingWith(w),
    phrasesPlain: known.phrasesStartingWith(w, true),
    find: known.find(w, undefined, lexicon, false),
    lastVowel: R.lastVowel(w),
    sameHighVowels: R.sameHighVowels(w),
    isVoicelessFinal: R.isVoicelessFinal(w),
    blocked: [false, true].flatMap((skip) => [false, true].map((allow) => R.hasBlockedApostropheStem(w, skip, allow))),
    fixDisharmoniousApostropheSuffix: R.fixDisharmoniousApostropheSuffix(w),
    isPlaceApostropheHead: R.isPlaceApostropheHead(w),
    isInverseFrontStem: P.isInverseFrontStem(w),
    isKinshipWord: P.isKinshipWord(w),
    isMediaTypeWord: P.isMediaTypeWord(w),
    splitHonorific: P.splitHonorific(w),
    splitPostTitle: P.splitPostTitle(w),
    splitHeadWord: HEAD_SETS.map((set) => P.splitHeadWord(w, P[set]) ?? null),
    splitCalendarWord: [P.splitCalendarWord(w, P.MONTHS) ?? null, P.splitCalendarWord(w, P.WEEKDAYS) ?? null],
    hasVowel: P.hasVowel(w),
    foldTurkish: folded,
    followsHarmony: R.followsHarmony(w, vowel),
    re: Object.values(REGEXES).map((re) => {
      const m = re.exec(w);
      return m ? [...m].map((g) => (g === undefined ? null : g)) : null;
    }),
  };
  if (tsvWords.has(w)) {
    r.tsvCount = tsvLexicon.count(w);
    r.tsvCapitalShare = tsvLexicon.capitalShare(w);
  }
  group("w", [w, vowel], r);
}
probe("REGEXES", [], Object.keys(REGEXES));
probe("HEAD_SETS", [], HEAD_SETS);

// Words as typed, capitals kept.
for (const w of cased) {
  group("c", [w], { matchAcronymApostrophe: R.matchAcronymApostrophe(w), capTr: P.capTr(w), foldTurkish: P.foldTurkish(w) });
}
for (const w of ["HEXIMden", "KDAların", "TLSnin", "MacOSdaki", "OpenAIden", "AIde", "EVde", "URLs", "NATOya", "İTÜden", "ODTÜnün", "BBCnin"]) {
  group("c", [w], { matchAcronymApostrophe: R.matchAcronymApostrophe(w), capTr: P.capTr(w), foldTurkish: P.foldTurkish(w) });
}

// Word pairs.
const tsvPairs = new Set(sample(pairs, (p) => "tsv" + p.join(" "), 1500).map((p) => p.join(" ")));
for (const [x, y] of unique([...pairs, ...knownPairs], (p) => p.join(" "))) {
  const replacement = y.replace(/^o/, "ö");
  const r: Record<string, unknown> = {
    pair: lexicon.pair(x, y),
    find: known.find(x, y, lexicon, false),
    findPlain: known.find(x, y, lexicon, true),
    collapsedDoubledLocative: R.collapsedDoubledLocative(x, y),
    isPlaceApostropheHead: R.isPlaceApostropheHead(y, x),
    isInstitutionHead: P.isInstitutionHead(y, x),
    harmonizeQuestionParticle: R.harmonizeQuestionParticle(y, x, false),
    harmonizeQuestionParticleInverse: R.harmonizeQuestionParticle(y, x, true),
    isAuxiliaryOl: R.isAuxiliaryOl(x, y, replacement),
  };
  if (tsvPairs.has(x + " " + y)) r.tsvPair = tsvLexicon.pair(x, y);
  group("p", [x, y, replacement], r);
}
for (const x of ["bölgedeler", "alandalar", "bölgelerde", "evdeler", "kitaptalar", "yerdeler"]) {
  for (const y of ["de", "da"]) probe("collapsedDoubledLocative", [x, y], R.collapsedDoubledLocative(x, y));
}
for (const q of ["mi", "mı", "mu", "mü", "misin", "musun", "miydi", "muymus", "misiniz", "mudur", "miyim", "miyiz", "misın", "mis", "mu?"]) {
  for (const prev of ["var", "hazır", "gördü", "saat", "alkol", "oldu", "ev", "okul", "kitap", "", "x", "gül"]) {
    for (const inv of [false, true]) probe("harmonizeQuestionParticle", [q, prev, inv], R.harmonizeQuestionParticle(q, prev, inv));
  }
}
for (const prev of ["ediyor", "gelmiş", "gidecek", "yapacak", "geliyor", "ev", "okumuş", "bakıyor"]) {
  for (const w of ["oluyorduk", "olacak", "olursa", "ölüyorduk", "olmuş", "ev"]) {
    probe("isAuxiliaryOl", [prev, w, "öl" + w.slice(2)], R.isAuxiliaryOl(prev, w, "öl" + w.slice(2)));
  }
}
for (const v of VOWELS) for (const s of ["", "ev", "kapı", "okul", "göz", "saat", "kitap", "üzüm", "kalem", "x"]) probe("followsHarmony", [s, v], R.followsHarmony(s, v));

// Stems and suffixes.
const isMorph = (w: string) => lexicon.count(w) > 0;
for (const [stem, suffix] of stemSuffix) {
  const capped = T.upperTr(stem.slice(0, 1)) + stem.slice(1);
  const plainSuffix = P.foldTurkish(suffix);
  group("s", [stem, suffix, capped, plainSuffix], {
    isValidPlaceApostrophe: R.isValidPlaceApostrophe(stem, suffix),
    isValidGenericApostrophe: [R.isValidGenericApostrophe(stem, suffix), R.isValidGenericApostrophe(stem, suffix, true)],
    isPhonologicallyValidProperSuffix: [{}, { acronym: true }, { p3sgHead: true }].map((o) => P.isPhonologicallyValidProperSuffix(stem, suffix, o)),
    harmonizeAsciiSuffix: [P.harmonizeAsciiSuffix(stem, suffix), P.harmonizeAsciiSuffix(stem, suffix, { p3sgHead: true })],
    joinWithoutApostrophe: [P.joinWithoutApostrophe(stem, suffix, isMorph), P.joinWithoutApostrophe(capped, suffix, isMorph)],
    isHarmoniousApostropheMarkChange: [
      R.isHarmoniousApostropheMarkChange(stem, plainSuffix, suffix),
      R.isHarmoniousApostropheMarkChange(stem, plainSuffix, suffix, true),
      R.isHarmoniousApostropheMarkChange(stem, suffix, plainSuffix),
    ],
    isValidKnownParticleSuffix: R.isValidKnownParticleSuffix(stem + " " + suffix, suffix),
  });
}
probe("particleSuffixes", [], suffixes.slice(0, 400));
for (const base of ["kim mi", "var mı", "ya da", "değil ki", "bir şey", "ne mu", "gel mü", "herkes"]) {
  group("k", [base], { isValidKnownParticleSuffix: suffixes.slice(0, 400).map((s) => R.isValidKnownParticleSuffix(base, s)) });
}

// Numbers and their suffixes.
const numbers = unique([
  ...rawTokens.filter((t) => /^[0-9]+$/.test(t)),
  ...["0", "00", "1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "20", "30", "40", "50", "60", "70", "80", "90", "100", "1000",
    "1000000", "1000000000", "1985", "2023", "2000", "1900", "007", "12a", "a0", "x"],
]);
const numberSuffixes = unique([...SUFFIXES, "nci", "ncı", "ncu", "ncü", "inci", "ıncı", "uncu", "üncü", "üncüsü", "ıncısında", "cm", "px",
  "a", "lere", "lerde", "larda", "lık", "De", "TE", "nc", "n", "y", "le", "lar", "ları", "ki", "teki", "deki"]);
probe("numberSuffixes", [], numberSuffixes);
for (const n of sample(numbers, (x) => x, 80)) {
  group("n", [n], { fixNumberSuffix: numberSuffixes.map((s) => [P.fixNumberSuffix(n, s, false), P.fixNumberSuffix(n, s, true)]) });
}
for (const t of unique(rawTokens.filter((x) => /^[0-9]+'?[a-zçğıöşü]+$/.test(x)))) {
  const m = /^([0-9]+)('?)(.*)$/.exec(t)!;
  probe("fixNumberSuffix", [m[1], m[3], m[2] === "'"], P.fixNumberSuffix(m[1]!, m[3]!, m[2] === "'"));
}

process.stdout.write(out.join("\n") + "\n");
