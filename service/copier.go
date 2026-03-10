package service

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"photo/model"
)

type Copier struct{}

func NewCopier() *Copier {
	return &Copier{}
}

func (c *Copier) CopyWithContext(ctx context.Context, photo model.Photo, destDir string, includeRAW bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	info, err := os.Stat(destDir)
	if err != nil {
		return fmt.Errorf("destination directory %s: %w", destDir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("destination %s is not a directory", destDir)
	}

	if err := copyFile(photo.ImagePath, filepath.Join(destDir, filepath.Base(photo.ImagePath))); err != nil {
		return fmt.Errorf("copying image %s: %w", photo.ImagePath, err)
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	if includeRAW && photo.HasRAW() {
		if err := copyFile(photo.RAWPath, filepath.Join(destDir, filepath.Base(photo.RAWPath))); err != nil {
			return fmt.Errorf("copying RAW %s: %w", photo.RAWPath, err)
		}
	}

	return nil
}

func (c *Copier) Copy(photo model.Photo, destDir string, includeRAW bool) error {
	return c.CopyWithContext(context.Background(), photo, destDir, includeRAW)
}

func copyFile(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}

	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}

	if err := out.Close(); err != nil {
		return err
	}

	return os.Chtimes(dst, srcInfo.ModTime(), srcInfo.ModTime())
}
