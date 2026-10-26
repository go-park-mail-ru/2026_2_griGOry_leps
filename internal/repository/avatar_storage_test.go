package repository

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFileStorage_SaveAndDelete(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "avatars")
	s, err := NewFileStorage(dir)
	require.NoError(t, err)
	ctx := context.Background()

	require.NoError(t, s.Save(ctx, "a.png", []byte("data")))
	got, err := os.ReadFile(filepath.Join(dir, "a.png"))
	require.NoError(t, err)
	require.Equal(t, []byte("data"), got)

	require.NoError(t, s.Delete(ctx, "a.png"))
	_, err = os.Stat(filepath.Join(dir, "a.png"))
	require.ErrorIs(t, err, os.ErrNotExist)

	require.NoError(t, s.Delete(ctx, "a.png"))
}

func TestFileStorage_StaysInsideDir(t *testing.T) {
	root := t.TempDir()
	s, err := NewFileStorage(filepath.Join(root, "avatars"))
	require.NoError(t, err)
	outside := filepath.Join(root, "secret.txt")
	require.NoError(t, os.WriteFile(outside, []byte("x"), 0o644))

	require.NoError(t, s.Delete(context.Background(), "../secret.txt"))

	_, err = os.Stat(outside)
	require.NoError(t, err)
}
