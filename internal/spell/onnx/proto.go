// Package onnx reads an ONNX model and runs it, in pure Go: a decoder of the protobuf wire format for the
// messages a model file holds, and an interpreter of the operators tinyfix-tr's context model uses. It is not a
// general ONNX runtime; an operator it does not know is an error when the model is loaded, not when it runs.
package onnx

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
)

// The ONNX tensor element types this package reads (onnx.proto's TensorProto.DataType).
const (
	typeFloat  = 1
	typeUint8  = 2
	typeInt8   = 3
	typeInt32  = 6
	typeInt64  = 7
	typeBool   = 9
	typeDouble = 11
)

// The ONNX attribute types (AttributeProto.AttributeType).
const (
	attrFloat   = 1
	attrInt     = 2
	attrString  = 3
	attrTensor  = 4
	attrFloats  = 6
	attrInts    = 7
	attrStrings = 8
)

// errTruncated is a message that ends in the middle of a field.
var errTruncated = errors.New("onnx: truncated message")

// wire is a cursor over a protobuf message.
type wire struct {
	b []byte
}

// varint reads a base-128 varint.
func (w *wire) varint() (uint64, error) {
	var x uint64
	for shift := uint(0); shift < 64; shift += 7 {
		if len(w.b) == 0 {
			return 0, errTruncated
		}
		c := w.b[0]
		w.b = w.b[1:]
		x |= uint64(c&0x7f) << shift
		if c < 0x80 {
			return x, nil
		}
	}
	return 0, errors.New("onnx: varint too long")
}

// field reads a field's key: its number and wire type.
func (w *wire) field() (num int, typ int, err error) {
	k, err := w.varint()
	if err != nil {
		return 0, 0, err
	}
	return int(k >> 3), int(k & 7), nil
}

// bytes reads a length-delimited field's payload.
func (w *wire) bytes() ([]byte, error) {
	n, err := w.varint()
	if err != nil {
		return nil, err
	}
	if n > uint64(len(w.b)) {
		return nil, errTruncated
	}
	p := w.b[:n]
	w.b = w.b[n:]
	return p, nil
}

// fixed32 reads a 4-byte little-endian field.
func (w *wire) fixed32() (uint32, error) {
	if len(w.b) < 4 {
		return 0, errTruncated
	}
	x := binary.LittleEndian.Uint32(w.b)
	w.b = w.b[4:]
	return x, nil
}

// skip passes over a field of wire type typ that the decoder does not read.
func (w *wire) skip(typ int) error {
	switch typ {
	case 0:
		_, err := w.varint()
		return err
	case 1:
		if len(w.b) < 8 {
			return errTruncated
		}
		w.b = w.b[8:]
	case 2:
		_, err := w.bytes()
		return err
	case 5:
		_, err := w.fixed32()
		return err
	default:
		return fmt.Errorf("onnx: unsupported wire type %d", typ)
	}
	return nil
}

// ints reads a repeated integer field, packed (wire type 2) or not (wire type 0), appending to dst.
func (w *wire) ints(typ int, dst []int64) ([]int64, error) {
	if typ == 0 {
		v, err := w.varint()
		return append(dst, int64(v)), err
	}
	p, err := w.bytes()
	if err != nil {
		return dst, err
	}
	sub := wire{p}
	for len(sub.b) > 0 {
		v, err := sub.varint()
		if err != nil {
			return dst, err
		}
		dst = append(dst, int64(v))
	}
	return dst, nil
}

// floats reads a repeated float field, packed or not, appending to dst.
func (w *wire) floats(typ int, dst []float32) ([]float32, error) {
	if typ == 5 {
		v, err := w.fixed32()
		return append(dst, math.Float32frombits(v)), err
	}
	p, err := w.bytes()
	if err != nil {
		return dst, err
	}
	if len(p)%4 != 0 {
		return dst, errTruncated
	}
	for i := 0; i < len(p); i += 4 {
		dst = append(dst, math.Float32frombits(binary.LittleEndian.Uint32(p[i:])))
	}
	return dst, nil
}

// modelProto is what the runner reads of a ModelProto.
type modelProto struct {
	opsets map[string]int64
	graph  graphProto
}

// graphProto is what the runner reads of a GraphProto.
type graphProto struct {
	nodes        []nodeProto
	initializers []*Tensor
	inputs       []valueInfo
	outputs      []valueInfo
}

// nodeProto is one operator of the graph.
type nodeProto struct {
	name, opType, domain string
	inputs, outputs      []string
	attrs                map[string]attribute
}

// attribute is one of a node's attributes; only the fields of its type are set.
type attribute struct {
	typ     int
	f       float32
	i       int64
	s       []byte
	t       *Tensor
	floats  []float32
	ints    []int64
	strings [][]byte
}

// valueInfo is a graph input's or output's name, element type and shape (-1 for a dimension with no value).
type valueInfo struct {
	name     string
	elemType int
	shape    []int
}

// decodeModel decodes a ModelProto.
func decodeModel(b []byte) (*modelProto, error) {
	m := &modelProto{opsets: map[string]int64{}}
	w := wire{b}
	for len(w.b) > 0 {
		num, typ, err := w.field()
		if err != nil {
			return nil, err
		}
		switch {
		case num == 7 && typ == 2:
			p, err := w.bytes()
			if err != nil {
				return nil, err
			}
			if err := decodeGraph(p, &m.graph); err != nil {
				return nil, err
			}
		case num == 8 && typ == 2:
			p, err := w.bytes()
			if err != nil {
				return nil, err
			}
			domain, version, err := decodeOpset(p)
			if err != nil {
				return nil, err
			}
			m.opsets[domain] = version
		default:
			if err := w.skip(typ); err != nil {
				return nil, err
			}
		}
	}
	return m, nil
}

// decodeOpset decodes an OperatorSetIdProto.
func decodeOpset(b []byte) (domain string, version int64, err error) {
	w := wire{b}
	for len(w.b) > 0 {
		num, typ, err := w.field()
		if err != nil {
			return "", 0, err
		}
		switch {
		case num == 1 && typ == 2:
			p, err := w.bytes()
			if err != nil {
				return "", 0, err
			}
			domain = string(p)
		case num == 2 && typ == 0:
			v, err := w.varint()
			if err != nil {
				return "", 0, err
			}
			version = int64(v)
		default:
			if err := w.skip(typ); err != nil {
				return "", 0, err
			}
		}
	}
	return domain, version, nil
}

// decodeGraph decodes a GraphProto into g.
func decodeGraph(b []byte, g *graphProto) error {
	w := wire{b}
	for len(w.b) > 0 {
		num, typ, err := w.field()
		if err != nil {
			return err
		}
		if typ != 2 {
			if err := w.skip(typ); err != nil {
				return err
			}
			continue
		}
		p, err := w.bytes()
		if err != nil {
			return err
		}
		switch num {
		case 1:
			n, err := decodeNode(p)
			if err != nil {
				return err
			}
			g.nodes = append(g.nodes, n)
		case 5:
			t, err := decodeTensor(p)
			if err != nil {
				return err
			}
			g.initializers = append(g.initializers, t)
		case 11, 12:
			vi, err := decodeValueInfo(p)
			if err != nil {
				return err
			}
			if num == 11 {
				g.inputs = append(g.inputs, vi)
			} else {
				g.outputs = append(g.outputs, vi)
			}
		case 15:
			return errors.New("onnx: sparse initializers are not supported")
		}
	}
	return nil
}

// decodeNode decodes a NodeProto.
func decodeNode(b []byte) (nodeProto, error) {
	n := nodeProto{attrs: map[string]attribute{}}
	w := wire{b}
	for len(w.b) > 0 {
		num, typ, err := w.field()
		if err != nil {
			return n, err
		}
		if typ != 2 {
			if err := w.skip(typ); err != nil {
				return n, err
			}
			continue
		}
		p, err := w.bytes()
		if err != nil {
			return n, err
		}
		switch num {
		case 1:
			n.inputs = append(n.inputs, string(p))
		case 2:
			n.outputs = append(n.outputs, string(p))
		case 3:
			n.name = string(p)
		case 4:
			n.opType = string(p)
		case 7:
			n.domain = string(p)
		case 5:
			name, a, err := decodeAttribute(p)
			if err != nil {
				return n, err
			}
			n.attrs[name] = a
		}
	}
	return n, nil
}

// decodeAttribute decodes an AttributeProto.
func decodeAttribute(b []byte) (string, attribute, error) {
	var name string
	var a attribute
	w := wire{b}
	for len(w.b) > 0 {
		num, typ, err := w.field()
		if err != nil {
			return "", a, err
		}
		switch {
		case num == 1 && typ == 2:
			p, err := w.bytes()
			if err != nil {
				return "", a, err
			}
			name = string(p)
		case num == 20 && typ == 0:
			v, err := w.varint()
			if err != nil {
				return "", a, err
			}
			a.typ = int(v)
		case num == 2 && typ == 5:
			v, err := w.fixed32()
			if err != nil {
				return "", a, err
			}
			a.f = math.Float32frombits(v)
		case num == 3 && typ == 0:
			v, err := w.varint()
			if err != nil {
				return "", a, err
			}
			a.i = int64(v)
		case num == 4 && typ == 2:
			p, err := w.bytes()
			if err != nil {
				return "", a, err
			}
			a.s = p
		case num == 5 && typ == 2:
			p, err := w.bytes()
			if err != nil {
				return "", a, err
			}
			if a.t, err = decodeTensor(p); err != nil {
				return "", a, err
			}
		case num == 7:
			if a.floats, err = w.floats(typ, a.floats); err != nil {
				return "", a, err
			}
		case num == 8:
			if a.ints, err = w.ints(typ, a.ints); err != nil {
				return "", a, err
			}
		case num == 9 && typ == 2:
			p, err := w.bytes()
			if err != nil {
				return "", a, err
			}
			a.strings = append(a.strings, p)
		default:
			if err := w.skip(typ); err != nil {
				return "", a, err
			}
		}
	}
	return name, a, nil
}

// decodeValueInfo decodes a ValueInfoProto with a tensor type.
func decodeValueInfo(b []byte) (valueInfo, error) {
	var vi valueInfo
	w := wire{b}
	for len(w.b) > 0 {
		num, typ, err := w.field()
		if err != nil {
			return vi, err
		}
		switch {
		case num == 1 && typ == 2:
			p, err := w.bytes()
			if err != nil {
				return vi, err
			}
			vi.name = string(p)
		case num == 2 && typ == 2:
			p, err := w.bytes()
			if err != nil {
				return vi, err
			}
			if err := decodeTypeProto(p, &vi); err != nil {
				return vi, err
			}
		default:
			if err := w.skip(typ); err != nil {
				return vi, err
			}
		}
	}
	return vi, nil
}

// decodeTypeProto reads a TypeProto's tensor_type into vi; other types are left empty.
func decodeTypeProto(b []byte, vi *valueInfo) error {
	w := wire{b}
	for len(w.b) > 0 {
		num, typ, err := w.field()
		if err != nil {
			return err
		}
		if num != 1 || typ != 2 {
			if err := w.skip(typ); err != nil {
				return err
			}
			continue
		}
		p, err := w.bytes()
		if err != nil {
			return err
		}
		tw := wire{p}
		for len(tw.b) > 0 {
			num, typ, err := tw.field()
			if err != nil {
				return err
			}
			switch {
			case num == 1 && typ == 0:
				v, err := tw.varint()
				if err != nil {
					return err
				}
				vi.elemType = int(v)
			case num == 2 && typ == 2:
				sp, err := tw.bytes()
				if err != nil {
					return err
				}
				if vi.shape, err = decodeShape(sp); err != nil {
					return err
				}
			default:
				if err := tw.skip(typ); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// decodeShape decodes a TensorShapeProto: a dimension with a value is that value, one with a name is -1.
func decodeShape(b []byte) ([]int, error) {
	shape := []int{}
	w := wire{b}
	for len(w.b) > 0 {
		num, typ, err := w.field()
		if err != nil {
			return nil, err
		}
		if num != 1 || typ != 2 {
			if err := w.skip(typ); err != nil {
				return nil, err
			}
			continue
		}
		p, err := w.bytes()
		if err != nil {
			return nil, err
		}
		dim := -1
		dw := wire{p}
		for len(dw.b) > 0 {
			num, typ, err := dw.field()
			if err != nil {
				return nil, err
			}
			if num == 1 && typ == 0 {
				v, err := dw.varint()
				if err != nil {
					return nil, err
				}
				dim = int(int64(v))
			} else if err := dw.skip(typ); err != nil {
				return nil, err
			}
		}
		shape = append(shape, dim)
	}
	return shape, nil
}

// decodeTensor decodes a TensorProto held in the file (raw_data or the typed fields); external data is not
// supported.
func decodeTensor(b []byte) (*Tensor, error) {
	var dims []int64
	var dataType int
	var name string
	var raw []byte
	var floatData []float32
	var intData []int64 // int32_data and int64_data alike.
	var doubleData []float64
	w := wire{b}
	for len(w.b) > 0 {
		num, typ, err := w.field()
		if err != nil {
			return nil, err
		}
		switch {
		case num == 1:
			if dims, err = w.ints(typ, dims); err != nil {
				return nil, err
			}
		case num == 2 && typ == 0:
			v, err := w.varint()
			if err != nil {
				return nil, err
			}
			dataType = int(v)
		case num == 4:
			if floatData, err = w.floats(typ, floatData); err != nil {
				return nil, err
			}
		case num == 5 || num == 7:
			if intData, err = w.ints(typ, intData); err != nil {
				return nil, err
			}
		case num == 8 && typ == 2:
			p, err := w.bytes()
			if err != nil {
				return nil, err
			}
			name = string(p)
		case num == 9 && typ == 2:
			if raw, err = w.bytes(); err != nil {
				return nil, err
			}
		case num == 10:
			if typ == 1 {
				if len(w.b) < 8 {
					return nil, errTruncated
				}
				doubleData = append(doubleData, math.Float64frombits(binary.LittleEndian.Uint64(w.b)))
				w.b = w.b[8:]
				continue
			}
			p, err := w.bytes()
			if err != nil {
				return nil, err
			}
			for i := 0; i+8 <= len(p); i += 8 {
				doubleData = append(doubleData, math.Float64frombits(binary.LittleEndian.Uint64(p[i:])))
			}
		case num == 13 || num == 14:
			if num == 14 {
				v, err := w.varint()
				if err != nil {
					return nil, err
				}
				if v != 0 {
					return nil, fmt.Errorf("onnx: tensor %q keeps its data outside the file", name)
				}
				continue
			}
			return nil, fmt.Errorf("onnx: tensor %q keeps its data outside the file", name)
		default:
			if err := w.skip(typ); err != nil {
				return nil, err
			}
		}
	}
	shape := make([]int, len(dims))
	size := 1
	const maxTensorElems = 1 << 28
	for i, d := range dims {
		if d < 0 || d > maxTensorElems {
			return nil, fmt.Errorf("onnx: tensor %q has invalid dim %d", name, d)
		}
		shape[i] = int(d)
		if d > 0 && size > maxTensorElems/int(d) {
			return nil, fmt.Errorf("onnx: tensor %q shape %v overflows", name, dims)
		}
		size *= int(d)
	}
	t := &Tensor{Shape: shape}
	switch dataType {
	case typeFloat:
		t.Type = Float32
		switch {
		case raw != nil:
			if len(raw) != 4*size {
				return nil, fmt.Errorf("onnx: tensor %q has %d bytes for %d floats", name, len(raw), size)
			}
			t.F = make([]float32, size)
			for i := range t.F {
				t.F[i] = math.Float32frombits(binary.LittleEndian.Uint32(raw[4*i:]))
			}
		default:
			t.F = floatData
		}
	case typeDouble:
		t.Type = Float32
		t.F = make([]float32, len(doubleData))
		for i, d := range doubleData {
			t.F[i] = float32(d)
		}
		if raw != nil {
			if len(raw) != 8*size {
				return nil, fmt.Errorf("onnx: tensor %q has %d bytes for %d doubles", name, len(raw), size)
			}
			t.F = make([]float32, size)
			for i := range t.F {
				t.F[i] = float32(math.Float64frombits(binary.LittleEndian.Uint64(raw[8*i:])))
			}
		}
	case typeInt8, typeUint8, typeBool:
		t.Type = map[int]DType{typeInt8: Int8, typeUint8: Uint8, typeBool: Bool}[dataType]
		t.B = make([]byte, size)
		if raw != nil {
			if len(raw) != size {
				return nil, fmt.Errorf("onnx: tensor %q has %d bytes for %d values", name, len(raw), size)
			}
			copy(t.B, raw)
		} else {
			for i := range min(size, len(intData)) {
				t.B[i] = byte(intData[i])
			}
		}
	case typeInt32, typeInt64:
		t.Type = Int64
		if dataType == typeInt32 {
			t.Type = Int32
		}
		t.I = make([]int64, size)
		switch {
		case raw != nil && dataType == typeInt64:
			if len(raw) != 8*size {
				return nil, fmt.Errorf("onnx: tensor %q has %d bytes for %d int64s", name, len(raw), size)
			}
			for i := range t.I {
				t.I[i] = int64(binary.LittleEndian.Uint64(raw[8*i:]))
			}
		case raw != nil:
			if len(raw) != 4*size {
				return nil, fmt.Errorf("onnx: tensor %q has %d bytes for %d int32s", name, len(raw), size)
			}
			for i := range t.I {
				t.I[i] = int64(int32(binary.LittleEndian.Uint32(raw[4*i:])))
			}
		default:
			for i := range min(size, len(intData)) {
				if dataType == typeInt32 {
					t.I[i] = int64(int32(intData[i]))
				} else {
					t.I[i] = intData[i]
				}
			}
		}
	default:
		return nil, fmt.Errorf("onnx: tensor %q has element type %d, which is not supported", name, dataType)
	}
	if t.len() != size {
		return nil, fmt.Errorf("onnx: tensor %q has %d values for shape %v", name, t.len(), shape)
	}
	t.Name = name
	return t, nil
}
