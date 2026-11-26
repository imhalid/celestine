package onnx

import "fmt"

// DType is a tensor's element type.
type DType int

// The element types the runner computes with. Int32 and Int64 both keep their values in Tensor.I; Int8, Uint8
// and Bool keep theirs in Tensor.B (an int8 as its two's-complement byte).
const (
	Float32 DType = iota + 1
	Int64
	Int32
	Int8
	Uint8
	Bool
)

// String is the type's ONNX name.
func (d DType) String() string {
	switch d {
	case Float32:
		return "float32"
	case Int64:
		return "int64"
	case Int32:
		return "int32"
	case Int8:
		return "int8"
	case Uint8:
		return "uint8"
	case Bool:
		return "bool"
	}
	return fmt.Sprintf("dtype(%d)", int(d))
}

// Tensor is a dense row-major tensor; one of F, I and B holds its values, by Type.
type Tensor struct {
	Name  string
	Type  DType
	Shape []int
	F     []float32
	I     []int64
	B     []byte
}

// len is the number of values the tensor holds.
func (t *Tensor) len() int {
	switch t.Type {
	case Float32:
		return len(t.F)
	case Int64, Int32:
		return len(t.I)
	}
	return len(t.B)
}

// size is the number of values a shape holds.
func size(shape []int) int {
	n := 1
	for _, d := range shape {
		n *= d
	}
	return n
}
