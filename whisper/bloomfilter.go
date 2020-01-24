package whisper

func CheckFilterMatch(filter, given []byte) bool {
	if filter == nil {
		return true
	}

	for i := 0; i < BloomFilterSize; i++ {
		f := filter[i]
		s := given[i]
		if (f | s) != f {
			return false
		}
	}

	return true
}

func AggregateBloom(a, b []byte) []byte {
	c := make([]byte, BloomFilterSize)
	for i := 0; i < BloomFilterSize; i++ {
		c[i] = a[i] | b[i]
	}
	return c
}

func HasAnyFilter(bloom []byte) bool {
	if bloom == nil {
		return false
	}
	for _, b := range bloom {
		if b != 255 {
			return true
		}
	}
	return false
}

func GetEmptyBloomFilter() []byte {
	bloom := make([]byte, BloomFilterSize)
	for i := 0; i < BloomFilterSize; i++ {
		bloom[i] = 0xFF
	}
	return bloom
}