package slice

import (
	"math"
	"slices"
	"sort"
	"testing"
)

func TestFloat32SliceSortNaN(t *testing.T) {
	nan := float32(math.NaN())
	negInf, posInf := float32(math.Inf(-1)), float32(math.Inf(1))
	tests := []struct {
		name string
		in   Float32Slice
		want []float32
	}{
		{"empty", nil, nil},
		{"NaN between finite values", Float32Slice{1, nan, 0}, []float32{nan, 0, 1}},
		{"only NaNs", Float32Slice{nan, nan}, []float32{nan, nan}},
		{"already sorted", Float32Slice{nan, negInf, 0, posInf}, []float32{nan, negInf, 0, posInf}},
		{"mixed values", Float32Slice{posInf, nan, 1, 0, negInf, nan, -1, float32(math.Copysign(0, -1))}, []float32{nan, nan, negInf, -1, 0, 0, 1, posInf}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := slices.Clone(tt.in)
			sort.Sort(got)
			if !equalOrderedFloats([]float32(got), tt.want) {
				t.Fatalf("sort.Sort(%v) = %v, want %v", tt.in, got, tt.want)
			}
			if !sort.IsSorted(got) || !slices.IsSorted(got) {
				t.Fatal("sort.Interface and slices disagree about sorted order")
			}
			if len(got) > 0 && got[0] != got[0] && OrderedIndexOf([]float32(got), nan) != 0 {
				t.Fatal("sorted NaN cannot be found by OrderedIndexOf")
			}
		})
	}
}

func TestFloat32SliceLessNaN(t *testing.T) {
	nan := float32(math.NaN())
	values := Float32Slice{nan, nan, float32(math.Inf(-1)), -1, 0, 1, float32(math.Inf(1))}
	for i := range values {
		for j := range values {
			want := i < j && !(i == 0 && j == 1)
			if got := values.Less(i, j); got != want {
				t.Errorf("Less(%d, %d) = %t, want %t", i, j, got, want)
			}
		}
	}
}
