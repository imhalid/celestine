package spell

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"
	"unicode/utf16"
)

// This file is tinyfix-tr's src/lexicon-binary.ts: the compact binary format (ALX1) of the Turkish unigram and
// bigram lexicon (#623).
//
// The format is front-coded words over a one-byte alphabet (sorted by those bytes, so a binary search over blocks
// finds a word without building strings), exact small counts and one-byte logarithmic buckets for large ones, two CSR
// tables of word pairs keyed by word ids (bySecond for frequent trailing particles like de/da/ki/mi, byFirst for the
// rest), and Bloom filter tiers for rare forms kept only to protect them (count 2, 3, ...), which cost no strings.
// Multi-byte fields are little-endian.

const (
	magic0        = 0x41 // 'A'
	magic1        = 0x4c // 'L'
	magic2        = 0x58 // 'X'
	magic3        = 0x31 // '1'
	formatVersion = 1
	maxWordLen    = 128
)

// BinaryUnigramInput is a word form for EncodeBinaryLexicon: how often it was seen, and how often with a capital
// letter in the middle of a sentence.
type BinaryUnigramInput struct {
	Form        string
	Count       float64
	Capitalized float64
}

// BinaryBigramInput is a word pair for EncodeBinaryLexicon.
type BinaryBigramInput struct {
	First, Second string
	Count         float64
	// GroupBySecond stores the pair under the bySecond CSR partition, best for frequent trailing particles like
	// de/da/ki/mi.
	GroupBySecond bool
}

// BinaryBloomTierInput is a set of rare forms for EncodeBinaryLexicon that count as returnCount when the
// lexicon is asked about them, kept in a Bloom filter of bitsPerItem bits a word.
type BinaryBloomTierInput struct {
	ReturnCount float64
	BitsPerItem float64
	Words       []string
}

// EncodeBinaryLexiconOptions are the contents and parameters of an ALX1 lexicon. A parameter left zero takes its
// default (the TypeScript takes its default only when one is missing, so zero itself cannot be asked for here).
type EncodeBinaryLexiconOptions struct {
	Unigrams      []BinaryUnigramInput
	Bigrams       []BinaryBigramInput
	BloomTiers    []BinaryBloomTierInput
	BlockSize     float64 // 64
	ExactCountMax float64 // 80
	CountBuckets  float64 // 256
	CountMaxVal   float64 // 3,000,000
	CapMax        float64 // 50: 51 levels, a step of 0.02, exact on 0.45..0.75
}

// csrTable is one partition of the word pairs: the outer word ids (keys) in order, and for each the inner word ids
// (values[offsets[g]:offsets[g+1]]) with their quantized counts.
type csrTable struct {
	keys    []uint32
	offsets []uint32
	values  []uint32
	counts  []byte
}

// bloomTier is one Bloom filter of rare forms and the count it answers for them.
type bloomTier struct {
	returnCount float64
	kHashes     int
	wordCount   uint32
	nBits       uint64
	bits        []byte
}

// buildQuantTable is the counts that the one-byte buckets stand for: exact below exactMax, then logarithmic steps up
// to maxVal.
func buildQuantTable(exactMax, buckets, maxVal float64) []uint32 {
	table := make([]uint32, int(buckets))
	for i := 0; float64(i) < exactMax && float64(i) < buckets; i++ {
		table[i] = uint32(i)
	}
	logBuckets := buckets - exactMax
	if logBuckets > 1 && exactMax > 0 && maxVal > exactMax {
		step := math.Log(maxVal/exactMax) / (logBuckets - 1)
		for i := 0; float64(i) < logBuckets; i++ {
			table[int(exactMax)+i] = toUint32(jsRound(exactMax * math.Exp(float64(i)*step)))
		}
	} else if logBuckets == 1 {
		table[int(exactMax)] = toUint32(exactMax)
	}
	return table
}

// quantizeCount is the bucket of a count.
func quantizeCount(count, exactMax, buckets, maxVal float64) float64 {
	if count <= 0 {
		return 0
	}
	if count < exactMax {
		return count
	}
	logBuckets := buckets - exactMax
	if logBuckets <= 1 || exactMax <= 0 || maxVal <= exactMax {
		return math.Min(buckets-1, exactMax)
	}
	step := math.Log(maxVal/exactMax) / (logBuckets - 1)
	return math.Min(buckets-1, exactMax+jsRound(math.Log(count/exactMax)/step))
}

// writeVarint appends value as an unsigned LEB128 varint.
func writeVarint(out []byte, value float64) []byte {
	v := toUint32(value)
	for v >= 0x80 {
		out = append(out, byte(v&0x7f|0x80))
		v >>= 7
	}
	return append(out, byte(v))
}

// utf16Units are a string's UTF-16 code units, what JavaScript indexes.
func utf16Units(s string) []uint16 { return utf16.Encode([]rune(s)) }

// hashWord2 is a deterministic pair of 32-bit FNV/Murmur-mixed hashes over a word's UTF-16 code units, with
// JavaScript's Math.imul and >>> arithmetic.
func hashWord2(word string) (uint32, uint32) {
	h1 := uint32(0x811c9dc5)
	h2 := uint32(0x9e3779b9)
	i := uint32(0)
	unit := func(c uint32) {
		h1 = (h1 ^ c) * 0x01000193
		h2 = (h2 ^ (c + i)) * 0x85ebca6b
		i++
	}
	for _, r := range word {
		if r >= 0x10000 {
			hi, lo := utf16.EncodeRune(r)
			unit(uint32(hi))
			unit(uint32(lo))
		} else {
			unit(uint32(r))
		}
	}
	h1 ^= h1 >> 16
	h1 *= 0x85ebca6b
	h1 ^= h1 >> 13
	h2 ^= h2 >> 16
	h2 *= 0xc2b2ae35
	h2 ^= h2 >> 16
	return h1, h2
}

// bloomIndex is the bit the i-th hash of (h1, h2) sets: ((h1 + Math.imul(i, h2)) >>> 0) % nBits.
func bloomIndex(h1, h2 uint32, i int, nBits uint64) uint64 {
	return uint64(h1+uint32(i)*h2) % nBits
}

func bloomAdd(bits []byte, nBits uint64, kHashes int, word string) {
	h1, h2 := hashWord2(word)
	for i := 0; i < kHashes; i++ {
		idx := bloomIndex(h1, h2, i, nBits)
		bits[idx>>3] |= 1 << (idx & 7)
	}
}

func bloomHas(bits []byte, nBits uint64, kHashes int, word string) bool {
	h1, h2 := hashWord2(word)
	for i := 0; i < kHashes; i++ {
		idx := bloomIndex(h1, h2, i, nBits)
		if idx>>3 >= uint64(len(bits)) || bits[idx>>3]&(1<<(idx&7)) == 0 {
			return false
		}
	}
	return true
}

// csrEntry is an inner word id and the quantized count of its pair.
type csrEntry struct {
	inner int
	q     float64
}

// encodedCsr is a CSR partition as bytes.
type encodedCsr struct {
	groupCount, pairCount int
	meta, inner, counts   []byte
}

// encodeCsrPartition writes the groups of pairs as delta-coded varints: keys and degrees in meta, inner ids in
// inner, one count byte a pair.
func encodeCsrPartition(groups map[int][]csrEntry) encodedCsr {
	keys := make([]int, 0, len(groups))
	for k := range groups {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	var meta, inner, counts []byte
	prevKey := 0
	for _, key := range keys {
		meta = writeVarint(meta, float64(key-prevKey))
		prevKey = key

		list := groups[key]
		slices.SortStableFunc(list, func(a, b csrEntry) int { return a.inner - b.inner })
		// Identical inner ids are kept once, with the larger count, just in case.
		uniqueLen := 0
		for _, entry := range list {
			if uniqueLen > 0 && list[uniqueLen-1].inner == entry.inner {
				if entry.q > list[uniqueLen-1].q {
					list[uniqueLen-1].q = entry.q
				}
			} else {
				list[uniqueLen] = entry
				uniqueLen++
			}
		}

		meta = writeVarint(meta, float64(uniqueLen))
		prevVal := 0
		for _, e := range list[:uniqueLen] {
			inner = writeVarint(inner, float64(e.inner-prevVal))
			prevVal = e.inner
			counts = append(counts, toUint8(e.q))
		}
	}
	return encodedCsr{groupCount: len(keys), pairCount: len(counts), meta: meta, inner: inner, counts: counts}
}

// byteAt is bytes[i], or 0 past the end, as JavaScript reads undefined there and its bit operations take it for 0.
func byteAt(bytes []byte, i int) uint32 {
	if i < 0 || i >= len(bytes) {
		return 0
	}
	return uint32(bytes[i])
}

// subarray is JavaScript's Uint8Array.prototype.subarray: bounds past the end are clamped.
func subarray(bytes []byte, start, end int) []byte {
	start = min(max(start, 0), len(bytes))
	end = min(max(end, start), len(bytes))
	return bytes[start:end:end]
}

// readVarint reads an unsigned LEB128 varint as the TypeScript does: in 32-bit arithmetic, whose shifts wrap at 32.
func readVarint(bytes []byte, off *int) uint32 {
	v, shift := uint32(0), uint32(0)
	for {
		b := byteAt(bytes, *off)
		*off++
		v |= (b & 0x7f) << (shift & 31)
		if b&0x80 == 0 {
			return v
		}
		shift += 7
	}
}

// decodeCsrPartition reads a CSR partition back into id arrays, returning it and the offset after it.
func decodeCsrPartition(bytes []byte, startOffset int, groupCount, pairCount, metaLen, innerLen uint32) (csrTable, int, error) {
	if uint64(groupCount) > uint64(metaLen) {
		return csrTable{}, 0, errors.New("spell: corrupt CSR group count")
	}
	keys := make([]uint32, groupCount)
	offsets := make([]uint32, int(groupCount)+1)
	values := make([]uint32, pairCount)

	mOff := startOffset
	metaEnd := startOffset + int(metaLen)
	iOff := metaEnd
	innerEnd := metaEnd + int(innerLen)
	counts := subarray(bytes, innerEnd, innerEnd+int(pairCount))

	curKey := uint64(0)
	curPair := uint64(0)
	for g := range int(groupCount) {
		if mOff >= metaEnd {
			return csrTable{}, 0, errors.New("spell: truncated CSR meta")
		}
		curKey += uint64(readVarint(bytes, &mOff))
		keys[g] = uint32(curKey)
		offsets[g] = uint32(curPair)

		if mOff >= metaEnd {
			return csrTable{}, 0, errors.New("spell: truncated CSR degree")
		}
		deg := readVarint(bytes, &mOff)
		if uint64(deg) > uint64(pairCount)-curPair {
			return csrTable{}, 0, errors.New("spell: corrupt CSR degree")
		}
		curVal := uint64(0)
		for range deg {
			if iOff >= innerEnd {
				return csrTable{}, 0, errors.New("spell: truncated CSR inner")
			}
			curVal += uint64(readVarint(bytes, &iOff))
			values[curPair] = uint32(curVal)
			curPair++
		}
	}
	offsets[groupCount] = uint32(curPair)
	return csrTable{keys: keys, offsets: offsets, values: values, counts: counts}, innerEnd + int(pairCount), nil
}

// lookupCsr is the quantized count of the pair (outerId, innerId) in a partition, or 0.
func lookupCsr(table *csrTable, outerID, innerID uint32) uint32 {
	keys := table.keys
	lo, hi := 0, len(keys)-1
	group := -1
	for lo <= hi {
		mid := int(uint(lo+hi) >> 1)
		k := keys[mid]
		if k == outerID {
			group = mid
			break
		}
		if k < outerID {
			lo = mid + 1
		} else {
			hi = mid - 1
		}
	}
	if group == -1 {
		return 0
	}
	pLo := int(table.offsets[group])
	pHi := int(table.offsets[group+1]) - 1
	values := table.values
	for pLo <= pHi {
		mid := int(uint(pLo+pHi) >> 1)
		if mid >= len(values) {
			return 0
		}
		v := values[mid]
		if v == innerID {
			return byteAt(table.counts, mid)
		}
		if v < innerID {
			pLo = mid + 1
		} else {
			pHi = mid - 1
		}
	}
	return 0
}

// EncodeBinaryLexicon encodes unigrams, bigrams and optional Bloom tail filters into an ALX1 payload.
func EncodeBinaryLexicon(options EncodeBinaryLexiconOptions) ([]byte, error) {
	or := func(v, def float64) float64 {
		if v == 0 {
			return def
		}
		return v
	}
	blockSize := or(options.BlockSize, 64)
	exactCountMax := or(options.ExactCountMax, 80)
	countBuckets := or(options.CountBuckets, 256)
	countMaxVal := or(options.CountMaxVal, 3_000_000)
	capMax := or(options.CapMax, 50)

	// The alphabet is every UTF-16 unit of the forms, in code-unit order so the encoding is deterministic.
	charSet := map[uint16]bool{}
	for _, u := range options.Unigrams {
		for _, c := range utf16Units(u.Form) {
			charSet[c] = true
		}
	}
	alphabet := make([]uint16, 0, len(charSet))
	for c := range charSet {
		alphabet = append(alphabet, c)
	}
	slices.Sort(alphabet)
	if len(alphabet) > 254 {
		return nil, fmt.Errorf("lexicon binary: alphabet too large (%d > 254)", len(alphabet))
	}
	charToByte := map[uint16]byte{}
	for i, c := range alphabet {
		charToByte[c] = byte(i + 1)
	}

	// The forms are sorted by their alphabet bytes, so a binary search over the bytes finds them.
	type encodedForm struct {
		form               string
		bytes              []byte
		count, capitalized float64
	}
	encoded := make([]encodedForm, len(options.Unigrams))
	for i, u := range options.Unigrams {
		units := utf16Units(u.Form)
		b := make([]byte, len(units))
		for j, c := range units {
			b[j] = charToByte[c]
		}
		encoded[i] = encodedForm{form: u.Form, bytes: b, count: u.Count, capitalized: u.Capitalized}
	}
	slices.SortStableFunc(encoded, func(a, b encodedForm) int {
		m := min(len(a.bytes), len(b.bytes))
		for i := range m {
			if d := int(a.bytes[i]) - int(b.bytes[i]); d != 0 {
				return d
			}
		}
		return len(a.bytes) - len(b.bytes)
	})

	wordCount := len(encoded)
	idOf := map[string]int{}
	prefixLens := make([]byte, wordCount)
	suffixLens := make([]byte, wordCount)
	counts := make([]byte, wordCount)
	caps := make([]byte, wordCount)
	totalChars := 0
	block := int(toUint32(blockSize))
	for i, cur := range encoded {
		idOf[cur.form] = i
		prefix := 0
		if block == 0 || i%block != 0 {
			prev := encoded[i-1].bytes
			maxP := min(len(prev), len(cur.bytes), 255)
			for prefix < maxP && prev[prefix] == cur.bytes[prefix] {
				prefix++
			}
		}
		suffix := len(cur.bytes) - prefix
		if suffix > 255 {
			return nil, fmt.Errorf("lexicon binary: word suffix exceeds 255 chars (%q)", cur.form)
		}
		prefixLens[i] = byte(prefix)
		suffixLens[i] = byte(suffix)
		totalChars += suffix
		counts[i] = toUint8(quantizeCount(cur.count, exactCountMax, countBuckets, countMaxVal))
		if cur.capitalized <= 0 || cur.count <= 0 {
			caps[i] = 0
		} else {
			caps[i] = toUint8(math.Min(capMax, math.Max(1, jsRound(capMax*cur.capitalized/cur.count))))
		}
	}

	chars := make([]byte, 0, totalChars)
	for i, e := range encoded {
		chars = append(chars, e.bytes[prefixLens[i]:]...)
	}

	// The pairs are split into the byFirst and bySecond partitions.
	byFirstGroups := map[int][]csrEntry{}
	bySecondGroups := map[int][]csrEntry{}
	for _, bg := range options.Bigrams {
		id1, ok1 := idOf[bg.First]
		id2, ok2 := idOf[bg.Second]
		if !ok1 || !ok2 {
			continue
		}
		qc := quantizeCount(bg.Count, exactCountMax, countBuckets, countMaxVal)
		if bg.GroupBySecond {
			bySecondGroups[id2] = append(bySecondGroups[id2], csrEntry{id1, qc})
		} else {
			byFirstGroups[id1] = append(byFirstGroups[id1], csrEntry{id2, qc})
		}
	}
	csr1 := encodeCsrPartition(byFirstGroups)
	csr2 := encodeCsrPartition(bySecondGroups)

	// The Bloom tiers go highest count first, so the highest count matches first.
	var tiers []BinaryBloomTierInput
	for _, t := range options.BloomTiers {
		if len(t.Words) > 0 {
			tiers = append(tiers, t)
		}
	}
	slices.SortStableFunc(tiers, func(a, b BinaryBloomTierInput) int {
		switch d := b.ReturnCount - a.ReturnCount; {
		case d < 0:
			return -1
		case d > 0:
			return 1
		}
		return 0
	})
	built := make([]bloomTier, len(tiers))
	for i, tier := range tiers {
		byteLen := int(math.Max(8, math.Ceil(float64(len(tier.Words))*tier.BitsPerItem/8)))
		nBits := uint64(byteLen) * 8
		kHashes := int(math.Max(4, math.Min(20, jsRound(tier.BitsPerItem*math.Ln2))))
		bits := make([]byte, byteLen)
		for _, w := range tier.Words {
			bloomAdd(bits, nBits, kHashes, w)
		}
		built[i] = bloomTier{returnCount: tier.ReturnCount, kHashes: kHashes, wordCount: uint32(len(tier.Words)), nBits: nBits, bits: bits}
	}

	alphabetBytes := []byte(string(utf16.Decode(alphabet)))
	// The header: 4 magic + 1 version + 1 blockSize + 1 exactCountMax + 2 countBuckets + 4 countMaxVal + 1 capMax,
	// 2 alphabetByteLen + the alphabet, 4 wordCount + 4 charsByteLen, 4*4 for each CSR partition (groups, pairs,
	// metaLen, innerLen), 1 bloomCount + 10 a tier (1 returnCount + 1 kHashes + 4 wordCount + 4 byteLen).
	le := binary.LittleEndian
	out := []byte{magic0, magic1, magic2, magic3, formatVersion, toUint8(blockSize), toUint8(exactCountMax)}
	out = le.AppendUint16(out, toUint16(countBuckets))
	out = le.AppendUint32(out, toUint32(countMaxVal))
	out = append(out, toUint8(capMax))
	out = le.AppendUint16(out, uint16(len(alphabetBytes)))
	out = append(out, alphabetBytes...)
	out = le.AppendUint32(out, uint32(wordCount))
	out = le.AppendUint32(out, uint32(len(chars)))
	for _, c := range []encodedCsr{csr1, csr2} {
		out = le.AppendUint32(out, uint32(c.groupCount))
		out = le.AppendUint32(out, uint32(c.pairCount))
		out = le.AppendUint32(out, uint32(len(c.meta)))
		out = le.AppendUint32(out, uint32(len(c.inner)))
	}
	out = append(out, byte(len(built)))
	for _, b := range built {
		out = append(out, toUint8(b.returnCount), byte(b.kHashes))
		out = le.AppendUint32(out, b.wordCount)
		out = le.AppendUint32(out, uint32(len(b.bits)))
	}
	for _, part := range [][]byte{prefixLens, suffixLens, chars, counts, caps, csr1.meta, csr1.inner, csr1.counts, csr2.meta, csr2.inner, csr2.counts} {
		out = append(out, part...)
	}
	for _, b := range built {
		out = append(out, b.bits...)
	}
	return out, nil
}

// errBadHeader is the error of bytes that are not an ALX1 lexicon.
var errBadHeader = errors.New("lexicon binary: not an ALX1 header")

// BinaryLexicon is a memory-compact lexicon read straight from ALX1 bytes, which it keeps: a lookup builds no
// strings. It is safe for concurrent use (the TypeScript caches the last lookup; this does not, so that it is).
type BinaryLexicon struct {
	blockSize      int
	wordCount      int
	totalFormCount int
	totalPairCount int
	capMax         int
	quantTable     []uint32
	alphabetSize   int
	charToByte     *[65536]byte
	byteToChar     []uint16
	prefixLens     []byte
	suffixLens     []byte
	chars          []byte
	counts         []byte
	caps           []byte
	blkCharOff     []uint32
	bktLo          []int32
	bktHi          []int32
	byFirst        csrTable
	bySecond       csrTable
	blooms         []bloomTier
}

// NewBinaryLexicon reads an ALX1 lexicon. The lexicon keeps bytes, which must not change after. Bytes whose
// header is not ALX1, or whose sections run past their end, are an error.
func NewBinaryLexicon(bytes []byte) (*BinaryLexicon, error) {
	if len(bytes) < 57 || bytes[0] != magic0 || bytes[1] != magic1 || bytes[2] != magic2 || bytes[3] != magic3 {
		return nil, errBadHeader
	}
	if version := bytes[4]; version != formatVersion {
		return nil, fmt.Errorf("lexicon binary: unsupported ALX1 format version %d", version)
	}
	truncated := errors.New("lexicon binary: ALX1 sections run past the end of the data")
	le := binary.LittleEndian
	off := 5
	need := func(n int) bool { return off+n <= len(bytes) }
	u16 := func() uint32 { v := le.Uint16(bytes[off:]); off += 2; return uint32(v) }
	u32 := func() uint32 { v := le.Uint32(bytes[off:]); off += 4; return v }
	l := &BinaryLexicon{}
	l.blockSize = int(bytes[off])
	exactCountMax := float64(bytes[off+1])
	off += 2
	countBuckets := float64(u16())
	countMaxVal := float64(u32())
	l.capMax = int(bytes[off])
	off++
	l.quantTable = buildQuantTable(exactCountMax, countBuckets, countMaxVal)

	alphabetByteLen := int(u16())
	if !need(alphabetByteLen) {
		return nil, truncated
	}
	alphabet := utf16Units(string(bytes[off : off+alphabetByteLen]))
	off += alphabetByteLen
	if len(alphabet) > 254 {
		return nil, errors.New("spell: lexicon alphabet exceeds 254 symbols")
	}

	l.alphabetSize = len(alphabet) + 1
	l.charToByte = new([65536]byte)
	l.byteToChar = make([]uint16, l.alphabetSize)
	for i, ch := range alphabet {
		l.charToByte[ch] = byte(i + 1)
		l.byteToChar[i+1] = ch
	}

	if !need(4*10 + 1) {
		return nil, truncated
	}
	wordCountU := u32()
	charsByteLenU := u32()
	csr1Groups, csr1Pairs, csr1MetaLen, csr1InnerLen := u32(), u32(), u32(), u32()
	csr2Groups, csr2Pairs, csr2MetaLen, csr2InnerLen := u32(), u32(), u32(), u32()

	type bloomMeta struct {
		returnCount, kHashes int
		wordCount, byteLen   uint32
	}
	bloomCount := int(bytes[off])
	off++
	if !need(bloomCount * 10) {
		return nil, truncated
	}
	metas := make([]bloomMeta, bloomCount)
	for i := range metas {
		metas[i].returnCount = int(bytes[off])
		metas[i].kHashes = int(bytes[off+1])
		off += 2
		metas[i].wordCount = u32()
		metas[i].byteLen = u32()
		if metas[i].byteLen == 0 && metas[i].kHashes > 0 {
			return nil, truncated
		}
	}

	// Every section must fit before any is read, so a lookup never reads past the end.
	body := uint64(4)*uint64(wordCountU) + uint64(charsByteLenU) +
		uint64(csr1MetaLen) + uint64(csr1InnerLen) + uint64(csr1Pairs) +
		uint64(csr2MetaLen) + uint64(csr2InnerLen) + uint64(csr2Pairs)
	for _, m := range metas {
		body += uint64(m.byteLen)
	}
	if body > uint64(len(bytes)-off) {
		return nil, truncated
	}
	l.wordCount = int(wordCountU)
	charsByteLen := int(charsByteLenU)
	n := l.wordCount
	l.prefixLens = bytes[off : off+n : off+n]
	off += n
	l.suffixLens = bytes[off : off+n : off+n]
	off += n
	l.chars = bytes[off : off+charsByteLen : off+charsByteLen]
	off += charsByteLen
	l.counts = bytes[off : off+n : off+n]
	off += n
	l.caps = bytes[off : off+n : off+n]
	off += n
	total := 0
	for _, s := range l.suffixLens {
		total += int(s)
	}
	if total != charsByteLen {
		return nil, truncated
	}

	var err error
	l.byFirst, off, err = decodeCsrPartition(bytes, off, csr1Groups, csr1Pairs, csr1MetaLen, csr1InnerLen)
	if err != nil {
		return nil, err
	}
	l.bySecond, off, err = decodeCsrPartition(bytes, off, csr2Groups, csr2Pairs, csr2MetaLen, csr2InnerLen)
	if err != nil {
		return nil, err
	}

	bloomWordTotal := 0
	for _, m := range metas {
		bits := bytes[off : off+int(m.byteLen) : off+int(m.byteLen)]
		off += int(m.byteLen)
		l.blooms = append(l.blooms, bloomTier{
			returnCount: float64(m.returnCount),
			kHashes:     m.kHashes,
			wordCount:   m.wordCount,
			nBits:       uint64(m.byteLen) * 8,
			bits:        bits,
		})
		bloomWordTotal += int(m.wordCount)
	}
	l.totalFormCount = n + bloomWordTotal
	l.totalPairCount = int(csr1Pairs) + int(csr2Pairs)

	// The offsets of the blocks and a table of blocks by their first two bytes, in one pass.
	blkSize := l.blockSize
	nBlocks := 0
	if n != 0 {
		if blkSize == 0 {
			return nil, truncated
		}
		nBlocks = (n + blkSize - 1) / blkSize
	}
	l.blkCharOff = make([]uint32, nBlocks)
	cSize := l.alphabetSize
	totalBuckets := cSize * cSize
	bucketStart := make([]int32, totalBuckets)
	bucketEnd := make([]int32, totalBuckets)
	for i := range bucketStart {
		bucketStart[i], bucketEnd[i] = -1, -1
	}
	charPos := 0
	for blk := range nBlocks {
		l.blkCharOff[blk] = uint32(charPos)
		base := blk * blkSize
		end := min(base+blkSize, n)
		len0 := l.suffixLens[base]
		b0, b1 := 0, 0
		if len0 > 0 {
			b0 = int(l.chars[charPos])
		}
		if len0 > 1 {
			b1 = int(l.chars[charPos+1])
		}
		// A byte outside the alphabet (only in a damaged file) has no bucket, as a typed array ignores the write.
		if bkt := b0*cSize + b1; bkt < totalBuckets {
			if bucketStart[bkt] == -1 {
				bucketStart[bkt] = int32(blk)
			}
			bucketEnd[bkt] = int32(blk + 1)
		}
		for i := base; i < end; i++ {
			charPos += int(l.suffixLens[i])
		}
	}
	l.bktLo = make([]int32, totalBuckets)
	l.bktHi = make([]int32, totalBuckets)
	lastSeen := int32(0)
	for bkt := range totalBuckets {
		if bucketStart[bkt] != -1 {
			l.bktLo[bkt] = max(bucketStart[bkt]-1, 0)
			l.bktHi[bkt] = bucketEnd[bkt]
			lastSeen = bucketEnd[bkt] - 1
		} else {
			l.bktLo[bkt] = lastSeen
			l.bktHi[bkt] = min(lastSeen+1, int32(nBlocks))
		}
	}
	return l, nil
}

// Size is the number of forms the lexicon knows, the Bloom tiers' included.
func (l *BinaryLexicon) Size() int { return l.totalFormCount }

// ExactSize is the number of forms in the exact table.
func (l *BinaryLexicon) ExactSize() int { return l.wordCount }

// PairCount is the number of word pairs.
func (l *BinaryLexicon) PairCount() int { return l.totalPairCount }

// quant is the count a bucket stands for; a bucket past the table (only in a damaged file) is NaN, as
// JavaScript's undefined turns into in arithmetic.
func (l *BinaryLexicon) quant(q uint32) float64 {
	if int(q) >= len(l.quantTable) {
		return math.NaN()
	}
	return float64(l.quantTable[q])
}

// findExactID is the id of form in the exact table, or -1: a binary search over the blocks' first words, then a
// scan through the block's front-coded words.
func (l *BinaryLexicon) findExactID(form string) int {
	var qBuf [maxWordLen]byte
	qLen := 0
	charMap := l.charToByte
	put := func(unit uint16) bool {
		if qLen >= maxWordLen {
			return false
		}
		code := charMap[unit]
		if code == 0 {
			return false
		}
		qBuf[qLen] = code
		qLen++
		return true
	}
	for _, r := range form {
		if r >= 0x10000 {
			hi, lo := utf16.EncodeRune(r)
			if !put(uint16(hi)) || !put(uint16(lo)) {
				return -1
			}
		} else if !put(uint16(r)) {
			return -1
		}
	}
	if qLen == 0 || l.wordCount == 0 {
		return -1
	}

	b0 := int(qBuf[0])
	b1 := 0
	if qLen > 1 {
		b1 = int(qBuf[1])
	}
	bkt := b0*l.alphabetSize + b1
	lo := int(l.bktLo[bkt])
	hi := int(l.bktHi[bkt]) - 1
	blkSize := l.blockSize
	blkOff := l.blkCharOff
	sArr := l.suffixLens
	cArr := l.chars

	blk := lo
	for lo <= hi {
		mid := int(uint(lo+hi) >> 1)
		idx := mid * blkSize
		cOff := int(blkOff[mid])
		hLen := int(sArr[idx])
		m := min(hLen, qLen)
		cmp := 0
		for j := range m {
			if diff := int(cArr[cOff+j]) - int(qBuf[j]); diff != 0 {
				cmp = diff
				break
			}
		}
		if cmp == 0 {
			cmp = hLen - qLen
		}
		if cmp == 0 {
			return idx
		}
		if cmp < 0 {
			blk = mid
			lo = mid + 1
		} else {
			hi = mid - 1
		}
	}

	base := blk * blkSize
	end := min(base+blkSize, l.wordCount)
	pos := int(blkOff[blk])
	len0 := int(sArr[base])
	matched := 0
	{
		m := min(len0, qLen)
		for matched < m && cArr[pos+matched] == qBuf[matched] {
			matched++
		}
		if matched == len0 && len0 == qLen {
			return base
		}
		// The block's first word is already past the query: the query is not in this block.
		if matched < m {
			if cArr[pos+matched] > qBuf[matched] {
				return -1
			}
		} else if len0 > qLen {
			return -1
		}
	}
	pos += len0

	pArr := l.prefixLens
	for i := base + 1; i < end; i++ {
		p := int(pArr[i])
		s := int(sArr[i])
		if p < matched {
			break
		}
		if p == matched {
			j := 0
			rem := qLen - p
			m := min(s, rem)
			for j < m && cArr[pos+j] == qBuf[p+j] {
				j++
			}
			matched = p + j
			if j == m {
				if s == rem {
					return i
				}
				if s > rem {
					break
				}
			} else if cArr[pos+j] > qBuf[p+j] {
				break
			}
		}
		pos += s
	}
	return -1
}

// Count is how often form was seen: its exact count, or a Bloom tier's count for a rare form, or 0.
func (l *BinaryLexicon) Count(form string) float64 {
	if id := l.findExactID(form); id != -1 {
		return l.quant(uint32(l.counts[id]))
	}
	// Apostrophe forms are never in the Bloom tiers; probing them could only find phantoms.
	if strings.Contains(form, "'") {
		return 0
	}
	for i := range l.blooms {
		b := &l.blooms[i]
		if bloomHas(b.bits, b.nBits, b.kHashes, form) {
			return b.returnCount
		}
	}
	return 0
}

// ExactCount is the exact table's count only, without the protection-only Bloom tiers.
func (l *BinaryLexicon) ExactCount(form string) float64 {
	if id := l.findExactID(form); id != -1 {
		return l.quant(uint32(l.counts[id]))
	}
	return 0
}

// CapitalShare is the share of form's occurrences written with a capital mid-sentence.
func (l *BinaryLexicon) CapitalShare(form string) float64 {
	id := l.findExactID(form)
	if id == -1 || l.capMax == 0 {
		return 0
	}
	return float64(l.caps[id]) / float64(l.capMax)
}

// Pair is how often first was followed by second.
func (l *BinaryLexicon) Pair(first, second string) float64 {
	id2 := l.findExactID(second)
	if id2 == -1 {
		return 0
	}
	id1 := l.findExactID(first)
	if id1 == -1 {
		return 0
	}
	if q2 := lookupCsr(&l.bySecond, uint32(id2), uint32(id1)); q2 != 0 {
		return l.quant(q2)
	}
	if q1 := lookupCsr(&l.byFirst, uint32(id1), uint32(id2)); q1 != 0 {
		return l.quant(q1)
	}
	return 0
}

// ToTSV rebuilds the exact table as TSV texts ("form\tcount\tcapitalized" and "first second\tcount" lines), for
// diagnostics. As in the TypeScript, a word is rebuilt in a buffer of maxWordLen bytes: what is longer comes out
// cut short, with "undefined" for each unit past it.
func (l *BinaryLexicon) ToTSV() (unigrams, bigrams string) {
	words := make([]string, l.wordCount)
	var cur [maxWordLen]byte
	charPos := 0
	uLines := make([]string, 0, l.wordCount)
	undefined := utf16Units("undefined")
	for i := range l.wordCount {
		p := int(l.prefixLens[i])
		s := int(l.suffixLens[i])
		for j := range s {
			if p+j < maxWordLen {
				cur[p+j] = l.chars[charPos]
			}
			charPos++
		}
		units := make([]uint16, 0, p+s)
		for j := range p + s {
			switch {
			case j >= maxWordLen:
				units = append(units, undefined...)
			case int(cur[j]) >= len(l.byteToChar):
				units = append(units, undefined...)
			case cur[j] != 0:
				units = append(units, l.byteToChar[cur[j]])
			}
		}
		w := string(utf16.Decode(units))
		words[i] = w
		c := l.quant(uint32(l.counts[i]))
		cp := 0.0
		if l.capMax != 0 {
			cp = jsRound(c * float64(l.caps[i]) / float64(l.capMax))
		}
		uLines = append(uLines, w+"\t"+jsNumberString(c)+"\t"+jsNumberString(cp))
	}
	var bLines []string
	word := func(id uint32) string {
		if int(id) < len(words) {
			return words[id]
		}
		return "undefined"
	}
	for g := range l.byFirst.keys {
		w1 := word(l.byFirst.keys[g])
		for p := l.byFirst.offsets[g]; p < l.byFirst.offsets[g+1]; p++ {
			bLines = append(bLines, w1+" "+word(valueAt(l.byFirst.values, p))+"\t"+jsNumberString(l.quant(byteAt(l.byFirst.counts, int(p)))))
		}
	}
	for g := range l.bySecond.keys {
		w2 := word(l.bySecond.keys[g])
		for p := l.bySecond.offsets[g]; p < l.bySecond.offsets[g+1]; p++ {
			bLines = append(bLines, word(valueAt(l.bySecond.values, p))+" "+w2+"\t"+jsNumberString(l.quant(byteAt(l.bySecond.counts, int(p)))))
		}
	}
	return strings.Join(uLines, "\n"), strings.Join(bLines, "\n")
}

// valueAt is values[i], or an id past every word where a damaged file's offsets run past the values.
func valueAt(values []uint32, i uint32) uint32 {
	if int(i) < len(values) {
		return values[i]
	}
	return math.MaxUint32
}
