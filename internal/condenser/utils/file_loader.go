package utils

import (
	"bytes"
	"fmt"
	"io"
	"os"
)

func TailBytes(path string, n int64) ([]byte, error) {
	if n <= 0 {
		return []byte{}, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	st, err := f.Stat()
	if err != nil {
		return nil, err
	}

	size := st.Size()
	if size == 0 {
		return []byte{}, nil
	}

	if n > size {
		n = size
	}

	_, err = f.Seek(-n, io.SeekEnd)
	if err != nil {
		return nil, err
	}

	buf := make([]byte, n)
	_, err = io.ReadFull(f, buf)
	if err != nil {
		return nil, err
	}
	return buf, nil
}

func TailLines(path string, lines int, maxBytes int64) ([]byte, error) {
	if lines <= 0 {
		return []byte{}, nil
	}
	if maxBytes <= 0 {
		return nil, fmt.Errorf("maxBytes must be > 0")
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	size := st.Size()
	if size == 0 {
		return []byte{}, nil
	}

	const chunkSize int64 = 64 * 1024

	var (
		readBytes int64
		pos       = size
		buf       []byte
	)

	for {
		if readBytes >= maxBytes || pos <= 0 {
			break
		}
		need := chunkSize
		if need > pos {
			need = pos
		}
		if readBytes+need > maxBytes {
			need = maxBytes - readBytes
		}

		pos -= need
		_, err := f.Seek(pos, io.SeekStart)
		if err != nil {
			return nil, err
		}

		tmp := make([]byte, need)
		_, err = io.ReadFull(f, tmp)
		if err != nil {
			return nil, err
		}

		buf = append(tmp, buf...)
		readBytes += need

		if bytes.Count(buf, []byte{'\n'}) >= lines {
			break
		}
	}

	splits := bytes.Split(buf, []byte{'\n'})
	if len(splits) > 0 && len(splits[len(splits)-1]) == 0 {
		splits = splits[:len(splits)-1]
	}

	if lines >= len(splits) {
		return append(buf, '\n'), nil
	}

	start := len(splits) - lines
	out := bytes.Join(splits[start:], []byte{'\n'})
	out = append(out, '\n')
	return out, nil
}
