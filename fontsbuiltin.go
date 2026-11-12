package main

import (
	_ "embed"
	"log"

	"github.com/egoist/mygo/ui"
	ryugo "github.com/imhalid/ryu-go"
)

// The typefaces the app comes with (#395): Geist for its words and the editor's, Departure Mono for code. Both are
// free fonts under the SIL Open Font License (assets/fonts/*-OFL.txt); they go in the binary so that the app looks
// the same on every computer, and a family chosen in the settings still takes their place. Geist is its variable
// font, one file for every weight the app asks for (520, 620, 650…), and one for its italics; Departure Mono has a
// single weight.

//go:embed assets/fonts/Geist.ttf
var geistFont []byte

//go:embed assets/fonts/Geist-Italic.ttf
var geistItalicFont []byte

//go:embed assets/fonts/DepartureMono.otf
var departureMonoFont []byte

// sansStack is the stack of the app's words when no family is chosen: Geist, then the system's, for what it has no
// letters of.
const sansStack = "Geist, " + ryugo.FontSans

// monoStack is the stack of code when no family is chosen: Departure Mono, then the system's.
const monoStack = "Departure Mono, " + ryugo.FontMono

func init() {
	for _, f := range []struct {
		data   []byte
		family string
	}{{geistFont, "Geist"}, {geistItalicFont, "Geist"}, {departureMonoFont, "Departure Mono"}} {
		if err := ui.RegisterFont(f.data, f.family); err != nil {
			log.Printf("font %s: %v", f.family, err)
		}
	}
}
