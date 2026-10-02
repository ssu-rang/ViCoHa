package verify

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Validate guards against accidental dangerous proposals, not malicious agents.
// Repository scripts and build tools remain trusted executable code.
func Validate(root string, argv []string) error {
	if len(argv) == 0 {
		return fmt.Errorf("verification argv must not be empty")
	}
	name := strings.TrimSuffix(strings.ToLower(filepath.Base(strings.ReplaceAll(argv[0], `\`, "/"))), ".exe")
	switch name {
	case "sudo", "doas", "su", "rm", "rmdir", "del", "erase", "remove-item", "mv", "move", "shutdown", "reboot", "halt", "format", "mkfs", "dd", "curl", "wget", "ssh", "scp", "sftp", "ftp", "rsync", "ping", "nc", "ncat", "netcat", "telnet", "git", "bash", "sh", "zsh", "fish", "cmd", "powershell", "pwsh", "env", "xargs", "npx", "pip", "pip3", "apt", "apt-get", "brew", "choco", "winget", "docker", "podman":
		return fmt.Errorf("unsafe verification executable %q", argv[0])
	}
	if strings.ContainsAny(argv[0], `/\`) {
		if err := repositoryPath(root, argv[0], true); err != nil {
			return err
		}
	}
	for i, arg := range argv {
		if windowsShim(argv[0]) && strings.IndexFunc(arg, func(r rune) bool {
			return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune("-._/:=", r))
		}) >= 0 {
			return fmt.Errorf("unsafe Windows shim argument %q", arg)
		}
		if strings.TrimSpace(arg) == "" || strings.ContainsAny(arg, "\x00\r\n") {
			return fmt.Errorf("invalid verification argument")
		}
		lower := strings.ToLower(arg)
		flag, _, _ := strings.Cut(lower, "=")
		if flag == "--eval" || flag == "--print" || flag == "--exec" || flag == "-exec" || flag == "--require" || flag == "--import" || flag == "--init-script" {
			return fmt.Errorf("unsafe verification argument %q", arg)
		}
		if strings.Contains(lower, "://") {
			return fmt.Errorf("network URL in verification command")
		}
		if i > 0 {
			if (name == "npm" || name == "pnpm" || name == "yarn" || name == "bun") && i == 1 && (lower == "i" || lower == "dlx" || lower == "x" || lower == "install-test" || lower == "install-ci-test") {
				return fmt.Errorf("package installation is not verification")
			}
			if name == "uv" && lower == "sync" {
				return fmt.Errorf("dependency synchronization is not verification")
			}
			if name == "node" && (strings.HasPrefix(lower, "-p") || strings.HasPrefix(lower, "-r")) {
				return fmt.Errorf("inline code is not a verification entrypoint")
			}
			if strings.HasPrefix(name, "python") && strings.HasPrefix(lower, "-m") && lower != "-m" {
				return fmt.Errorf("use separate Python module arguments")
			}
			switch lower {
			case "install", "ci", "get", "add", "update", "upgrade", "publish", "deploy", "download", "fetch", "push", "pull", "clone", "exec", "-c", "-e", "--eval", "--command", "-command", "--exec", "-exec", "--require", "--import", "--init-script":
				return fmt.Errorf("unsafe verification argument %q", arg)
			}
			if (strings.HasPrefix(name, "python") || name == "node" || name == "ruby" || name == "perl") && (strings.HasPrefix(lower, "-c") || strings.HasPrefix(lower, "-e")) {
				return fmt.Errorf("inline code is not a verification entrypoint")
			}
			if strings.HasPrefix(name, "python") && lower == "-m" && (i+1 == len(argv) || argv[i+1] != "pytest" && argv[i+1] != "unittest" && argv[i+1] != "compileall") {
				return fmt.Errorf("unsupported Python verification module")
			}
		}
		path := arg
		if _, value, ok := strings.Cut(path, "="); ok {
			path = value
		}
		path = strings.ReplaceAll(path, `\`, "/")
		if strings.HasPrefix(path, "-o") && len(path) > 2 {
			path = strings.TrimPrefix(path, "-o")
		}
		if strings.Contains(path, "/") || path == ".." || strings.Contains(path, ":") || strings.HasPrefix(path, "~") {
			if err := repositoryPath(root, path, false); err != nil {
				return err
			}
		}
	}
	return nil
}

func windowsShim(command string) bool {
	return runtime.GOOS == "windows" && (command == "npm" || command == "pnpm" || command == "yarn" || command == "./gradlew.bat" || command == "./mvnw.cmd")
}

func repositoryPath(root, name string, mustExist bool) error {
	name = strings.ReplaceAll(name, `\`, "/")
	if filepath.IsAbs(name) || strings.Contains(name, ":") || strings.HasPrefix(name, "~") {
		return fmt.Errorf("path must stay inside repository: %q", name)
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return err
	}
	path := filepath.Join(root, filepath.FromSlash(name))
	inside := func(path string) bool {
		rel, err := filepath.Rel(root, path)
		return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
	}
	if !inside(path) {
		return fmt.Errorf("path escapes repository: %q", name)
	}
	if mustExist {
		info, err := os.Stat(path)
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("expected repository file: %s", name)
		}
	}
	// Resolve existing parents too, to catch outputs through external symlinks.
	for {
		resolved, err := filepath.EvalSymlinks(path)
		if err == nil {
			if !inside(resolved) {
				return fmt.Errorf("symlink escapes repository: %s", name)
			}
			return nil
		}
		if !os.IsNotExist(err) {
			return err
		}
		parent := filepath.Dir(path)
		if parent == path {
			return err
		}
		path = parent
	}
}
