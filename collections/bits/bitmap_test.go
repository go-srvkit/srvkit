package bits

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBitWordsCount(t *testing.T) {
	tests := []struct {
		name    string
		bitSize int
		want    int
	}{
		{"zero", 0, 0},
		{"one bit", 1, 1},
		{"exact one word", 64, 1},
		{"one bit over one word", 65, 2},
		{"exact two words", 128, 2},
		{"large value", 1000, 16},
		{"negative", -1, 0},
		{"negative large", -64, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange & Act
			got := BitWordsCount(tt.bitSize)
			// Assert
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestBitBytesCount(t *testing.T) {
	tests := []struct {
		name    string
		bitSize int
		want    int
	}{
		{"zero", 0, 0},
		{"one bit", 1, 1},
		{"exact one byte", 8, 1},
		{"one bit over one byte", 9, 2},
		{"exact one word", 64, 8},
		{"large value", 1000, 125},
		{"negative", -1, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BitBytesCount(tt.bitSize)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTestBit(t *testing.T) {
	tests := []struct {
		name string
		bm   []uint64
		i    int
		want bool
	}{
		{"empty slice", nil, 0, false},
		{"zero index set", []uint64{1}, 0, true},
		{"zero index unset", []uint64{0}, 0, false},
		{"last bit of first word", []uint64{1 << 63}, 63, true},
		{"first bit of second word", []uint64{0, 1}, 64, true},
		{"out of range - too high", []uint64{1}, 64, false},
		{"out of range - negative", []uint64{1}, -1, false},
		{"multi-word middle bit", []uint64{0, 0, 4}, 130, true},
		{"multi-word miss", []uint64{0, 0, 4}, 131, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TestBit(tt.bm, tt.i)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestSetBit(t *testing.T) {
	tests := []struct {
		name   string
		bm     []uint64
		i      int
		wantOk bool
		check  func(t *testing.T, bm []uint64)
	}{
		{
			"set bit 0",
			[]uint64{0}, 0, true,
			func(t *testing.T, bm []uint64) { assert.Equal(t, uint64(1), bm[0]) },
		},
		{
			"set bit 63",
			[]uint64{0}, 63, true,
			func(t *testing.T, bm []uint64) { assert.Equal(t, uint64(1<<63), bm[0]) },
		},
		{
			"set bit in second word",
			[]uint64{0, 0}, 64, true,
			func(t *testing.T, bm []uint64) { assert.Equal(t, uint64(1), bm[1]) },
		},
		{
			"set already set bit",
			[]uint64{1}, 0, true,
			func(t *testing.T, bm []uint64) { assert.Equal(t, uint64(1), bm[0]) },
		},
		{"out of range - too high", []uint64{0}, 64, false, nil},
		{"out of range - negative", []uint64{1}, -1, false, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			bm := make([]uint64, len(tt.bm))
			copy(bm, tt.bm)
			// Act
			ok := SetBit(bm, tt.i)
			// Assert
			assert.Equal(t, tt.wantOk, ok)
			if tt.check != nil {
				tt.check(t, bm)
			}
		})
	}
}

func TestMustSetBit(t *testing.T) {
	tests := []struct {
		name     string
		bm       []uint64
		i        int
		wantLen  int
		wantTrue bool
	}{
		{"no grow needed", []uint64{0}, 0, 1, true},
		{"grow to 2 words", []uint64{0}, 64, 2, true},
		{"grow from empty", nil, 0, 1, true},
		{"grow to 3 words", []uint64{1}, 128, 3, true},
		{"set bit 200 from 1 word", []uint64{0}, 200, 4, true},
		{"preserve existing data", []uint64{0xFF}, 100, 2, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			bm := make([]uint64, len(tt.bm))
			copy(bm, tt.bm)
			// Act
			result := MustSetBit(bm, tt.i)
			// Assert
			assert.Equal(t, tt.wantLen, len(result))
			assert.True(t, TestBit(result, tt.i))
		})
	}

	t.Run("preserve existing data after grow", func(t *testing.T) {
		// Arrange
		bm := []uint64{0xFF}
		// Act
		result := MustSetBit(bm, 100)
		// Assert
		assert.Equal(t, uint64(0xFF), result[0])
		assert.True(t, TestBit(result, 100))
	})
}

func TestClearBit(t *testing.T) {
	tests := []struct {
		name   string
		bm     []uint64
		i      int
		wantOk bool
		want   []uint64
	}{
		{"clear set bit", []uint64{1}, 0, true, []uint64{0}},
		{"clear already zero", []uint64{0}, 0, true, []uint64{0}},
		{"clear high bit", []uint64{1 << 63}, 63, true, []uint64{0}},
		{"clear in second word", []uint64{0, 3}, 64, true, []uint64{0, 2}},
		{"out of range", []uint64{1}, 64, false, []uint64{1}},
		{"negative index", []uint64{1}, -1, false, []uint64{1}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			bm := make([]uint64, len(tt.bm))
			copy(bm, tt.bm)
			// Act
			ok := ClearBit(bm, tt.i)
			// Assert
			assert.Equal(t, tt.wantOk, ok)
			assert.Equal(t, tt.want, bm)
		})
	}
}

func TestFlipBit(t *testing.T) {
	tests := []struct {
		name   string
		bm     []uint64
		i      int
		wantOk bool
		want   []uint64
	}{
		{"flip 0 to 1", []uint64{0}, 0, true, []uint64{1}},
		{"flip 1 to 0", []uint64{1}, 0, true, []uint64{0}},
		{"flip high bit", []uint64{0}, 63, true, []uint64{1 << 63}},
		{"flip in second word", []uint64{0, 0}, 64, true, []uint64{0, 1}},
		{"double flip restores", []uint64{5}, 0, true, []uint64{4}},
		{"out of range", []uint64{1}, 64, false, []uint64{1}},
		{"negative index", []uint64{1}, -1, false, []uint64{1}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bm := make([]uint64, len(tt.bm))
			copy(bm, tt.bm)
			ok := FlipBit(bm, tt.i)
			assert.Equal(t, tt.wantOk, ok)
			assert.Equal(t, tt.want, bm)
		})
	}
}

func TestTestAndSetBit(t *testing.T) {
	tests := []struct {
		name    string
		bm      []uint64
		i       int
		wantOld bool
		wantOk  bool
	}{
		{"was unset", []uint64{0}, 0, false, true},
		{"was set", []uint64{1}, 0, true, true},
		{"second word", []uint64{0, 0}, 64, false, true},
		{"second word already set", []uint64{0, 1}, 64, true, true},
		{"out of range", []uint64{0}, 64, false, false},
		{"negative", []uint64{1}, -1, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bm := make([]uint64, len(tt.bm))
			copy(bm, tt.bm)
			old := TestAndSetBit(bm, tt.i)
			assert.Equal(t, tt.wantOld, old)
			if tt.wantOk {
				assert.True(t, TestBit(bm, tt.i))
			}
		})
	}
}

func TestTestAndClearBit(t *testing.T) {
	tests := []struct {
		name    string
		bm      []uint64
		i       int
		wantOld bool
		wantOk  bool
	}{
		{"was set", []uint64{1}, 0, true, true},
		{"was unset", []uint64{0}, 0, false, true},
		{"second word set", []uint64{0, 1}, 64, true, true},
		{"second word unset", []uint64{0, 0}, 64, false, true},
		{"out of range", []uint64{1}, 64, false, false},
		{"negative", []uint64{1}, -1, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bm := make([]uint64, len(tt.bm))
			copy(bm, tt.bm)
			old := TestAndClearBit(bm, tt.i)
			assert.Equal(t, tt.wantOld, old)
			if tt.wantOk {
				assert.False(t, TestBit(bm, tt.i))
			}
		})
	}
}

func TestOnesCount(t *testing.T) {
	tests := []struct {
		name string
		bm   []uint64
		want int
	}{
		{"nil slice", nil, 0},
		{"empty slice", []uint64{}, 0},
		{"all zero", []uint64{0, 0}, 0},
		{"single bit", []uint64{1}, 1},
		{"full word", []uint64{^uint64(0)}, 64},
		{"multi word", []uint64{0xFF, 0xFF00}, 16},
		{"all ones multi word", []uint64{^uint64(0), ^uint64(0)}, 128},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := OnesCount(tt.bm)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTrailingZeros(t *testing.T) {
	tests := []struct {
		name string
		bm   []uint64
		want int
	}{
		{"nil slice", nil, 0},
		{"empty slice", []uint64{}, 0},
		{"all zero", []uint64{0, 0}, 128},
		{"bit 0 set", []uint64{1}, 0},
		{"bit 3 set", []uint64{8}, 3},
		{"first word zero second has bit", []uint64{0, 1}, 64},
		{"first word zero second has bit 66", []uint64{0, 4}, 66},
		{"high bit of first word", []uint64{1 << 63}, 63},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TrailingZeros(tt.bm)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestIsAllZero(t *testing.T) {
	tests := []struct {
		name string
		bm   []uint64
		want bool
	}{
		{"nil slice", nil, true},
		{"empty slice", []uint64{}, true},
		{"all zero", []uint64{0, 0, 0}, true},
		{"single bit set", []uint64{1}, false},
		{"last word has bit", []uint64{0, 0, 1}, false},
		{"full words", []uint64{^uint64(0)}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsAllZero(tt.bm)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFindFirst(t *testing.T) {
	tests := []struct {
		name string
		bm   []uint64
		want int
	}{
		{"nil slice", nil, -1},
		{"empty slice", []uint64{}, -1},
		{"all zero", []uint64{0, 0}, -1},
		{"bit 0", []uint64{1}, 0},
		{"bit 63", []uint64{1 << 63}, 63},
		{"bit 64 in second word", []uint64{0, 1}, 64},
		{"bit 127", []uint64{0, 1 << 63}, 127},
		{"first word has bits", []uint64{0b1010}, 1},
		{"skip empty first word", []uint64{0, 0b100}, 66},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FindFirst(tt.bm)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFormat(t *testing.T) {
	tests := []struct {
		name string
		bm   []uint64
		want string
	}{
		{"nil slice", nil, ""},
		{"empty slice", []uint64{}, ""},
		{"single zero word", []uint64{0}, "0000000000000000000000000000000000000000000000000000000000000000"},
		{"bit 0 set", []uint64{1}, "1000000000000000000000000000000000000000000000000000000000000000"},
		{"bit 1 set", []uint64{2}, "0100000000000000000000000000000000000000000000000000000000000000"},
		{"all ones one word", []uint64{^uint64(0)}, "1111111111111111111111111111111111111111111111111111111111111111"},
		{"two words", []uint64{1, 1}, "10000000000000000000000000000000000000000000000000000000000000001000000000000000000000000000000000000000000000000000000000000000"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Format(tt.bm)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFormatWidth(t *testing.T) {
	tests := []struct {
		name  string
		bm    []uint64
		width int
		want  string
	}{
		{
			"width 8 one word",
			[]uint64{0xFF},
			8,
			"\n11111111" +
				"\n00000000" +
				"\n00000000" +
				"\n00000000" +
				"\n00000000" +
				"\n00000000" +
				"\n00000000" +
				"\n00000000",
		},
		{
			"width 64 one word",
			[]uint64{1},
			64,
			"\n1000000000000000000000000000000000000000000000000000000000000000",
		},
		{
			"nil slice",
			nil,
			8,
			"",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatWidth(tt.bm, tt.width)
			assert.Equal(t, tt.want, got)
		})
	}
}
