package onnx

import (
	"math"
	"math/rand/v2"
	"slices"
	"testing"
)

func run(t *testing.T, opType string, attrs map[string]attribute, in ...*Tensor) []*Tensor {
	t.Helper()
	out, err := ops[opType](&nodeProto{opType: opType, attrs: attrs}, in)
	if err != nil {
		t.Fatalf("%s: %v", opType, err)
	}
	return out
}

func floats(shape []int, v ...float32) *Tensor { return &Tensor{Type: Float32, Shape: shape, F: v} }
func int64s(v ...int64) *Tensor                { return &Tensor{Type: Int64, Shape: []int{len(v)}, I: v} }

// DynamicQuantizeLinear gives the ONNX specification's own example: the range with 0 in it, the scale over 255, the
// zero point and the values rounded half to even (#623).
func TestDynamicQuantizeLinear(t *testing.T) {
	out := run(t, "DynamicQuantizeLinear", nil, floats([]int{6}, 0, 2, -3, -2.5, 1.34, 0.5))
	if !slices.Equal(out[0].B, []byte{153, 255, 0, 26, 221, 179}) || out[2].B[0] != 153 || out[1].F[0] != float32(5)/255 {
		t.Fatalf("y %v, scale %v, zero point %v", out[0].B, out[1].F, out[2].B)
	}
	out = run(t, "DynamicQuantizeLinear", nil, floats([]int{4}, 1, 2.1, 1.3, 2.5)) // all above 0: the range from 0
	if out[2].B[0] != 0 || !slices.Equal(out[0].B, []byte{102, 214, 133, 255}) {
		t.Fatalf("y %v, zero point %v", out[0].B, out[2].B)
	}
}

// MatMulInteger, two columns a multiply, is the exact integer product of the matrices less their zero points, for
// uint8 and int8 inputs, odd sizes, a batch, a zero point for all columns and one a column (#623).
func TestMatMulIntegerExact(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 2))
	for _, c := range []struct {
		batch, m, k, n int
		aType, bType   DType
		perColumn      bool
	}{
		{1, 3, 5, 7, Uint8, Int8, false}, {2, 9, 64, 33, Uint8, Int8, true}, {1, 17, 192, 576, Uint8, Int8, false},
		{3, 4, 13, 2, Int8, Uint8, true}, {1, 1, 1, 1, Uint8, Uint8, false},
	} {
		a := &Tensor{Type: c.aType, Shape: []int{c.batch, c.m, c.k}, B: make([]byte, c.batch*c.m*c.k)}
		b := &Tensor{Type: c.bType, Shape: []int{c.k, c.n}, B: make([]byte, c.k*c.n)}
		for i := range a.B {
			a.B[i] = byte(rng.IntN(256))
		}
		for i := range b.B {
			b.B[i] = byte(rng.IntN(256))
		}
		azp := &Tensor{Type: c.aType, Shape: []int{}, B: []byte{byte(rng.IntN(256))}}
		bzp := &Tensor{Type: c.bType, Shape: []int{}, B: []byte{byte(rng.IntN(256))}}
		if c.perColumn {
			bzp = &Tensor{Type: c.bType, Shape: []int{c.n}, B: make([]byte, c.n)}
			for i := range bzp.B {
				bzp.B[i] = byte(rng.IntN(256))
			}
		}
		got := run(t, "MatMulInteger", nil, a, b, azp, bzp)[0]
		for p := range c.batch {
			for i := range c.m {
				for j := range c.n {
					var want int32
					zj := 0
					if c.perColumn {
						zj = j
					}
					for kk := range c.k {
						want += (byteVal(a, (p*c.m+i)*c.k+kk) - byteVal(azp, 0)) * (byteVal(b, kk*c.n+j) - byteVal(bzp, zj))
					}
					if g := got.I[(p*c.m+i)*c.n+j]; g != int64(want) {
						t.Fatalf("%+v: [%d %d %d] %d, want %d", c, p, i, j, g, want)
					}
				}
			}
		}
	}
}

// The shape operators move values as the ONNX specification says: Transpose, Slice (negative steps and ends),
// Pad, Gather (a negative index), Reshape (0 and -1), Unsqueeze and Squeeze, Concat, ReduceSum and Range (#623).
func TestShapeOperators(t *testing.T) {
	x := floats([]int{2, 3}, 1, 2, 3, 4, 5, 6)
	if got := run(t, "Transpose", nil, x)[0]; !slices.Equal(got.F, []float32{1, 4, 2, 5, 3, 6}) || !slices.Equal(got.Shape, []int{3, 2}) {
		t.Fatalf("Transpose %v %v", got.Shape, got.F)
	}
	if got := run(t, "Slice", nil, x, int64s(-1), int64s(-4), int64s(1), int64s(-2))[0]; !slices.Equal(got.F, []float32{3, 1, 6, 4}) {
		t.Fatalf("Slice %v %v", got.Shape, got.F)
	}
	if got := run(t, "Pad", map[string]attribute{"mode": {s: []byte("constant")}}, x, int64s(0, 1, 1, 0), floats([]int{}, 9))[0]; !slices.Equal(got.F, []float32{9, 1, 2, 3, 9, 4, 5, 6, 9, 9, 9, 9}) {
		t.Fatalf("Pad %v %v", got.Shape, got.F)
	}
	if got := run(t, "Gather", map[string]attribute{"axis": {i: 1}}, x, int64s(-1, 0))[0]; !slices.Equal(got.F, []float32{3, 1, 6, 4}) {
		t.Fatalf("Gather %v %v", got.Shape, got.F)
	}
	if got := run(t, "Reshape", nil, x, int64s(0, -1, 1))[0]; !slices.Equal(got.Shape, []int{2, 3, 1}) {
		t.Fatalf("Reshape %v", got.Shape)
	}
	u := run(t, "Unsqueeze", nil, x, int64s(0, -1))[0]
	if !slices.Equal(u.Shape, []int{1, 2, 3, 1}) {
		t.Fatalf("Unsqueeze %v", u.Shape)
	}
	if got := run(t, "Squeeze", nil, u)[0]; !slices.Equal(got.Shape, []int{2, 3}) {
		t.Fatalf("Squeeze %v", got.Shape)
	}
	if got := run(t, "Concat", map[string]attribute{"axis": {i: -1}}, x, x)[0]; !slices.Equal(got.F, []float32{1, 2, 3, 1, 2, 3, 4, 5, 6, 4, 5, 6}) {
		t.Fatalf("Concat %v %v", got.Shape, got.F)
	}
	if got := run(t, "ReduceSum", map[string]attribute{"keepdims": {i: 0}}, x, int64s(0))[0]; !slices.Equal(got.F, []float32{5, 7, 9}) || !slices.Equal(got.Shape, []int{3}) {
		t.Fatalf("ReduceSum %v %v", got.Shape, got.F)
	}
	if got := run(t, "Range", nil, &Tensor{Type: Int64, Shape: []int{}, I: []int64{2}}, &Tensor{Type: Int64, Shape: []int{}, I: []int64{9}}, &Tensor{Type: Int64, Shape: []int{}, I: []int64{3}})[0]; !slices.Equal(got.I, []int64{2, 5, 8}) {
		t.Fatalf("Range %v", got.I)
	}
	if got := run(t, "Add", nil, x, floats([]int{3}, 10, 20, 30))[0]; !slices.Equal(got.F, []float32{11, 22, 33, 14, 25, 36}) {
		t.Fatalf("Add with a trailing vector %v", got.F)
	}
	if got := run(t, "Mul", nil, floats([]int{2, 1}, 2, 3), x)[0]; !slices.Equal(got.F, []float32{2, 4, 6, 12, 15, 18}) {
		t.Fatalf("Mul with a column %v", got.F)
	}
}

func TestOnnxSafetyAndLayerNorm(t *testing.T) {
	// Two-pass LayerNormalization must not produce NaN on large nearly-constant rows.
	large := floats([]int{1, 4}, 1e7, 1e7+1, 1e7, 1e7+1)
	scale := floats([]int{4}, 1, 1, 1, 1)
	got := run(t, "LayerNormalization", map[string]attribute{"axis": {i: -1}, "epsilon": {f: 1e-5}}, large, scale)[0]
	for i, v := range got.F {
		if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
			t.Fatalf("LayerNormalization[%d] = %v", i, v)
		}
	}
	// Equal must reject mismatched byte dtypes (Int8 vs Uint8).
	if _, err := ops["Equal"](&nodeProto{opType: "Equal"}, []*Tensor{
		{Type: Int8, Shape: []int{1}, B: []byte{0xff}},
		{Type: Uint8, Shape: []int{1}, B: []byte{0xff}},
	}); err == nil {
		t.Fatal("Equal accepted Int8 and Uint8")
	}
	// Float32 Range with delta 0 must return an error instead of panicking/hanging.
	if _, err := ops["Range"](&nodeProto{opType: "Range"}, []*Tensor{
		floats([]int{}, 0), floats([]int{}, 5), floats([]int{}, 0),
	}); err == nil {
		t.Fatal("Range accepted float32 delta=0")
	}
	// Duplicate axes in Unsqueeze must return an error instead of panicking.
	if _, err := ops["Unsqueeze"](&nodeProto{opType: "Unsqueeze"}, []*Tensor{
		floats([]int{2}, 1, 2), int64s(0, 0),
	}); err == nil {
		t.Fatal("Unsqueeze accepted duplicate axes")
	}
}
