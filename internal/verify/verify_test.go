package verify

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestVerificationDiscovery(t *testing.T) {
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
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			for name, content := range tc.files {
				writeTestFile(t, filepath.Join(root, name), content)
			}
			got, err := verificationCommands(root)
			if (err != nil) != tc.wantError {
				t.Fatalf("error: %v", err)
			}
			if !tc.wantError && !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("commands %v, want %v", got, tc.want)
			}
		})
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
