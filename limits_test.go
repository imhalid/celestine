package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestReadLimits: a file larger than its limit is not read, and says so; a picture whose header says it has more
// pixels than are decoded is not decoded; the pictures kept are the ones used last (#177).
func TestReadLimits(t *testing.T) {
	dir := t.TempDir()
	big := filepath.Join(dir, "big.bin")
	os.WriteFile(big, bytes.Repeat([]byte{1}, 3000), 0o644)
	if _, err := readCapped(big, 2048); err == nil || !strings.Contains(err.Error(), "big.bin") || !strings.Contains(err.Error(), "2.9 KB") {
		t.Fatalf("a file over its limit: %v", err)
	}
	if data, err := readCapped(big, 4096); err != nil || len(data) != 3000 {
		t.Fatalf("a file under its limit: %d %v", len(data), err)
	}
	for n, want := range map[int64]string{512: "512 B", 1536: "1.5 KB", 512 << 20: "512 MB", 3 << 30: "3.0 GB"} {
		if got := bytesText(n); got != want {
			t.Errorf("bytesText(%d) = %q, want %q", n, got, want)
		}
	}

	// a small PNG that says it is 100 000 × 100 000
	var b bytes.Buffer
	png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 1, 1)))
	huge := b.Bytes()
	ihdr := huge[8+8 : 8+8+13] // after the signature, the chunk's length and type
	binary.BigEndian.PutUint32(ihdr[0:], 100_000)
	binary.BigEndian.PutUint32(ihdr[4:], 100_000)
	binary.BigEndian.PutUint32(huge[8+8+13:], crc32.ChecksumIEEE(huge[8+4:8+8+13]))
	if _, err := decodePicture("dev.png", huge); !errors.As(err, new(tooManyPixels)) || !strings.Contains(err.Error(), "dev.png") {
		t.Fatalf("a picture of ten billion pixels: %v", err)
	}
	if _, err := checkImage(huge); !errors.As(err, new(tooManyPixels)) {
		t.Fatalf("a cover of ten billion pixels: %v", err)
	}
	if _, err := decodePicture("ok.png", pngBytes(4, 4)); err != nil {
		t.Fatalf("a small picture: %v", err)
	}

	// the pictures kept: the one used longest ago goes
	a := &app{}
	src, _ := decodePicture("x.png", pngBytes(2, 2))
	for i := range keptImages {
		a.keepImage("p"+strconv.Itoa(i), src)
	}
	a.useImage("p0") // used again: kept
	a.keepImage("new", src)
	if len(a.images) != keptImages || a.images["p0"] == nil || a.images["p1"] != nil || a.images["new"] == nil {
		t.Fatalf("kept %d; p0 %v, p1 %v", len(a.images), a.images["p0"] != nil, a.images["p1"] != nil)
	}
}
