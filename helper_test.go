package gokalman

import (
	"testing"

	"github.com/gonum/matrix/mat64"
)

func assertPanic(t *testing.T, f func()) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("code did not panic")
		}
	}()
	f()
}

func TestIsNil(t *testing.T) {
	if IsNil(Identity(2)) {
		t.Fatal("i22 said to be nil")
	}
	if !IsNil(mat64.NewSymDense(2, []float64{0, 0, 0, 0})) {
		t.Fatal("zeros 4x4 said to NOT be nil")
	}
}

func TestIdentity(t *testing.T) {
	n := 3
	i33 := Identity(n)
	if r, c := i33.Dims(); r != n || r != c {
		t.Fatalf("i11 has dimensions (%dx%d)", r, c)
	}
	for i := 0; i < n; i++ {
		if i33.At(i, i) != 1 {
			t.Fatalf("i33(%d,%d) != 1", i, i)
		}
		for j := 0; j < n; j++ {
			if i != j && i33.At(i, j) != 0 {
				t.Fatalf("i33(%d,%d) != 0", i, j)
			}
		}
	}
}

func TestAsSymDense(t *testing.T) {
	d := mat64.NewDense(2, 2, []float64{1, 0, 0, 1})
	dsym, err := AsSymDense(d)
	if err != nil {
		t.Fatal("AsSymDense failed on i22")
	}
	r, c := d.Dims()
	for i := 0; i < r; i++ {
		for j := 0; j < c; j++ {
			if dsym.At(i, j) != d.At(i, j) {
				t.Fatalf("returned symmetric matrix invalid: %+v %+v", dsym, d)
			}
		}
	}
	_, err = AsSymDense(mat64.NewDense(2, 2, []float64{1, 0, 1, 1}))
	if err == nil {
		t.Fatal("non symmetric matrix did not fail")
	}

	_, err = AsSymDense(mat64.NewDense(2, 3, []float64{1, 0, 1, 1, 2, 3}))
	if err == nil {
		t.Fatal("non square matrix did not fail")
	}
}

func TestCheckDims(t *testing.T) {
	i22 := Identity(2)
	i33 := Identity(3)
	methods := []DimensionAgreement{rows2cols, cols2rows, cols2cols, rows2rows, rowsAndcols}
	for _, meth := range methods {
		if err := checkMatDims(i22, i22, "i22", "i22", meth); err != nil {
			t.Fatalf("method %+v fails: %s", meth, err)
		}
		if err := checkMatDims(i22, i33, "i22", "i33", meth); err == nil {
			t.Fatalf("method %+v does not error when using i22 and i33 ", meth)
		}
	}
}
