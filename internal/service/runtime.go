package service

import (
	"FunPDF/internal/dto"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

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
