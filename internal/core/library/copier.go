package library

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"photo/internal/core/model"
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

	if mode < CopyJPEGOnly || mode > CopyOnlyRAW {
		return fmt.Errorf("unknown copy mode %d", mode)
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
		return copyRAW(ctx, photo.RAWPath, destDir)
	}

	if err := copyFile(ctx, photo.ImagePath, filepath.Join(destDir, filepath.Base(photo.ImagePath))); err != nil {
		return fmt.Errorf("copying image %s: %w", photo.ImagePath, err)
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	if mode == CopyWithRAW && photo.HasRAW() {
		return copyRAW(ctx, photo.RAWPath, destDir)
	}

	return nil
}

func copyRAW(ctx context.Context, rawPath, destDir string) error {
	if err := copyFile(ctx, rawPath, filepath.Join(destDir, filepath.Base(rawPath))); err != nil {
		return fmt.Errorf("copying RAW %s: %w", rawPath, err)
	}
	return copySidecar(ctx, rawPath, destDir)
}

// The XMP sidecar carries the tags and the develop settings of the RAW, so it
// travels with it. A RAW that never got one is copied alone.
func copySidecar(ctx context.Context, rawPath, destDir string) error {
	sidecar := model.SidecarPath(rawPath)
	if _, err := os.Stat(sidecar); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("reading sidecar %s: %w", sidecar, err)
	}
	if err := copyFile(ctx, sidecar, filepath.Join(destDir, filepath.Base(sidecar))); err != nil {
		return fmt.Errorf("copying sidecar %s: %w", sidecar, err)
	}
	return nil
}

func (c *Copier) Copy(photo model.Photo, destDir string, mode CopyMode) error {
	return c.CopyWithContext(context.Background(), photo, destDir, mode)
}

func copyFile(ctx context.Context, src, dst string) error {
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

	if _, err := io.Copy(out, cancellableReader{ctx: ctx, r: in}); err != nil {
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

// A cancelled copy stops between chunks instead of finishing the file, so
// cancelling does not wait out a multi-hundred-megabyte RAW.
type cancellableReader struct {
	//nolint:containedctx // carrying the context into io.Copy is the type's whole purpose
	ctx context.Context
	r   io.Reader
}

func (c cancellableReader) Read(p []byte) (int, error) {
	if err := c.ctx.Err(); err != nil {
		return 0, err
	}
	return c.r.Read(p)
}
