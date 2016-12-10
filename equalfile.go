package equalfile

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"os"
)

const defaultMaxSize = 10000000000 // Only the first 10^10 bytes are compared.

type Options struct {
	Debug         bool // enable debugging to stdout
	ForceFileRead bool // prevent shortcut at filesystem level (link, pathname, etc)
}

// CompareFile verifies that files with names path1, path2 have identical contents.
// Only the first 10^10 bytes are compared.
func CompareFile(path1, path2 string) (bool, error) {
	return CompareFileBufLimit(path1, path2, createBuf(), defaultMaxSize)
}

func getHash(path string, maxSize int64) ([]byte, error) {
	h, found := hashTable[path]
	if found {
		return h.result, h.err
	}

	f, openErr := os.Open(path)
	if openErr != nil {
		return nil, openErr
	}
	defer f.Close()

	sum := make([]byte, hashType.Size())
	hashType.Reset()
	n, copyErr := io.CopyN(hashType, f, maxSize)
	copy(sum, hashType.Sum(nil))

	if copyErr == io.EOF && n < maxSize {
		copyErr = nil
	}

	return newHash(path, sum, copyErr)
}

func newHash(path string, sum []byte, e error) ([]byte, error) {

	hashTable[path] = hashSum{sum, e}

	if options.Debug {
		fmt.Printf("newHash[%s]=%v: error=[%v]\n", path, hex.EncodeToString(sum), e)
	}

	return sum, e
}

// CompareFileBufLimit verifies that files with names path1, path2 have same contents.
// You must provide a pre-allocated memory buffer.
// You must provide the maximum number of bytes to compare.
func CompareFileBufLimit(path1, path2 string, buf []byte, maxSize int64) (bool, error) {

	if multipleMode() {
		h1, err1 := getHash(path1, maxSize)
		if err1 != nil {
			return false, err1
		}
		h2, err2 := getHash(path2, maxSize)
		if err2 != nil {
			return false, err2
		}
		if !bytes.Equal(h1, h2) {
			return false, nil // hashes mismatch
		}
		// hashes match
		if !hashMatchCompare {
			return true, nil // accept hash match without byte-by-byte comparison
		}
		// do byte-by-byte comparison
		if options.Debug {
			fmt.Printf("CompareFileBufLimit(%s,%s): hash match, will compare bytes\n", path1, path2)
		}
	}

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

	if !options.ForceFileRead {
		// shortcut: ask the filesystem: are these files the same? (link, pathname, etc)
		if os.SameFile(info1, info2) {
			return true, nil
		}
	}

	return CompareReaderBufLimit(r1, r2, buf, maxSize)
}

// CompareReader verifies that two readers provide same content.
// Only the first 10^10 bytes are compared.
func CompareReader(r1, r2 io.Reader) (bool, error) {
	return CompareReaderBufLimit(r1, r2, createBuf(), defaultMaxSize)
}

var (
	options Options

	readCount int
	readMin   int
	readMax   int
	readSum   int64

	hashType         hash.Hash
	hashTable        map[string]hashSum
	hashMatchCompare bool
)

type hashSum struct {
	result []byte
	err    error
}

// CompareSingle puts package in single comparison mode.
// Single comparison is the default mode.
// In single comparison mode, files are always compared byte-by-byte.
func CompareSingle() {
	hashType = nil
	hashTable = nil
	rejectMultiple()
}

// CompareMultiple puts package in multiple comparison mode.
// In multiple comparison mode, file hashes are used to speed up repeated comparisons of the same file.
// Use compareOnMatch to control byte-by-byte comparison when the hashes do match.
func CompareMultiple(h hash.Hash, compareOnMatch bool) {
	hashType = h
	hashTable = map[string]hashSum{}
	hashMatchCompare = compareOnMatch
	requireMultiple()
}

func SetOptions(o Options) {
	options = o
}

func multipleMode() bool {
	return hashType != nil && hashTable != nil
}

func requireMultiple() {
	if !multipleMode() {
		panic("refusing to run in single mode")
	}
}

func rejectMultiple() {
	if multipleMode() {
		panic("refusing to run in multiple mode")
	}
}

func read(r io.Reader, buf []byte) (int, error) {
	n, err := r.Read(buf)

	if options.Debug {
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

// CompareReaderBufLimit verifies that two readers provide same content.
// You must provide a pre-allocated memory buffer.
// You must provide the maximum number of bytes to compare.
func CompareReaderBufLimit(r1, r2 io.Reader, buf []byte, maxSize int64) (bool, error) {

	if options.Debug {
		readCount = 0
		readMin = 2000000000
		readMax = 0
		readSum = 0
	}

	equal, err := compareReaderBufLimit(r1, r2, buf, maxSize)

	if options.Debug {
		fmt.Printf("DEBUG compareReaderBufLimit(%d,%d): readCount=%d readMin=%d readMax=%d readSum=%d\n", len(buf), maxSize, readCount, readMin, readMax, readSum)
	}

	return equal, err
}

// CompareReaderBufLimit: verifies that two readers provide same content.
// You must provide a pre-allocated memory buffer.
// You must provide the maximum number of bytes to compare.
func compareReaderBufLimit(r1, r2 io.Reader, buf []byte, maxSize int64) (bool, error) {

	if maxSize < 1 {
		return false, fmt.Errorf("nonpositive max size")
	}

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
			return false, fmt.Errorf("compareReaderBufLimit: internal failure: readers returned different sizes")
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
