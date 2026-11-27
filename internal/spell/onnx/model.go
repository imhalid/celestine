package onnx

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

// Model is an ONNX graph ready to run: its nodes in the file's order (ONNX keeps them in an order each can run in),
// its weights, and the names of its inputs and outputs. Run does not change its graph: a Model may run in several
// goroutines at once.
type Model struct {
	nodes   []nodeProto
	run     []op
	weights map[string]*Tensor
	last    map[string]int
	outputs map[string]bool
	intW    sync.Map // [2]*Tensor -> *intWeights, freed with the Model
	Inputs  []string
	Outputs []string
}

// Load reads an ONNX model: an operator of a domain other than ONNX's own, or one this package does not run, is an
// error now, not as the model runs.
func Load(b []byte) (*Model, error) {
	mp, err := decodeModel(b)
	if err != nil {
		return nil, err
	}
	if v, ok := mp.opsets[""]; ok && (v < 13 || v > 17) {
		return nil, fmt.Errorf("onnx: opset %d; the operators here are those of opsets 13 to 17", v)
	}
	m := &Model{
		nodes:   mp.graph.nodes,
		weights: map[string]*Tensor{},
		last:    map[string]int{},
		outputs: map[string]bool{},
	}
	for _, t := range mp.graph.initializers {
		m.weights[t.Name] = t
	}
	unknown := map[string]bool{}
	for i, n := range m.nodes {
		for _, name := range n.inputs {
			m.last[name] = i
		}
		f, ok := ops[n.opType]
		if !ok || n.domain != "" && n.domain != "ai.onnx" {
			unknown[n.domain+"::"+n.opType] = true
		}
		if n.opType == "MatMulInteger" && ok && (n.domain == "" || n.domain == "ai.onnx") {
			constB := len(n.inputs) > 1 && m.weights[n.inputs[1]] != nil
			constZP := len(n.inputs) <= 3 || n.inputs[3] == "" || m.weights[n.inputs[3]] != nil
			if constB && constZP {
				f = func(node *nodeProto, in []*Tensor) ([]*Tensor, error) {
					return opMatMulIntegerCached(node, in, &m.intW)
				}
			}
		}
		m.run = append(m.run, f)
	}
	if len(unknown) > 0 {
		var names []string
		for k := range unknown {
			names = append(names, strings.TrimPrefix(k, "::"))
		}
		sort.Strings(names)
		return nil, fmt.Errorf("onnx: operators not run here: %s", strings.Join(names, ", "))
	}
	for _, vi := range mp.graph.inputs {
		if _, ok := m.weights[vi.name]; !ok { // a weight may be listed among the inputs too
			m.Inputs = append(m.Inputs, vi.name)
		}
	}
	for _, vi := range mp.graph.outputs {
		m.Outputs = append(m.Outputs, vi.name)
		m.outputs[vi.name] = true
	}
	return m, nil
}

// Run runs the graph on feeds, its inputs by name, and returns its outputs by name. A value is dropped once the last
// node that reads it has run, so a run holds little more than its widest step.
func (m *Model) Run(feeds map[string]*Tensor) (map[string]*Tensor, error) {
	for _, name := range m.Inputs {
		if feeds[name] == nil {
			return nil, fmt.Errorf("onnx: no value for the input %q", name)
		}
	}
	values := make(map[string]*Tensor, len(m.nodes))
	for k, v := range feeds {
		values[k] = v
	}
	for i := range m.nodes {
		n := &m.nodes[i]
		in := make([]*Tensor, len(n.inputs))
		for j, name := range n.inputs {
			if name == "" {
				continue // an optional input left out
			}
			v := values[name]
			if v == nil {
				v = m.weights[name]
			}
			if v == nil {
				return nil, fmt.Errorf("onnx: %s (%s) reads %q, which nothing made", n.name, n.opType, name)
			}
			in[j] = v
		}
		out, err := m.run[i](n, in)
		if err != nil {
			return nil, fmt.Errorf("%s (%s): %w", n.name, n.opType, err)
		}
		for j, name := range n.outputs {
			if j < len(out) && name != "" {
				values[name] = out[j]
			}
		}
		for _, name := range n.inputs { // what no later node reads
			if m.last[name] == i && !m.outputs[name] {
				delete(values, name)
			}
		}
	}
	res := map[string]*Tensor{}
	for _, name := range m.Outputs {
		if values[name] == nil {
			return nil, fmt.Errorf("onnx: the output %q was not made", name)
		}
		res[name] = values[name]
	}
	return res, nil
}
