package tdigest

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
)

const smallEncoding int32 = 2

// Marshal serializes the digest into a byte array so it can be
// saved to disk or sent over the wire. buf is used as a backing array, but
// the returned array may be different if it does not fit.
func (t TDigest) Marshal(buf []byte) []byte {
	var scratch [8]byte

	binary.BigEndian.PutUint32(scratch[:], uint32(smallEncoding))
	buf = append(buf, scratch[:4]...)

	binary.BigEndian.PutUint64(scratch[:], math.Float64bits(t.compression))
	buf = append(buf, scratch[:8]...)

	binary.BigEndian.PutUint32(scratch[:], uint32(t.summary.Len()))
	buf = append(buf, scratch[:4]...)

	var x float64
	t.summary.ForEach(func(mean float64, count uint32) bool {
		delta := mean - x
		x = mean

		var scratch [4]byte
		binary.BigEndian.PutUint32(scratch[:], math.Float32bits(float32(delta)))
		buf = append(buf, scratch[:4]...)

		return true
	})

	t.summary.ForEach(func(mean float64, count uint32) bool {
		buf = encodeUint32(buf, count)
		return true
	})

	return buf
}

// and deserializes it. It will panic if the byte slice is not large enough to
// decode.
func FromBytes(buf []byte) (t *TDigest, err error) {
	encoding := int32(binary.BigEndian.Uint32(buf))
	buf = buf[4:]

	if encoding != smallEncoding {
		return nil, fmt.Errorf("Unsupported encoding version: %d", encoding)
	}

	compression := math.Float64frombits(binary.BigEndian.Uint64(buf))
	buf = buf[8:]

	t = New(compression)

	numCentroids := int32(binary.BigEndian.Uint32(buf))
	buf = buf[4:]

	if numCentroids < 0 || numCentroids > 1<<22 {
		return nil, errors.New("bad number of centroids in serialization")
	}

	means := make([]float64, numCentroids)
	var x float64
	for i := 0; i < int(numCentroids); i++ {
		delta := float64(math.Float32frombits(binary.BigEndian.Uint32(buf)))
		buf = buf[4:]

		x += delta
		means[i] = x
	}

	for i := 0; i < int(numCentroids); i++ {
		var decUint uint32
		decUint, buf, err = decodeUint32(buf)
		if err != nil {
			return nil, err
		}

		err = t.AddWeighted(means[i], decUint)
		if err != nil {
			return nil, err
		}
	}

	return t, nil
}

func encodeUint32(buf []byte, n uint32) []byte {
	var b [binary.MaxVarintLen32]byte
	l := binary.PutUvarint(b[:], uint64(n))
	return append(buf, b[:l]...)
}

func decodeUint32(buf []byte) (uint32, []byte, error) {
	v, n := binary.Uvarint(buf)
	if v > 0xffffffff {
		return 0, nil, fmt.Errorf("value too large: %d", v)
	}
	return uint32(v), buf[n:], nil
}
