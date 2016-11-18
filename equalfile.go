package equalfile

import (
	"bytes"
	"fmt"
	"io"
	"os"
)

const DEFAULT_MAX_SIZE = 10000000000 // Only the first 10^10 bytes are compared.

// CompareFile: verify that files with names path1, path2 have identical contents
// Only the first 10^10 bytes are compared.
func CompareFile(path1, path2 string) (bool, error) {
	return CompareFileBufLimit(path1, path2, createBuf(), DEFAULT_MAX_SIZE)
}

// CompareFileBufLimit: verify that files with names path1, path2 have same contents
// You must provide a pre-allocated memory buffer.
// You must provide the maximum number of bytes read.
func CompareFileBufLimit(path1, path2 string, buf []byte, maxSize int64) (bool, error) {
	r1, openErr1 := os.Open(path1)
	if openErr1 != nil {
		return false, openErr1
	}
	defer r1.Close()
	info1, statErr1 := r1.Stat()
	if statErr1 != nil {
		return false, statErr1
	}

	r2, openErr2 := os.Open(path2)
	if openErr2 != nil {
		return false, openErr2
	}
	defer r2.Close()
	info2, statErr2 := r2.Stat()
	if statErr2 != nil {
		return false, statErr2
	}

	if info1.Size() != info2.Size() {
		return false, nil
	}

	return CompareReaderBufLimit(r1, r2, buf, maxSize)
}

// CompareReader: verify that two readers provide same content
// Only the first 10^10 bytes are compared.
func CompareReader(r1, r2 io.Reader) (bool, error) {
	return CompareReaderBufLimit(r1, r2, createBuf(), DEFAULT_MAX_SIZE)
}

var (
	debug     bool
	readCount int
	readMin   int
	readMax   int
	readSum   int64
)

func read(r io.Reader, buf []byte) (int, error) {
	n, err := r.Read(buf)

	if debug {
		readCount++
		readSum += int64(n)
		if n < readMin {
			readMin = n
		}
		if n > readMax {
			readMax = n
		}
	}

	return n, err
}

// CompareReaderBufLimit: verify that two readers provide same content
// You must provide a pre-allocated memory buffer.
// You must provide the maximum number of bytes read.
func CompareReaderBufLimit(r1, r2 io.Reader, buf []byte, maxSize int64) (bool, error) {

	// debug
	if debug {
		readCount = 0
		readMin = 2000000000
		readMax = 0
		readSum = 0
	}

	equal, err := compareReaderBufLimit(r1, r2, buf, maxSize)

	if debug {
		fmt.Printf("DEBUG compareReaderBufLimit: readCount=%d readMin=%d readMax=%d readSum=%d\n", readCount, readMin, readMax, readSum)
	}

	return equal, err
}

// CompareReaderBufLimit: verify that two readers provide same content
// You must provide a pre-allocated memory buffer.
// You must provide the maximum number of bytes read.
func compareReaderBufLimit(r1, r2 io.Reader, buf []byte, maxSize int64) (bool, error) {

	size := len(buf) / 2
	if size < 1 {
		return false, fmt.Errorf("insufficient buffer size")
	}
	buf1 := buf[:size]
	buf2 := buf[size:]
	eof1 := false
	eof2 := false

	var readSize int64

	for !eof1 && !eof2 {
		n1, err1 := read(r1, buf1)
		switch err1 {
		case io.EOF:
			eof1 = true
		case nil:
		default:
			return false, err1
		}

		n2, err2 := read(r2, buf2)
		switch err2 {
		case io.EOF:
			eof2 = true
		case nil:
		default:
			return false, err2
		}

		if n1 != n2 {
			return false, nil
		}

		if !bytes.Equal(buf1[:n1], buf2[:n2]) {
			return false, nil
		}

		readSize += int64(n1)
		if readSize > maxSize {
			return true, fmt.Errorf("max read size reached")
		}
	}

	if !eof1 || !eof2 {
		return false, nil
	}

	return true, nil
}

func createBuf() []byte {
	return make([]byte, 20000)
}
