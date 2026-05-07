// Package storage defines file storage primitives for uploaded source
// documents.
package storage

import "context"

// Object describes one immutable stored object.
type Object struct {
	Key         string
	Content     []byte
	ContentType string
}

// Store writes and reads immutable objects by storage key.
type Store interface {
	Put(ctx context.Context, object Object) error
	Get(ctx context.Context, key string) (Object, error)
}
