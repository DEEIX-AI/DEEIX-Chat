package filecache

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// 外部目录快照在 storage 下的统一位置：storage/catalogs/<数据源>.json。
// 运行时最近一次同步（或由内置快照写入的种子）都存放在这里，内置快照见 infra/catalogdata。
const catalogsDir = "catalogs"

// catalogFile 是单个目录快照文件：受大小限制的读取，以及同目录临时文件 + 原子重命名的写入。
// legacyPath 为旧版本的存放位置；新位置缺失时先迁移旧文件，避免升级后丢失已同步的数据。
type catalogFile struct {
	label      string
	path       string
	legacyPath string
	maxBytes   int
}

func newCatalogFile(storageRoot string, name string, legacyRelPath string, label string, maxBytes int) catalogFile {
	root := filepath.Clean(storageRoot)
	file := catalogFile{
		label:    label,
		path:     filepath.Join(root, catalogsDir, name),
		maxBytes: maxBytes,
	}
	if legacyRelPath != "" {
		file.legacyPath = filepath.Join(root, filepath.FromSlash(legacyRelPath))
	}
	return file
}

func (f catalogFile) configured() error {
	if f.path == "" || f.path == "." {
		return fmt.Errorf("%s path is not configured", f.label)
	}
	return nil
}

// load 读取快照；文件不存在时 found=false、err=nil。
func (f catalogFile) load(ctx context.Context) ([]byte, bool, error) {
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}
	if err := f.configured(); err != nil {
		return nil, false, err
	}
	if err := f.migrateLegacy(); err != nil {
		return nil, false, err
	}
	file, err := os.Open(f.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("open %s: %w", f.label, err)
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, int64(f.maxBytes)+1))
	if err != nil {
		return nil, false, fmt.Errorf("read %s: %w", f.label, err)
	}
	if len(data) > f.maxBytes {
		return nil, false, fmt.Errorf("%s exceeds %d bytes", f.label, f.maxBytes)
	}
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}
	return data, true, nil
}

// migrateLegacy 在新位置缺失、旧位置存在时把旧文件移到新位置；内容是否可用由调用方解码判断。
func (f catalogFile) migrateLegacy() error {
	if f.legacyPath == "" {
		return nil
	}
	if _, err := os.Stat(f.path); err == nil || !os.IsNotExist(err) {
		return nil
	}
	if _, err := os.Stat(f.legacyPath); err != nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(f.path), 0o755); err != nil {
		return fmt.Errorf("create %s directory: %w", f.label, err)
	}
	if err := os.Rename(f.legacyPath, f.path); err != nil {
		return fmt.Errorf("migrate legacy %s: %w", f.label, err)
	}
	return nil
}

// store 通过同目录临时文件和原子重命名写入快照字节。
func (f catalogFile) store(ctx context.Context, data []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := f.configured(); err != nil {
		return err
	}
	if len(data) == 0 || len(data) > f.maxBytes {
		return fmt.Errorf("%s payload must be between 1 and %d bytes", f.label, f.maxBytes)
	}

	dir := filepath.Dir(f.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create %s directory: %w", f.label, err)
	}
	temporary, err := os.CreateTemp(dir, filepath.Base(f.path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create %s temporary file: %w", f.label, err)
	}
	temporaryPath := temporary.Name()
	committed := false
	defer func() {
		_ = temporary.Close()
		if !committed {
			_ = os.Remove(temporaryPath)
		}
	}()

	if err := temporary.Chmod(0o644); err != nil {
		return fmt.Errorf("set %s permissions: %w", f.label, err)
	}
	if _, err := temporary.Write(data); err != nil {
		return fmt.Errorf("write %s: %w", f.label, err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close %s: %w", f.label, err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, f.path); err != nil {
		return fmt.Errorf("replace %s: %w", f.label, err)
	}
	committed = true
	return nil
}
