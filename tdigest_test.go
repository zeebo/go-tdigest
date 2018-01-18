package tdigest

import (
	"math"
	"math/rand"
	"sort"
	"testing"
	"time"
)

func init() { rand.Seed(time.Now().UnixNano()) }

// Test of tdigest internals and accuracy. Note no t.Parallel():
// during tests the default random seed is consistent, but varying
// concurrency scheduling mixes up the random values used in each test.
// Since there's a random number call inside tdigest this breaks repeatability
// for all tests. So, no test concurrency here.

func TestTInternals(t *testing.T) {
	tdigest := New(100)

	if !math.IsNaN(tdigest.Quantile(0.1)) {
		t.Errorf("Quantile() on an empty digest should return NaN. Got: %.4f", tdigest.Quantile(0.1))
	}

	if !math.IsNaN(tdigest.CDF(1)) {
		t.Errorf("CDF() on an empty digest should return NaN. Got: %.4f", tdigest.CDF(1))
	}

	_ = tdigest.Add(0.4)

	if tdigest.Quantile(0.1) != 0.4 {
		t.Errorf("Quantile() on a single-sample digest should return the samples's mean. Got %.4f", tdigest.Quantile(0.1))
	}

	if tdigest.CDF(0.3) != 0 {
		t.Errorf("CDF(x) on digest with a single centroid should return 0 if x < mean")
	}

	if tdigest.CDF(0.5) != 1 {
		t.Errorf("CDF(x) on digest with a single centroid should return 1 if x >= mean")
	}

	_ = tdigest.Add(0.5)

	if tdigest.summary.Len() != 2 {
		t.Errorf("Expected size 2, got %d", tdigest.summary.Len())
	}

	err := tdigest.AddWeighted(0, 0)

	if err == nil {
		t.Errorf("Expected AddWeighted() to error out with input (0,0)")
	}
}

func closeEnough(a float64, b float64) bool {
	const EPS = 0.000001
	if (a-b < EPS) && (b-a < EPS) {
		return true
	}
	return false
}

func assertDifferenceSmallerThan(tdigest *TDigest, p float64, m float64, t *testing.T) {
	tp := tdigest.Quantile(p)
	if math.Abs(tp-p) >= m {
		t.Errorf("T-Digest.Quantile(%.4f) = %.4f. Diff (%.4f) >= %.4f", p, tp, math.Abs(tp-p), m)
	}
}

func TestUniformDistribution(t *testing.T) {
	tdigest := New(100)

	for i := 0; i < 100000; i++ {
		_ = tdigest.Add(rand.Float64())
	}

	assertDifferenceSmallerThan(tdigest, 0.5, 0.02, t)
	assertDifferenceSmallerThan(tdigest, 0.1, 0.01, t)
	assertDifferenceSmallerThan(tdigest, 0.9, 0.01, t)
	assertDifferenceSmallerThan(tdigest, 0.01, 0.005, t)
	assertDifferenceSmallerThan(tdigest, 0.99, 0.005, t)
	assertDifferenceSmallerThan(tdigest, 0.001, 0.001, t)
	assertDifferenceSmallerThan(tdigest, 0.999, 0.001, t)
}

// Asserts quantile p is no greater than absolute m off from "true"
// fractional quantile for supplied data. So m must be scaled
// appropriately for source data range.
func assertDifferenceFromQuantile(data []float64, tdigest *TDigest, p float64, m float64, t *testing.T) {
	q := quantile(p, data)
	tp := tdigest.Quantile(p)

	if math.Abs(tp-q) >= m {
		t.Fatalf("T-Digest.Quantile(%.4f) = %.4f vs actual %.4f. Diff (%.4f) >= %.4f", p, tp, q, math.Abs(tp-q), m)
	}
}

func TestSequentialInsertion(t *testing.T) {
	tdigest := New(100)

	data := make([]float64, 10000)
	for i := 0; i < len(data); i++ {
		data[i] = float64(i)
	}

	for i := 0; i < len(data); i++ {
		_ = tdigest.Add(data[i])

		assertDifferenceFromQuantile(data[:i+1], tdigest, 0.001, 1.0+0.001*float64(i), t)
		assertDifferenceFromQuantile(data[:i+1], tdigest, 0.01, 1.0+0.005*float64(i), t)
		assertDifferenceFromQuantile(data[:i+1], tdigest, 0.05, 1.0+0.01*float64(i), t)
		assertDifferenceFromQuantile(data[:i+1], tdigest, 0.25, 1.0+0.03*float64(i), t)
		assertDifferenceFromQuantile(data[:i+1], tdigest, 0.5, 1.0+0.03*float64(i), t)
		assertDifferenceFromQuantile(data[:i+1], tdigest, 0.75, 1.0+0.03*float64(i), t)
		assertDifferenceFromQuantile(data[:i+1], tdigest, 0.95, 1.0+0.01*float64(i), t)
		assertDifferenceFromQuantile(data[:i+1], tdigest, 0.99, 1.0+0.005*float64(i), t)
		assertDifferenceFromQuantile(data[:i+1], tdigest, 0.999, 1.0+0.001*float64(i), t)
	}
}

func TestNonSequentialInsertion(t *testing.T) {
	tdigest := New(100)

	// Not quite a uniform distribution, but close.
	data := make([]float64, 1000)
	for i := 0; i < len(data); i++ {
		tmp := (i * 1627) % len(data)
		data[i] = float64(tmp)
	}

	sorted := make([]float64, 0, len(data))

	for i := 0; i < len(data); i++ {
		_ = tdigest.Add(data[i])
		sorted = append(sorted, data[i])

		// Estimated quantiles are all over the place for low counts, which is
		// OK given that something like P99 is not very meaningful when there are
		// 25 samples. To account for this, increase the error tolerance for
		// smaller counts.
		if i == 0 {
			continue
		}

		max := float64(len(data))
		fac := 1.0 + max/float64(i)

		sort.Float64s(sorted)
		assertDifferenceFromQuantile(sorted, tdigest, 0.001, fac+0.001*max, t)
		assertDifferenceFromQuantile(sorted, tdigest, 0.01, fac+0.005*max, t)
		assertDifferenceFromQuantile(sorted, tdigest, 0.05, fac+0.01*max, t)
		assertDifferenceFromQuantile(sorted, tdigest, 0.25, fac+0.01*max, t)
		assertDifferenceFromQuantile(sorted, tdigest, 0.5, fac+0.02*max, t)
		assertDifferenceFromQuantile(sorted, tdigest, 0.75, fac+0.01*max, t)
		assertDifferenceFromQuantile(sorted, tdigest, 0.95, fac+0.01*max, t)
		assertDifferenceFromQuantile(sorted, tdigest, 0.99, fac+0.005*max, t)
		assertDifferenceFromQuantile(sorted, tdigest, 0.999, fac+0.001*max, t)
	}
}

func TestSingletonInACrowd(t *testing.T) {
	tdigest := New(100)
	for i := 0; i < 10000; i++ {
		_ = tdigest.Add(10)
	}
	_ = tdigest.Add(20)
	_ = tdigest.Compress()

	for _, q := range []float64{0, 0.5, 0.8, 0.9, 0.99, 0.999} {
		if q == 0.999 {
			// Test for 0.999 disabled since it doesn't
			// pass in the reference implementation
			continue
		}
		result := tdigest.Quantile(q)
		if !closeEnough(result, 10) {
			t.Errorf("Expected Quantile(%.3f) = 10, but got %.4f (size=%d)", q, result, tdigest.summary.Len())
		}
	}

	result := tdigest.Quantile(1)
	if result != 20 {
		t.Errorf("Expected Quantile(1) = 20, but got %.4f (size=%d)", result, tdigest.summary.Len())
	}
}

func TestRespectBounds(t *testing.T) {
	tdigest := New(10)

	data := []float64{0, 279, 2, 281}
	for _, f := range data {
		_ = tdigest.Add(f)
	}

	quantiles := []float64{0.01, 0.25, 0.5, 0.75, 0.999}
	for _, q := range quantiles {
		result := tdigest.Quantile(q)
		if result < 0 {
			t.Errorf("q(%.3f) = %.4f < 0", q, result)
		}
		if tdigest.Quantile(q) > 281 {
			t.Errorf("q(%.3f) = %.4f > 281", q, result)
		}
	}
}

func TestWeights(t *testing.T) {
	tdigest := New(10)

	// Create data slice with repeats matching weights we gave to tdigest
	data := []float64{}
	for i := 0; i < 100; i++ {
		_ = tdigest.AddWeighted(float64(i), uint32(i))

		for j := 0; j < i; j++ {
			data = append(data, float64(i))
		}
	}

	assertDifferenceFromQuantile(data, tdigest, 0.001, 1.0+0.001*100.0, t)
	assertDifferenceFromQuantile(data, tdigest, 0.01, 1.0+0.005*100.0, t)
	assertDifferenceFromQuantile(data, tdigest, 0.05, 1.0+0.01*100.0, t)
	assertDifferenceFromQuantile(data, tdigest, 0.25, 1.0+0.01*100.0, t)
	assertDifferenceFromQuantile(data, tdigest, 0.5, 1.0+0.02*100.0, t)
	assertDifferenceFromQuantile(data, tdigest, 0.75, 1.0+0.01*100.0, t)
	assertDifferenceFromQuantile(data, tdigest, 0.95, 1.0+0.01*100.0, t)
	assertDifferenceFromQuantile(data, tdigest, 0.99, 1.0+0.005*100.0, t)
	assertDifferenceFromQuantile(data, tdigest, 0.999, 1.0+0.001*100.0, t)
}

func TestIntegers(t *testing.T) {
	tdigest := New(100)

	_ = tdigest.Add(1)
	_ = tdigest.Add(2)
	_ = tdigest.Add(3)

	if tdigest.Quantile(0.5) != 2 {
		t.Errorf("Expected p(0.5) = 2, Got %.2f instead", tdigest.Quantile(0.5))
	}

	tdigest = New(100)

	for _, i := range []float64{1, 2, 2, 2, 2, 2, 2, 2, 3} {
		_ = tdigest.Add(i)
	}

	if tdigest.Quantile(0.5) != 2 {
		t.Errorf("Expected p(0.5) = 2, Got %.2f instead", tdigest.Quantile(0.5))
	}

	var tot uint32
	tdigest.ForEachCentroid(func(mean float64, count uint32) bool {
		tot += count
		return true
	})

	if tot != 9 {
		t.Errorf("Expected the centroid count to be 9, Got %d instead", tot)
	}
}

func cdf(x float64, data []float64) float64 {
	var n1, n2 int
	for i := 0; i < len(data); i++ {
		if data[i] < x {
			n1++
		}
		if data[i] <= x {
			n2++
		}
	}
	return float64(n1+n2) / 2.0 / float64(len(data))
}

func quantile(q float64, data []float64) float64 {
	if len(data) == 0 {
		return math.NaN()
	}

	if q == 1 || len(data) == 1 {
		return data[len(data)-1]
	}

	index := q * (float64(len(data)) - 1)
	return data[int(index)+1]*(index-float64(int(index))) + data[int(index)]*(float64(int(index)+1)-index)
}

func TestMerge(t *testing.T) {
	if testing.Short() {
		t.Skipf("Skipping merge test. Short flag is on")
	}

	const numItems = 100000

	for _, numSubs := range []int{2, 5, 10, 20, 50, 100} {
		data := make([]float64, numItems)

		subs := make([]*TDigest, numSubs)
		for i := 0; i < numSubs; i++ {
			subs[i] = New(100)
		}

		dist := New(100)
		for i := 0; i < numItems; i++ {
			num := rand.Float64()

			data[i] = num
			_ = dist.Add(num)
			_ = subs[i%numSubs].Add(num)
		}

		_ = dist.Compress()

		dist2 := New(100)
		for i := 0; i < numSubs; i++ {
			_ = dist2.Merge(subs[i])
		}

		if dist.Count() != dist2.Count() {
			t.Errorf("Expected the number of centroids to be the same. %d != %d", dist.Count(), dist2.Count())
		}

		if dist2.Count() != numItems {
			t.Errorf("Items shouldn't have disappeared. %d != %d", dist2.Count(), numItems)
		}

		sort.Float64s(data)

		for _, q := range []float64{0.001, 0.01, 0.1, 0.2, 0.3, 0.5} {
			z := quantile(q, data)
			p1 := dist.Quantile(q)
			p2 := dist2.Quantile(q)

			e1 := p1 - z
			e2 := p2 - z

			if math.Abs(e2)/q >= 0.3 {
				t.Errorf("rel >= 0.3: parts=%3d q=%.3f e1=%.4f e2=%.4f rel=%.3f real=%.3f",
					numSubs, q, e1, e2, math.Abs(e2)/q, z-q)
			}
			if math.Abs(e2) >= 0.015 {
				t.Errorf("e2 >= 0.015: parts=%3d q=%.3f e1=%.4f e2=%.4f rel=%.3f real=%.3f",
					numSubs, q, e1, e2, math.Abs(e2)/q, z-q)
			}

			z = cdf(q, data)
			e1 = dist.CDF(q) - z
			e2 = dist2.CDF(q) - z

			if math.Abs(e2)/q > 0.3 {
				t.Errorf("CDF e2 < 0.015: parts=%3d q=%.3f e1=%.4f e2=%.4f rel=%.3f",
					numSubs, q, e1, e2, math.Abs(e2)/q)
			}

			if math.Abs(e2) >= 0.015 {
				t.Errorf("CDF e2 < 0.015: parts=%3d q=%.3f e1=%.4f e2=%.4f rel=%.3f",
					numSubs, q, e1, e2, math.Abs(e2)/q)
			}
		}
	}
}

func TestCompressDoesntChangeCount(t *testing.T) {
	tdigest := New(100)

	for i := 0; i < 1000; i++ {
		_ = tdigest.Add(rand.Float64())
	}

	initialCount := tdigest.Count()

	err := tdigest.Compress()
	if err != nil {
		t.Errorf("Compress() triggered an unexpected error: %s", err)
	}

	if tdigest.Count() != initialCount {
		t.Errorf("Compress() should not change count. Wanted %d, got %d", initialCount, tdigest.Count())
	}
}

func shouldPanic(f func(), t *testing.T, message string) {
	defer func() {
		tryRecover := recover()
		if tryRecover == nil {
			t.Errorf(message)
		}
	}()
	f()
}

func TestPanic(t *testing.T) {
	tdigest := New(100)

	shouldPanic(func() {
		tdigest.Quantile(-42)
	}, t, "Quantile < 0 should panic!")

	shouldPanic(func() {
		tdigest.Quantile(42)
	}, t, "Quantile > 1 should panic!")
}

func TestForEachCentroid(t *testing.T) {
	tdigest := New(10)

	for i := 0; i < 100; i++ {
		_ = tdigest.Add(float64(i))
	}

	// Iterate limited number.
	means := []float64{}
	tdigest.ForEachCentroid(func(mean float64, count uint32) bool {
		means = append(means, mean)
		return len(means) != 3
	})
	if len(means) != 3 {
		t.Errorf("ForEachCentroid handled incorrect number of data items")
	}

	// Iterate all datapoints.
	means = []float64{}
	tdigest.ForEachCentroid(func(mean float64, count uint32) bool {
		means = append(means, mean)
		return true
	})
	if len(means) != tdigest.summary.Len() {
		t.Errorf("ForEachCentroid did not handle all data")
	}
}

func benchmarkAdd(compression float64, b *testing.B) {
	t := New(compression)

	data := make([]float64, b.N)
	for n := 0; n < b.N; n++ {
		data[n] = rand.Float64()
	}

	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		err := t.AddWeighted(data[n], 1)
		if err != nil {
			b.Error(err)
		}
	}
	b.StopTimer()
}

func BenchmarkAdd1(b *testing.B) {
	benchmarkAdd(1, b)
}

func BenchmarkAdd10(b *testing.B) {
	benchmarkAdd(10, b)
}

func BenchmarkAdd100(b *testing.B) {
	benchmarkAdd(100, b)
}
