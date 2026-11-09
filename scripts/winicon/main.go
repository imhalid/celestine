// Command winicon puts the app's icon into the Windows program: it writes rsrc_windows_amd64.syso, an object
// file with the icon as a resource, which go build links into the .exe by itself (the File Explorer, the
// Start menu, a shortcut show it). Run it again when assets/icon.ico changes:
//
//	go run ./scripts/winicon
//
// The object file is COFF with one .rsrc section: the resource directory (icons, the icon group), its data,
// and a relocation for each data entry's address, which the linker fills in.
package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
)

const (
	rtIcon      = 3
	rtGroupIcon = 14
	langEnUS    = 0x0409
)

type image struct {
	w, h byte
	bits uint16
	data []byte
}

func main() {
	ico, err := os.ReadFile("assets/icon.ico")
	check(err)
	imgs := parseICO(ico)
	check(os.WriteFile("rsrc_windows_amd64.syso", coff(imgs), 0o644))
	fmt.Printf("rsrc_windows_amd64.syso: %d images\n", len(imgs))
}

func parseICO(b []byte) []image {
	le := binary.LittleEndian
	if len(b) < 6 || le.Uint16(b[2:]) != 1 {
		check(fmt.Errorf("not an .ico"))
	}
	n := int(le.Uint16(b[4:]))
	var out []image
	for i := 0; i < n; i++ {
		e := b[6+16*i:]
		size, off := le.Uint32(e[8:]), le.Uint32(e[12:])
		out = append(out, image{w: e[0], h: e[1], bits: le.Uint16(e[6:]), data: b[off : off+size]})
	}
	return out
}

// coff lays out: directories (root, the two types, their names, their languages), then the data entries,
// then the data; each data entry's address is relocated against the section.
func coff(imgs []image) []byte {
	// the icon group: GRPICONDIR, an entry of 14 bytes for each image, naming it by its id (1..n)
	var grp bytes.Buffer
	w16 := func(v uint16) { binary.Write(&grp, binary.LittleEndian, v) }
	w16(0)
	w16(1)
	w16(uint16(len(imgs)))
	for i, im := range imgs {
		grp.Write([]byte{im.w, im.h, 0, 0})
		w16(1)
		w16(im.bits)
		binary.Write(&grp, binary.LittleEndian, uint32(len(im.data)))
		w16(uint16(i + 1))
	}

	type leaf struct {
		typ, id uint32
		data    []byte
	}
	var leaves []leaf
	for i, im := range imgs {
		leaves = append(leaves, leaf{rtIcon, uint32(i + 1), im.data})
	}
	leaves = append(leaves, leaf{rtGroupIcon, 1, grp.Bytes()})

	dir := func(entries int) int { return 16 + 8*entries }
	types := map[uint32][]int{} // type → the leaves in it
	order := []uint32{rtIcon, rtGroupIcon}
	for i, l := range leaves {
		types[l.typ] = append(types[l.typ], i)
	}

	// offsets
	off := dir(len(order))
	typeDir := map[uint32]int{}
	for _, t := range order {
		typeDir[t] = off
		off += dir(len(types[t]))
	}
	langDir := make([]int, len(leaves))
	for i := range leaves {
		langDir[i] = off
		off += dir(1)
	}
	dataEntry := make([]int, len(leaves))
	for i := range leaves {
		dataEntry[i] = off
		off += 16
	}
	dataOff := make([]int, len(leaves))
	for i, l := range leaves {
		off = (off + 7) &^ 7
		dataOff[i] = off
		off += len(l.data)
	}
	raw := make([]byte, (off+7)&^7)

	le := binary.LittleEndian
	putDir := func(at int, ids []uint32, targets []int, sub bool) {
		le.PutUint16(raw[at+14:], uint16(len(ids)))
		for i, id := range ids {
			e := at + 16 + 8*i
			le.PutUint32(raw[e:], id)
			t := uint32(targets[i])
			if sub {
				t |= 0x80000000
			}
			le.PutUint32(raw[e+4:], t)
		}
	}
	var tds []int
	for _, t := range order {
		tds = append(tds, typeDir[t])
	}
	putDir(0, order, tds, true)
	for _, t := range order {
		var ids []uint32
		var lds []int
		for _, i := range types[t] {
			ids = append(ids, leaves[i].id)
			lds = append(lds, langDir[i])
		}
		putDir(typeDir[t], ids, lds, true)
	}
	var relocs []uint32
	for i, l := range leaves {
		putDir(langDir[i], []uint32{langEnUS}, []int{dataEntry[i]}, false)
		le.PutUint32(raw[dataEntry[i]:], uint32(dataOff[i])) // + the section's address, by the relocation
		le.PutUint32(raw[dataEntry[i]+4:], uint32(len(l.data)))
		copy(raw[dataOff[i]:], l.data)
		relocs = append(relocs, uint32(dataEntry[i]))
	}

	// the file: header, section header, raw data, relocations, symbol table, string table
	const hdr, sec = 20, 40
	rawAt := hdr + sec
	relAt := rawAt + len(raw)
	symAt := relAt + 10*len(relocs)
	var f bytes.Buffer
	put := func(v any) { binary.Write(&f, binary.LittleEndian, v) }
	put(uint16(0x8664)) // AMD64
	put(uint16(1))      // one section
	put(uint32(0))      // time stamp
	put(uint32(symAt))
	put(uint32(1)) // one symbol
	put(uint16(0)) // no optional header
	put(uint16(0))
	f.WriteString(".rsrc\x00\x00\x00")
	put(uint32(0)) // virtual size
	put(uint32(0)) // virtual address
	put(uint32(len(raw)))
	put(uint32(rawAt))
	put(uint32(relAt))
	put(uint32(0))
	put(uint16(len(relocs)))
	put(uint16(0))
	put(uint32(0x40000040)) // initialized data, readable
	f.Write(raw)
	for _, r := range relocs {
		put(r)
		put(uint32(0)) // the symbol: .rsrc
		put(uint16(3)) // IMAGE_REL_AMD64_ADDR32NB
	}
	f.WriteString(".rsrc\x00\x00\x00")
	put(uint32(0)) // value
	put(uint16(1)) // section 1
	put(uint16(0))
	f.WriteByte(3) // static
	f.WriteByte(0)
	put(uint32(4)) // an empty string table
	return f.Bytes()
}

func check(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
