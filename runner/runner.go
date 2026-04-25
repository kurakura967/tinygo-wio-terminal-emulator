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
	"strconv"
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

// findModuleRoot walks up from the working directory to find go.mod.
func findModuleRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	dir := cwd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("go.mod not found from %s", cwd)
}

func runGoModTidy(dir string) error {
	cmd := exec.Command("go", "mod", "tidy")
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func runGoBuild(dir, outPath string) error {
	cmd := exec.Command("go", "build", "-o", outPath, ".")
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
