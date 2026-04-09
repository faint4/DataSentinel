package scanner

import (
	"context"
	"os"
	"path/filepath"

	"github.com/datasentinel/datasentinel/model"
)

const maxFileSize = 50 * 1024 * 1024 // 50MB

// FileInfo holds lightweight file metadata for the scanning pipeline.
type FileInfo struct {
	Path string
	Name string
	Ext  string
	Size int64
}

// WalkFiles recursively walks a directory tree, filtering by extension.
// Files are sent to the returned channel. The channel is closed when
// the walk completes or the context is cancelled.
func WalkFiles(ctx context.Context, root string, exts []string) (<-chan FileInfo, error) {
	extSet := make(map[string]bool, len(exts))
	for _, e := range exts {
		extSet[e] = true
	}

	ch := make(chan FileInfo, 256)

	go func() {
		defer close(ch)
		filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil // skip errors, continue walking
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			if d.IsDir() {
				return nil
			}

			info, err := d.Info()
			if err != nil {
				return nil
			}
			if info.Size() > maxFileSize {
				return nil
			}

			ext := FilePathExt(path)
			if !IsScannable(ext) {
				return nil
			}
			if len(extSet) > 0 && !extSet[ext] {
				return nil
			}

			fi := FileInfo{
				Path: path,
				Name: d.Name(),
				Ext:  ext,
				Size: info.Size(),
			}

			select {
			case ch <- fi:
			case <-ctx.Done():
				return ctx.Err()
			}
			return nil
		})
	}()

	return ch, nil
}

// AllSupportedExts returns the list of all supported extensions for the UI.
func AllSupportedExts() []string {
	return model.SupportedExtensions()
}
