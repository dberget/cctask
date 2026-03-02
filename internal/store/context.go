package store

import (
	"os"
	"path/filepath"
)

const contextFileName = "context.md"

func ContextPath(projectRoot string) string {
	return filepath.Join(CctaskDir(projectRoot), contextFileName)
}

func LoadContext(projectRoot string) string {
	data, err := os.ReadFile(ContextPath(projectRoot))
	if err != nil {
		return ""
	}
	return string(data)
}

func SaveContext(projectRoot string, content string) error {
	dir := CctaskDir(projectRoot)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(ContextPath(projectRoot), []byte(content), 0o644)
}
