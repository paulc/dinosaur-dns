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

	if out := b.Get(0); !compareSlice[int](out, []int{}) {
		t.Error("Slice Error:", b.buffer, out)
	} else {
		t.Log(out)
	}

	if out := b.Get(-1); !compareSlice[int](out, []int{}) {
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

	if out := b.Tail(100); !compareSlice[int](out, []int{98, 97, 96, 95, 94, 93, 92, 91, 90, 89}) {
		t.Error("Tail Error:", out)
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
