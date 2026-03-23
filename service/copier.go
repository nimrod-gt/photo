package service

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"photo/model"
)

type CopyMode int

const (
	CopyJPEGOnly CopyMode = iota
	CopyWithRAW
	CopyOnlyRAW
)

type Copier struct{}

func NewCopier() *Copier {
	return &Copier{}
}

func (c *Copier) CopyWithContext(ctx context.Context, photo model.Photo, destDir string, mode CopyMode) error {
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

	if mode == CopyOnlyRAW {
		if !photo.HasRAW() {
			return fmt.Errorf("no RAW file for %s", photo.Name)
		}
		if err := copyFile(photo.RAWPath, filepath.Join(destDir, filepath.Base(photo.RAWPath))); err != nil {
			return fmt.Errorf("copying RAW %s: %w", photo.RAWPath, err)
		}
		return nil
	}

	if err := copyFile(photo.ImagePath, filepath.Join(destDir, filepath.Base(photo.ImagePath))); err != nil {
		return fmt.Errorf("copying image %s: %w", photo.ImagePath, err)
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	if mode == CopyWithRAW && photo.HasRAW() {
		if err := copyFile(photo.RAWPath, filepath.Join(destDir, filepath.Base(photo.RAWPath))); err != nil {
			return fmt.Errorf("copying RAW %s: %w", photo.RAWPath, err)
		}
	}

	return nil
}

func (c *Copier) Copy(photo model.Photo, destDir string, mode CopyMode) error {
	return c.CopyWithContext(context.Background(), photo, destDir, mode)
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
		_ = os.Remove(dst)
		return err
	}

	if err := out.Close(); err != nil {
		_ = os.Remove(dst)
		return err
	}

	return os.Chtimes(dst, srcInfo.ModTime(), srcInfo.ModTime())
}
