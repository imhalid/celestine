// Command notices writes the notices of the third-party software compiled into the app, as Markdown: each module
// the builds link (macOS, Linux, Windows and the browser's), with its licence's text as the module ships it. The
// release packs it beside the program (THIRD_PARTY_NOTICES.md): MIT, BSD and Apache-2.0 ask that their notice go
// with the binaries (#256). NOTICE.md, by hand, holds what is drawn after other works without their code.
//
//	go run ./scripts/notices > dist/THIRD_PARTY_NOTICES.md
package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
)

// targets are the builds whose modules are listed.
var targets = [][2]string{{"darwin", "arm64"}, {"darwin", "amd64"}, {"linux", "amd64"}, {"linux", "arm64"}, {"windows", "amd64"}, {"js", "wasm"}}

type module struct{ path, version, dir string }

func main() {
	mods := map[string]module{}
	for _, t := range targets {
		cmd := exec.Command("go", "list", "-deps", "-f", "{{with .Module}}{{if not .Main}}{{with .Replace}}{{.Path}} {{.Version}} {{.Dir}}{{else}}{{.Path}} {{.Version}} {{.Dir}}{{end}}{{end}}{{end}}", ".")
		cmd.Env = append(os.Environ(), "GOOS="+t[0], "GOARCH="+t[1], "CGO_ENABLED=0")
		cmd.Stderr = os.Stderr
		out, err := cmd.Output()
		if err != nil {
			fmt.Fprintln(os.Stderr, "go list", t, err)
			os.Exit(1)
		}
		sc := bufio.NewScanner(bytes.NewReader(out))
		for sc.Scan() {
			f := strings.Fields(sc.Text())
			if len(f) == 3 {
				mods[f[0]] = module{f[0], f[1], f[2]}
			}
		}
	}
	paths := make([]string, 0, len(mods))
	for p := range mods {
		paths = append(paths, p)
	}
	slices.Sort(paths)
	w := bufio.NewWriter(os.Stdout)
	defer w.Flush()
	fmt.Fprintln(w, "# Third-party notices")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Celestine includes the software below, each under its own licence, whose text follows. Celestine itself is")
	fmt.Fprintln(w, "under its own licence (LICENSE).")
	// Go's own runtime and standard library, in every build: its licence in GOROOT, or where a system's package of Go
	// puts it (Fedora's)
	missing := 0
	root, _ := exec.Command("go", "env", "GOROOT").Output()
	goLicence := ""
	for _, f := range []string{filepath.Join(strings.TrimSpace(string(root)), "LICENSE"), "/usr/share/licenses/golang/LICENSE"} {
		if b, err := os.ReadFile(f); err == nil {
			goLicence = string(b)
			break
		}
	}
	if goLicence == "" {
		fmt.Fprintln(os.Stderr, "no licence of Go found")
		missing++
	}
	fmt.Fprintf(w, "\n## Go (its runtime and standard library)\n\n```\n%s\n```\n", strings.TrimRight(goLicence, "\n"))
	for _, p := range paths {
		m := mods[p]
		fmt.Fprintf(w, "\n## %s %s\n\n", m.path, m.version)
		texts := licenceTexts(m.dir)
		if len(texts) == 0 {
			fmt.Fprintln(os.Stderr, "no licence file in", m.path)
			missing++
			fmt.Fprintln(w, "(no licence file in the module)")
			continue
		}
		for _, t := range texts {
			fmt.Fprintf(w, "```\n%s\n```\n", strings.TrimRight(t, "\n"))
		}
	}
	if missing > 0 {
		w.Flush()
		os.Exit(1)
	}
}

// licenceTexts are the texts of the licence and notice files at the top of a module's folder.
func licenceTexts(dir string) []string {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range ents {
		n := strings.ToUpper(e.Name())
		if e.IsDir() || !(strings.HasPrefix(n, "LICENSE") || strings.HasPrefix(n, "LICENCE") || strings.HasPrefix(n, "COPYING") || strings.HasPrefix(n, "NOTICE")) {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err == nil {
			out = append(out, strings.ReplaceAll(string(b), "```", "'''"))
		}
	}
	return out
}
