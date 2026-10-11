package treemap

import "testing"

func BenchmarkSwap(b *testing.B) {
	for _, tt := range []struct {
		name string
		size int
	}{
		{"1K", 1024},
		{"64K", 65536},
	} {
		b.Run(tt.name, func(b *testing.B) {
			m := NewOrderedTreeMap[int, int]()
			for i := 0; i < tt.size; i++ {
				m.Put(i, i)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				m.Swap((i*7919)&(tt.size-1), i)
			}
		})
	}
}
