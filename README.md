# T-Digest

A fast map-reduce and parallel streaming friendly data-structure for accurate
quantile approximation.

This package provides an implementation of Ted Dunning's t-digest data
structure in Go.

[![GoDoc](https://godoc.org/github.com/zeebo/tdigest?status.svg)](http://godoc.org/github.com/zeebo/tdigest)

Forked from the excellent [caio/go-tdigest](https://github.com/caio/go-tdigest)
to clean up the API and optimize.

## Example Usage

```go
package main

import (
	"fmt"
	"math/rand"

	"github.com/zeebo/tdigest"
)

func main() {
	t := tdigest.New(100)

	for i := 0; i < 10000; i++ {
		// Analogue to t.AddWeighted(rand.Float64(), 1)
		t.Add(rand.Float64())
	}

	fmt.Printf("p(.5) = %.6f\n", t.Quantile(0.5))
	fmt.Printf("CDF(Quantile(.5)) = %.6f\n", t.CDF(t.Quantile(0.5)))
}
```
