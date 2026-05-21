package objectstore

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type LocalStore struct {
	root string
}

func NewLocal(root string) *LocalStore {
	normalized := strings.TrimSpace(root)
	if normalized == "" {
		normalized = "./storage"
	}
	return &LocalStore{root: normalized}
}

func (s *LocalStore) Put(ctx context.Context, key string, body io.Reader, opts PutOptions) (ObjectInfo, error) {
	_ = ctx
	path, err := s.resolve(key)
	if err != nil {
		return ObjectInfo{}, err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return ObjectInfo{}, err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*.tmp")
	if err != nil {
		return ObjectInfo{}, err
	}
	tmpName := tmp.Name()
	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
	}()
	written, err := io.Copy(tmp, body)
	if err != nil {
		return ObjectInfo{}, err
	}
	if err = tmp.Close(); err != nil {
		return ObjectInfo{}, err
	}
	if err = os.Rename(tmpName, path); err != nil {
		return ObjectInfo{}, err
	}
	return ObjectInfo{Key: normalizeKey(key), SizeBytes: written, ContentType: strings.TrimSpace(opts.ContentType), ModTime: time.Now()}, nil
}

func (s *LocalStore) Open(ctx context.Context, key string) (io.ReadCloser, ObjectInfo, error) {
	_ = ctx
	path, err := s.resolve(key)
	if err != nil {
		return nil, ObjectInfo{}, err
	}
	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ObjectInfo{}, ErrNotFound
		}
		return nil, ObjectInfo{}, err
	}
	stat, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, ObjectInfo{}, err
	}
	return file, ObjectInfo{Key: normalizeKey(key), SizeBytes: stat.Size(), ModTime: stat.ModTime()}, nil
}

func (s *LocalStore) Delete(ctx context.Context, key string) error {
	_ = ctx
	path, err := s.resolve(key)
	if err != nil {
		return nil
	}
	if err = os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func (s *LocalStore) Materialize(ctx context.Context, key string) (string, func(), error) {
	_ = ctx
	path, err := s.resolve(key)
	if err != nil {
		return "", nil, err
	}
	if _, err = os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil, ErrNotFound
		}
		return "", nil, err
	}
	return path, func() {}, nil
}

func (s *LocalStore) resolve(key string) (string, error) {
	root := strings.TrimSpace(s.root)
	if root == "" {
		root = "./storage"
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	cleanKey := filepath.Clean(filepath.FromSlash(normalizeKey(key)))
	if cleanKey == "" || cleanKey == "." || strings.HasPrefix(cleanKey, "..") || filepath.IsAbs(cleanKey) {
		return "", ErrInvalidKey
	}
	path := filepath.Join(rootAbs, cleanKey)
	rel, err := filepath.Rel(rootAbs, path)
	if err != nil {
		return "", err
	}
	if rel == "." || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
		return "", ErrInvalidKey
	}
	return path, nil
}

func normalizeKey(key string) string {
	return strings.Trim(strings.ReplaceAll(strings.TrimSpace(key), "\\", "/"), "/")
}
