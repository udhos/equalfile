package equalfile

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"math"
	"os"
)

// Only the first 10^10 bytes of io.Reader are compared.  Ignored for io.LimitedReader
const defaultMaxSize = 10000000000
const defaultBufSize = 20000

type Options struct {
	Debug         bool  // enable debugging to stdout
	ForceFileRead bool  // prevent shortcut at filesystem level (link, pathname, etc)
	MaxSize       int64 // prevent forever reading from an infinite reader. Ignored when using LimitedReader.
}

type Cmp struct {
	Opt Options

	readCount int
	readMin   int
	readMax   int
	readSum   int64

	hashType         hash.Hash
	hashMatchCompare bool
	hashTable        map[string]hashSum

	buf []byte
}

type hashSum struct {
	result []byte
	err    error
}

// New creates Cmp for multiple comparison mode.
func NewMultiple(buf []byte, options Options, h hash.Hash, compareOnMatch bool) *Cmp {
	c := &Cmp{
		Opt:              options,
		hashType:         h,
		hashMatchCompare: compareOnMatch,
		hashTable:        map[string]hashSum{},
		buf:              buf,
	}
	if c.buf == nil || len(c.buf) == 0 {
		c.buf = make([]byte, defaultBufSize)
	}
	return c
}

// New creates Cmp for single comparison mode.
func New(buf []byte, options Options) *Cmp {
	return NewMultiple(buf, options, nil, true)
}

func (c *Cmp) getHash(path string, maxSize int64) ([]byte, error) {
	h, found := c.hashTable[path]
	if found {
		return h.result, h.err
	}

	f, openErr := os.Open(path)
	if openErr != nil {
		return nil, openErr
	}
	defer f.Close()

	sum := make([]byte, c.hashType.Size())
	c.hashType.Reset()
	n, copyErr := io.CopyN(c.hashType, f, maxSize)
	copy(sum, c.hashType.Sum(nil))

	if copyErr == io.EOF && n < maxSize {
		copyErr = nil
	}

	return c.newHash(path, sum, copyErr)
}

func (c *Cmp) newHash(path string, sum []byte, e error) ([]byte, error) {

	c.hashTable[path] = hashSum{sum, e}

	if c.Opt.Debug {
		fmt.Printf("newHash[%s]=%v: error=[%v]\n", path, hex.EncodeToString(sum), e)
	}

	return sum, e
}

func (c *Cmp) multipleMode() bool {
	return c.hashType != nil
}

// CompareFile verifies that files with names path1, path2 have same contents.
func (c *Cmp) CompareFile(path1, path2 string) (bool, error) {

	if c.Opt.MaxSize < 0 {
		return false, fmt.Errorf("negative MaxSize")
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

	// Non-regular files other than symlinks (ie. directories, character
	// devices, etc) should be excluded from file comparison.  They can
	// have size 0, while also never returning EOF.  CompareReaders() can
	// be used instead if it's really necessary.
	//
	// Note - Stat() resolved symlinks, so we needn't check for them.
	if !info1.Mode().IsRegular() {
		return false, fmt.Errorf("can't compare non-regular file: %v", path1)
	}
	if !info2.Mode().IsRegular() {
		return false, fmt.Errorf("can't compare non-regular file: %v", path2)
	}

	if !c.Opt.ForceFileRead {
		// shortcut: ask the filesystem: are these files the same? (link, pathname, etc)
		if os.SameFile(info1, info2) {
			return true, nil
		}
	}

	if info1.Size() != info2.Size() {
		return false, nil
	}

	// If Opt.MaxSize not initialized, set maxSize to the larger of the
	// filesize or 1. Pass maxSize to getHash to ensure the hash is
	// computed only up to the user specified MaxSize amount.
	maxSize := c.Opt.MaxSize
	if maxSize == 0 {
		maxSize = info1.Size()
		if maxSize == 0 {
			maxSize = 1
		}
	}

	if c.multipleMode() {
		h1, err1 := c.getHash(path1, maxSize)
		if err1 != nil {
			return false, err1
		}
		h2, err2 := c.getHash(path2, maxSize)
		if err2 != nil {
			return false, err2
		}
		if !bytes.Equal(h1, h2) {
			return false, nil // hashes mismatch
		}
		// hashes match
		if !c.hashMatchCompare {
			return true, nil // accept hash match without byte-by-byte comparison
		}
		// do byte-by-byte comparison
		if c.Opt.Debug {
			fmt.Printf("CompareFile(%s,%s): hash match, will compare bytes\n", path1, path2)
		}
	}

	// Use our maxSize to avoid triggering the defaultMaxSize for files.
	// We still need to preserve the error returning properties of the
	// input amount exceeding MaxSize, so we can't use LimitedReader.
	c.resetDebugging()

	eq, err := c.compareReader(r1, r2, maxSize)

	c.printDebugCompareReader()

	return eq, err
}

func (c *Cmp) read(r io.Reader, buf []byte) (int, error) {
	n, err := r.Read(buf)

	if c.Opt.Debug {
		c.readCount++
		c.readSum += int64(n)
		if n < c.readMin {
			c.readMin = n
		}
		if n > c.readMax {
			c.readMax = n
		}
	}

	return n, err
}

// CompareReader verifies that two readers provide same content.
func (c *Cmp) CompareReader(r1, r2 io.Reader) (bool, error) {

	c.resetDebugging()

	equal, err := c.compareReader(r1, r2, c.Opt.MaxSize)

	c.printDebugCompareReader()

	return equal, err
}

func (c *Cmp) resetDebugging() {
	if c.Opt.Debug {
		c.readCount = 0
		c.readMin = 2000000000
		c.readMax = 0
		c.readSum = 0
	}
}

func (c *Cmp) printDebugCompareReader() {
	if c.Opt.Debug {
		fmt.Printf("DEBUG CompareReader(%d,%d): readCount=%d readMin=%d readMax=%d readSum=%d\n",
			len(c.buf), c.Opt.MaxSize, c.readCount, c.readMin, c.readMax, c.readSum)
	}
}

func readPartial(c *Cmp, r io.Reader, buf []byte, n1, n2 int) (int, error) {
	for n1 < n2 {
		n, err := c.read(r, buf[n1:n2])
		n1 += n
		if err != nil {
			return n1, err
		}
	}
	return n1, nil
}

func (c *Cmp) compareReader(r1, r2 io.Reader, maxSize int64) (bool, error) {

	var useMaxSize bool

	_, ok1 := r1.(*io.LimitedReader)
	_, ok2 := r2.(*io.LimitedReader)

	// Use maxSize if neither Reader is a LimitedReader
	if !(ok1 || ok2) {
		// Only use if maxSize can be exceeded
		useMaxSize = (maxSize < math.MaxInt64)
		if maxSize == 0 {
			maxSize = defaultMaxSize
		}

		if maxSize < 1 {
			return false, fmt.Errorf("nonpositive max size")
		}
	}

	buf := c.buf

	size := len(buf) / 2
	if size < 1 {
		return false, fmt.Errorf("insufficient buffer size")
	}

	buf1 := buf[:size]
	buf2 := buf[size : 2*size] // must force same size as buf1

	if len(buf1) != len(buf2) {
		return false, fmt.Errorf("buffer size mismatch buf1=%d buf2=%d", len(buf1), len(buf2))
	}

	eof1 := false
	eof2 := false
	var readSize int64

	for !eof1 && !eof2 {
		n1, err1 := c.read(r1, buf1)
		switch err1 {
		case io.EOF:
			eof1 = true
		case nil:
		default:
			return false, err1
		}

		n2, err2 := c.read(r2, buf2)
		switch err2 {
		case io.EOF:
			eof2 = true
		case nil:
		default:
			return false, err2
		}

		switch {
		case n1 < n2:
			n, errPart := readPartial(c, r1, buf1, n1, n2)
			switch errPart {
			case io.EOF:
				eof1 = true
			case nil:
			default:
				return false, errPart
			}
			n1 = n
		case n2 < n1:
			n, errPart := readPartial(c, r2, buf2, n2, n1)
			switch errPart {
			case io.EOF:
				eof2 = true
			case nil:
			default:
				return false, errPart
			}
			n2 = n
		}

		if n1 != n2 {
			return false, nil
		}

		if !bytes.Equal(buf1[:n1], buf2[:n2]) {
			return false, nil
		}

		readSize += int64(n1)
		if useMaxSize && readSize > maxSize {
			return true, fmt.Errorf("max read size reached")
		}
	}

	if !eof1 || !eof2 {
		return false, nil
	}

	return true, nil
}
