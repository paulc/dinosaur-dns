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

func TestBufferEmpty(t *testing.T) {

	b := NewCircularBuffer[int](10)

	if out := b.GetAll(); !compareSlice[int](out, []int{}) {
		t.Error("GetAll Error:", out)
	} else {
		t.Log(out)
	}

	if out := b.Get(2); !compareSlice[int](out, []int{}) {
		t.Error("Get Error:", out)
	} else {
		t.Log(out)
	}

	if out := b.GetOffset(2, 10); !compareSlice[int](out, []int{}) {
		t.Error("GetOffset Error:", out)
	} else {
		t.Log(out)
	}

	if out := b.Tail(2); !compareSlice[int](out, []int{}) {
		t.Error("Tail Error:", out)
	} else {
		t.Log(out)
	}

	if out := b.TailFilter(2, func(i int) bool { return true }); !compareSlice[int](out, []int{}) {
		t.Error("Tail Error:", out)
	} else {
		t.Log(out)
	}

	if out := b.TailBetween(0, func(i int) bool { return i < 10 }, func(i int) bool { return i < 5 }, nil); !compareSlice[int](out, []int{}) {
		t.Error("TailBetween Error:", out)
	} else {
		t.Log(out)
	}

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
		t.Error("GetAll Error:", out)
	} else {
		t.Log(out)
	}

	if out := b.Get(2); !compareSlice[int](out, []int{0, 1}) {
		t.Error("Get Error:", out)
	} else {
		t.Log(out)
	}

	if out := b.GetOffset(2, 10); !compareSlice[int](out, []int{2, 3, 4}) {
		t.Error("GetOffset Error:", out)
	} else {
		t.Log(out)
	}

	if out := b.Tail(2); !compareSlice[int](out, []int{4, 3}) {
		t.Error("Tail Error:", out)
	} else {
		t.Log(out)
	}

	if out := b.Tail(10); !compareSlice[int](out, []int{4, 3, 2, 1, 0}) {
		t.Error("Tail Error:", out)
	} else {
		t.Log(out)
	}

	if out := b.TailFilter(10, func(i int) bool { return i%2 == 0 }); !compareSlice[int](out, []int{4, 2, 0}) {
		t.Error("Tail Error:", out)
	} else {
		t.Log(out)
	}

	if out := b.TailBetween(0, func(i int) bool { return i < 10 }, func(i int) bool { return i < 2 }, nil); !compareSlice[int](out, []int{4, 3, 2}) {
		t.Error("TailBetween Error:", out)
	} else {
		t.Log(out)
	}
}

func TestBufferFull(t *testing.T) {

	b := NewCircularBuffer[int](50)

	for i := 0; i < 99; i++ {
		b.Insert(i)
	}

	if out := b.GetAll(); len(out) != 50 {
		t.Error("Invalid Length:", out)
	} else {
		t.Log(out)
	}

	if out := b.GetAll(); !compareSlice[int](out[:10], []int{49, 50, 51, 52, 53, 54, 55, 56, 57, 58}) {
		t.Error("Slice Error:", out)
	} else {
		t.Log(out)
	}

	if out := b.Get(0); !compareSlice[int](out, []int{}) {
		t.Error("Slice Error:", out)
	} else {
		t.Log(out)
	}

	if out := b.Get(-1); !compareSlice[int](out, []int{}) {
		t.Error("Slice Error:", out)
	} else {
		t.Log(out)
	}

	if out := b.Get(2); !compareSlice[int](out, []int{49, 50}) {
		t.Error("Slice Error:", out)
	} else {
		t.Log(out)
	}

	if out := b.GetOffset(0, 1); !compareSlice[int](out, []int{49}) {
		t.Error("Slice Error:", out)
	} else {
		t.Log(out)
	}

	if out := b.GetOffset(5, 10); !compareSlice[int](out, []int{54, 55, 56, 57, 58, 59, 60, 61, 62, 63}) {
		t.Error("Slice Error:", out)
	} else {
		t.Log(out)
	}

	if out := b.GetOffset(50, 10); !compareSlice[int](out, []int{}) {
		t.Error("Slice Error:", out)
	} else {
		t.Log(out)
	}

	if out := b.Tail(0); !compareSlice[int](out, []int{}) {
		t.Error("Tail Error:", out)
	} else {
		t.Log(out)
	}

	if out := b.Tail(2); !compareSlice[int](out, []int{98, 97}) {
		t.Error("Tail Error:", out)
	} else {
		t.Log(out)
	}

	if out := b.Tail(10); !compareSlice[int](out, []int{98, 97, 96, 95, 94, 93, 92, 91, 90, 89}) {
		t.Error("Tail Error:", out)
	} else {
		t.Log(out)
	}

	if out := b.TailFilter(10, func(i int) bool { return i%2 == 0 }); !compareSlice[int](out, []int{98, 96, 94, 92, 90, 88, 86, 84, 82, 80}) {
		t.Error("TailFilter Error:", out)
	} else {
		t.Log(out)
	}

	if out := b.TailFilter(0, func(i int) bool { return i%2 == 0 }); len(out) != 25 {
		t.Error("TailFilter Error:", out)
	} else {
		t.Log(out)
	}

	// Test different TailBetween options

	if out := b.TailBetween(0, func(i int) bool { return i < 95 }, func(i int) bool { return i < 90 }, nil); !compareSlice[int](out, []int{94, 93, 92, 91, 90}) {
		t.Error("TailBetween Error:", out)
	} else {
		t.Log(out)
	}

	if out := b.TailBetween(2, func(i int) bool { return i < 95 }, func(i int) bool { return i < 90 }, nil); !compareSlice[int](out, []int{94, 93}) {
		t.Error("TailBetween Error:", out)
	} else {
		t.Log(out)
	}

	if out := b.TailBetween(2, nil, func(i int) bool { return i < 95 }, nil); !compareSlice[int](out, []int{98, 97}) {
		t.Error("TailBetween Error:", out)
	} else {
		t.Log(out)
	}

	if out := b.TailBetween(10, nil, func(i int) bool { return i < 95 }, nil); !compareSlice[int](out, []int{98, 97, 96, 95}) {
		t.Error("TailBetween Error:", out)
	} else {
		t.Log(out)
	}

	if out := b.TailBetween(10, nil, nil, nil); !compareSlice[int](out, []int{98, 97, 96, 95, 94, 93, 92, 91, 90, 89}) {
		t.Error("TailBetween Error:", out)
	} else {
		t.Log(out)
	}

	if out := b.TailBetween(0, nil, nil, nil); len(out) != 50 {
		t.Error("TailBetween Error:", out)
	} else {
		t.Log(out)
	}

	if out := b.TailBetween(0, nil, nil, func(i int) bool { return i%2 == 0 }); len(out) != 25 {
		t.Error("TailBetween Error:", out)
	} else {
		t.Log(out)
	}

	if out := b.TailBetween(5, nil, nil, func(i int) bool { return i%2 == 0 }); !compareSlice[int](out, []int{98, 96, 94, 92, 90}) {
		t.Error("TailBetween Error:", out)
	} else {
		t.Log(out)
	}

}

func TestBufferHook(t *testing.T) {

	b := NewCircularBuffer[int](10)

	ref := 0
	counter := 0

	hookf1 := func(i int) {
		t.Logf(">> hookf1: %d", i)
		counter += i
	}

	hookf2 := func(i int) {
		t.Logf(">> hookf2: %d", i)
	}

	b.AddHook("f1", hookf1)
	b.AddHook("f2", hookf2)

	for i := 0; i < 5; i++ {
		b.Insert(i)
		ref += i
	}

	if counter != ref {
		t.Errorf("Hook error: counter=%d ref=%d", counter, ref)
	} else {
		t.Log(counter)
	}

	if out := b.GetAll(); !compareSlice[int](out, []int{0, 1, 2, 3, 4}) {
		t.Error("GetAll Error:", out)
	} else {
		t.Log(out)
	}

	b.DeleteHook("f2")

	for i := 0; i < 5; i++ {
		b.Insert(i)
		ref += i
	}

	if counter != ref {
		t.Errorf("Hook error: counter=%d ref=%d", counter, ref)
	} else {
		t.Log(counter)
	}

}
