package harness

import (
	"fmt"
	"os"
	"path/filepath"
)

func ReadSkill(skillDir, name string) (string, error) {
	b, err := os.ReadFile(filepath.Join(skillDir, name, "SKILL.md"))
	if err != nil {
		return "", fmt.Errorf("read %s skill: %w", name, err)
	}
	return string(b), nil
}
