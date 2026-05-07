// Package local provides filesystem-backed object storage for local document
// intake development.
package local

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/deplagene/revenueleakageengine/internal/platform/storage"
)

var (
	ErrRootRequired = errors.New("local storage root is required")
	ErrKeyRequired  = errors.New("storage key is required")
)

// Store persists objects under a configured root directory.
type Store struct {
	root string
}

// NewStore creates a local filesystem storage adapter.
func NewStore(root string) (*Store, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil, ErrRootRequired
	}
	return &Store{root: root}, nil
}

func (s *Store) Put(ctx context.Context, object storage.Object) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if strings.TrimSpace(object.Key) == "" {
		return ErrKeyRequired
	}

	path, err := s.safePath(object.Key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return fmt.Errorf("create storage directory: %w", err)
	}
	if err := os.WriteFile(path, object.Content, 0o640); err != nil {
		return fmt.Errorf("write storage object: %w", err)
	}
	return nil
}

func (s *Store) Get(ctx context.Context, key string) (storage.Object, error) {
	if err := ctx.Err(); err != nil {
		return storage.Object{}, err
	}
	if strings.TrimSpace(key) == "" {
		return storage.Object{}, ErrKeyRequired
	}

	path, err := s.safePath(key)
	if err != nil {
		return storage.Object{}, err
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return storage.Object{}, fmt.Errorf("read storage object: %w", err)
	}
	return storage.Object{
		Key:     key,
		Content: content,
	}, nil
}

func (s *Store) safePath(key string) (string, error) {
	cleanKey := filepath.Clean(key)
	if cleanKey == "." || strings.HasPrefix(cleanKey, "..") || filepath.IsAbs(cleanKey) {
		return "", fmt.Errorf("storage key escapes root: %s", key)
	}

	root, err := filepath.Abs(s.root)
	if err != nil {
		return "", fmt.Errorf("resolve storage root: %w", err)
	}
	path := filepath.Join(root, cleanKey)
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return "", fmt.Errorf("resolve storage key: %w", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("storage key escapes root: %s", key)
	}

	return path, nil
}
