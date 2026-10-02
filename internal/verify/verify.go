package verify

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"vicoha/internal/result"
)

// Run selects checks from configuration files actually present in the project.
func Run(root string) ([]result.Verification, error) {
	commands, err := verificationCommands(root)
	if err != nil {
		return nil, err
	}
	var checks []result.Verification
	for _, args := range commands {
		cmd := exec.Command(args[0], args[1:]...)
		// npm is distributed as npm.cmd on Windows, which requires cmd.exe.
		if runtime.GOOS == "windows" && args[0] == "npm" {
			cmd = exec.Command("cmd.exe", append([]string{"/d", "/c"}, args...)...)
		}
		cmd.Dir = root
		out, err := cmd.CombinedOutput()
		status := "passed"
		if err != nil {
			status = "failed"
		}
		checks = append(checks, result.Verification{Command: strings.Join(args, " "), Status: status, Output: string(out)})
		if err != nil {
			return checks, fmt.Errorf("verification %q failed: %w\n%s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
		}
	}
	return checks, nil
}

func verificationCommands(root string) ([][]string, error) {
	var commands [][]string
	goProject, err := configExists(filepath.Join(root, "go.mod"))
	if err != nil {
		return nil, err
	}
	if goProject {
		commands = append(commands, []string{"go", "test", "./..."}, []string{"go", "build", "./..."})
	}
	b, err := os.ReadFile(filepath.Join(root, "package.json"))
	if err == nil {
		var data struct {
			Scripts map[string]string `json:"scripts"`
		}
		if err := json.Unmarshal(b, &data); err != nil {
			return nil, fmt.Errorf("read package.json: %w", err)
		}
		// Only invoke package scripts explicitly declared by the project.
		for _, name := range []string{"test", "build", "lint", "typecheck"} {
			if strings.TrimSpace(data.Scripts[name]) != "" {
				commands = append(commands, []string{"npm", "run", name})
			}
		}
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("read package.json: %w", err)
	}
	pytest, err := configExists(filepath.Join(root, "pytest.ini"))
	if err != nil {
		return nil, err
	}
	b, err = os.ReadFile(filepath.Join(root, "pyproject.toml"))
	if err == nil {
		for _, line := range strings.Split(string(b), "\n") {
			line = strings.TrimSpace(strings.SplitN(line, "#", 2)[0])
			if line == "[tool.pytest.ini_options]" {
				pytest = true
			}
		}
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("read pyproject.toml: %w", err)
	}
	if pytest {
		commands = append(commands, []string{"python", "-m", "pytest"})
	}
	if len(commands) == 0 {
		return nil, fmt.Errorf("no supported deterministic verification commands configured")
	}
	return commands, nil
}

func configExists(path string) (bool, error) {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("inspect configuration %s: %w", path, err)
	}
	if info.IsDir() {
		return false, fmt.Errorf("configuration %s is a directory", path)
	}
	return true, nil
}
