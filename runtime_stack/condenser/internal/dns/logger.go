package dns

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

type DnsLogger struct {
	mu         sync.Mutex
	path       string
	f          *os.File
	w          *bufio.Writer
	size       int64
	maxBytes   int64
	maxBackups int
}

func NewDnsLogger(path string) (*DnsLogger, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}
	w := bufio.NewWriterSize(f, 256*1024)
	var size int64
	if st, err := f.Stat(); err == nil {
		size = st.Size()
	}

	return &DnsLogger{
		path:       path,
		f:          f,
		w:          w,
		size:       size,
		maxBytes:   50 * 1024 * 1024,
		maxBackups: 5,
	}, nil
}

func (l *DnsLogger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	_ = l.w.Flush()
	return l.f.Close()
}

func (l *DnsLogger) Flush() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.w.Flush()
}

func (l *DnsLogger) Write(v any) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return err
	}
	if err := l.rotateIfNeeded(int64(buf.Len())); err != nil {
		return err
	}
	if _, err := l.w.Write(buf.Bytes()); err != nil {
		return err
	}
	l.size += int64(buf.Len())
	return nil
}

func Timestamp() string {
	return time.Now().Format(time.RFC3339Nano)
}

func (l *DnsLogger) rotateIfNeeded(nextBytes int64) error {
	if l.maxBytes <= 0 {
		return nil
	}
	if l.size+nextBytes <= l.maxBytes {
		return nil
	}
	return l.rotate()
}

func (l *DnsLogger) rotate() error {
	if l.w != nil {
		_ = l.w.Flush()
	}
	if l.f != nil {
		_ = l.f.Close()
	}

	if l.maxBackups > 0 {
		last := l.path + "." + strconv.Itoa(l.maxBackups)
		_ = os.Remove(last)
		for i := l.maxBackups - 1; i >= 1; i-- {
			src := l.path + "." + strconv.Itoa(i)
			dst := l.path + "." + strconv.Itoa(i+1)
			if _, err := os.Stat(src); err == nil {
				_ = os.Rename(src, dst)
			}
		}
		_ = os.Rename(l.path, l.path+".1")
	} else {
		_ = os.Remove(l.path)
	}

	f, err := os.OpenFile(l.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	l.f = f
	l.w = bufio.NewWriterSize(f, 256*1024)
	l.size = 0
	return nil
}
