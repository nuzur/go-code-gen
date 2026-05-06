package files

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func CreateGoMod(goModPath, moduleName string) error {
	err := os.MkdirAll(goModPath, 0o755)
	if err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}
	cmd := exec.Command("go", "mod", "init", moduleName)
	cmd.Dir = goModPath
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("go mod init failed in %s: %w\noutput: %s", goModPath, err, string(out))
	}
	return nil
}

func ReadGoMod(goModPath string) (string, error) {
	err := os.MkdirAll(filepath.Dir(goModPath), 0o755)
	if err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}
	data, err := os.ReadFile(goModPath)
	if err != nil {
		return "", fmt.Errorf("failed to read go.mod: %w", err)
	}
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module ")), nil
		}
	}
	return "", fmt.Errorf("module name not found in go.mod")
}
