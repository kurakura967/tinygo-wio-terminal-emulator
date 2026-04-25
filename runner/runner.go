package runner

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"strings"
	"text/template"
)

const emulatorModule = "github.com/kurakura967/tinygo-wio-terminal-emulator"

// importReplacements maps original TinyGo import paths to emulator stubs.
var importReplacements = map[string]string{
	"machine":                       emulatorModule + "/machine",
	"tinygo.org/x/drivers/ili9341": emulatorModule + "/driver/ili9341",
}

// Run rewrites imports in filePath, generates a temporary project, and
// executes it with `go run`.
func Run(filePath string) error {
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return fmt.Errorf("resolving path: %w", err)
	}

	src, err := os.ReadFile(absPath)
	if err != nil {
		return fmt.Errorf("reading file: %w", err)
	}

	rewritten, err := rewriteImportsAndRenameMain(src)
	if err != nil {
		return fmt.Errorf("rewriting source: %w", err)
	}

	moduleRoot, err := findModuleRoot()
	if err != nil {
		return fmt.Errorf("finding emulator module root: %w", err)
	}

	tmpDir, err := os.MkdirTemp("", "wio-emu-*")
	if err != nil {
		return fmt.Errorf("creating temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	if err := writeUserFile(tmpDir, rewritten); err != nil {
		return err
	}
	if err := writeEntrypoint(tmpDir); err != nil {
		return err
	}
	if err := writeGoMod(tmpDir, moduleRoot); err != nil {
		return err
	}
	if err := runGoModTidy(tmpDir); err != nil {
		return err
	}

	binPath := filepath.Join(tmpDir, "wio-emu-app")
	if err := runGoBuild(tmpDir, binPath); err != nil {
		return err
	}

	return runBinary(binPath)
}

// rewriteImportsAndRenameMain parses src, replaces known import paths,
// and renames func main() to func userMain().
func rewriteImportsAndRenameMain(src []byte) ([]byte, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "user.go", src, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	for _, imp := range f.Imports {
		path, _ := strconv.Unquote(imp.Path.Value)
		if replacement, ok := importReplacements[path]; ok {
			imp.Path.Value = strconv.Quote(replacement)
		}
	}

	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if ok && fn.Name.Name == "main" {
			fn.Name.Name = "userMain"
		}
	}

	var buf bytes.Buffer
	if err := format.Node(&buf, fset, f); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func writeUserFile(dir string, src []byte) error {
	return os.WriteFile(filepath.Join(dir, "user.go"), src, 0644)
}

const entrypointTmpl = `package main

import "{{ .Module }}/emulator"

func main() {
	go userMain()
	emulator.Run()
}
`

func writeEntrypoint(dir string) error {
	tmpl := template.Must(template.New("ep").Parse(entrypointTmpl))
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, map[string]string{"Module": emulatorModule}); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "main.go"), buf.Bytes(), 0644)
}

const goModTmpl = `module wio-emu-tmp

go 1.23

require {{ .Module }} v0.0.0

replace {{ .Module }} => {{ .LocalPath }}
`

func writeGoMod(dir, moduleRoot string) error {
	tmpl := template.Must(template.New("mod").Parse(goModTmpl))
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, map[string]string{
		"Module":    emulatorModule,
		"LocalPath": moduleRoot,
	}); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "go.mod"), buf.Bytes(), 0644)
}

// findModuleRoot returns the emulator module root directory.
// It first searches upward from CWD (works for `go run .` in development),
// then falls back to the Go module cache (works for `go install`).
func findModuleRoot() (string, error) {
	// 1. Walk up from CWD (development mode).
	if root, err := findModuleRootFromCWD(); err == nil {
		return root, nil
	}

	// 2. Module cache (installed via go install).
	return findModuleRootInCache()
}

func findModuleRootFromCWD() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	dir := cwd
	for {
		candidate := filepath.Join(dir, "go.mod")
		if data, err := os.ReadFile(candidate); err == nil {
			if strings.Contains(string(data), emulatorModule) {
				return dir, nil
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("emulator go.mod not found from %s", cwd)
}

func findModuleRootInCache() (string, error) {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "", fmt.Errorf("build info not available")
	}
	version := info.Main.Version
	suffix := emulatorModule + "@" + version

	for _, cache := range modCacheCandidates() {
		p := filepath.Join(cache, suffix)
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("module not found in any cache: %s", suffix)
}

// modCacheCandidates returns possible GOMODCACHE directories to search.
func modCacheCandidates() []string {
	seen := map[string]bool{}
	var caches []string
	add := func(p string) {
		if p != "" && !seen[p] {
			seen[p] = true
			caches = append(caches, p)
		}
	}

	// 1. GOMODCACHE env var (explicitly set).
	add(os.Getenv("GOMODCACHE"))

	// 2. Derive from executable path: goenv installs to $GOPATH/bin/,
	//    so $GOPATH = parent of bin dir, and GOMODCACHE = $GOPATH/pkg/mod.
	if exe, err := os.Executable(); err == nil {
		binDir := filepath.Dir(exe)
		gopath := filepath.Dir(binDir)
		add(filepath.Join(gopath, "pkg", "mod"))
	}

	// 3. Default $HOME/go/pkg/mod.
	if home, err := os.UserHomeDir(); err == nil {
		add(filepath.Join(home, "go", "pkg", "mod"))
	}

	// 4. go binary in PATH.
	if out, err := exec.Command("go", "env", "GOMODCACHE").Output(); err == nil {
		add(strings.TrimSpace(string(out)))
	}

	return caches
}

// goBinary returns the path to the go binary that matches the running
// executable. When installed via `go install`, this ensures we use the same
// Go version that compiled the binary, regardless of goenv's active version.
func goBinary() string {
	if exe, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(exe), "go")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return "go"
}

func runGoModTidy(dir string) error {
	cmd := exec.Command(goBinary(), "mod", "tidy")
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func runGoBuild(dir, outPath string) error {
	cmd := exec.Command(goBinary(), "build", "-o", outPath, ".")
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func runBinary(path string) error {
	cmd := exec.Command(path)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
