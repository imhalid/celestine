package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestSysoMatchesIcon(t *testing.T) {
	root := filepath.Join("..", "..")
	ico, err := os.ReadFile(filepath.Join(root, "assets", "icon.ico"))
	if err != nil {
		t.Fatalf("read icon.ico: %v", err)
	}
	want := coff(parseICO(ico))
	got, err := os.ReadFile(filepath.Join(root, "rsrc_windows_amd64.syso"))
	if err != nil {
		t.Fatalf("read rsrc_windows_amd64.syso: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("rsrc_windows_amd64.syso is out of date with assets/icon.ico (%d bytes vs %d bytes); run `go run ./scripts/winicon`", len(got), len(want))
	}
}
