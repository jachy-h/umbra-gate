package logging

import (
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

type Options struct {
	Path                              string
	MaxSizeMB, MaxBackups, MaxAgeDays int
	Compress                          bool
}

type RotatingWriter struct {
	mu   sync.Mutex
	opt  Options
	file *os.File
	size int64
	day  string
}

func New(opt Options) (*RotatingWriter, error) {
	if opt.MaxSizeMB <= 0 {
		opt.MaxSizeMB = 50
	}
	if opt.MaxBackups <= 0 {
		opt.MaxBackups = 7
	}
	if opt.MaxAgeDays <= 0 {
		opt.MaxAgeDays = 7
	}
	if err := os.MkdirAll(filepath.Dir(opt.Path), 0o700); err != nil {
		return nil, err
	}
	w := &RotatingWriter{opt: opt}
	return w, w.open()
}

func (w *RotatingWriter) open() error {
	f, err := os.OpenFile(w.opt.Path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return err
	}
	w.file, w.size, w.day = f, info.Size(), time.Now().Format("20060102")
	return nil
}

func (w *RotatingWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		if err := w.open(); err != nil {
			return 0, err
		}
	}
	if w.day != time.Now().Format("20060102") || w.size+int64(len(p)) > int64(w.opt.MaxSizeMB)*1024*1024 {
		if err := w.rotate(); err != nil {
			return 0, err
		}
	}
	n, err := w.file.Write(p)
	w.size += int64(n)
	return n, err
}

func (w *RotatingWriter) rotate() error {
	if err := w.file.Close(); err != nil {
		return err
	}
	name := fmt.Sprintf("%s.%s", w.opt.Path, time.Now().Format("20060102-150405.000000000"))
	if err := os.Rename(w.opt.Path, name); err != nil && !os.IsNotExist(err) {
		return err
	}
	if w.opt.Compress {
		if err := gzipFile(name); err == nil {
			name += ".gz"
		}
	}
	w.file = nil
	w.size = 0
	if err := w.open(); err != nil {
		return err
	}
	return w.cleanup()
}

func gzipFile(path string) error {
	in, err := os.Open(path)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(path+".gz", os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	gz := gzip.NewWriter(out)
	_, copyErr := io.Copy(gz, in)
	closeErr := gz.Close()
	fileErr := out.Close()
	if copyErr != nil || closeErr != nil || fileErr != nil {
		return fmt.Errorf("compress log")
	}
	return os.Remove(path)
}
func (w *RotatingWriter) cleanup() error {
	files, err := filepath.Glob(w.opt.Path + ".*")
	if err != nil {
		return err
	}
	sort.Slice(files, func(i, j int) bool {
		a, _ := os.Stat(files[i])
		b, _ := os.Stat(files[j])
		return a.ModTime().After(b.ModTime())
	})
	cutoff := time.Now().AddDate(0, 0, -w.opt.MaxAgeDays)
	for i, p := range files {
		info, err := os.Stat(p)
		if err == nil && (i >= w.opt.MaxBackups || info.ModTime().Before(cutoff)) {
			_ = os.Remove(p)
		}
	}
	return nil
}
func (w *RotatingWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		return nil
	}
	return w.file.Close()
}
