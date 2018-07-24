package equalfile

import (
	"fmt"
	"io"
	"os"
	"testing"
)

const (
	expectError   = 0
	expectEqual   = 1
	expectUnequal = 2
)

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
