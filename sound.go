package main

import (
	"math"
	"sync"
	"time"

	"github.com/egoist/mygo/audio"
)

// The app's sounds (#447): a soft chime under each notice, after its kind, a warm chord when the welcome opens and
// a shorter one when the window comes. They are made in Go, once, the first time each is wanted, and kept in memory:
// no files, no programs started, the same on every system and in the browser (MyGo's audio). Settings has a switch
// for them and a Try button for each; tests hear nothing (a.noSound).

// soundKind is one of the app's sounds.
type soundKind int

const (
	soundInfo soundKind = iota
	soundOK
	soundWarn
	soundErr
	soundWelcome
	soundStartup
	soundKinds // how many there are
)

// soundRate is the rate the sounds are made at; the output turns it into the device's.
const soundRate = 48000

// soundDebounce is how soon a sound may come again: notices that come in a burst chime once, not as a roll.
const soundDebounce = 150 * time.Millisecond

// soundForToast is the sound of a notice of kind.
func soundForToast(kind toastKind) soundKind {
	switch kind {
	case toastOK:
		return soundOK
	case toastWarn:
		return soundWarn
	case toastErr:
		return soundErr
	default:
		return soundInfo
	}
}

// playSound plays kind unless sounds are off or the same sound has just been heard. The caller holds a.mu.
func (a *app) playSound(kind soundKind) {
	if a.noSound || a.settings.NoSound {
		return
	}
	now := a.clock()
	if a.soundAt == nil {
		a.soundAt = map[soundKind]time.Time{}
	}
	if prev, ok := a.soundAt[kind]; ok && now.Sub(prev) < soundDebounce {
		return
	}
	a.soundAt[kind] = now
	a.sound(kind)
}

// playStartupSound plays the chord of the window coming, from outside a frame (the window's first paint), when the
// settings ask for it (StartChord, off by default: a chord at every start tires). The welcome has its own chord, so
// with the welcome showing there is none.
func (a *app) playStartupSound() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.settings.StartChord && !a.wel.on {
		a.playSound(soundStartup)
	}
}

// playSoundPreview plays kind for Settings' Try buttons: heard even with sounds turned off, as the button asks.
func (a *app) playSoundPreview(kind soundKind) {
	if !a.noSound {
		a.sound(kind)
	}
}

// sound plays kind: to the test that listens (a.soundPlay), or out. The sound is made away from the window the first
// time, and the output mixes it in from there.
func (a *app) sound(kind soundKind) {
	if a.soundPlay != nil {
		a.soundPlay(kind)
		return
	}
	go func() {
		defer a.guard()
		soundOf(kind).Play()
	}()
}

var (
	soundsMu sync.Mutex
	sounds   [soundKinds]*audio.Sound // made when first wanted (soundOf)
)

// soundOf is kind's sound, made the first time it is asked for.
func soundOf(kind soundKind) *audio.Sound {
	soundsMu.Lock()
	defer soundsMu.Unlock()
	if sounds[kind] == nil {
		sounds[kind] = audio.NewSound(soundRate, 1, soundSamples(kind))
	}
	return sounds[kind]
}

// synthNote is one note of a sound: when it starts and how long it rings (seconds), its pitch (Hz), how loud it is,
// how slowly it comes in (seconds), and how much of the octave above sounds with it, which makes it warmer.
type synthNote struct {
	start, dur, freq, amp, attack, warmth float64
}

// soundNotes is kind's notes and how long the whole sound lasts. The notices are two notes or one, under a second:
// the good one rises, the warning and the error fall, the error lowest. The chords are long and come in slowly.
func soundNotes(kind soundKind) (total float64, notes []synthNote) {
	switch kind {
	case soundOK: // C5, then G5
		return 0.70, []synthNote{
			{0.00, 0.38, 523.25, 0.20, 0.012, 0.25},
			{0.09, 0.58, 783.99, 0.18, 0.014, 0.20},
		}
	case soundWarn: // D4, then C4
		return 0.65, []synthNote{
			{0.00, 0.28, 293.66, 0.20, 0.012, 0.30},
			{0.14, 0.45, 261.63, 0.18, 0.014, 0.30},
		}
	case soundErr: // A3, then E3
		return 0.72, []synthNote{
			{0.00, 0.32, 220.00, 0.22, 0.014, 0.35},
			{0.13, 0.55, 164.81, 0.22, 0.018, 0.35},
		}
	case soundWelcome: // Cmaj9, its notes coming one after another, a high C over it last
		return 3.60, []synthNote{
			{0.00, 3.50, 130.81, 0.16, 0.45, 0.28},
			{0.08, 3.40, 196.00, 0.13, 0.50, 0.22},
			{0.18, 3.30, 246.94, 0.11, 0.55, 0.18},
			{0.30, 3.15, 293.66, 0.10, 0.55, 0.15},
			{0.45, 2.95, 329.63, 0.09, 0.60, 0.12},
			{0.70, 2.60, 523.25, 0.06, 0.50, 0.10},
		}
	case soundStartup: // a fifth on D held, F#, A and E over it in turn
		return 2.40, []synthNote{
			{0.00, 2.30, 146.83, 0.14, 0.18, 0.25},
			{0.04, 2.25, 220.00, 0.11, 0.20, 0.20},
			{0.10, 1.80, 369.99, 0.10, 0.05, 0.15},
			{0.22, 1.75, 440.00, 0.09, 0.05, 0.12},
			{0.36, 1.85, 659.25, 0.08, 0.06, 0.10},
		}
	default: // E5
		return 0.55, []synthNote{
			{0.00, 0.52, 659.25, 0.18, 0.012, 0.20},
		}
	}
}

// soundSamples makes kind's sound. Each note comes in on half a cosine (no click), dies away exponentially, and is
// faded to nothing over its last 15% so it never stops on a step; the sum is rounded off softly (tanh) and can
// never clip.
func soundSamples(kind soundKind) []float32 {
	total, notes := soundNotes(kind)
	out := make([]float32, max(int(total*soundRate), 1))
	for i := range out {
		t := float64(i) / soundRate
		var s float64
		for _, n := range notes {
			lt := t - n.start
			if lt < 0 || lt > n.dur {
				continue
			}
			env := 1.0
			if lt < n.attack {
				env = 0.5 * (1 - math.Cos(math.Pi*lt/n.attack))
			} else {
				rest := (lt - n.attack) / max(n.dur-n.attack, 0.001)
				env = math.Exp(-4.2 * rest)
				if rest > 0.85 {
					env *= (1 - rest) / 0.15
				}
			}
			phase := 2 * math.Pi * n.freq * lt
			s += n.amp * env * (math.Sin(phase) + n.warmth*math.Sin(2*phase)) / (1 + n.warmth)
		}
		out[i] = float32(math.Tanh(s))
	}
	return out
}
