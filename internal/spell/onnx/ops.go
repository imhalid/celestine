package onnx

import (
	"fmt"
	"math"
	"runtime"
	"sync"
)

// The operators of the graphs this package runs (ONNX opset 17), each to the ONNX specification: the arithmetic in
// float32 where the inputs are float32 (as onnxruntime's), in int64 for integers; the quantized ones in integers, as
// exactly as onnxruntime's.

// op is an operator: its node's attributes, and its inputs (nil for an optional one left out) to its outputs.
type op func(n *nodeProto, in []*Tensor) ([]*Tensor, error)

// ops are the operators the runner knows, by their ONNX names.
var ops map[string]op

func init() {
	ops = map[string]op{
		"Add":                   binaryOp(func(x, y float32) float32 { return x + y }, func(x, y int64) int64 { return x + y }),
		"Sub":                   binaryOp(func(x, y float32) float32 { return x - y }, func(x, y int64) int64 { return x - y }),
		"Mul":                   binaryOp(func(x, y float32) float32 { return x * y }, func(x, y int64) int64 { return x * y }),
		"Div":                   binaryOp(func(x, y float32) float32 { return x / y }, divInt),
		"Equal":                 opEqual,
		"Not":                   opNot,
		"Cast":                  opCast,
		"Clip":                  opClip,
		"Concat":                opConcat,
		"Constant":              opConstant,
		"ConstantOfShape":       opConstantOfShape,
		"DynamicQuantizeLinear": opDynamicQuantizeLinear,
		"Erf":                   opErf,
		"Gather":                opGather,
		"Identity":              func(_ *nodeProto, in []*Tensor) ([]*Tensor, error) { return []*Tensor{in[0]}, nil },
		"LayerNormalization":    opLayerNormalization,
		"MatMul":                opMatMul,
		"MatMulInteger":         opMatMulInteger,
		"Pad":                   opPad,
		"Range":                 opRange,
		"ReduceMax":             opReduceMax,
		"ReduceSum":             opReduceSum,
		"Reshape":               opReshape,
		"Shape":                 opShape,
		"Slice":                 opSlice,
		"Softmax":               opSoftmax,
		"Squeeze":               opSqueeze,
		"Transpose":             opTranspose,
		"Unsqueeze":             opUnsqueeze,
	}
}

// ---- tensors made ----

func newFloat(shape []int) *Tensor {
	return &Tensor{Type: Float32, Shape: shape, F: make([]float32, size(shape))}
}
func newInt(t DType, shape []int) *Tensor {
	return &Tensor{Type: t, Shape: shape, I: make([]int64, size(shape))}
}
func newByte(t DType, shape []int) *Tensor {
	return &Tensor{Type: t, Shape: shape, B: make([]byte, size(shape))}
}

// view is t with another shape and the same values: the operators never change their inputs' values, so a view may
// share them.
func view(t *Tensor, shape []int) *Tensor {
	return &Tensor{Type: t.Type, Shape: shape, F: t.F, I: t.I, B: t.B}
}

// ints are the values of an integer tensor.
func ints(t *Tensor) ([]int64, error) {
	if t == nil {
		return nil, nil
	}
	if t.Type != Int64 && t.Type != Int32 {
		return nil, fmt.Errorf("onnx: %s tensor where integers are wanted", t.Type)
	}
	return t.I, nil
}

// strides are the row-major strides of a shape.
func strides(shape []int) []int {
	s := make([]int, len(shape))
	n := 1
	for i := len(shape) - 1; i >= 0; i-- {
		s[i] = n
		n *= shape[i]
	}
	return s
}

// axis is a (possibly negative) axis of a tensor of rank r, made positive.
func axis(a int64, r int) (int, error) {
	if a < 0 {
		a += int64(r)
	}
	if a < 0 || a >= int64(max(r, 1)) {
		return 0, fmt.Errorf("onnx: axis %d out of a rank %d", a, r)
	}
	return int(a), nil
}

// intAttr is an integer attribute of n, or def.
func intAttr(n *nodeProto, name string, def int64) int64 {
	if a, ok := n.attrs[name]; ok {
		return a.i
	}
	return def
}

// ---- broadcasting ----

// broadcast is the shape two shapes broadcast to, as NumPy's.
func broadcast(a, b []int) ([]int, error) {
	r := max(len(a), len(b))
	out := make([]int, r)
	for i := range r {
		da, db := 1, 1
		if j := len(a) - r + i; j >= 0 {
			da = a[j]
		}
		if j := len(b) - r + i; j >= 0 {
			db = b[j]
		}
		switch {
		case da == db, db == 1:
			out[i] = da
		case da == 1:
			out[i] = db
		default:
			return nil, fmt.Errorf("onnx: shapes %v and %v do not broadcast", a, b)
		}
	}
	return out, nil
}

// broadcastIndex maps each flat index of the shape out to the flat index of a tensor of shape in broadcast to it.
func broadcastIndex(in, out []int) []int {
	idx := make([]int, size(out))
	if size(in) == size(out) && len(in) == len(out) {
		for i := range idx {
			idx[i] = i
		}
		return idx
	}
	st := strides(in)
	r := len(out)
	pad := r - len(in)
	coord := make([]int, r)
	for i := range idx {
		k := 0
		for d := pad; d < r; d++ {
			if in[d-pad] != 1 {
				k += coord[d] * st[d-pad]
			}
		}
		idx[i] = k
		for d := r - 1; d >= 0; d-- { // the next coordinate
			coord[d]++
			if coord[d] < out[d] {
				break
			}
			coord[d] = 0
		}
	}
	return idx
}

// binaryOp is an element-wise operator of two inputs that broadcast, float32 or integer.
func binaryOp(f func(x, y float32) float32, g func(x, y int64) int64) op {
	return func(_ *nodeProto, in []*Tensor) ([]*Tensor, error) {
		a, b := in[0], in[1]
		shape, err := broadcast(a.Shape, b.Shape)
		if err != nil {
			return nil, err
		}
		ia, ib := indexer(a.Shape, shape), indexer(b.Shape, shape)
		switch {
		case a.Type == Float32 && b.Type == Float32:
			out := newFloat(shape)
			parallelRows(len(out.F), len(out.F)*4, func(from, to int) {
				for i := from; i < to; i++ {
					out.F[i] = f(a.F[ia(i)], b.F[ib(i)])
				}
			})
			return []*Tensor{out}, nil
		case (a.Type == Int64 || a.Type == Int32) && a.Type == b.Type:
			out := newInt(a.Type, shape)
			for i := range out.I {
				out.I[i] = g(a.I[ia(i)], b.I[ib(i)])
				if a.Type == Int32 {
					out.I[i] = int64(int32(out.I[i]))
				}
			}
			return []*Tensor{out}, nil
		}
		return nil, fmt.Errorf("onnx: %s and %s in an arithmetic operator", a.Type, b.Type)
	}
}

// indexer maps each flat index of the shape out to the flat index of a tensor of shape in broadcast to it: the same
// index, or 0 for a scalar, or the index modulo a trailing block's size for a tensor that is the last dimensions of
// out (a bias, a scale), without the table of broadcastIndex, which the other cases fall back on.
func indexer(in, out []int) func(int) int {
	n := size(in)
	switch {
	case n == size(out):
		return func(i int) int { return i }
	case n == 1:
		return func(int) int { return 0 }
	}
	trim := in
	for len(trim) > 0 && trim[0] == 1 {
		trim = trim[1:]
	}
	if len(trim) <= len(out) && slicesEqual(trim, out[len(out)-len(trim):]) {
		return func(i int) int { return i % n }
	}
	idx := broadcastIndex(in, out)
	return func(i int) int { return idx[i] }
}

// slicesEqual reports whether two shapes are the same.
func slicesEqual(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// divInt is an integer division, truncated toward zero, as C's.
func divInt(x, y int64) int64 {
	if y == 0 {
		return 0
	}
	return x / y
}

func opEqual(_ *nodeProto, in []*Tensor) ([]*Tensor, error) {
	a, b := in[0], in[1]
	shape, err := broadcast(a.Shape, b.Shape)
	if err != nil {
		return nil, err
	}
	ia, ib := broadcastIndex(a.Shape, shape), broadcastIndex(b.Shape, shape)
	out := newByte(Bool, shape)
	for i := range out.B {
		var eq bool
		switch {
		case a.Type == Float32 && b.Type == Float32:
			eq = a.F[ia[i]] == b.F[ib[i]]
		case a.I != nil && b.I != nil:
			eq = a.I[ia[i]] == b.I[ib[i]]
		case a.B != nil && b.B != nil:
			eq = a.B[ia[i]] == b.B[ib[i]]
		default:
			return nil, fmt.Errorf("onnx: Equal of %s and %s", a.Type, b.Type)
		}
		if eq {
			out.B[i] = 1
		}
	}
	return []*Tensor{out}, nil
}

func opNot(_ *nodeProto, in []*Tensor) ([]*Tensor, error) {
	if in[0].Type != Bool {
		return nil, fmt.Errorf("onnx: Not of %s", in[0].Type)
	}
	out := newByte(Bool, in[0].Shape)
	for i, v := range in[0].B {
		if v == 0 {
			out.B[i] = 1
		}
	}
	return []*Tensor{out}, nil
}

// ---- values ----

// floatOf is value i of t as a float64.
func floatOf(t *Tensor, i int) float64 {
	switch t.Type {
	case Float32:
		return float64(t.F[i])
	case Int64, Int32:
		return float64(t.I[i])
	case Int8:
		return float64(int8(t.B[i]))
	}
	return float64(t.B[i])
}

// intOf is value i of t as an int64, a float truncated toward zero as C's cast does.
func intOf(t *Tensor, i int) int64 {
	switch t.Type {
	case Float32:
		f := t.F[i]
		if f != f {
			return 0
		}
		return int64(f)
	case Int64, Int32:
		return t.I[i]
	case Int8:
		return int64(int8(t.B[i]))
	}
	return int64(t.B[i])
}

func opCast(n *nodeProto, in []*Tensor) ([]*Tensor, error) {
	x := in[0]
	to := intAttr(n, "to", 0)
	m := x.len()
	switch to {
	case typeFloat:
		out := newFloat(x.Shape)
		for i := range m {
			if x.Type == Float32 {
				out.F[i] = x.F[i]
			} else {
				out.F[i] = float32(floatOf(x, i))
			}
		}
		return []*Tensor{out}, nil
	case typeInt64, typeInt32:
		t := Int64
		if to == typeInt32 {
			t = Int32
		}
		out := newInt(t, x.Shape)
		for i := range m {
			out.I[i] = intOf(x, i)
			if t == Int32 {
				out.I[i] = int64(int32(out.I[i]))
			}
		}
		return []*Tensor{out}, nil
	case typeBool:
		out := newByte(Bool, x.Shape)
		for i := range m {
			if floatOf(x, i) != 0 {
				out.B[i] = 1
			}
		}
		return []*Tensor{out}, nil
	case typeUint8, typeInt8:
		t := Uint8
		if to == typeInt8 {
			t = Int8
		}
		out := newByte(t, x.Shape)
		for i := range m {
			out.B[i] = byte(intOf(x, i))
		}
		return []*Tensor{out}, nil
	}
	return nil, fmt.Errorf("onnx: Cast to element type %d", to)
}

func opClip(_ *nodeProto, in []*Tensor) ([]*Tensor, error) {
	x := in[0]
	switch x.Type {
	case Float32:
		lo, hi := float32(math.Inf(-1)), float32(math.Inf(1))
		if len(in) > 1 && in[1] != nil {
			lo = in[1].F[0]
		}
		if len(in) > 2 && in[2] != nil {
			hi = in[2].F[0]
		}
		out := newFloat(x.Shape)
		for i, v := range x.F {
			out.F[i] = min(max(v, lo), hi)
		}
		return []*Tensor{out}, nil
	case Int64, Int32:
		lo, hi := int64(math.MinInt64), int64(math.MaxInt64)
		if len(in) > 1 && in[1] != nil {
			lo = in[1].I[0]
		}
		if len(in) > 2 && in[2] != nil {
			hi = in[2].I[0]
		}
		out := newInt(x.Type, x.Shape)
		for i, v := range x.I {
			out.I[i] = min(max(v, lo), hi)
		}
		return []*Tensor{out}, nil
	}
	return nil, fmt.Errorf("onnx: Clip of %s", x.Type)
}

func opConstant(n *nodeProto, _ []*Tensor) ([]*Tensor, error) {
	if a, ok := n.attrs["value"]; ok && a.t != nil {
		return []*Tensor{a.t}, nil
	}
	if a, ok := n.attrs["value_float"]; ok {
		return []*Tensor{{Type: Float32, Shape: []int{}, F: []float32{a.f}}}, nil
	}
	if a, ok := n.attrs["value_int"]; ok {
		return []*Tensor{{Type: Int64, Shape: []int{}, I: []int64{a.i}}}, nil
	}
	if a, ok := n.attrs["value_ints"]; ok {
		return []*Tensor{{Type: Int64, Shape: []int{len(a.ints)}, I: a.ints}}, nil
	}
	if a, ok := n.attrs["value_floats"]; ok {
		return []*Tensor{{Type: Float32, Shape: []int{len(a.floats)}, F: a.floats}}, nil
	}
	return nil, fmt.Errorf("onnx: a Constant of a kind not read")
}

func opConstantOfShape(n *nodeProto, in []*Tensor) ([]*Tensor, error) {
	dims, err := ints(in[0])
	if err != nil {
		return nil, err
	}
	shape := make([]int, len(dims))
	for i, d := range dims {
		shape[i] = int(d)
	}
	v := &Tensor{Type: Float32, Shape: []int{1}, F: []float32{0}}
	if a, ok := n.attrs["value"]; ok && a.t != nil {
		v = a.t
	}
	out := &Tensor{Type: v.Type, Shape: shape}
	switch v.Type {
	case Float32:
		out.F = make([]float32, size(shape))
		for i := range out.F {
			out.F[i] = v.F[0]
		}
	case Int64, Int32:
		out.I = make([]int64, size(shape))
		for i := range out.I {
			out.I[i] = v.I[0]
		}
	default:
		out.B = make([]byte, size(shape))
		for i := range out.B {
			out.B[i] = v.B[0]
		}
	}
	return []*Tensor{out}, nil
}

func opErf(_ *nodeProto, in []*Tensor) ([]*Tensor, error) {
	out := newFloat(in[0].Shape)
	x := in[0].F
	parallelRows(len(x), len(x)*32, func(from, to int) {
		for i := from; i < to; i++ {
			out.F[i] = float32(math.Erf(float64(x[i])))
		}
	})
	return []*Tensor{out}, nil
}

// ---- shapes ----

func opShape(n *nodeProto, in []*Tensor) ([]*Tensor, error) {
	r := len(in[0].Shape)
	start, end := intAttr(n, "start", 0), intAttr(n, "end", int64(r))
	if start < 0 {
		start += int64(r)
	}
	if end < 0 {
		end += int64(r)
	}
	start, end = max(0, min(start, int64(r))), max(0, min(end, int64(r)))
	out := newInt(Int64, []int{int(max(0, end-start))})
	for i := start; i < end; i++ {
		out.I[i-start] = int64(in[0].Shape[i])
	}
	return []*Tensor{out}, nil
}

func opReshape(n *nodeProto, in []*Tensor) ([]*Tensor, error) {
	dims, err := ints(in[1])
	if err != nil {
		return nil, err
	}
	allowZero := intAttr(n, "allowzero", 0) != 0
	shape := make([]int, len(dims))
	infer, known := -1, 1
	for i, d := range dims {
		switch {
		case d == 0 && !allowZero:
			shape[i] = in[0].Shape[i]
		case d == -1:
			infer = i
			continue
		default:
			shape[i] = int(d)
		}
		known *= shape[i]
	}
	if infer >= 0 {
		if known == 0 {
			shape[infer] = 0
		} else {
			shape[infer] = in[0].len() / known
		}
	}
	if size(shape) != in[0].len() {
		return nil, fmt.Errorf("onnx: Reshape of %v to %v", in[0].Shape, shape)
	}
	return []*Tensor{view(in[0], shape)}, nil
}

func opUnsqueeze(_ *nodeProto, in []*Tensor) ([]*Tensor, error) {
	axes, err := ints(in[1])
	if err != nil {
		return nil, err
	}
	r := len(in[0].Shape) + len(axes)
	at := map[int]bool{}
	for _, a := range axes {
		p, err := axis(a, r)
		if err != nil {
			return nil, err
		}
		at[p] = true
	}
	shape := make([]int, 0, r)
	j := 0
	for i := range r {
		if at[i] {
			shape = append(shape, 1)
		} else {
			shape = append(shape, in[0].Shape[j])
			j++
		}
	}
	return []*Tensor{view(in[0], shape)}, nil
}

func opSqueeze(_ *nodeProto, in []*Tensor) ([]*Tensor, error) {
	x := in[0]
	at := map[int]bool{}
	if len(in) > 1 && in[1] != nil {
		axes, err := ints(in[1])
		if err != nil {
			return nil, err
		}
		for _, a := range axes {
			p, err := axis(a, len(x.Shape))
			if err != nil {
				return nil, err
			}
			at[p] = true
		}
	} else {
		for i, d := range x.Shape {
			if d == 1 {
				at[i] = true
			}
		}
	}
	shape := []int{}
	for i, d := range x.Shape {
		if !at[i] {
			shape = append(shape, d)
		}
	}
	return []*Tensor{view(x, shape)}, nil
}

// copyBlock copies count values of src from index from to dst at index to, whatever the tensors' element type.
func copyBlock(dst, src *Tensor, to, from, count int) {
	switch {
	case src.F != nil:
		copy(dst.F[to:to+count], src.F[from:from+count])
	case src.I != nil:
		copy(dst.I[to:to+count], src.I[from:from+count])
	default:
		copy(dst.B[to:to+count], src.B[from:from+count])
	}
}

// like is a tensor of x's type and of shape, empty.
func like(x *Tensor, shape []int) *Tensor {
	switch {
	case x.Type == Float32:
		return newFloat(shape)
	case x.Type == Int64 || x.Type == Int32:
		return newInt(x.Type, shape)
	}
	return newByte(x.Type, shape)
}

func opGather(n *nodeProto, in []*Tensor) ([]*Tensor, error) {
	data, idx := in[0], in[1]
	ax, err := axis(intAttr(n, "axis", 0), len(data.Shape))
	if err != nil {
		return nil, err
	}
	is, err := ints(idx)
	if err != nil {
		return nil, err
	}
	outer := size(data.Shape[:ax])
	dim := data.Shape[ax]
	inner := size(data.Shape[ax+1:])
	shape := append(append(append([]int{}, data.Shape[:ax]...), idx.Shape...), data.Shape[ax+1:]...)
	out := like(data, shape)
	for o := range outer {
		for j, k := range is {
			if k < 0 {
				k += int64(dim)
			}
			if k < 0 || k >= int64(dim) {
				return nil, fmt.Errorf("onnx: Gather index %d out of %d", k, dim)
			}
			copyBlock(out, data, (o*len(is)+j)*inner, (o*dim+int(k))*inner, inner)
		}
	}
	return []*Tensor{out}, nil
}

func opConcat(n *nodeProto, in []*Tensor) ([]*Tensor, error) {
	ax, err := axis(intAttr(n, "axis", 0), len(in[0].Shape))
	if err != nil {
		return nil, err
	}
	shape := append([]int{}, in[0].Shape...)
	shape[ax] = 0
	for _, t := range in {
		shape[ax] += t.Shape[ax]
	}
	out := like(in[0], shape)
	outer := size(shape[:ax])
	inner := size(shape[ax+1:])
	off := 0
	for _, t := range in {
		block := t.Shape[ax] * inner
		for o := range outer {
			copyBlock(out, t, o*shape[ax]*inner+off, o*block, block)
		}
		off += block
	}
	return []*Tensor{out}, nil
}

func opTranspose(n *nodeProto, in []*Tensor) ([]*Tensor, error) {
	x := in[0]
	r := len(x.Shape)
	perm := make([]int, r)
	if a, ok := n.attrs["perm"]; ok {
		for i, p := range a.ints {
			perm[i] = int(p)
		}
	} else {
		for i := range perm {
			perm[i] = r - 1 - i
		}
	}
	shape := make([]int, r)
	for i, p := range perm {
		shape[i] = x.Shape[p]
	}
	st := strides(x.Shape)
	out := like(x, shape)
	coord := make([]int, r)
	for i := range size(shape) {
		k := 0
		for d := range r {
			k += coord[d] * st[perm[d]]
		}
		copyBlock(out, x, i, k, 1)
		for d := r - 1; d >= 0; d-- {
			coord[d]++
			if coord[d] < shape[d] {
				break
			}
			coord[d] = 0
		}
	}
	return []*Tensor{out}, nil
}

func opSlice(_ *nodeProto, in []*Tensor) ([]*Tensor, error) {
	x := in[0]
	r := len(x.Shape)
	starts, err := ints(in[1])
	if err != nil {
		return nil, err
	}
	ends, err := ints(in[2])
	if err != nil {
		return nil, err
	}
	axes := make([]int64, len(starts))
	for i := range axes {
		axes[i] = int64(i)
	}
	if len(in) > 3 && in[3] != nil {
		axes = in[3].I
	}
	steps := make([]int64, len(starts))
	for i := range steps {
		steps[i] = 1
	}
	if len(in) > 4 && in[4] != nil {
		steps = in[4].I
	}
	from, step := make([]int, r), make([]int, r)
	shape := append([]int{}, x.Shape...)
	for d := range r {
		step[d] = 1
	}
	for i, a := range axes {
		d, err := axis(a, r)
		if err != nil {
			return nil, err
		}
		dim := int64(x.Shape[d])
		s, e, st := starts[i], ends[i], steps[i]
		if st == 0 {
			return nil, fmt.Errorf("onnx: Slice with a step of 0")
		}
		if s < 0 {
			s += dim
		}
		if e < 0 {
			e += dim
		}
		if st > 0 {
			s, e = max(0, min(s, dim)), max(0, min(e, dim))
			shape[d] = int(max(0, (e-s+st-1)/st))
		} else {
			s, e = max(-1, min(s, dim-1)), max(-1, min(e, dim-1))
			shape[d] = int(max(0, (s-e-st-1)/(-st)))
		}
		from[d], step[d] = int(s), int(st)
	}
	st := strides(x.Shape)
	out := like(x, shape)
	coord := make([]int, r)
	for i := range size(shape) {
		k := 0
		for d := range r {
			k += (from[d] + coord[d]*step[d]) * st[d]
		}
		copyBlock(out, x, i, k, 1)
		for d := r - 1; d >= 0; d-- {
			coord[d]++
			if coord[d] < shape[d] {
				break
			}
			coord[d] = 0
		}
	}
	return []*Tensor{out}, nil
}

func opPad(n *nodeProto, in []*Tensor) ([]*Tensor, error) {
	if a, ok := n.attrs["mode"]; ok && string(a.s) != "constant" {
		return nil, fmt.Errorf("onnx: Pad of mode %q", a.s)
	}
	if len(in) > 3 && in[3] != nil {
		return nil, fmt.Errorf("onnx: Pad with axes")
	}
	x := in[0]
	r := len(x.Shape)
	pads, err := ints(in[1])
	if err != nil {
		return nil, err
	}
	if len(pads) != 2*r {
		return nil, fmt.Errorf("onnx: Pad of %d values for a rank %d", len(pads), r)
	}
	shape := make([]int, r)
	for d := range r {
		shape[d] = x.Shape[d] + int(pads[d]) + int(pads[r+d])
	}
	out := like(x, shape)
	if len(in) > 2 && in[2] != nil && in[2].len() > 0 { // the constant, where it is not 0
		for i := range size(shape) {
			switch {
			case out.F != nil:
				out.F[i] = in[2].F[0]
			case out.I != nil:
				out.I[i] = in[2].I[0]
			default:
				out.B[i] = in[2].B[0]
			}
		}
	}
	st := strides(shape)
	coord := make([]int, r)
	for i := range x.len() {
		k, inside := 0, true
		for d := range r {
			c := coord[d] + int(pads[d])
			if c < 0 || c >= shape[d] {
				inside = false
				break
			}
			k += c * st[d]
		}
		if inside {
			copyBlock(out, x, k, i, 1)
		}
		for d := r - 1; d >= 0; d-- {
			coord[d]++
			if coord[d] < x.Shape[d] {
				break
			}
			coord[d] = 0
		}
	}
	return []*Tensor{out}, nil
}

func opRange(_ *nodeProto, in []*Tensor) ([]*Tensor, error) {
	start, limit, delta := in[0], in[1], in[2]
	switch start.Type {
	case Int64, Int32:
		s, l, d := start.I[0], limit.I[0], delta.I[0]
		if d == 0 {
			return nil, fmt.Errorf("onnx: Range with a delta of 0")
		}
		m := int(max(0, int64(math.Ceil(float64(l-s)/float64(d)))))
		out := newInt(start.Type, []int{m})
		for i := range out.I {
			out.I[i] = s + int64(i)*d
		}
		return []*Tensor{out}, nil
	case Float32:
		s, l, d := start.F[0], limit.F[0], delta.F[0]
		m := int(max(0, math.Ceil(float64((l-s)/d))))
		out := newFloat([]int{m})
		for i := range out.F {
			out.F[i] = s + float32(i)*d
		}
		return []*Tensor{out}, nil
	}
	return nil, fmt.Errorf("onnx: Range of %s", start.Type)
}

// ---- reductions ----

// reduceAxes are the axes a reduction runs over, all of them when none are given (unless noopEmpty).
func reduceAxes(axes []int64, r int, noopEmpty bool) (map[int]bool, error) {
	at := map[int]bool{}
	if len(axes) == 0 {
		if !noopEmpty {
			for d := range r {
				at[d] = true
			}
		}
		return at, nil
	}
	for _, a := range axes {
		d, err := axis(a, r)
		if err != nil {
			return nil, err
		}
		at[d] = true
	}
	return at, nil
}

// reduce reduces x over the axes at, starting each output value at init and folding each input value with f; the
// output keeps the axes as 1 when keep is set.
func reduce(x *Tensor, at map[int]bool, keep bool, init float32, f func(acc, v float32) float32, initI int64, g func(acc, v int64) int64) (*Tensor, error) {
	r := len(x.Shape)
	full := make([]int, r) // the output's shape with the axes kept as 1
	var shape []int
	for d := range r {
		full[d] = x.Shape[d]
		if at[d] {
			full[d] = 1
			if keep {
				shape = append(shape, 1)
			}
		} else {
			shape = append(shape, x.Shape[d])
		}
	}
	if shape == nil {
		shape = []int{}
	}
	ost := strides(full)
	coord := make([]int, r)
	var out *Tensor
	switch x.Type {
	case Float32:
		out = newFloat(shape)
		for i := range out.F {
			out.F[i] = init
		}
	case Int64, Int32:
		out = newInt(x.Type, shape)
		for i := range out.I {
			out.I[i] = initI
		}
	default:
		return nil, fmt.Errorf("onnx: a reduction of %s", x.Type)
	}
	for i := range x.len() {
		k := 0
		for d := range r {
			if !at[d] {
				k += coord[d] * ost[d]
			}
		}
		if out.F != nil {
			out.F[k] = f(out.F[k], x.F[i])
		} else {
			out.I[k] = g(out.I[k], x.I[i])
		}
		for d := r - 1; d >= 0; d-- {
			coord[d]++
			if coord[d] < x.Shape[d] {
				break
			}
			coord[d] = 0
		}
	}
	return out, nil
}

func opReduceSum(n *nodeProto, in []*Tensor) ([]*Tensor, error) {
	var axes []int64
	if len(in) > 1 && in[1] != nil {
		axes = in[1].I
	}
	at, err := reduceAxes(axes, len(in[0].Shape), intAttr(n, "noop_with_empty_axes", 0) != 0)
	if err != nil {
		return nil, err
	}
	out, err := reduce(in[0], at, intAttr(n, "keepdims", 1) != 0, 0, func(a, v float32) float32 { return a + v }, 0, func(a, v int64) int64 { return a + v })
	return []*Tensor{out}, err
}

func opReduceMax(n *nodeProto, in []*Tensor) ([]*Tensor, error) {
	var axes []int64
	if a, ok := n.attrs["axes"]; ok {
		axes = a.ints
	}
	if len(in) > 1 && in[1] != nil { // opset 18's input
		axes = in[1].I
	}
	at, err := reduceAxes(axes, len(in[0].Shape), intAttr(n, "noop_with_empty_axes", 0) != 0)
	if err != nil {
		return nil, err
	}
	out, err := reduce(in[0], at, intAttr(n, "keepdims", 1) != 0, float32(math.Inf(-1)), func(a, v float32) float32 {
		if v > a || v != v {
			return v
		}
		return a
	}, math.MinInt64, func(a, v int64) int64 { return max(a, v) })
	return []*Tensor{out}, err
}

// ---- the network's own ----

func opSoftmax(n *nodeProto, in []*Tensor) ([]*Tensor, error) {
	x := in[0]
	ax, err := axis(intAttr(n, "axis", -1), len(x.Shape))
	if err != nil {
		return nil, err
	}
	dim := x.Shape[ax]
	inner := size(x.Shape[ax+1:])
	outer := size(x.Shape[:ax])
	out := newFloat(x.Shape)
	parallelRows(outer*inner, x.len()*16, func(from, to int) {
		for ok := from; ok < to; ok++ {
			o, k := ok/inner, ok%inner
			base := o*dim*inner + k
			mx := float32(math.Inf(-1))
			for j := range dim {
				mx = max(mx, x.F[base+j*inner])
			}
			var sum float32
			for j := range dim {
				e := float32(math.Exp(float64(x.F[base+j*inner] - mx)))
				out.F[base+j*inner] = e
				sum += e
			}
			for j := range dim {
				out.F[base+j*inner] /= sum
			}
		}
	})
	return []*Tensor{out}, nil
}

func opLayerNormalization(n *nodeProto, in []*Tensor) ([]*Tensor, error) {
	x, scale := in[0], in[1]
	var bias *Tensor
	if len(in) > 2 {
		bias = in[2]
	}
	ax, err := axis(intAttr(n, "axis", -1), len(x.Shape))
	if err != nil {
		return nil, err
	}
	eps := float32(1e-5)
	if a, ok := n.attrs["epsilon"]; ok {
		eps = a.f
	}
	norm := size(x.Shape[ax:])
	rows := x.len() / max(norm, 1)
	out := newFloat(x.Shape)
	parallelRows(rows, x.len()*8, func(from, to int) {
		for r := from; r < to; r++ {
			row := x.F[r*norm : (r+1)*norm]
			var mean, sq float64 // onnxruntime sums in double for float input
			for _, v := range row {
				mean += float64(v)
				sq += float64(v) * float64(v)
			}
			mean /= float64(norm)
			variance := sq/float64(norm) - mean*mean
			inv := 1 / math.Sqrt(variance+float64(eps))
			for i, v := range row {
				y := float32((float64(v) - mean) * inv)
				y *= scale.F[i%scale.len()]
				if bias != nil {
					y += bias.F[i%bias.len()]
				}
				out.F[r*norm+i] = y
			}
		}
	})
	return []*Tensor{out}, nil
}

// matShapes are the batch shape a MatMul's two inputs broadcast to and its m, k, n, NumPy's matmul rules, a
// one-dimensional input a row or a column.
func matShapes(a, b []int) (batch []int, m, k, n int, err error) {
	if len(a) < 2 || len(b) < 2 {
		return nil, 0, 0, 0, fmt.Errorf("onnx: MatMul of %v and %v", a, b)
	}
	m, k = a[len(a)-2], a[len(a)-1]
	if b[len(b)-2] != k {
		return nil, 0, 0, 0, fmt.Errorf("onnx: MatMul of %v and %v", a, b)
	}
	n = b[len(b)-1]
	batch, err = broadcast(a[:len(a)-2], b[:len(b)-2])
	return batch, m, k, n, err
}

// batchIndex maps each matrix of the batch shape to the matrix of a tensor of batch shape in broadcast to it.
func batchIndex(in, batch []int) []int {
	return broadcastIndex(in, batch)
}

func opMatMul(_ *nodeProto, in []*Tensor) ([]*Tensor, error) {
	a, b := in[0], in[1]
	if a.Type != Float32 || b.Type != Float32 {
		return nil, fmt.Errorf("onnx: MatMul of %s and %s", a.Type, b.Type)
	}
	batch, m, k, n, err := matShapes(a.Shape, b.Shape)
	if err != nil {
		return nil, err
	}
	out := newFloat(append(append([]int{}, batch...), m, n))
	ia, ib := batchIndex(a.Shape[:len(a.Shape)-2], batch), batchIndex(b.Shape[:len(b.Shape)-2], batch)
	for p := range size(batch) {
		am, bm, om := a.F[ia[p]*m*k:], b.F[ib[p]*k*n:], out.F[p*m*n:]
		for i := range m {
			row := om[i*n : (i+1)*n]
			for kk := range k {
				av := am[i*k+kk]
				if av == 0 {
					continue
				}
				bk := bm[kk*n : (kk+1)*n]
				for j := range row {
					row[j] += av * bk[j]
				}
			}
		}
	}
	return []*Tensor{out}, nil
}

// intWeights is a constant matrix of MatMulInteger made ready once, for two of its columns at a time in one 64-bit
// multiply: each value offset to 0..255 (an int8 one by 128), two columns side by side in the halves of a uint64,
// row by row, and each column's sum of the offset values. A row of activations (0..255) times a pair of columns
// sums both columns' products at once, each in its own 32-bit half: 255·255·k stays under 2³² for any k under 66051,
// so a half never carries into the other. The offsets and the zero points are taken out after (intDot): the result
// is the exact integer of the multiply as written.
type intWeights struct {
	pairs  []uint64 // for each pair of columns, k rows
	colSum []int64  // each column's sum of its offset values
	off    int64    // the offset added: 128 for int8, 0 for uint8
	zp     []int64  // each column's zero point
	k, n   int
}

// weightsReady are the matrices made ready, by the weight tensor and its zero point: a graph's weights do not change.
var weightsReady sync.Map // [2]*Tensor → *intWeights

// byteVal is value i of an int8 or uint8 tensor.
func byteVal(t *Tensor, i int) int32 {
	if t.Type == Int8 {
		return int32(int8(t.B[i]))
	}
	return int32(t.B[i])
}

// readyWeights is the matrix b (k by n, with its zero point zp, nil for 0) made ready, once for a weight.
func readyWeights(b, zp *Tensor, k, n int) *intWeights {
	key := [2]*Tensor{b, zp}
	if w, ok := weightsReady.Load(key); ok {
		return w.(*intWeights)
	}
	w := &intWeights{pairs: make([]uint64, (n+1)/2*k), colSum: make([]int64, n), zp: make([]int64, n), k: k, n: n}
	if b.Type == Int8 {
		w.off = 128
	}
	for j := range n {
		if zp != nil {
			if zp.len() == 1 {
				w.zp[j] = int64(byteVal(zp, 0))
			} else {
				w.zp[j] = int64(byteVal(zp, j))
			}
		}
		shift := uint(32 * (j % 2))
		for kk := range k {
			v := uint64(int64(byteVal(b, kk*n+j)) + w.off) // 0..255
			w.pairs[(j/2)*k+kk] |= v << shift
			w.colSum[j] += int64(v)
		}
	}
	weightsReady.Store(key, w)
	return w
}

// pairDot4 is pairDot of four rows at once, each weight loaded once for the four: the weights, not the products, are
// what a row waits for.
func pairDot4(a0, a1, a2, a3, p []uint64) (s0, s1, s2, s3 uint64) {
	n := len(p)
	a0, a1, a2, a3 = a0[:n], a1[:n], a2[:n], a3[:n]
	for i, w := range p {
		s0 += a0[i] * w
		s1 += a1[i] * w
		s2 += a2[i] * w
		s3 += a3[i] * w
	}
	return
}

// pairDot is the row a times the pair of columns p, both of length k: the low half of the result the first column's
// sum of products, the high half the second's.
func pairDot(a, p []uint64) uint64 {
	p = p[:len(a)]
	var s0, s1, s2, s3 uint64
	i := 0
	for ; i+4 <= len(a); i += 4 {
		aa, pp := a[i:i+4:i+4], p[i:i+4:i+4]
		s0 += aa[0] * pp[0]
		s1 += aa[1] * pp[1]
		s2 += aa[2] * pp[2]
		s3 += aa[3] * pp[3]
	}
	for ; i < len(a); i++ {
		s0 += a[i] * p[i]
	}
	return s0 + s1 + s2 + s3
}

// parallelRows runs fn over [0, rows) in parts, on as many goroutines as there are processors when the work, work
// multiply-adds in all, is worth it.
func parallelRows(rows, work int, fn func(from, to int)) {
	parts := runtime.GOMAXPROCS(0)
	if work < 1<<20 || parts < 2 || rows < 2 {
		fn(0, rows)
		return
	}
	parts = min(parts, rows)
	var wg sync.WaitGroup
	for p := range parts {
		from, to := rows*p/parts, rows*(p+1)/parts
		wg.Add(1)
		go func() {
			defer wg.Done()
			fn(from, to)
		}()
	}
	wg.Wait()
}

// opMatMulInteger multiplies a uint8 or int8 matrix by a two-dimensional int8 or uint8 one, each less its zero point,
// as onnxruntime does: exact, the products summed two columns at a time (intWeights). The second is a weight of the
// graph, made ready once; the rows of the first are shared among the processors.
func opMatMulInteger(_ *nodeProto, in []*Tensor) ([]*Tensor, error) {
	a, b := in[0], in[1]
	if len(b.Shape) != 2 {
		return nil, fmt.Errorf("onnx: MatMulInteger by a matrix of rank %d", len(b.Shape))
	}
	batch, m, k, n, err := matShapes(a.Shape, b.Shape)
	if err != nil {
		return nil, err
	}
	var azp int32
	if len(in) > 2 && in[2] != nil {
		if in[2].len() != 1 {
			return nil, fmt.Errorf("onnx: MatMulInteger with a zero point per row")
		}
		azp = byteVal(in[2], 0)
	}
	var bzp *Tensor
	if len(in) > 3 {
		bzp = in[3]
	}
	if k >= 66051 { // the halves of a pair's sums would carry into each other
		return nil, fmt.Errorf("onnx: MatMulInteger over %d values a row; at most 66050 here", k)
	}
	w := readyWeights(b, bzp, k, n)
	rows := size(batch) * m // a's rows, the batch's matrices one after another: b is the same for all
	out := newInt(Int32, append(append([]int{}, batch...), m, n))
	var aoff int64 // the activations offset to 0..255 too, for an int8 a
	if a.Type == Int8 {
		aoff = 128
	}
	A := int64(azp) + aoff
	// put writes column j of row r from its pair's half h of s: Σ(a'−A)(w'−W) with a' = a+aoff, w' = w+off,
	// A = azp+aoff, W = zp+off, is Σa'w' − W·Σa' − A·Σw' + k·A·W.
	put := func(r, j int, s uint64, h uint, sumA int64) {
		W := w.zp[j] + w.off
		out.I[r*n+j] = int64(int32(int64(uint32(s>>(32*h))) - W*sumA - A*w.colSum[j] + int64(k)*A*W))
	}
	parallelRows((rows+3)/4, rows*k*n, func(from, to int) { // in blocks of four rows
		av := make([][]uint64, 4)
		for q := range av {
			av[q] = make([]uint64, k)
		}
		var sumA [4]int64
		for blk := from; blk < to; blk++ {
			r0 := blk * 4
			nr := min(4, rows-r0)
			for q := range 4 {
				sumA[q] = 0
				if q >= nr { // a block's missing rows: zeros, whose sums are not kept
					clear(av[q])
					continue
				}
				for kk := range k {
					v := int64(byteVal(a, (r0+q)*k+kk)) + aoff
					av[q][kk] = uint64(v)
					sumA[q] += v
				}
			}
			for jp := 0; jp < n; jp += 2 {
				pairs := w.pairs[(jp/2)*k : (jp/2+1)*k]
				var s [4]uint64
				if nr == 4 {
					s[0], s[1], s[2], s[3] = pairDot4(av[0], av[1], av[2], av[3], pairs)
				} else {
					for q := range nr {
						s[q] = pairDot(av[q], pairs)
					}
				}
				for q := range nr {
					for h := uint(0); h < 2 && jp+int(h) < n; h++ {
						put(r0+q, jp+int(h), s[q], h, sumA[q])
					}
				}
			}
		}
	})
	return []*Tensor{out}, nil
}

// roundEven is x rounded to the nearest integer, a tie to the even one, as nearbyintf in the default rounding mode.
func roundEven(x float32) float32 { return float32(math.RoundToEven(float64(x))) }

// opDynamicQuantizeLinear quantizes a float32 tensor to uint8 with a scale and a zero point of its own range, as the
// ONNX specification and onnxruntime have it: the range taken with 0 in it, the scale (max-min)/255 in float32, the
// zero point and each value rounded half to even and saturated to [0, 255].
func opDynamicQuantizeLinear(_ *nodeProto, in []*Tensor) ([]*Tensor, error) {
	x := in[0]
	if x.Type != Float32 {
		return nil, fmt.Errorf("onnx: DynamicQuantizeLinear of %s", x.Type)
	}
	lo, hi := float32(0), float32(0)
	for _, v := range x.F {
		lo, hi = min(lo, v), max(hi, v)
	}
	scale := (hi - lo) / 255
	var zp float32
	if scale != 0 {
		zp = roundEven(min(255, max(0, 0-lo/scale)))
	}
	y := newByte(Uint8, x.Shape)
	if scale != 0 {
		parallelRows(len(x.F), len(x.F)*4, func(from, to int) {
			for i := from; i < to; i++ {
				y.B[i] = byte(min(255, max(0, roundEven(x.F[i]/scale)+zp)))
			}
		})
	}
	return []*Tensor{
		y,
		{Type: Float32, Shape: []int{}, F: []float32{scale}},
		{Type: Uint8, Shape: []int{}, B: []byte{byte(zp)}},
	}, nil
}
