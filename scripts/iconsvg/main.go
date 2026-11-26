// Command iconsvg writes the app's icon from its one drawing (internal/appicon): assets/icon.svg in the Paper look,
// which scripts/icons.sh makes every size of, and assets/brand/icon-<look>.svg for each other look, with SVG's
// filters for the shadow and the glow. Run it when a look or the drawing changes (scripts/icons.sh runs it first):
//
//	go run ./scripts/iconsvg
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/imhalid/celestine/internal/appicon"
)

func main() {
	for _, l := range appicon.Looks {
		out := filepath.Join("assets", "brand", "icon-"+l.ID+".svg")
		if l.ID == "paper" {
			out = filepath.Join("assets", "icon.svg")
		}
		if err := os.WriteFile(out, []byte(appicon.SVG(l, appicon.PartAll, true)), 0o644); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Println(out)
	}
}
