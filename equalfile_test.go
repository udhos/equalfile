package equalfile

import (
	"fmt"
	"io"
	"testing"
)

const (
	expectError   = 0
	expectEqual   = 1
	expectUnequal = 2
)

func TestCompareLimitBroken(t *testing.T) {
	buf := make([]byte, 1000)
	compare(t, -1, nil, "/etc/passwd", "/etc/passwd", expectError)
	compare(t, 0, buf, "/etc/passwd", "/etc/passwd", expectError)
	compare(t, 1, buf, "/etc/passwd", "/etc/passwd", expectError) // will reach 1-byte limit
	compare(t, 1000000, buf, "/etc/passwd", "/etc/passwd", expectEqual)
}

func TestCompareBufBroken(t *testing.T) {
	var limit int64 = 1000000
	compare(t, limit, nil, "/etc/passwd", "/etc/passwd", expectError)
	compare(t, limit, make([]byte, 0), "/etc/passwd", "/etc/passwd", expectError)
	compare(t, limit, make([]byte, 1), "/etc/passwd", "/etc/passwd", expectError)
	compare(t, limit, make([]byte, 2), "/etc/passwd", "/etc/passwd", expectEqual)
}

func TestCompareBufSmall(t *testing.T) {
	batch(t, 1000000, make([]byte, 10))
}

func TestCompareBufLarge(t *testing.T) {
	batch(t, 100000000, make([]byte, 10000000))
}

func TestBrokenReaders(t *testing.T) {
	limit := int64(50000)
	buf := make([]byte, 4000)
	chunk := int64(1000)
	total := int64(10000)

	r1 := &testReader{label: "test1 r1", chunkSize: chunk, totalSize: total}
	r2 := &testReader{label: "test1 r2", chunkSize: chunk + 1, totalSize: total}
	compareReader(t, limit, buf, r1, r2, expectError)

	r1 = &testReader{label: "test2 r1", chunkSize: chunk, totalSize: total}
	r2 = &testReader{label: "test2 r2", chunkSize: chunk, totalSize: total}
	compareReader(t, limit, buf, r1, r2, expectEqual)

	r1 = &testReader{label: "test3 r1", chunkSize: chunk, totalSize: total, lastByte: '0'}
	r2 = &testReader{label: "test3 r2", chunkSize: chunk, totalSize: total, lastByte: '1'}
	compareReader(t, limit, buf, r1, r2, expectUnequal)
}

type testReader struct {
	label     string
	chunkSize int64
	totalSize int64
	lastByte  byte
}

func (r *testReader) Read(buf []byte) (int, error) {

	n, err := testRead(r, buf)

	if debug {
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

func batch(t *testing.T, limit int64, buf []byte) {
	compare(t, limit, buf, "/etc", "/etc", expectError)
	compare(t, limit, buf, "/etc/ERROR", "/etc/passwd", expectError)
	compare(t, limit, buf, "/etc/passwd", "/etc/ERROR", expectError)
	compare(t, limit, buf, "/etc/passwd", "/etc/passwd", expectEqual)
	compare(t, limit, buf, "/etc/passwd", "/etc/group", expectUnequal)
	compare(t, limit, buf, "/dev/null", "/dev/null", expectEqual)
	compare(t, limit, buf, "/dev/urandom", "/dev/urandom", expectUnequal)
	compare(t, limit, buf, "/dev/zero", "/dev/zero", expectError)
}

func compare(t *testing.T, limit int64, buf []byte, path1, path2 string, expect int) {
	equal, err := CompareFileBufLimit(path1, path2, buf, limit)
	if err != nil {
		if expect != expectError {
			t.Errorf("compare: unexpected error: CompareFileBufLimit(%s,%s,%d,%d): %v", path1, path2, limit, len(buf), err)
		}
		return
	}
	if equal {
		if expect != expectEqual {
			t.Errorf("compare: unexpected equal: CompareFileBufLimit(%s,%s,%d,%d)", path1, path2, limit, len(buf))
		}
		return
	}
	if expect != expectUnequal {
		t.Errorf("compare: unexpected unequal: CompareFileBufLimit(%s,%s,%d,%d)", path1, path2, limit, len(buf))
	}
}

func compareReader(t *testing.T, limit int64, buf []byte, r1, r2 io.Reader, expect int) {
	equal, err := CompareReaderBufLimit(r1, r2, buf, limit)
	if err != nil {
		if expect != expectError {
			t.Errorf("compare: unexpected error: CompareReaderBufLimit(%d,%d): %v", limit, len(buf), err)
		}
		return
	}
	if equal {
		if expect != expectEqual {
			t.Errorf("compare: unexpected equal: CompareReaderBufLimit(%d,%d)", limit, len(buf))
		}
		return
	}
	if expect != expectUnequal {
		t.Errorf("compare: unexpected unequal: CompareReaderBufLimit(%d,%d)", limit, len(buf))
	}
}
