package main

import (
	"fmt"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// TestAccessNames: in each of the main views - a page, the palette, the find bar, a collection, the search, the
// tasks, the trash, every tab of the settings, a tag and the statistics - every control assistive technology is
// told of has a name, and nothing takes the focus without one (#362).
func TestAccessNames(t *testing.T) {
	a, tt := newTestApp(t, "# Head\n\nsome #tag text\n\n- [ ] a task\n")
	tt.SetSize(1200, 800)
	tt.Frame()
	named := map[ui.Role]bool{ui.RoleButton: true, ui.RoleLink: true, ui.RoleCheckBox: true, ui.RoleRadio: true,
		ui.RoleSwitch: true, ui.RoleSlider: true, ui.RoleTextField: true, ui.RoleImage: true, ui.RolePopUpButton: true,
		ui.RoleTab: true, ui.RoleMenuButton: true, ui.RoleToggleButton: true, ui.RoleComboBox: true,
		ui.RoleDisclosure: true, ui.RoleMenuItem: true, ui.RoleTreeItem: true, ui.RoleDialog: true,
		ui.RoleSplitter: true, ui.RoleStepper: true, ui.RoleColorWell: true, ui.RoleProgress: true, ui.RoleMeter: true}
	check := func(view string) {
		t.Helper()
		tt.Advance(400 * time.Millisecond)
		tt.Frame()
		all := tt.AccessAll()
		if len(all) < 8 {
			t.Errorf("%s: %d elements: the view is not there", view, len(all))
		}
		for _, x := range all {
			bare := x.Focusable && (x.Role == ui.RoleGroup || x.Role == ui.RoleNone || x.Role == ui.RoleAuto)
			if (named[x.Role] || bare) && x.Name == "" {
				t.Errorf("%s: an element of role %d at %v has no name (value %q)", view, x.Role, x.Bounds, x.Value)
			}
		}
	}
	check("page")
	a.openPalette()
	check("palette")
	a.pal.open = false
	a.openFind()
	check("find")
	a.openCollection(a.lib.Collections[0])
	check("collection")
	a.openSearch("text")
	check("search")
	a.openTasks()
	check("tasks")
	a.openTrash()
	check("trash")
	a.openSettings()
	for i := range settingsSections {
		a.settingsTab = i
		check(fmt.Sprint("settings tab ", i))
	}
	a.openTag("tag")
	check("tag")
	a.openStats()
	check("statistics")
}
