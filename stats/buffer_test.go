package stats

import "testing"

func compareSlice[T comparable](a, b []T) bool {
	if len(a) != len(b) {
		return false
	}
	for i, _ := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestBufferNotFull(t *testing.T) {

	b := NewCircularBuffer[int](10)

	for i := 0; i < 5; i++ {
		b.Insert(i)
	}

	if out := b.GetAll(); len(out) != 5 {
		t.Error("Invalid Length:", out)
	} else {
		t.Log(out)
	}

	if out := b.GetAll(); !compareSlice[int](out, []int{0, 1, 2, 3, 4}) {
		t.Error("Slice Error:", out)
	} else {
		t.Log(out)
	}

	if out := b.Get(2); !compareSlice[int](out, []int{0, 1}) {
		t.Error("Slice Error:", out)
	} else {
		t.Log(out)
	}

	if out := b.GetOffset(2, 10); !compareSlice[int](out, []int{2, 3, 4}) {
		t.Error("Slice Error:", out)
	} else {
		t.Log(out)
	}
}

func TestBufferFull(t *testing.T) {

	b := NewCircularBuffer[int](10)

	for i := 0; i < 99; i++ {
		b.Insert(i)
	}

	if out := b.GetAll(); len(out) != 10 {
		t.Error("Invalid Length:", b.buffer, out)
	} else {
		t.Log(out)
	}

	if out := b.GetAll(); !compareSlice[int](out, []int{89, 90, 91, 92, 93, 94, 95, 96, 97, 98}) {
		t.Error("Slice Error:", b.buffer, out)
	} else {
		t.Log(out)
	}

	if out := b.Get(2); !compareSlice[int](out, []int{89, 90}) {
		t.Error("Slice Error:", b.buffer, out)
	} else {
		t.Log(out)
	}

	if out := b.GetOffset(0, 1); !compareSlice[int](out, []int{89}) {
		t.Error("Slice Error:", b.buffer, out)
	} else {
		t.Log(out)
	}

	if out := b.GetOffset(5, 10); !compareSlice[int](out, []int{94, 95, 96, 97, 98}) {
		t.Error("Slice Error:", b.buffer, out)
	} else {
		t.Log(out)
	}

	if out := b.GetOffset(10, 10); !compareSlice[int](out, []int{}) {
		t.Error("Slice Error:", b.buffer, out)
	} else {
		t.Log(out)
	}
}
