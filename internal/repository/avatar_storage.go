package repository

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

type FileStorage struct {
	dir string
}

func NewFileStorage(dir string) (*FileStorage, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create storage dir: %w", err)
	}
	return &FileStorage{dir: dir}, nil
}

func (s *FileStorage) Save(_ context.Context, name string, data []byte) error {
	if err := os.WriteFile(s.path(name), data, 0o644); err != nil {
		return fmt.Errorf("save file: %w", err)
	}
	return nil
}

func (s *FileStorage) Delete(_ context.Context, name string) error {
	err := os.Remove(s.path(name))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("delete file: %w", err)
	}
	return nil
}

func (s *FileStorage) path(name string) string {
	return filepath.Join(s.dir, filepath.Base(name))
}
