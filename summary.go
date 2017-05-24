package tdigest

import (
	"fmt"
	"math"
)

type centroid struct {
	mean  float64
	count uint32
	index int
}

func (c centroid) isValid() bool {
	return !math.IsNaN(c.mean) && c.count > 0
}

func (c *centroid) Update(x float64, weight uint32) {
	c.count += weight
	c.mean += float64(weight) * (x - c.mean) / float64(c.count)
}

var invalidCentroid = centroid{mean: math.NaN(), count: 0}

type summary struct {
	keys   []float64
	counts []uint32
	fen    fen
}

func newSummary(initialCapacity uint) *summary {
	return &summary{
		keys:   make([]float64, 0, initialCapacity),
		counts: make([]uint32, 0, initialCapacity),
		fen:    fen{buf: make([]uint32, initialCapacity)},
	}
}

func (s summary) Len() int {
	return len(s.keys)
}

func (s *summary) Add(key float64, value uint32) error {
	if math.IsNaN(key) {
		return fmt.Errorf("Key must not be NaN")
	}

	if value == 0 {
		return fmt.Errorf("Count must be >0")
	}

	idx := s.FindIndex(key)

	if s.meanAtIndexIs(idx, key) {
		s.updateAt(idx, key, value)
		return nil
	}

	s.keys = append(s.keys, 0)
	copy(s.keys[idx+1:], s.keys[idx:])

	s.counts = append(s.counts, 0)
	copy(s.counts[idx+1:], s.counts[idx:])

	// man i wish there was a better way to handle an insertion, but Add
	// appears to happen far less than UpdateAt, and when Add does happen,
	// the size of keycounts is small. There may be some optimization here
	// by doing deltas, though.
	for i := idx + 1; i < len(s.keys); i++ {
		s.fen.Set(i, s.counts[i])
	}

	s.keys[idx] = key
	s.counts[idx] = value
	s.fen.Set(idx, value)

	return nil
}

func (s summary) Find(x float64) centroid {
	idx := s.FindIndex(x)

	if idx < s.Len() && s.keys[idx] == x {
		return centroid{x, s.counts[idx], idx}
	}

	return invalidCentroid
}

func (s summary) FindIndex(x float64) int {
	i, j := 0, len(s.keys)
	for i < j {
		h := int(uint(i+j) >> 1)
		if s.keys[h] < x {
			i = h + 1
		} else {
			j = h
		}
	}
	return i
}

func (s summary) At(index int) centroid {
	if s.Len()-1 < index || index < 0 {
		return invalidCentroid
	}

	return centroid{s.keys[index], s.counts[index], index}
}

func (s summary) Iterate(f func(c centroid) bool) {
	for i := 0; i < s.Len(); i++ {
		if !f(centroid{s.keys[i], s.counts[i], i}) {
			break
		}
	}
}

func (s summary) Min() centroid {
	return s.At(0)
}

func (s summary) Max() centroid {
	return s.At(s.Len() - 1)
}

func (s summary) Data() []centroid {
	data := make([]centroid, 0, s.Len())
	s.Iterate(func(c centroid) bool {
		data = append(data, c)
		return true
	})
	return data
}

func (s summary) successorAndPredecessorItems(mean float64) (centroid, centroid) {
	idx := s.FindIndex(mean)
	return s.At(idx + 1), s.At(idx - 1)
}

func (s summary) ceilingAndFloorItems(mean float64) (centroid, centroid) {
	idx := s.FindIndex(mean)

	// Case 1: item is greater than all items in the summary
	if idx == s.Len() {
		return invalidCentroid, s.Max()
	}

	item := s.At(idx)

	// Case 2: item exists in the summary
	if item.isValid() && mean == item.mean {
		return item, item
	}

	// Case 3: item is smaller than all items in the summary
	if idx == 0 {
		return s.Min(), invalidCentroid
	}

	return item, s.At(idx - 1)
}

func (s *summary) sumUntilMean(mean float64) uint32 {
	return s.fen.Sum(s.FindIndex(mean))
}

func (s *summary) updateAt(index int, mean float64, count uint32) {
	old_key := s.keys[index]
	old_count := s.counts[index]
	c := centroid{old_key, old_count, index}
	c.Update(mean, count)

	s.keys[index] = c.mean
	s.counts[index] = c.count
	s.fen.Add(index, c.count-old_count)

	if c.mean > old_key {
		s.adjustRight(index)
	} else if c.mean < old_key {
		s.adjustLeft(index)
	}
}

func (s *summary) adjustRight(index int) {
	amount := 0
	for i := index + 1; i < len(s.keys) && s.keys[i-1] > s.keys[i]; i++ {
		s.keys[i-1], s.keys[i] = s.keys[i], s.keys[i-1]
		s.counts[i-1], s.counts[i] = s.counts[i], s.counts[i-1]
		amount++
	}
	for i := 0; i < amount; i++ {
		j := index + i + 1
		s.fen.Set(j, s.counts[j])
	}
}

func (s *summary) adjustLeft(index int) {
	amount := 0
	for i := index - 1; i >= 0 && s.keys[i] > s.keys[i+1]; i-- {
		s.keys[i], s.keys[i+1] = s.keys[i+1], s.keys[i]
		s.counts[i], s.counts[i+1] = s.counts[i+1], s.counts[i]
		amount++
	}
	for i := 0; i < amount; i++ {
		j := index - i - 1
		s.fen.Set(j, s.counts[j])
	}
}

func (s summary) meanAtIndexIs(index int, mean float64) bool {
	return index < len(s.keys) && s.keys[index] == mean
}
