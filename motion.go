package main

import (
	"github.com/egoist/mygo/ui"
	"github.com/imhalid/celestine/locales"
	ryugo "github.com/imhalid/ryu-go"
)

// Motion, and what can be turned off of it (#397, #399). The app follows the system's Reduce Motion (MyGo's
// Preferences.ReduceMotion) and a setting of its own: with either, every value that animates goes to its target at once
// (Context.DisableAnimations), and the app's own timed motion - its shaders, the caret's effects, the update card and
// chip, the wave after an update, the welcome's opening, a task's confetti - is skipped or shown still. Shader effects
// can also be turned off alone, for a slow computer: each of them then has a plain way to show. Nothing waits on a
// motion that does not run.

// fx is how the frame being drawn moves: read by views and by painters, which have no app at hand. It is set at the
// start of each frame, under the frame's lock (motionFrame).
var fx struct {
	still     bool // reduce motion: the system's, or the app's setting
	noShaders bool // the shader effects are off
}

// shadersOn reports whether this frame draws the app's shader effects: not when they are turned off, and not with
// reduced motion, as each of them moves.
func shadersOn() bool { return !fx.noShaders && !fx.still }

// motionFrame sets how this frame moves, from the settings and the system's, and with reduced motion makes the values
// that animate in it go to their targets at once.
func (a *app) motionFrame(c *ui.Context) {
	fx.still = a.settings.ReduceMotion || c.Preferences().ReduceMotion
	fx.noShaders = a.settings.NoShaders
	if fx.still {
		c.DisableAnimations()
	}
}

// settingsMotion is the appearance's section of motion: reduce motion, and the shader effects.
func (a *app) settingsMotion(c *ui.Context) {
	still := a.settings.ReduceMotion
	effects := !a.settings.NoShaders
	settingGroup(c, "set-motion",
		func() {
			settingRow(c, locales.SettingsReduceMotion(), locales.SettingsReduceMotionHelp(), func() {
				ryugo.Switch(c.Key("set-motion-still"), &still, ryugo.SwitchOptions{}).Label(locales.SettingsReduceMotion())
			})
		},
		func() {
			settingRow(c, locales.SettingsShaderEffects(), locales.SettingsShaderEffectsHelp(), func() {
				ryugo.Switch(c.Key("set-motion-shaders"), &effects, ryugo.SwitchOptions{}).Label(locales.SettingsShaderEffects())
			})
		})
	if still != a.settings.ReduceMotion {
		a.settings.ReduceMotion = still
		a.saveSettings()
	}
	if effects == a.settings.NoShaders {
		a.settings.NoShaders = !effects
		a.saveSettings()
	}
}

// stillNow reports whether this frame moves as little as it can: where a name of its own reads better than fx.still,
// as in code that has its own fx at hand.
func stillNow() bool { return fx.still }
