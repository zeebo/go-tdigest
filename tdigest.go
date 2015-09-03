// Package tdigest provides a highly accurate mergeable data-structure
// for quantile estimation.
package tdigest

import (
	"fmt"
	"math"
	"math/rand"
)

// TDigest is a quantile summary structure
type TDigest struct {
	summary     *summary
	compression float64
	count       uint32
}

// New creates a new digest.
// The compression parameter rules the threshold in which samples are
// merged together - the more often distinct samples are merged the more
// precision is lost. Compression should be tuned according to your data
// distribution, but a value of 10 is often good enough. A higher
// compression value means holding more centroids in memory, which means
// a bigger serialization payload and higher memory footprint.
func New(compression float64) *TDigest {
	return &TDigest{
		compression: compression,
		summary:     newSummary(estimateCapacity(compression)),
		count:       0,
	}
}

// Quantile returns the desired percentile estimation.
// Values of p must be between 0 and 1 (inclusive), will panic otherwise.
func (t *TDigest) Quantile(q float64) float64 {
	if q < 0 || q > 1 {
		panic("q must be between 0 and 1 (inclusive)")
	}

	if t.summary.Len() == 0 {
		return math.NaN()
	} else if t.summary.Len() == 1 {
		return t.summary.Min().mean
	}

	q *= float64(t.count)
	var total float64
	i := 0

	found := false
	var result float64

	t.summary.Iterate(func(item centroid) bool {
		k := float64(item.count)

		if q < total+k {
			if i == 0 || i+1 == t.summary.Len() {
				result = item.mean
				found = true
				return false
			}
			succ, pred := t.summary.successorAndPredecessorItems(item.mean)
			delta := (succ.mean - pred.mean) / 2
			result = item.mean + ((q-total)/k-0.5)*delta
			found = true
			return false
		}

		i++
		total += k
		return true
	})

	if found {
		return result
	}
	return t.summary.Max().mean
}

// Add registers a new sample in the digest.
// It's the main entry point for the digest and very likely the only
// method to be used for collecting samples. The count parameter is for
// when you are registering a sample that occurred multiple times - the
// most common value for this is 1.
func (t *TDigest) Add(value float64, count uint32) error {

	if count == 0 {
		return fmt.Errorf("Illegal datapoint <value: %.4f, count: %d>", value, count)
	}

	t.count += count

	if t.summary.Len() == 0 {
		t.addCentroid(value, count)
		return nil
	}

	candidates := t.findNearestCentroids(value)

	for len(candidates) > 0 && count > 0 {
		j := rand.Intn(len(candidates))
		chosen := candidates[j]

		quantile := t.computeCentroidQuantile(chosen)

		if float64(chosen.count+count) > t.threshold(quantile) {
			candidates = append(candidates[:j], candidates[j+1:]...)
			continue
		}

		deltaW := math.Min(t.threshold(quantile)-float64(chosen.count), float64(count))
		t.updateCentroid(chosen, value, uint32(deltaW))
		count -= uint32(deltaW)

		candidates = append(candidates[:j], candidates[j+1:]...)
	}

	if count > 0 {
		t.addCentroid(value, count)
	}

	if float64(t.summary.Len()) > 20*t.compression {
		t.Compress()
	}

	return nil
}

// Compress tries to reduce the number of individual centroids stored
// in the digest.
// Compression trades off accuracy for performance and happens
// automatically after a certain amount of distinct samples have been
// stored.
func (t *TDigest) Compress() {
	if t.summary.Len() <= 1 {
		return
	}

	oldTree := t.summary
	t.summary = newSummary(estimateCapacity(t.compression))

	nodes := oldTree.Data()
	shuffle(nodes)

	for _, item := range nodes {
		t.Add(item.mean, item.count)
	}
}

// Merge joins a given digest into itself.
// Merging is useful when you have multiple TDigest instances running
// in separate threads and you want to compute quantiles over all the
// samples. This is particularly important on a scatter-gather/map-reduce
// scenario.
func (t *TDigest) Merge(other *TDigest) {
	if other.summary.Len() == 0 {
		return
	}

	nodes := other.summary.Data()
	shuffle(nodes)

	for _, item := range nodes {
		t.Add(item.mean, item.count)
	}
}

func shuffle(data []centroid) {
	for i := len(data) - 1; i > 1; i-- {
		other := rand.Intn(i + 1)
		tmp := data[other]
		data[other] = data[i]
		data[i] = tmp
	}
}

func estimateCapacity(compression float64) uint {
	return uint(compression) * 10
}

func (t *TDigest) updateCentroid(c *centroid, mean float64, count uint32) {
	idx := t.summary.FindIndex(c.mean)

	if !t.summary.meanAtIndexIs(idx, c.mean) {
		panic(fmt.Sprintf("Trying to update a centroid that doesn't exist: %v. %v", c, t.summary))
	}

	t.summary.updateAt(idx, mean, count)
}

func (t *TDigest) threshold(q float64) float64 {
	return (4 * float64(t.count) * q * (1 - q)) / t.compression
}

func (t *TDigest) computeCentroidQuantile(c *centroid) float64 {
	cumSum := t.summary.sumUntilMean(c.mean)
	return (float64(c.count)/2.0 + float64(cumSum)) / float64(t.count)
}

func (t *TDigest) addCentroid(mean float64, count uint32) {
	current := t.summary.Find(mean)

	if current.isValid() {
		removed := t.summary.Remove(current.mean)
		removed.Update(mean, count)
		// FIXME oftentimes this can be done inplace. Care?
		t.summary.Add(removed.mean, removed.count)
	} else {
		t.summary.Add(mean, count)
	}
}

func (t *TDigest) findNearestCentroids(mean float64) []*centroid {
	ceil, floor := t.summary.ceilingAndFloorItems(mean)

	if !ceil.isValid() && !floor.isValid() {
		panic("findNearestCentroids called on an empty tree")
	}

	if !ceil.isValid() {
		return []*centroid{&floor}
	}

	if !floor.isValid() {
		return []*centroid{&ceil}
	}

	if math.Abs(floor.mean-mean) < math.Abs(ceil.mean-mean) {
		return []*centroid{&floor}
	} else if math.Abs(floor.mean-mean) == math.Abs(ceil.mean-mean) && floor.mean != ceil.mean {
		return []*centroid{&floor, &ceil}
	} else {
		return []*centroid{&ceil}
	}
}
