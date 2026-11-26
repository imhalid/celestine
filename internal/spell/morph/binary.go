package morph

// This file is binary.ts: the compact AMR1 form of parsed dictionaries (AMR2 with a prior after it), morph.bin.
// The items are sorted by their roots' UTF-8 bytes and front-coded, in columns, little-endian.

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"sort"
	"unicode/utf8"
)

// The AMR header's magic and versions.
const (
	formatVersionV1 = 1
	formatVersionV2 = 2
)

var amrMagic = [4]byte{'A', 'M', 'R', '1'}

// secondaryPosList is the secondary parts of speech by their index in morph.bin.
var secondaryPosList = []SecondaryPos{
	"None", "ProperNoun", "Abbreviation", "Time", "PersonalPron", "DemonstrativePron", "QuantitivePron",
	"QuestionPron", "ReflexivePron", "Card", "Ord", "Dist", "PCDat", "PCAbl", "PCNom", "PCGen", "PCIns", "PCAcc",
}

// The bits of an item's flags byte.
const (
	lemmaModeMask     = 0x03
	lemmaModeRoot     = 0
	lemmaModeVerbMak  = 1
	lemmaModeVerbMek  = 2
	lemmaModeExplicit = 3
	flagHasPron       = 1 << 2
	flagHasRef        = 1 << 3
	flagHasIndex      = 1 << 4
	flagHasKey        = 1 << 5
)

// defaultItemKey is the key an item has unless its flags say it has another.
func defaultItemKey(root, lemma string, primaryPos PrimaryPos, secondaryPos SecondaryPos, attributes int) string {
	if attributes&raDummy != 0 {
		return root + "_Noun_CompoundRoot"
	}
	return makeKey(normalizeCircumflex(lemma), primaryPos, secondaryPos)
}

func le16(b []byte) uint16 { return binary.LittleEndian.Uint16(b) }
func le32(b []byte) uint32 { return binary.LittleEndian.Uint32(b) }

func appendUint16(b []byte, v uint16) []byte { return binary.LittleEndian.AppendUint16(b, v) }
func appendUint32(b []byte, v uint32) []byte { return binary.LittleEndian.AppendUint32(b, v) }

// appendVarint writes an unsigned LEB128 number.
func appendVarint(out []byte, v uint32) []byte {
	for v >= 0x80 {
		out = append(out, byte(v&0x7f|0x80))
		v >>= 7
	}
	return append(out, byte(v))
}

// appendString writes a string's length as a varint and its UTF-8 bytes.
func appendString(out []byte, s string) []byte {
	return append(appendVarint(out, uint32(len(s))), s...)
}

// encodeBinaryDictionary writes items as AMR1, or as AMR2 with prior when prior is not nil.
func encodeBinaryDictionary(items []*dictionaryItem, prior *morphPrior) ([]byte, error) {
	// The items sorted by their roots' UTF-8 bytes for the front coding, ties kept in order.
	sorted := append([]*dictionaryItem{}, items...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].root < sorted[j].root })
	newIndexOf := make(map[*dictionaryItem]int, len(sorted))
	for i, item := range sorted {
		newIndexOf[item] = i
	}

	// The palette of attribute masks (fewer than 50 across some 31K items).
	var attrPalette []uint32
	attrToIdx := map[uint32]int{}
	for _, item := range sorted {
		a := uint32(item.attributes)
		if _, ok := attrToIdx[a]; !ok {
			attrToIdx[a] = len(attrPalette)
			attrPalette = append(attrPalette, a)
		}
	}
	wideAttr := len(attrPalette) > 255
	secCount := len(secondaryPosList)

	n := len(sorted)
	prefixLens, suffixLens := make([]byte, n), make([]byte, n)
	posSec, flagsCol := make([]byte, n), make([]byte, n)
	var rootStream, attrCol, extras []byte
	prev := ""
	for i, item := range sorted {
		prefix := 0
		for prefix < min(len(prev), len(item.root), 255) && prev[prefix] == item.root[prefix] {
			prefix++
		}
		suffix := len(item.root) - prefix
		if suffix > 255 {
			return nil, fmt.Errorf("morph binary: root suffix exceeds 255 bytes (%q)", item.root)
		}
		prefixLens[i], suffixLens[i] = byte(prefix), byte(suffix)
		rootStream = append(rootStream, item.root[prefix:]...)
		prev = item.root

		pIdx, sIdx := indexOf(primaryPosList, item.primaryPos), indexOf(secondaryPosList, item.secondaryPos)
		if pIdx == -1 || sIdx == -1 {
			return nil, fmt.Errorf("morph binary: unknown POS (%s, %s)", item.primaryPos, item.secondaryPos)
		}
		posSec[i] = byte(pIdx*secCount + sIdx)

		aIdx := attrToIdx[uint32(item.attributes)]
		if wideAttr {
			attrCol = append(attrCol, byte(aIdx), byte(aIdx>>8))
		} else {
			attrCol = append(attrCol, byte(aIdx))
		}

		var flags byte
		switch {
		case item.lemma == item.root:
			flags |= lemmaModeRoot
		case item.primaryPos == "Verb" && item.lemma == item.root+"mak":
			flags |= lemmaModeVerbMak
		case item.primaryPos == "Verb" && item.lemma == item.root+"mek":
			flags |= lemmaModeVerbMek
		default:
			flags |= lemmaModeExplicit
			extras = appendString(extras, item.lemma)
		}
		if item.pronunciation != item.root {
			flags |= flagHasPron
			extras = appendString(extras, item.pronunciation)
		}
		if item.referenceItem != nil {
			if refIdx, ok := newIndexOf[item.referenceItem]; ok {
				flags |= flagHasRef
				extras = appendVarint(extras, uint32(refIdx))
			}
		}
		if item.index != 0 {
			flags |= flagHasIndex
			extras = appendVarint(extras, uint32(item.index))
		}
		if item.key != defaultItemKey(item.root, item.lemma, item.primaryPos, item.secondaryPos, item.attributes) {
			flags |= flagHasKey
			extras = appendString(extras, item.key)
		}
		flagsCol[i] = flags
	}

	var priorBytes []byte
	if prior != nil {
		var err error
		if priorBytes, err = encodeMorphPrior(prior, sorted); err != nil {
			return nil, err
		}
	}

	out := append([]byte{}, amrMagic[:]...)
	if priorBytes != nil {
		out = append(out, formatVersionV2)
	} else {
		out = append(out, formatVersionV1)
	}
	out = appendUint32(out, uint32(n))
	out = appendUint16(out, uint16(len(attrPalette)))
	for _, a := range attrPalette {
		out = appendUint32(out, a)
	}
	out = appendUint32(out, uint32(len(rootStream)))
	out = appendUint32(out, uint32(len(extras)))
	for _, part := range [][]byte{prefixLens, suffixLens, rootStream, posSec, attrCol, flagsCol, extras, priorBytes} {
		out = append(out, part...)
	}
	return out, nil
}

// errTruncated is the error of an AMR payload that ends too soon.
var errTruncated = errors.New("morph: truncated morphology binary (AMR1)")

// decodeUTF8 is TextDecoder's UTF-8 decoding: an invalid byte becomes U+FFFD.
func decodeUTF8(b []byte) string {
	if utf8.Valid(b) {
		return string(b)
	}
	return string(bytes.ToValidUTF8(b, []byte("�")))
}

// parseBinaryDictionaryWithPrior reads an AMR1 or AMR2 payload into the dictionary items and, for AMR2, the prior
// (nil when there is none).
func parseBinaryDictionaryWithPrior(b []byte) ([]*dictionaryItem, *morphPrior, error) {
	if len(b) < 19 || [4]byte(b[:4]) != amrMagic {
		return nil, nil, errors.New("morph: invalid morphology binary header (AMR1 expected)")
	}
	version := b[4]
	if version != formatVersionV1 && version != formatVersionV2 {
		return nil, nil, fmt.Errorf("morph: unsupported morphology binary version %d", version)
	}

	off := 5
	itemCount := int(le32(b[off:]))
	off += 4
	attrPaletteLen := int(le16(b[off:]))
	off += 2
	if off+attrPaletteLen*4+8 > len(b) {
		return nil, nil, errTruncated
	}
	attrPalette := make([]int, attrPaletteLen)
	for i := range attrPalette {
		attrPalette[i] = int(int32(le32(b[off:])))
		off += 4
	}
	rootsByteLen := int(le32(b[off:]))
	off += 4
	extrasByteLen := int(le32(b[off:]))
	off += 4

	wideAttr := attrPaletteLen > 255
	attrColLen := itemCount
	if wideAttr {
		attrColLen = itemCount * 2
	}
	if off+itemCount*4+attrColLen+rootsByteLen+extrasByteLen > len(b) {
		return nil, nil, errTruncated
	}
	prefixLens := b[off : off+itemCount]
	off += itemCount
	suffixLens := b[off : off+itemCount]
	off += itemCount
	rootStream := b[off : off+rootsByteLen]
	off += rootsByteLen
	posSec := b[off : off+itemCount]
	off += itemCount
	attrCol := b[off : off+attrColLen]
	off += attrColLen
	flagsCol := b[off : off+itemCount]
	off += itemCount
	extras := b[off : off+extrasByteLen]
	priorOffset := off + extrasByteLen

	var curRoot [256]byte
	items := make([]*dictionaryItem, itemCount)
	type ref struct{ item, ref int }
	var pendingRefs []ref
	secCount := len(secondaryPosList)
	rootOff, extOff := 0, 0
	var err error

	readVarint := func() int {
		result, shift := uint32(0), uint(0)
		for {
			if extOff >= len(extras) {
				err = errTruncated
				return 0
			}
			c := extras[extOff]
			extOff++
			result |= uint32(c&0x7f) << (shift & 31)
			if c&0x80 == 0 {
				return int(result)
			}
			shift += 7
		}
	}
	readString := func() string {
		n := readVarint()
		if extOff+n > len(extras) {
			err = errTruncated
			return ""
		}
		s := decodeUTF8(extras[extOff : extOff+n])
		extOff += n
		return s
	}

	for i := range items {
		p, s := int(prefixLens[i]), int(suffixLens[i])
		if rootOff+s > len(rootStream) || p+s > len(curRoot) {
			return nil, nil, errTruncated
		}
		copy(curRoot[p:], rootStream[rootOff:rootOff+s])
		rootOff += s
		root := decodeUTF8(curRoot[:p+s])

		ps := int(posSec[i])
		primaryPos := listItem(primaryPosList, ps/secCount)
		secondaryPos := listItem(secondaryPosList, ps%secCount)
		aIdx := int(attrCol[i])
		if wideAttr {
			aIdx = int(attrCol[i*2]) | int(attrCol[i*2+1])<<8
		}
		if aIdx >= len(attrPalette) {
			return nil, nil, errTruncated
		}
		attributes := attrPalette[aIdx]
		flags := flagsCol[i]

		var lemma string
		switch flags & lemmaModeMask {
		case lemmaModeRoot:
			lemma = root
		case lemmaModeVerbMak:
			lemma = root + "mak"
		case lemmaModeVerbMek:
			lemma = root + "mek"
		default:
			lemma = readString()
		}
		pronunciation := root
		if flags&flagHasPron != 0 {
			pronunciation = readString()
		}
		if flags&flagHasRef != 0 {
			pendingRefs = append(pendingRefs, ref{i, readVarint()})
		}
		index := 0
		if flags&flagHasIndex != 0 {
			index = readVarint()
		}
		var key string
		if flags&flagHasKey != 0 {
			key = readString()
		} else {
			key = defaultItemKey(root, lemma, primaryPos, secondaryPos, attributes)
		}
		if err != nil {
			return nil, nil, err
		}
		items[i] = &dictionaryItem{
			key:           key,
			lemma:         lemma,
			root:          root,
			pronunciation: pronunciation,
			primaryPos:    primaryPos,
			secondaryPos:  secondaryPos,
			attributes:    attributes,
			index:         index,
		}
	}

	for _, r := range pendingRefs {
		if r.ref < len(items) {
			items[r.item].referenceItem = items[r.ref]
		}
	}

	var prior *morphPrior
	if version == formatVersionV2 && priorOffset < len(b) {
		if prior, err = decodeMorphPrior(b, priorOffset, items); err != nil {
			return nil, nil, err
		}
	}
	return items, prior, nil
}
