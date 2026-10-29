package desktopstorage

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	locationFile    = "storage-location.json"
	currentRootName = "VisiDeepAI"
	legacyRootName  = "BeefTV"
)

type Location struct {
	DataDir       string            `json:"dataDir"`
	PreviousDir   string            `json:"previousDir,omitempty"`
	PreviousFiles map[string]string `json:"previousFiles,omitempty"`
}

func DefaultRoot() (string, error) {
	root, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	current := filepath.Join(root, currentRootName)
	legacy := filepath.Join(root, legacyRootName)
	if hasWorkspaceContent(current) {
		return current, nil
	}
	if hasWorkspaceContent(legacy) {
		if err := os.MkdirAll(root, 0700); err != nil {
			return "", err
		}
		if err := os.Rename(legacy, current); err == nil {
			return current, nil
		}
		// A failed earlier start can leave an empty VisiDeepAI directory behind.
		// Remove only that empty directory and retry, never a populated one.
		if entries, readErr := os.ReadDir(current); readErr == nil && len(entries) == 0 {
			if removeErr := os.Remove(current); removeErr == nil {
				if err := os.Rename(legacy, current); err == nil {
					return current, nil
				}
			}
		}
		// If another process or a file lock blocks the rename, keep using the
		// legacy workspace so existing projects are never orphaned.
		return legacy, nil
	}
	return current, nil
}

func hasWorkspaceContent(path string) bool {
	entries, err := os.ReadDir(path)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		switch entry.Name() {
		case "cache", "logs", "tmp", "startup-error.log":
			continue
		default:
			return true
		}
	}
	return false
}

// The pointer stays in the system profile, independent of the selected data volume.
// A missing selected volume must never silently create an empty workspace.
func Resolve(root string) (string, error) {
	body, err := os.ReadFile(filepath.Join(root, locationFile))
	if errors.Is(err, os.ErrNotExist) {
		return root, nil
	}
	if err != nil {
		return "", fmt.Errorf("读取存储目录设置失败: %w", err)
	}
	var location Location
	if err := json.Unmarshal(body, &location); err != nil {
		return "", fmt.Errorf("存储目录设置损坏: %w", err)
	}
	if !filepath.IsAbs(location.DataDir) {
		return "", errors.New("存储目录设置无效")
	}
	if info, err := os.Stat(location.DataDir); err != nil || !info.IsDir() {
		return "", fmt.Errorf("存储目录无法访问，请连接对应磁盘后重试：%s", location.DataDir)
	}
	return location.DataDir, nil
}

func Save(root, dataDir, previousDir string) error {
	return saveLocation(root, Location{DataDir: dataDir, PreviousDir: previousDir})
}

func SaveMigration(root, dataDir, previousDir string, files map[string]string) error {
	return saveLocation(root, Location{DataDir: dataDir, PreviousDir: previousDir, PreviousFiles: files})
}

func saveLocation(root string, location Location) error {
	body, err := json.Marshal(location)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(root, 0700); err != nil {
		return err
	}
	file, err := os.CreateTemp(root, ".storage-location-*")
	if err != nil {
		return err
	}
	path := file.Name()
	defer os.Remove(path)
	if err = file.Chmod(0600); err == nil {
		_, err = file.Write(body)
	}
	if err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if err := os.Rename(path, filepath.Join(root, locationFile)); err != nil {
		return err
	}
	return nil
}

func canonical(path string) (string, error) {
	path = strings.TrimSpace(path)
	if !filepath.IsAbs(path) || filepath.Dir(filepath.Clean(path)) == filepath.Clean(path) {
		return "", errors.New("请选择磁盘中的文件夹，例如 D:\\VisiDeepAI")
	}
	path = filepath.Clean(path)
	missing := []string{}
	for {
		resolved, err := filepath.EvalSymlinks(path)
		if err == nil {
			for i := len(missing) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, missing[i])
			}
			return resolved, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		parent := filepath.Dir(path)
		if parent == path {
			return "", err
		}
		missing = append(missing, filepath.Base(path))
		path = parent
	}
}

func contains(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
