package tdigest

import (
	"math/rand"
	"testing"
)

func assertNoError(t testing.TB, err error) {
	t.Helper()

	if err != nil {
		t.Fatal(err)
	}
}

func TestEncodeDecode(t *testing.T) {
	testUints := []uint32{0, 10, 100, 1000, 10000, 65535, 2147483647}
	var buf []byte

	for _, i := range testUints {
		buf = encodeUint32(buf, i)
	}

	for _, i := range testUints {
		var j uint32
		var err error

		j, buf, err = decodeUint32(buf)
		assertNoError(t, err)

		if i != j {
			t.Errorf("Basic encode/decode failed. Got %d, wanted %d", j, i)
		}
	}
}

func TestSerialization(t *testing.T) {
	t1 := New(100)
	for i := 0; i < 100; i++ {
		assertNoError(t, t1.Add(rand.Float64()))
	}

	serialized := t1.Marshal(nil)

	t2, err := FromBytes(serialized)
	assertNoError(t, err)

	if t1.count != t2.count ||
		t1.summary.Len() != t2.summary.Len() ||
		t1.compression != t2.compression {

		t.Fatal("Deserialized to something different.")
	}
}

func BenchmarkSerialization(b *testing.B) {
	t := New(10)
	for i := 0; i < 10000; i++ {
		t.Add(rand.Float64())
	}

	buf := t.Marshal(nil)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		t.Marshal(buf[:0])
	}
}

func BenchmarkDeserialization(b *testing.B) {
	t := New(10)
	for i := 0; i < 1000000; i++ {
		t.Add(rand.Float64())
	}

	buf := t.Marshal(nil)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		FromBytes(buf)
	}
}
