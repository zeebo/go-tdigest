package tdigest

import "testing"

func TestFenwickTree(t *testing.T) {
	var f fen

	assertSum := func(i int, v uint32) {
		if got := f.Sum(i); got != v {
			t.Logf("fen: %v", f)
			t.Errorf("sum %d: got %v != exp %v", i, got, v)
		}
	}

	assertGet := func(i int, v uint32) {
		if got := f.Get(i); got != v {
			t.Logf("fen: %v", f)
			t.Errorf("get %d: got %v != exp %v", i, got, v)
		}
	}

	f.Set(0, 1)
	f.Set(1, 1)
	f.Set(2, 2)
	f.Set(3, 1)
	f.Set(4, 1)

	assertSum(0, 0)
	assertGet(0, 1)
	assertSum(1, 1)
	assertGet(1, 1)
	assertSum(2, 2)
	assertGet(2, 2)
	assertSum(3, 4)
	assertGet(3, 1)
	assertSum(4, 5)
	assertGet(4, 1)
	assertSum(5, 6)

	f.Set(2, 5)

	assertSum(0, 0)
	assertGet(0, 1)
	assertSum(1, 1)
	assertGet(1, 1)
	assertSum(2, 2)
	assertGet(2, 5)
	assertSum(3, 7)
	assertGet(3, 1)
	assertSum(4, 8)
	assertGet(4, 1)
	assertSum(5, 9)
}
