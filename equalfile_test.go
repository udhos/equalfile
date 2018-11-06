package equalfile

import (
	"bytes"
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

func TestPartialDueToMaxSizeLimit(t *testing.T) {
	pat := "equalfiles_test_partial"
	contents := [][]byte{[]byte("aaaaaxxxxx"), []byte("aaaaayyyyy")}
	tmpFiles := makeTmpFiles(t, pat, contents)
	defer cleanupTmpFiles(tmpFiles)

	c := New([]byte{'1', '2'}, Options{MaxSize: 3})
	compareExpectErrorAndEqual(t, c, tmpFiles[0].Name(), tmpFiles[1].Name())
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
		r1, r2  io.Reader
		want    bool
		wantErr bool
		desc    string
	}{
		{r1: LR(NR("wow"), 4), r2: LR(NR("wow"), 4), want: true, desc: ", limit unreached (test 1)"},
		{r1: LR(NR("woz"), 4), r2: LR(NR("wow"), 4), want: false, desc: ", limit unreached, inputs unequal at end"},
		{r1: NR("wow"), r2: LR(NR("wow"), 4), want: true, desc: ", limit unreached (test 3)"},
		{r2: NR("wow"), r1: LR(NR("wow"), 4), want: true, desc: ", limit unreached (test 4)"},
		{r1: LR(NR("wxy"), 1), r2: LR(NR("wow"), 1), want: true, desc: ", inputs equal up to limit 1"},
		{r1: LR(NR("wxy"), 2), r2: LR(NR("wow"), 2), want: false, desc: ", inputs unequal at limit 2"},
		{r1: LR(NR("abc"), 0), r2: LR(NR("wow"), 0), want: true, desc: ", limit 0 w/ unequal inputs"},
		{r1: NR("wxy"), r2: LR(NR("wow"), 1), want: false, wantErr: false, desc: ", unequal input EOFs"},
		{r1: NR("w"), r2: LR(NR("wow"), 1), want: true, wantErr: false, desc: ", equal input EOFs"},
		{r1: NR("w"), r2: NR("wow"), want: true, wantErr: true, desc: ", unequal input maxsize EOFs w/o LimitedReader"},
		{r1: NR(""), r2: NR("w"), want: false, wantErr: false, desc: ", unequal input EOFs w/o LimitedReader"},
	}

	for _, v := range tests {
		eq, err := c.CompareReader(v.r1, v.r2) // Should ignore MaxSize
		if eq != v.want {
			t.Errorf("CompareReader() got %v expected %v%v", eq, v.want, v.desc)
		}
		if !v.wantErr && err != nil {
			t.Errorf("unexpected error: %v%v", err, v.desc)
		}
		if v.wantErr && err == nil {
			t.Errorf("missing expected error%v", v.desc)
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
		wantErr bool
	}{
		{s1: "abc", s2: "xyz", bufSize: 2, maxSize: 1, want: false, wantErr: false}, // no letters match
		{s1: "wbc", s2: "wyz", bufSize: 2, maxSize: 1, want: true, wantErr: true},   // one letter matches
		{s1: "woz", s2: "wow", bufSize: 2, maxSize: 1, want: true, wantErr: true},   // two letters match
		{s1: "woz", s2: "wow", bufSize: 2, maxSize: 2, want: true, wantErr: true},   // 2 letters, 2 size

		// Make sure the buffer size doesn't affect the results
		{s1: "wow", s2: "wow", bufSize: 2, maxSize: 1, want: true, wantErr: true},  // three letters match
		{s1: "wow", s2: "wow", bufSize: 8, maxSize: 1, want: true, wantErr: true},  // three letters match
		{s1: "wow", s2: "wow", bufSize: 2, maxSize: 2, want: true, wantErr: true},  // three letters match
		{s1: "wow", s2: "wow", bufSize: 8, maxSize: 2, want: true, wantErr: true},  // three letters match
		{s1: "wow", s2: "wow", bufSize: 2, maxSize: 3, want: true, wantErr: false}, // three letters match
		{s1: "wow", s2: "wow", bufSize: 8, maxSize: 3, want: true, wantErr: false}, // three letters match
	}

	for _, v := range tests {
		c := New(make([]byte, v.bufSize), Options{MaxSize: int64(v.maxSize), Debug: debug})
		eq, err := c.CompareReader(strings.NewReader(v.s1), strings.NewReader(v.s2))
		if eq != v.want {
			t.Errorf("CompareReader() got %v expected %v. s1: %v s2: %v, buf: %v, maxSz: %v",
				eq, v.want, v.s1, v.s2, v.bufSize, v.maxSize)
		}
		if !v.wantErr && err != nil {
			t.Errorf("unexpected error when MaxSize not reached. s1: %v s2: %v, buf: %v, maxSz: %v",
				v.s1, v.s2, v.bufSize, v.maxSize)
		}
		if v.wantErr && err == nil {
			t.Errorf("missing expected error when MaxSize reached. s1: %v s2: %v, buf: %v, maxSz: %v",
				v.s1, v.s2, v.bufSize, v.maxSize)
		}
	}
}

type equalThenUnequalReader struct {
	n            int
	ValueCount   int
	Value        byte
	PostMaxValue byte
}

func newEqualThenUnequal(N int, value byte, post byte) *equalThenUnequalReader {
	return &equalThenUnequalReader{ValueCount: N, Value: value, PostMaxValue: post}
}

func (r *equalThenUnequalReader) Read(p []byte) (n int, err error) {
	var i int
	for i < len(p) {
		if r.n < r.ValueCount {
			p[i] = r.Value
		} else {
			p[i] = r.PostMaxValue
		}
		r.n++
		i++
	}
	return i, nil
}

// This is just a test of a helper used in other tests.
func TestEqualThenUnequalHelper(t *testing.T) {
	ER := newEqualThenUnequal
	var tests = []struct {
		r    io.Reader
		want []byte
	}{
		{r: ER(0, 'a', 'b'), want: []byte{'b', 'b', 'b'}},
		{r: ER(1, 'a', 'b'), want: []byte{'a', 'b', 'b'}},
		{r: ER(2, 'a', 'b'), want: []byte{'a', 'a', 'b'}},
		{r: ER(3, 'a', 'b'), want: []byte{'a', 'a', 'a'}},
	}
	for _, v := range tests {
		buf := make([]byte, len(v.want))
		n, err := v.r.Read(buf)
		if err != nil || n < len(v.want) {
			t.Fatalf("unexpected Read() failure for equalThenUnequalReader, got n=%v, %v expected %v %v", n, buf, v.want, err)
		}
		if !bytes.Equal(buf, v.want) {
			t.Errorf("equalThenUnequalReader.Read() got %v expected %v", buf, v.want)
		}
	}
}

// This is a test to ensure that reading an io.Reader doesn't include bytes
// beyond the MaxSize limit in the equality check.  Only bytes up to and
// including the MaxSize (or until an EOF) are compared for equality.
//
// It also confirms the equality result is not dependent on the buffer size,
// and that using LimitedReader works to restrict errors from exceeding the
// MaxSize (when MaxSize is not less than the LimitedReader limit).
func TestCompareReadersMaxSize(t *testing.T) {
	debug := os.Getenv("DEBUG") != ""

	const MaxUint = ^uint(0)
	const MaxInt = int(MaxUint >> 1)

	// Produce Readers that are equal up to MaxSize, but unequal after.
	LR := io.LimitReader
	ER := newEqualThenUnequal
	var tests = []struct {
		r1, r2  io.Reader
		want    bool
		wantErr bool
		maxSize int
		bufSize int
		desc    string
	}{
		// Since MaxSize == 1, we should get true, since Readers are equal up to the
		// MaxSize.  Read()s beyond MaxSize shouldn't affect equality result.  May be
		// dependent on bufSize.
		{r1: ER(1, 'a', 'b'), r2: ER(1, 'a', 'c'), want: true, wantErr: true, maxSize: 1, desc: ", test 1a"},
		{r1: ER(1, 'a', 'b'), r2: ER(1, 'a', 'c'), want: true, wantErr: true, maxSize: 1, bufSize: 2, desc: ", test 1b"},
		{r1: ER(2, 'a', 'b'), r2: ER(2, 'a', 'c'), want: true, wantErr: true, maxSize: 1, desc: ", test 1c"},
		{r1: ER(2, 'a', 'b'), r2: ER(2, 'a', 'c'), want: true, wantErr: true, maxSize: 1, bufSize: 2, desc: ", test 1d"},
		{r1: ER(2, 'a', 'b'), r2: ER(2, 'a', 'c'), want: true, wantErr: true, maxSize: 2, desc: ", test 1e"},
		{r1: ER(2, 'a', 'b'), r2: ER(2, 'a', 'c'), want: true, wantErr: true, maxSize: 2, bufSize: 2, desc: ", test 1f"},
		// Verify inequality as well
		{r1: ER(1, 'a', 'b'), r2: ER(1, 'z', 'b'), want: false, wantErr: false, maxSize: 1, desc: ", test 1g"},
		{r1: ER(1, 'a', 'b'), r2: ER(1, 'z', 'b'), want: false, wantErr: false, maxSize: 1, bufSize: 2, desc: ", test 1h"},
		{r1: ER(2, 'a', 'b'), r2: ER(2, 'z', 'b'), want: false, wantErr: false, maxSize: 1, desc: ", test 1i"},
		{r1: ER(2, 'a', 'b'), r2: ER(2, 'z', 'b'), want: false, wantErr: false, maxSize: 1, bufSize: 2, desc: ", test 1j"},

		// Since Readers are unequal before the MaxSize, we should get false, and no
		// error.  May be dependent on read buffer size.
		{r1: ER(1, 'a', 'b'), r2: ER(1, 'a', 'c'), want: false, wantErr: false, maxSize: 2, desc: ", test 2a"},
		{r1: ER(1, 'a', 'b'), r2: ER(1, 'a', 'c'), want: false, wantErr: false, maxSize: 2, bufSize: 2, desc: ", test 2b"},

		// Since LimitedReader used for both Readers, MaxSize is ignored
		{r1: LR(ER(1, 'a', 'b'), 1), r2: LR(ER(1, 'a', 'c'), 1), want: true, wantErr: false, maxSize: 1, desc: ", test 3a"},
		{r1: LR(ER(1, 'a', 'b'), 1), r2: LR(ER(1, 'a', 'c'), 1), want: true, wantErr: false, maxSize: 1, bufSize: 2, desc: ", test 3b"},
		{r1: LR(ER(2, 'a', 'b'), 1), r2: LR(ER(2, 'a', 'c'), 1), want: true, wantErr: false, maxSize: 2, desc: ", test 3c"},
		{r1: LR(ER(2, 'a', 'b'), 1), r2: LR(ER(2, 'a', 'c'), 1), want: true, wantErr: false, maxSize: 2, bufSize: 2, desc: ", test 3d"},
		{r1: LR(ER(2, 'a', 'b'), 2), r2: LR(ER(2, 'a', 'c'), 2), want: true, wantErr: false, maxSize: 2, desc: ", test 3e"},
		{r1: LR(ER(2, 'a', 'b'), 2), r2: LR(ER(2, 'a', 'c'), 2), want: true, wantErr: false, maxSize: 2, bufSize: 2, desc: ", test 3f"},
		// Check that inequality works also for LimitedReaders (without errors)
		{r1: LR(ER(1, 'a', 'b'), 1), r2: LR(ER(1, 'z', 'b'), 1), want: false, wantErr: false, maxSize: 1, desc: ", test 3g"},
		{r1: LR(ER(1, 'a', 'b'), 1), r2: LR(ER(1, 'z', 'b'), 1), want: false, wantErr: false, maxSize: 1, bufSize: 2, desc: ", test 3h"},
		{r1: LR(ER(2, 'a', 'b'), 2), r2: LR(ER(2, 'z', 'b'), 2), want: false, wantErr: false, maxSize: 2, desc: ", test 3i"},
		{r1: LR(ER(2, 'a', 'b'), 2), r2: LR(ER(2, 'z', 'b'), 2), want: false, wantErr: false, maxSize: 2, bufSize: 2, desc: ", test 3j"},

		// Also check with mixed LimitedReader and non-LimitedReader (unequal because the
		// non-LimitedReader in these tests will always return more data than the LimitedReader)
		{r1: LR(ER(1, 'a', 'b'), 1), r2: ER(1, 'a', 'c'), want: false, wantErr: false, maxSize: 1, desc: ", test 4a"},
		{r1: LR(ER(1, 'a', 'b'), 1), r2: ER(1, 'a', 'c'), want: false, wantErr: false, maxSize: 1, bufSize: 2, desc: ", test 4b"},
		{r1: ER(2, 'a', 'b'), r2: LR(ER(2, 'a', 'c'), 2), want: false, wantErr: false, maxSize: 2, desc: ", test 4c"},
		{r1: ER(2, 'a', 'b'), r2: LR(ER(2, 'a', 'c'), 2), want: false, wantErr: false, maxSize: 2, bufSize: 2, desc: ", test 4d"},
		{r1: LR(ER(1, 'a', 'b'), 1), r2: ER(1, 'z', 'b'), want: false, wantErr: false, maxSize: 1, desc: ", test 4e"},
		{r1: LR(ER(1, 'a', 'b'), 1), r2: ER(1, 'z', 'b'), want: false, wantErr: false, maxSize: 1, bufSize: 2, desc: ", test 4f"},
		{r1: LR(ER(2, 'a', 'b'), 2), r2: ER(2, 'z', 'b'), want: false, wantErr: false, maxSize: 2, desc: ", test 4g"},
		{r1: LR(ER(2, 'a', 'b'), 2), r2: ER(2, 'z', 'b'), want: false, wantErr: false, maxSize: 2, bufSize: 2, desc: ", test 4h"},

		// Try with MaxSize < LimitReader limits, to show that MaxSize is ignored by
		// CompareReader() when given one or more LimitedReader
		{r1: LR(ER(1, 'a', 'b'), 2), r2: LR(ER(1, 'a', 'c'), 2), want: false, wantErr: false, maxSize: 1, desc: ", test 5a"},
		{r1: LR(ER(1, 'a', 'b'), 2), r2: LR(ER(1, 'a', 'c'), 2), want: false, wantErr: false, maxSize: 1, bufSize: 2, desc: ", test 5b"},
		{r1: LR(ER(2, 'a', 'b'), 2), r2: LR(ER(2, 'a', 'c'), 2), want: true, wantErr: false, maxSize: 1, desc: ", test 5c"},
		{r1: LR(ER(2, 'a', 'b'), 2), r2: LR(ER(2, 'a', 'c'), 2), want: true, wantErr: false, maxSize: 1, bufSize: 2, desc: ", test 5d"},

		// Remove MaxSize from the equation.  Shouldn't return any errors.  Either
		// unequal before EOF, or equal up to EOF.
		{r1: ER(1, 'a', 'b'), r2: ER(1, 'a', 'c'), want: false, wantErr: false, maxSize: MaxInt, desc: ", test 6a"},
		{r1: LR(ER(1, 'a', 'b'), 1), r2: LR(ER(1, 'a', 'c'), 1), want: true, wantErr: false, maxSize: MaxInt, desc: ", test 6b"},
	}

	for _, v := range tests {
		var buf []byte
		if v.bufSize >= 0 {
			buf = make([]byte, v.bufSize)
		}
		c := New(buf, Options{MaxSize: int64(v.maxSize), Debug: debug})

		eq, err := c.CompareReader(v.r1, v.r2)
		if eq != v.want {
			t.Errorf("CompareReader() got %v expected %v%v", eq, v.want, v.desc)
		}
		if !v.wantErr && err != nil {
			t.Errorf("unexpected error: %v%v", err, v.desc)
		}
		if v.wantErr && err == nil {
			t.Errorf("expected error, but got none: %v", v.desc)
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

func compare(t *testing.T, c *Cmp, path1, path2 string, expect int) {
	//t.Logf("compare(%s,%s) limit=%d buf=%d", path1, path2, c.Opt.MaxSize, len(c.buf))
	equal, err := c.CompareFile(path1, path2)
	if err != nil {
		if expect != expectError {
			t.Errorf("compare: unexpected error: CompareFile(%s,%s,%d,%d): %v", path1, path2, c.Opt.MaxSize, len(c.buf), err)
		}
		return
	}
	if equal {
		if expect != expectEqual {
			t.Errorf("compare: unexpected equal: CompareFile(%s,%s,%d,%d)", path1, path2, c.Opt.MaxSize, len(c.buf))
		}
		return
	}
	if expect != expectUnequal {
		t.Errorf("compare: unexpected unequal: CompareFile(%s,%s,%d,%d)", path1, path2, c.Opt.MaxSize, len(c.buf))
	}
}

func compareExpectErrorAndEqual(t *testing.T, c *Cmp, path1, path2 string) {
	//t.Logf("compare(%s,%s) limit=%d buf=%d", path1, path2, c.Opt.MaxSize, len(c.buf))
	equal, err := c.CompareFile(path1, path2)
	if err == nil {
		t.Errorf("compareExpectErrorAndEqual: unexpected non-error: CompareFile(%s,%s,%d,%d): %v", path1, path2, c.Opt.MaxSize, len(c.buf), err)
	}
	if !equal {
		t.Errorf("compareExpectErrorAndEqual: unexpected unequal: CompareFile(%s,%s,%d,%d)", path1, path2, c.Opt.MaxSize, len(c.buf))
	}
}
