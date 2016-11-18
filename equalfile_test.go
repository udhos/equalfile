package equalfile

import (
	"testing"
)

const (
	EXPECT_ERROR   = 0
	EXPECT_EQUAL   = 1
	EXPECT_UNEQUAL = 2
)

func TestCompare1(t *testing.T) {
	var limit int64 = 1000000

	compare(t, limit, nil, "/etc/passwd", "/etc/passwd", EXPECT_ERROR)
	compare(t, limit, make([]byte, 0), "/etc/passwd", "/etc/passwd", EXPECT_ERROR)
	compare(t, limit, make([]byte, 1), "/etc/passwd", "/etc/passwd", EXPECT_ERROR)
	compare(t, limit, make([]byte, 2), "/etc/passwd", "/etc/passwd", EXPECT_EQUAL)

	buf := make([]byte, 100)
	compare(t, limit, buf, "/etc", "/etc", EXPECT_ERROR)
	compare(t, limit, buf, "/etc/ERROR", "/etc/passwd", EXPECT_ERROR)
	compare(t, limit, buf, "/etc/passwd", "/etc/ERROR", EXPECT_ERROR)
	compare(t, limit, buf, "/etc/passwd", "/etc/passwd", EXPECT_EQUAL)
	compare(t, limit, buf, "/etc/passwd", "/etc/group", EXPECT_UNEQUAL)
	compare(t, limit, buf, "/dev/null", "/dev/null", EXPECT_EQUAL)
	compare(t, limit, buf, "/dev/urandom", "/dev/urandom", EXPECT_UNEQUAL)
	compare(t, limit, buf, "/dev/zero", "/dev/zero", EXPECT_ERROR)
}

func compare(t *testing.T, limit int64, buf []byte, path1, path2 string, expect int) {
	equal, err := CompareFileBufLimit(path1, path2, buf, limit)
	if err != nil {
		if expect != EXPECT_ERROR {
			t.Errorf("compare: unexpected error: CompareFile(%s,%s): %v", path1, path2, err)
		}
		return
	}
	if equal {
		if expect != EXPECT_EQUAL {
			t.Errorf("compare: unexpected equal: CompareFile(%s,%s)", path1, path2)
		}
		return
	}
	if expect != EXPECT_UNEQUAL {
		t.Errorf("compare: unexpected unequal: CompareFile(%s,%s)", path1, path2)
	}
}
