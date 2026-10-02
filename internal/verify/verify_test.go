package verify

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"vicoha/internal/harness"
)

func TestMain(m *testing.M) {
	if os.Getenv("VICOHA_VERIFY_FIXTURE") == "1" {
		_ = json.NewEncoder(os.Stdout).Encode(os.Args[1:])
		return
	}
	os.Exit(m.Run())
}

func TestVerificationDiscovery(t *testing.T) {
	gradle, maven := "./gradlew", "./mvnw"
	if runtime.GOOS == "windows" {
		gradle, maven = "./gradlew.bat", "./mvnw.cmd"
	}
	for _, tc := range []struct {
		name      string
		files     map[string]string
		want      [][]string
		wantError bool
	}{
		{"empty", nil, nil, true},
		{"broken package", map[string]string{"package.json": "{"}, nil, true},
		{"declared scripts", map[string]string{"package.json": `{"scripts":{"lint":"eslint .","build":"tsc","dev":"server"}}`}, [][]string{{"npm", "run", "build"}, {"npm", "run", "lint"}}, false},
		{"no pytest assumption", map[string]string{"pyproject.toml": "[project]\nname='fixture'", "tests/test_example.py": ""}, nil, true},
		{"configured pytest", map[string]string{"pyproject.toml": "[tool.pytest.ini_options] # configured\ntestpaths=['tests']"}, [][]string{{"python", "-m", "pytest"}}, false},
		{"pytest ini", map[string]string{"pytest.ini": "[pytest]"}, [][]string{{"python", "-m", "pytest"}}, false},
		{"all npm scripts", map[string]string{"package.json": `{"scripts":{"test":"test","build":"build","lint":"lint","typecheck":"check"}}`}, [][]string{{"npm", "run", "test"}, {"npm", "run", "build"}, {"npm", "run", "lint"}, {"npm", "run", "typecheck"}}, false},
		{"go config cannot hide broken package", map[string]string{"go.mod": "module fixture", "package.json": "{"}, nil, true},
		{"make precedence", map[string]string{"go.mod": "module fixture", "Makefile": "test:\n\tgo test ./...\nverify: test\n"}, [][]string{{"make", "verify"}}, false},
		{"make CRLF", map[string]string{"Makefile": "test:\r\n\ttool test\r\n"}, [][]string{{"make", "test"}}, false},
		{"make assignment is not target", map[string]string{"Makefile": "test: = something\n"}, nil, true},
		{"task variables are not tasks", map[string]string{"Taskfile.yml": "vars:\n  test:\n    value: x\n"}, nil, true},
		{"just", map[string]string{"justfile": "check:\n  tool test\n"}, [][]string{{"just", "check"}}, false},
		{"task", map[string]string{"Taskfile.yml": "version: '3'\ntasks:\n  test:\n    cmds: [tool test]\n"}, [][]string{{"task", "test"}}, false},
		{"cargo", map[string]string{"Cargo.toml": "[package]"}, [][]string{{"cargo", "test"}}, false},
		{"gradle wrapper", map[string]string{"build.gradle.kts": "", "gradlew": "", "gradlew.bat": ""}, [][]string{{gradle, "test"}}, false},
		{"maven wrapper", map[string]string{"pom.xml": "", "mvnw": "", "mvnw.cmd": ""}, [][]string{{maven, "test"}}, false},
		{"pnpm", map[string]string{"package.json": `{"scripts":{"check":"custom"}}`, "pnpm-lock.yaml": ""}, [][]string{{"pnpm", "run", "check"}}, false},
		{"manager declaration", map[string]string{"package.json": `{"packageManager":"yarn@4.0.0","scripts":{"test":"custom"}}`, "package-lock.json": "{}"}, [][]string{{"yarn", "run", "test"}}, false},
		{"documentation overrides default", map[string]string{"go.mod": "module fixture", "README.md": "Use `custom verify`."}, nil, true},
		{"CI needs interpretation", map[string]string{"go.mod": "module fixture", ".github/workflows/test.yml": "run: custom verify"}, nil, true},
		{"custom tool", map[string]string{"CMakeLists.txt": "enable_testing()"}, nil, true},
		{"npm placeholder", map[string]string{"package.json": `{"scripts":{"test":"echo no test specified"}}`}, nil, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			for name, content := range tc.files {
				writeTestFile(t, filepath.Join(root, name), content)
			}
			discovered, err := Discover(root)
			if err == nil && !discovered.Confident {
				err = fmt.Errorf("insufficient discovery")
			}
			var got [][]string
			for _, command := range discovered.Commands {
				got = append(got, command.Argv)
			}
			if (err != nil) != tc.wantError {
				t.Fatalf("error: %v", err)
			}
			if !tc.wantError && !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("commands %v, want %v", got, tc.want)
			}
		})
	}
}

func TestValidationAndArgvExecution(t *testing.T) {
	root := t.TempDir()
	for _, argv := range [][]string{
		nil, {"sudo", "make", "test"}, {"rm", "-rf", "."}, {"shutdown"}, {"npm", "install"}, {"npm", "ci"}, {"go", "get", "example.org/x"}, {"curl", "example.org"}, {"git", "reset", "--hard"}, {"git", "clean", "-fd"}, {"sh", "-c", "true"}, {"cmd.exe", "/c", "echo ok"}, {"powershell", "-Command", "echo ok"}, {"python", "-cprint(1)"}, {"node", "--eval=bad"}, {"python", "-m", "pip"}, {"tool", "--output=../outside"}, {"tool", "-o../outside"}, {"tool", "C:\\outside"}, {"tool", "https://example.org"},
	} {
		if err := Validate(root, argv); err == nil {
			t.Errorf("accepted unsafe argv %q", argv)
		}
	}
	writeTestFile(t, filepath.Join(root, "README.md"), "tool tests")
	if _, err := FromProposals(root, []harness.Proposal{{Argv: []string{"go", "test", "./..."}, Evidence: []string{"missing.md"}}}); err == nil {
		t.Fatal("accepted nonexistent evidence")
	}
	if _, err := FromProposals(root, []harness.Proposal{{Argv: []string{"go", "test", "./..."}, Evidence: []string{"../README.md"}}}); err == nil {
		t.Fatal("accepted outside evidence")
	}
	exe, _ := os.Executable()
	data, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	name := "fixture"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	if err := os.WriteFile(filepath.Join(root, name), data, 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("VICOHA_VERIFY_FIXTURE", "1")
	argv := []string{"./" + name, "literal;echo injected", "$(echo injected)", "a & b", "space value"}
	checks, err := Execute(context.Background(), root, []Command{{Argv: argv, Source: "ai", Evidence: []string{"README.md"}}})
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	if err := json.Unmarshal([]byte(checks[0].Output), &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, argv[1:]) || checks[0].Source != "ai" || checks[0].ExitCode == nil || *checks[0].ExitCode != 0 || checks[0].DurationMS < 0 {
		t.Fatalf("incorrect execution: %+v", checks)
	}
	checks, err = Execute(context.Background(), root, []Command{{Argv: argv}, {Argv: []string{"rm", "file"}}})
	if err == nil || len(checks) != 0 {
		t.Fatal("executed before validating entire selection")
	}
	if runtime.GOOS == "windows" {
		checks, err = Execute(context.Background(), root, []Command{{Argv: argv}, {Argv: []string{"npm", "run", "test&echo"}}})
		if err == nil || len(checks) != 0 {
			t.Fatal("executed before validating Windows shim arguments")
		}
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "outside")); err == nil {
		if err := Validate(root, []string{"tool", "outside/output"}); err == nil {
			t.Fatal("accepted external symlink output")
		}
	}
}

func TestDiscoveryEvidenceAndProvenance(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "package.json"), `{"scripts":{"test":"existing"}}`)
	writeTestFile(t, filepath.Join(root, "README.md"), "Use `npm run test`.")
	d, err := Discover(root)
	if err != nil || d.Confident || !strings.Contains(d.Evidence, "package.json") {
		t.Fatalf("discovery=%+v error=%v", d, err)
	}
	commands, err := FromProposals(root, []harness.Proposal{{Argv: []string{"npm", "run", "test"}, Evidence: []string{"package.json"}}})
	if err != nil {
		t.Fatal(err)
	}
	MarkDeclared(commands, d.Commands)
	if !commands[0].Declared || commands[0].Source != "ai" {
		t.Fatal(commands)
	}
}

func TestVerificationFailure(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "go.mod"), "module fixture\n\ngo 1.22\n")
	writeTestFile(t, filepath.Join(root, "main.go"), "this cannot compile")
	checks, err := Run(root)
	if err == nil || len(checks) != 1 || checks[0].Status != "failed" || checks[0].Output == "" {
		t.Fatalf("checks=%+v, error=%v", checks, err)
	}
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
}
