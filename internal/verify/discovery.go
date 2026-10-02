package verify

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"vicoha/internal/harness"
)

type Discovery struct {
	Commands  []Command
	Confident bool
	Evidence  string
}

// Discover recognizes a few explicit entrypoints and conventional defaults.
// Unparsed build configuration, CI or documented commands require discovery by
// an agent rather than assuming a language marker fully describes verification.
func Discover(root string) (Discovery, error) {
	d := Discovery{}
	files := map[string]string{}
	names := []string{"README.md", "AGENTS.md", "Makefile", "makefile", "GNUmakefile", "Taskfile.yml", "Taskfile.yaml", "justfile", "Justfile", "package.json", "package-lock.json", "npm-shrinkwrap.json", "pnpm-lock.yaml", "yarn.lock", "bun.lock", "bun.lockb", "go.mod", "go.work", "Cargo.toml", "Cargo.lock", "gradlew", "gradlew.bat", "mvnw", "mvnw.cmd", "build.gradle", "build.gradle.kts", "settings.gradle", "settings.gradle.kts", "pom.xml", "pyproject.toml", "pytest.ini", "tox.ini", "CMakeLists.txt", "pubspec.yaml", "pubspec.lock", "uv.lock", "poetry.lock"}
	workflows, err := filepath.Glob(filepath.Join(root, ".github", "workflows", "*"))
	if err != nil {
		return d, err
	}
	for _, path := range workflows {
		name, _ := filepath.Rel(root, path)
		names = append(names, filepath.ToSlash(name))
	}
	for _, name := range names {
		path := filepath.Join(root, filepath.FromSlash(name))
		if _, err := os.Lstat(path); os.IsNotExist(err) {
			continue
		} else if err != nil {
			return d, err
		}
		if err := repositoryPath(root, name, true); err != nil {
			return d, err
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return d, fmt.Errorf("read verification evidence %s: %w", name, err)
		}
		files[name] = string(b)
		// Lockfile contents are unnecessary for choosing the package manager.
		content := string(b)
		if strings.Contains(name, "lock") || name == "npm-shrinkwrap.json" {
			content = "(lockfile present)"
		}
		if len(content) > 32768 {
			content = content[:32768] + "\n(truncated; inspect file if needed)"
		}
		d.Evidence += "--- " + name + " ---\n" + content + "\n\n"
	}
	has := func(name string) bool { _, ok := files[name]; return ok }
	ambiguousInstructions := len(workflows) > 0
	verificationWords := regexp.MustCompile(`(?i)\b(test|tests|testing|verify|verification|check|checks|build|lint|typecheck)\b`)
	for _, name := range []string{"README.md", "AGENTS.md"} {
		text := files[name]
		if verificationWords.MatchString(text) && (strings.Contains(text, "`") || strings.Contains(strings.ToLower(text), "run ") || strings.Contains(strings.ToLower(text), "verification:")) {
			ambiguousInstructions = true
		}
	}
	add := func(declared bool, evidence string, argv ...string) {
		d.Commands = append(d.Commands, Command{Argv: argv, Source: "deterministic", Declared: declared, Evidence: []string{evidence}})
	}
	var pkg struct {
		Scripts        map[string]string `json:"scripts"`
		PackageManager string            `json:"packageManager"`
	}
	if has("package.json") {
		if err := json.Unmarshal([]byte(files["package.json"]), &pkg); err != nil {
			return d, fmt.Errorf("read package.json: %w", err)
		}
	}
	makeSeen := false
	for _, name := range []string{"GNUmakefile", "makefile", "Makefile", "justfile", "Justfile", "Taskfile.yml", "Taskfile.yaml"} {
		config := files[name]
		if strings.EqualFold(name, "makefile") || name == "GNUmakefile" {
			if makeSeen {
				continue
			}
			makeSeen = has(name)
		}
		if strings.HasPrefix(name, "Taskfile") {
			section := regexp.MustCompile(`(?m)^tasks:[ \t]*(?:#[^\r\n]*)?\r?\n((?:[ \t]+[^\n]*(?:\n|$)|\r?\n)*)`).FindStringSubmatch(config)
			if len(section) != 2 {
				continue
			}
			config = section[1]
		}
		for _, target := range []string{"verify", "check", "test"} {
			pattern := `(?m)^` + target + `[ \t]*:[ \t]*(?:[^= \t\r\n]|\r?$)`
			tool := "make"
			if strings.EqualFold(name, "justfile") {
				tool = "just"
			}
			if strings.HasPrefix(name, "Taskfile") {
				tool = "task"
				pattern = `(?m)^  ` + target + `:\s*(?:#.*)?$`
			}
			if regexp.MustCompile(pattern).MatchString(config) {
				add(true, name, tool, target)
				d.Confident = !ambiguousInstructions
				return d, nil
			}
		}
	}
	if has("package.json") {
		manager := "npm"
		if has("pnpm-lock.yaml") {
			manager = "pnpm"
		} else if has("yarn.lock") {
			manager = "yarn"
		} else if has("bun.lock") || has("bun.lockb") {
			manager = "bun"
		}
		if pkg.PackageManager != "" {
			manager = strings.Split(pkg.PackageManager, "@")[0]
			if manager != "npm" && manager != "pnpm" && manager != "yarn" && manager != "bun" {
				return d, nil
			}
		}
		for _, target := range []string{"verify", "check"} {
			if strings.TrimSpace(pkg.Scripts[target]) != "" {
				add(true, "package.json", manager, "run", target)
				d.Confident = !ambiguousInstructions && !has("go.mod") && !has("Cargo.toml") && !has("pyproject.toml") && !has("pom.xml") && !has("build.gradle") && !has("build.gradle.kts")
				return d, nil
			}
		}
		for _, target := range []string{"test", "build", "lint", "typecheck"} {
			if script := strings.TrimSpace(pkg.Scripts[target]); script != "" && !strings.Contains(script, "no test specified") {
				add(true, "package.json", manager, "run", target)
			}
		}
	}
	if has("go.mod") {
		add(false, "go.mod", "go", "test", "./...")
		add(false, "go.mod", "go", "build", "./...")
	}
	if has("Cargo.toml") {
		add(false, "Cargo.toml", "cargo", "test")
	}
	if has("gradlew") && (has("build.gradle") || has("build.gradle.kts")) {
		wrapper := "./gradlew"
		if runtime.GOOS == "windows" {
			wrapper = "./gradlew.bat"
		}
		if has(strings.TrimPrefix(wrapper, "./")) {
			add(false, strings.TrimPrefix(wrapper, "./"), wrapper, "test")
		}
	}
	if has("mvnw") && has("pom.xml") {
		wrapper := "./mvnw"
		if runtime.GOOS == "windows" {
			wrapper = "./mvnw.cmd"
		}
		if has(strings.TrimPrefix(wrapper, "./")) {
			add(false, strings.TrimPrefix(wrapper, "./"), wrapper, "test")
		}
	}
	pytest := has("pytest.ini") || regexp.MustCompile(`(?m)^\s*\[tool\.pytest\.ini_options\]\s*(?:#.*)?$`).MatchString(files["pyproject.toml"])
	if pytest {
		evidence := "pyproject.toml"
		if has("pytest.ini") {
			evidence = "pytest.ini"
		}
		add(true, evidence, "python", "-m", "pytest")
	}
	d.Confident = len(d.Commands) > 0
	for _, name := range []string{"Makefile", "makefile", "GNUmakefile", "Taskfile.yml", "Taskfile.yaml", "justfile", "Justfile", "tox.ini", "CMakeLists.txt", "pubspec.yaml", "go.work", "uv.lock", "poetry.lock"} {
		if has(name) {
			d.Confident = false
		}
	}
	if ambiguousInstructions || has("pyproject.toml") && !pytest || has("pom.xml") && !has("mvnw") || (has("build.gradle") || has("build.gradle.kts")) && !has("gradlew") {
		d.Confident = false
	}
	return d, nil
}

func FromProposals(root string, proposals []harness.Proposal) ([]Command, error) {
	commands := make([]Command, 0, len(proposals))
	for _, p := range proposals {
		for _, path := range p.Evidence {
			if err := repositoryPath(root, path, true); err != nil {
				return nil, fmt.Errorf("invalid evidence: %w", err)
			}
		}
		if err := Validate(root, p.Argv); err != nil {
			return nil, err
		}
		commands = append(commands, Command{Argv: p.Argv, Source: "ai", Evidence: p.Evidence})
	}
	return commands, nil
}

// Mark known repository-declared commands even when selected by AI.
func MarkDeclared(commands, known []Command) {
	for i := range commands {
		for _, k := range known {
			if k.Declared && strings.Join(k.Argv, "\x00") == strings.Join(commands[i].Argv, "\x00") {
				commands[i].Declared = true
			}
		}
	}
}
