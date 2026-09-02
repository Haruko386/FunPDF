package service

import (
	"FunPDF/internal/dao"
	"FunPDF/internal/dto"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

var (
	cacheDirMu       sync.RWMutex
	cacheMigrationMu sync.RWMutex
	cacheDir         = "./Cache"
)

// CurrentCacheDir returns the effective cache dir shared by file/chat services.
func CurrentCacheDir() string {
	cacheDirMu.RLock()
	defer cacheDirMu.RUnlock()
	return cacheDir
}

// SetCacheDir updates the effective cache dir; empty value is ignored.
func SetCacheDir(dir string) {
	if strings.TrimSpace(dir) == "" {
		return
	}
	cacheDirMu.Lock()
	defer cacheDirMu.Unlock()
	cacheDir = dir
}

type RuntimeService struct{}

func NewRuntimeService() *RuntimeService {
	return &RuntimeService{}
}

func (s *RuntimeService) OpenPath(_ context.Context, req dto.OpenPathRequest) (path string, err error) {
	reqPath := strings.TrimSpace(req.Path)
	if reqPath == "" {
		return "", errors.New("empty path")
	}

	absPath, err := filepath.Abs(reqPath)
	if err != nil {
		return "", err
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return "", err
	}

	if runtime.GOOS != "windows" {
		return "", errors.New("only support windows for now")
	}

	if info.IsDir() {
		err = exec.Command("explorer.exe", absPath).Start()
	} else {
		err = exec.Command("explorer.exe", `/select,"`+absPath+`"`).Start()
	}
	if err != nil {
		return "", err
	}
	return absPath, nil
}

// PickCacheDir opens a native folder dialog and returns the chosen dir; empty when user cancels.
func (s *RuntimeService) PickCacheDir() (string, error) {
	return pickDirectory()
}

// ChangeCacheDir migrates file storage from the current cache dir to newDir,
// persists the change and makes it effective immediately.
func (s *RuntimeService) ChangeCacheDir(ctx context.Context, newDir string) (string, error) {
	newDir = strings.TrimSpace(newDir)
	if newDir == "" {
		return "", errors.New("cache dir is empty")
	}
	abs, err := filepath.Abs(newDir)
	if err != nil {
		return "", err
	}
	newDir = abs

	cacheMigrationMu.Lock()
	defer cacheMigrationMu.Unlock()

	oldDir, err := filepath.Abs(CurrentCacheDir())
	if err != nil {
		return "", err
	}

	cleanOld := filepath.Clean(oldDir)
	cleanNew := filepath.Clean(newDir)

	if strings.EqualFold(cleanOld, cleanNew) {
		return newDir, nil
	}
	sep := string(os.PathSeparator)
	if strings.HasPrefix(cleanNew, cleanOld+sep) || strings.HasPrefix(cleanOld, cleanNew+sep) {
		return "", errors.New("new cache dir cannot contain or be contained by the current cache dir")
	}

	if err := ensureEmptyDir(newDir); err != nil {
		return "", err
	}
	if err := copyDirTree(oldDir, newDir); err != nil {
		_ = os.RemoveAll(newDir)
		return "", fmt.Errorf("migrate cache dir: %w", err)
	}
	if err := dao.NewRuntimeInfoDAO().UpdateCacheDir(ctx, dao.DB, newDir); err != nil {
		_ = os.RemoveAll(newDir)
		return "", fmt.Errorf("persist cache dir: %w", err)
	}

	SetCacheDir(newDir)
	return newDir, nil
}

func ensureEmptyDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return os.MkdirAll(dir, 0755)
		}
		return err
	}
	if len(entries) > 0 {
		return fmt.Errorf("target cache dir is not empty: %s", dir)
	}
	return nil
}

func copyDirTree(srcDir, dstDir string) error {
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		return err
	}
	return filepath.WalkDir(srcDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		target := filepath.Join(dstDir, rel)
		info, err := d.Info()
		if err != nil {
			return err
		}
		if d.IsDir() {
			return os.MkdirAll(target, info.Mode().Perm())
		}
		if info.Mode()&os.ModeSymlink != 0 {
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			return os.Symlink(link, target)
		}
		return copyFileContent(path, target, info.Mode().Perm())
	})
}

func copyFileContent(srcPath, dstPath string, perm fs.FileMode) error {
	src, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.OpenFile(dstPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
	if err != nil {
		return err
	}

	if _, err := io.Copy(dst, src); err != nil {
		_ = dst.Close()
		return err
	}
	return dst.Close()
}
