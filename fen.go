package tdigest

type fen struct {
	buf []uint32
}

func lsb(i int) int {
	return i & -i
}

func (f *fen) accomodate(i int) {
	if len(f.buf) <= i {
		f.buf = append(f.buf, make([]uint32, i-len(f.buf)+1)...)
	}
}

func (f *fen) Add(i int, delta uint32) {
	f.accomodate(i)
	for i < len(f.buf) {
		f.buf[i] += delta
		i += lsb(i + 1)
	}
}

func (f *fen) Range(i, j int) (sum uint32) {
	f.accomodate(j - 1)
	for j > i {
		sum += f.buf[j-1]
		j -= lsb(j)
	}
	for i > j {
		sum -= f.buf[i-1]
		i -= lsb(i)
	}
	return sum
}

func (f *fen) Get(i int) uint32 {
	return f.Range(i, i+1)
}

func (f *fen) Set(i int, value uint32) {
	delta := value - f.Range(i, i+1)
	f.Add(i, delta)
}

func (f *fen) Sum(i int) (sum uint32) {
	f.accomodate(i - 1)
	for i > 0 {
		sum += f.buf[i-1]
		i -= lsb(i)
	}
	return sum
}
