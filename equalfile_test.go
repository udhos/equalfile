package equalfile

import (
	"crypto/sha256"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"testing"
)

const (
	expectError   = 0
	expectEqual   = 1
	expectUnequal = 2
)

func cleanupTmpFiles(files []*os.File) {
	for _, f := range files {
		_ = f.Close()
		_ = os.Remove(f.Name())
	}
}

// Make tmp files each filled with the corresponding bytes of the contents slice.
func makeTmpFiles(t *testing.T, pat string, contents [][]byte) []*os.File {
	tmpFiles := []*os.File{}
	for i, v := range contents {
		tmpPat := pat + strconv.Itoa(i) + "_*"
		tmpfile, err := ioutil.TempFile("", tmpPat)
		if err != nil {
			cleanupTmpFiles(tmpFiles)
			t.Fatal("couldn't open tmpfile")
		}
		if _, err := tmpfile.Write(v); err != nil {
			tmpfile.Close()
			cleanupTmpFiles(tmpFiles)
			t.Fatal(err)
		}
		tmpFiles = append(tmpFiles, tmpfile)
	}
	return tmpFiles
}

// Determine if hash usage is so broken that it allows two arbitrarily
// different files to compare equal (such as if the hash input is truncated to
// length zero, or ignored in the hash result).
func TestCompareBrokenHashMultiple(t *testing.T) {
	pat := "equalfiles_test_brokenhash"
	contents := [][]byte{[]byte("a"), []byte("b")}
	tmpFiles := makeTmpFiles(t, pat, contents)
	defer cleanupTmpFiles(tmpFiles)

	c := NewMultiple(nil, Options{}, sha256.New(), false) // Matching hash will determine equality
	compare(t, c, tmpFiles[0].Name(), tmpFiles[1].Name(), expectUnequal)
}

func TestReader1(t *testing.T) {
	debug := os.Getenv("DEBUG") != ""
	c := New(make([]byte, 4000), Options{MaxSize: int64(50000), Debug: debug})
	r1 := strings.NewReader("wow")
	r2 := strings.NewReader("somethingthatlong")
	equal, err := c.CompareReader(r1, r2)
	if equal {
		t.Fatal("CompareReader should return false")
	}
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestReader2(t *testing.T) {
	debug := os.Getenv("DEBUG") != ""
	c := New(make([]byte, 2), Options{MaxSize: int64(50000), Debug: debug})
	r1 := strings.NewReader("wow")
	r2 := strings.NewReader("somethingthatlong")
	equal, err := c.CompareReader(r1, r2)
	if equal {
		t.Fatal("CompareReader should return false")
	}
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
func TestLimitedReaders(t *testing.T) {
	debug := os.Getenv("DEBUG") != ""
	// MaxSize should be ignored.  Set to lowest value (1) to confirm
	c := New(nil, Options{MaxSize: int64(1), Debug: debug})

	LR := io.LimitReader
	NR := strings.NewReader
	var tests = []struct {
		r1, r2 io.Reader
		want   bool
		desc   string
	}{
		{r1: LR(NR("wow"), 4), r2: LR(NR("wow"), 4), want: true, desc: ", limit unreached"},
		{r1: LR(NR("woz"), 4), r2: LR(NR("wow"), 4), want: false, desc: ", limit unreached, inputs unequal at end"},
		{r1: NR("wow"), r2: LR(NR("wow"), 4), want: true, desc: ", limit unreached"},
		{r2: NR("wow"), r1: LR(NR("wow"), 4), want: true, desc: ", limit unreached"},
		{r1: LR(NR("wxy"), 1), r2: LR(NR("wow"), 1), want: true, desc: ", inputs equal up to limit 1"},
		{r1: LR(NR("wxy"), 2), r2: LR(NR("wow"), 2), want: false, desc: ", inputs unequal at limit 2"},
		{r1: LR(NR("abc"), 0), r2: LR(NR("wow"), 0), want: true, desc: ", limit 0 w/ unequal inputs"},
		{r1: NR("wxy"), r2: LR(NR("wow"), 1), want: false, desc: ", unequal input EOFs"},
	}

	for _, v := range tests {
		eq, err := c.CompareReader(v.r1, v.r2) // Should ignore MaxSize
		if eq != v.want {
			t.Errorf("CompareReader() got %v expected %v%v", eq, v.want, v.desc)
		}
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}
}

func TestMaxSizeReaders(t *testing.T) {
	debug := os.Getenv("DEBUG") != ""

	var tests = []struct {
		s1, s2  string
		bufSize int
		maxSize int64
		want    bool
		errWant bool
	}{
		// If a mismatch is found, it is returned before the checking for the
		// MaxSize option, so the compared length can exceed the MaxSize (by one).
		{s1: "abc", s2: "xyz", bufSize: 2, maxSize: 1, want: false, errWant: false}, // no letters match
		{s1: "wbc", s2: "wyz", bufSize: 2, maxSize: 1, want: false, errWant: false}, // one letter matches
		{s1: "woz", s2: "wow", bufSize: 2, maxSize: 1, want: true, errWant: true},   // two letters match
		{s1: "woz", s2: "wow", bufSize: 2, maxSize: 2, want: false, errWant: false}, // 2 letters, 2 size

		// Make sure the buffer size doesn't affect the results
		{s1: "wow", s2: "wow", bufSize: 2, maxSize: 1, want: true, errWant: true},  // three letters match
		{s1: "wow", s2: "wow", bufSize: 8, maxSize: 1, want: true, errWant: true},  // three letters match
		{s1: "wow", s2: "wow", bufSize: 2, maxSize: 2, want: true, errWant: true},  // three letters match
		{s1: "wow", s2: "wow", bufSize: 8, maxSize: 2, want: true, errWant: true},  // three letters match
		{s1: "wow", s2: "wow", bufSize: 2, maxSize: 3, want: true, errWant: false}, // three letters match
		{s1: "wow", s2: "wow", bufSize: 8, maxSize: 3, want: true, errWant: false}, // three letters match
	}

	for _, v := range tests {
		c := New(make([]byte, v.bufSize), Options{MaxSize: int64(v.maxSize), Debug: debug})
		eq, err := c.CompareReader(strings.NewReader(v.s1), strings.NewReader(v.s2))
		if eq != v.want {
			t.Errorf("CompareReader() got %v expected %v. s1: %v s2: %v, buf: %v, maxSz: %v",
				eq, v.want, v.s1, v.s2, v.bufSize, v.maxSize)
		}
		if v.errWant && err == nil {
			t.Errorf("unexpected non-error when MaxSize reached. s1: %v s2: %v, buf: %v, maxSz: %v",
				v.s1, v.s2, v.bufSize, v.maxSize)
		}
		if !v.errWant && err != nil {
			t.Errorf("unexpected error when MaxSize not reached. s1: %v s2: %v, buf: %v, maxSz: %v",
				v.s1, v.s2, v.bufSize, v.maxSize)
		}
	}
}

func TestBrokenReaders(t *testing.T) {
	limit := int64(50000)
	buf := make([]byte, 4000)
	chunk := int64(1000)
	total := int64(10000)
	debug := os.Getenv("DEBUG") != ""

	r1 := &testReader{label: "test1 r1", chunkSize: chunk, totalSize: total, debug: debug}
	r2 := &testReader{label: "test1 r2", chunkSize: chunk + 1, totalSize: total, debug: debug}
	compareReader(t, limit, buf, r1, r2, expectEqual, debug)

	r1 = &testReader{label: "test2 r1", chunkSize: chunk + 2, totalSize: total, debug: debug}
	r2 = &testReader{label: "test2 r2", chunkSize: chunk, totalSize: total, debug: debug}
	compareReader(t, limit, buf, r1, r2, expectEqual, debug)

	r1 = &testReader{label: "test3 r1", chunkSize: chunk, totalSize: total, debug: debug}
	r2 = &testReader{label: "test3 r2", chunkSize: chunk, totalSize: total, debug: debug}
	compareReader(t, limit, buf, r1, r2, expectEqual, debug)

	r1 = &testReader{label: "test4 r1", chunkSize: chunk, totalSize: total, lastByte: '0', debug: debug}
	r2 = &testReader{label: "test4 r2", chunkSize: chunk, totalSize: total, lastByte: '1', debug: debug}
	compareReader(t, limit, buf, r1, r2, expectUnequal, debug)

	r1 = &testReader{label: "test5 r1", chunkSize: chunk, totalSize: total, debug: debug}
	r2 = &testReader{label: "test5 r2", chunkSize: chunk + 1, totalSize: total, debug: debug}
	compareReader(t, limit, buf, r1, r2, expectUnequal, debug)

	r1 = &testReader{label: "test6 r1", chunkSize: chunk + 2, totalSize: total, debug: debug}
	r2 = &testReader{label: "test6 r2", chunkSize: chunk, totalSize: total, debug: debug}
	compareReader(t, limit, buf, r1, r2, expectUnequal, debug)

	r1 = &testReader{label: "test7 r1", chunkSize: chunk, totalSize: total, debug: debug}
	r2 = &testReader{label: "test7 r2", chunkSize: chunk, totalSize: total + 1, debug: debug}
	compareReader(t, limit, buf, r1, r2, expectUnequal, debug)
}

type testReader struct {
	label     string
	chunkSize int64
	totalSize int64
	lastByte  byte
	debug     bool
}

func (r *testReader) Read(buf []byte) (int, error) {

	n, err := testRead(r, buf)

	if r.debug {
		fmt.Printf("DEBUG testReader.Read: label=%s chunk=%d buf=%d total=%d last=%q size=%d error=%v\n", r.label, r.chunkSize, len(buf), r.totalSize, r.lastByte, n, err)
	}

	return n, err
}

func testRead(r *testReader, buf []byte) (int, error) {

	bufSize := int64(len(buf))
	if bufSize < 1 {
		return 0, fmt.Errorf("insufficient buffer size")
	}
	if r.totalSize < 1 {
		return 0, io.EOF
	}

	drainSize := r.totalSize // want to deliver all
	if r.chunkSize < drainSize {
		drainSize = r.chunkSize // cant deliver larger than chunk
	}
	if bufSize < drainSize {
		drainSize = bufSize // cant deliver larger than buf
	}

	r.totalSize -= drainSize

	if r.totalSize == 0 {
		buf[drainSize-1] = r.lastByte
	}

	return int(drainSize), nil
}

func compareReader(t *testing.T, limit int64, buf []byte, r1, r2 *testReader, expect int, debug bool) {
	c := New(buf, Options{MaxSize: limit, Debug: debug})
	equal, err := c.CompareReader(r1, r2)
	if err != nil {
		if expect != expectError {
			t.Errorf("compare: r1=%s r2=%s unexpected error: CompareReader(%d,%d): %v", r1.label, r2.label, c.Opt.MaxSize, len(c.buf), err)
		}
		return
	}
	if equal {
		if expect != expectEqual {
			t.Errorf("compare: r1=%s r2=%s unexpected equal: CompareReader(%d,%d)", r1.label, r2.label, c.Opt.MaxSize, len(c.buf))
		}
		return
	}
	if expect != expectUnequal {
		t.Errorf("compare: r1=%s r2=%s unexpected unequal: CompareReader(%d,%d)", r1.label, r2.label, c.Opt.MaxSize, len(c.buf))
	}
}
