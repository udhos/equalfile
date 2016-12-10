// +build !windows

package equalfile

import (
	"crypto/sha256"
	"testing"
)

func TestCompareLimitBroken(t *testing.T) {
	SetOptions(Options{ForceFileRead: true})
	CompareSingle()
	rejectMultiple()
	buf := make([]byte, 1000)
	compare(t, -1, nil, "/etc/passwd", "/etc/passwd", expectError)
	compare(t, 0, buf, "/etc/passwd", "/etc/passwd", expectError)
	compare(t, 1, buf, "/etc/passwd", "/etc/passwd", expectError) // will reach 1-byte limit
	compare(t, 1000000, buf, "/etc/passwd", "/etc/passwd", expectEqual)
}

func TestCompareBufBroken(t *testing.T) {
	SetOptions(Options{ForceFileRead: true})
	CompareSingle()
	rejectMultiple()
	var limit int64 = 1000000
	compare(t, limit, nil, "/etc/passwd", "/etc/passwd", expectError)
	compare(t, limit, make([]byte, 0), "/etc/passwd", "/etc/passwd", expectError)
	compare(t, limit, make([]byte, 1), "/etc/passwd", "/etc/passwd", expectError)
	compare(t, limit, make([]byte, 2), "/etc/passwd", "/etc/passwd", expectEqual)
}

func TestCompareBufSmall(t *testing.T) {
	CompareSingle()
	rejectMultiple()
	batch(t, 1000000, make([]byte, 10))
}

func TestCompareBufLarge(t *testing.T) {
	CompareSingle()
	rejectMultiple()
	batch(t, 100000000, make([]byte, 10000000))
}

func TestCompareLimitBrokenMultiple(t *testing.T) {
	SetOptions(Options{ForceFileRead: true})
	CompareSingle()
	rejectMultiple()
	CompareMultiple(sha256.New(), true)
	requireMultiple()
	buf := make([]byte, 1000)
	compare(t, -1, nil, "/etc/passwd", "/etc/passwd", expectError)
	compare(t, 0, buf, "/etc/passwd", "/etc/passwd", expectError)
	compare(t, 1, buf, "/etc/passwd", "/etc/passwd", expectError) // will reach 1-byte limit
	compare(t, 1000000, buf, "/etc/passwd", "/etc/passwd", expectEqual)
}

func TestCompareBufBrokenMultiple(t *testing.T) {
	SetOptions(Options{ForceFileRead: true})
	CompareSingle()
	rejectMultiple()
	CompareMultiple(sha256.New(), true)
	requireMultiple()
	var limit int64 = 1000000
	compare(t, limit, nil, "/etc/passwd", "/etc/passwd", expectError)
	compare(t, limit, make([]byte, 0), "/etc/passwd", "/etc/passwd", expectError)
	compare(t, limit, make([]byte, 1), "/etc/passwd", "/etc/passwd", expectError)
	compare(t, limit, make([]byte, 2), "/etc/passwd", "/etc/passwd", expectEqual)
}

func TestCompareBufSmallMultiple(t *testing.T) {
	CompareSingle()
	rejectMultiple()
	CompareMultiple(sha256.New(), true)
	requireMultiple()
	batch(t, 1000000, make([]byte, 10))
}

func TestCompareBufLargeMultiple(t *testing.T) {
	CompareSingle()
	rejectMultiple()
	CompareMultiple(sha256.New(), true)
	requireMultiple()
	batch(t, 100000000, make([]byte, 10000000))
}

func batch(t *testing.T, limit int64, buf []byte) {
	SetOptions(Options{ForceFileRead: true})
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
