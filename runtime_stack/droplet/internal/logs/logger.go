package logs

import (
	"bytes"
	"droplet/internal/utils"
	"encoding/json"
	"errors"
	"io"
	"log"
	"os"
	"sync"

	"golang.org/x/sys/unix"
)

type FileLogger struct {
	mu      sync.Mutex
	path    string
	fd      int
	maxLine int
}

var AuditLogger *FileLogger

const (
	MaxAuditLines  = 15000
	AuditTrimLines = 15000
)

func InitAuditLogger() error {
	l, err := OpenFileLogger(utils.AuditLog, 64*1024)
	if err != nil {
		return err
	}
	AuditLogger = l
	return nil
}

func OpenFileLogger(path string, maxLine int) (*FileLogger, error) {
	if maxLine <= 0 {
		maxLine = 64 * 1024
	}
	flags := unix.O_WRONLY | unix.O_CREAT | unix.O_APPEND | unix.O_CLOEXEC | unix.O_NOFOLLOW
	fd, err := unix.Open(path, flags, 0640)
	if err != nil {
		return nil, err
	}
	return &FileLogger{path: path, fd: fd, maxLine: maxLine}, nil
}

func (l *FileLogger) Reopen() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.fd > 0 {
		_ = unix.Close(l.fd)
		l.fd = -1
	}
	flags := unix.O_WRONLY | unix.O_CREAT | unix.O_APPEND | unix.O_CLOEXEC | unix.O_NOFOLLOW
	fd, err := unix.Open(l.path, flags, 0640)
	if err != nil {
		return err
	}
	l.fd = fd
	return nil
}

func (l *FileLogger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.fd <= 0 {
		return nil
	}
	err := unix.Close(l.fd)
	l.fd = -1
	return err
}

func (l *FileLogger) WriteRecord(rec *Record) error {
	if rec == nil {
		return errors.New("nil record")
	}

	b, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	if len(b)+1 > l.maxLine {
		return errors.New("audit log line too large")
	}
	var buf bytes.Buffer
	buf.Grow(len(b) + 1)
	buf.Write(b)
	buf.WriteByte('\n')

	l.mu.Lock()
	defer l.mu.Unlock()
	if l.fd <= 0 {
		return errors.New("logger closed")
	}

	_, err = unix.Write(l.fd, buf.Bytes())
	return err
}

func CountLines(path string) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	buf := make([]byte, 32*1024)
	count := 0
	for {
		n, err := f.Read(buf)
		count += bytes.Count(buf[:n], []byte{'\n'})
		if err == io.EOF {
			break
		}
		if err != nil {
			return 0, err
		}
	}
	return count, nil
}

func TrimFileToLastNLines(path string, keep int) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return err
	}

	var (
		size   = stat.Size()
		bufSz  = int64(32 * 1024)
		pos    = size
		lines  = 0
		chunks [][]byte
	)

	for pos > 0 && lines <= keep {
		readSz := bufSz
		if pos < readSz {
			readSz = pos
		}
		pos -= readSz

		buf := make([]byte, readSz)
		if _, err := f.ReadAt(buf, pos); err != nil {
			return err
		}

		lines += bytes.Count(buf, []byte{'\n'})
		chunks = append(chunks, buf)
	}

	var data []byte
	for i := len(chunks) - 1; i >= 0; i-- {
		data = append(data, chunks[i]...)
	}

	idx := len(data)
	for i := 0; i < keep; i++ {
		p := bytes.LastIndexByte(data[:idx], '\n')
		if p < 0 {
			break
		}
		idx = p
	}
	data = data[idx+1:]

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0640); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func StartAuditLogTrimmer() {
	lines, err := CountLines(utils.AuditLog)
	if err != nil || lines <= MaxAuditLines {
		return
	}
	if err := TrimFileToLastNLines(utils.AuditLog, AuditTrimLines); err != nil {
		log.Printf("audit log trim failed: %v", err)
	}
}
