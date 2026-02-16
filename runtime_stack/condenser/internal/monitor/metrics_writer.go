package monitor

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
)

type MetricsWriter struct {
	path       string
	file       *os.File
	buf        *bufio.Writer
	size       int64
	maxBytes   int64
	maxBackups int
}

const (
	defaultMaxBytes   = 50 * 1024 * 1024
	defaultMaxBackups = 5
)

func NewMetricsWriter(path string) (*MetricsWriter, error) {
	return NewMetricsWriterWithRotation(path, defaultMaxBytes, defaultMaxBackups)
}

func NewMetricsWriterWithRotation(path string, maxBytes int64, maxBackups int) (*MetricsWriter, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	var size int64
	if st, err := f.Stat(); err == nil {
		size = st.Size()
	}
	return &MetricsWriter{
		path:       path,
		file:       f,
		buf:        bufio.NewWriterSize(f, 64*1024),
		size:       size,
		maxBytes:   maxBytes,
		maxBackups: maxBackups,
	}, nil
}

func (w *MetricsWriter) Close() error {
	if w == nil {
		return nil
	}
	if w.buf != nil {
		_ = w.buf.Flush()
	}
	if w.file != nil {
		return w.file.Close()
	}
	return nil
}

func (w *MetricsWriter) WriteJSONL(v any) error {
	if w == nil || w.buf == nil {
		return nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if err := w.rotateIfNeeded(int64(len(b) + 1)); err != nil {
		return err
	}
	if _, err := w.buf.Write(b); err != nil {
		return err
	}
	if err := w.buf.WriteByte('\n'); err != nil {
		return err
	}
	if err := w.buf.Flush(); err != nil {
		return err
	}
	w.size += int64(len(b) + 1)
	return nil
}

func (w *MetricsWriter) rotateIfNeeded(nextBytes int64) error {
	if w.maxBytes <= 0 {
		return nil
	}
	if w.size+nextBytes <= w.maxBytes {
		return nil
	}
	if err := w.rotate(); err != nil {
		return err
	}
	return nil
}

func (w *MetricsWriter) rotate() error {
	if w.buf != nil {
		_ = w.buf.Flush()
	}
	if w.file != nil {
		_ = w.file.Close()
	}

	if w.maxBackups > 0 {
		last := w.path + "." + strconv.Itoa(w.maxBackups)
		_ = os.Remove(last)
		for i := w.maxBackups - 1; i >= 1; i-- {
			src := w.path + "." + strconv.Itoa(i)
			dst := w.path + "." + strconv.Itoa(i+1)
			if _, err := os.Stat(src); err == nil {
				_ = os.Rename(src, dst)
			}
		}
		_ = os.Rename(w.path, w.path+".1")
	} else {
		_ = os.Remove(w.path)
	}

	f, err := os.OpenFile(w.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	w.file = f
	w.buf = bufio.NewWriterSize(f, 64*1024)
	w.size = 0
	return nil
}
