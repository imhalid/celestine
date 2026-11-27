package spell

import (
	_ "embed"
	"fmt"
	"strings"
	"sync"
	"unicode"

	"github.com/imhalid/celestine/internal/spell/onnx"
	"github.com/imhalid/celestine/internal/spell/tr"
)

// This file is the rest of tinyfix-tr's src/context.ts and pkg/src/context.ts: the context model's subword tokenizer
// (bit-identical to its py/build_tokenizer.py), the packing of each candidate between the text on its left and right,
// and a ContextModel that runs context.onnx (a v3 subword Transformer) with the pure Go runner of package onnx, as
// the npm package runs it with onnxruntime. Its scores are onnxruntime's within float32 rounding: the integer part of
// the quantized network is exact, the float operations differ only in their last bits (TestContextModelOutputs).

//go:embed contextvocab.txt
var contextVocabText string

// The context model's subword sequence: the [CLS] token, then at most leftSub tokens of the text on the left (its
// last), spanSub of the candidate and rightSub of the text on the right (its first), maxSubLen in all.
const (
	leftSub   = 24
	spanSub   = 8
	rightSub  = 20
	maxSubLen = 64
)

// The vocabulary's special tokens.
const (
	subPadID    = 0
	subUnkID    = 1
	subClsID    = 2
	subCapID    = 4
	subAllCapID = 5
	subNumID    = 6
)

// wordPrefix marks a subword that starts a word.
const wordPrefix = "▁"

// subVocab is the vocabulary by token, and the length of its longest token but the special ones (<pad>, <cls>…),
// read once.
var subVocab = sync.OnceValues(func() (map[string]int, int) {
	idOf := map[string]int{}
	maxLen := 1
	for i, tok := range strings.Split(contextVocabText, "\n") {
		idOf[tok] = i
		if n := len([]rune(tok)); !(strings.HasPrefix(tok, "<") && strings.HasSuffix(tok, ">")) && n > maxLen {
			maxLen = n
		}
	}
	return idOf, maxLen
})

// encodeSubwordPiece appends the subwords of the lowercase word low to out, greedily the longest the vocabulary has
// at each point, the first with the word prefix when wordStart is set; a letter no token starts with is <unk>.
func encodeSubwordPiece(low []rune, wordStart bool, out []int64) []int64 {
	idOf, maxLen := subVocab()
	n := len(low)
	pos := 0
	for pos < n {
		found := -1
		if pos == 0 && wordStart {
			for end := min(n, maxLen-1); end > 0; end-- {
				if id, ok := idOf[wordPrefix+string(low[:end])]; ok {
					found, pos = id, end
					break
				}
			}
		} else {
			for end := min(n, pos+maxLen); end > pos; end-- {
				if id, ok := idOf[string(low[pos:end])]; ok {
					found, pos = id, end
					break
				}
			}
		}
		if found == -1 {
			out = append(out, subUnkID)
			pos++
		} else {
			out = append(out, int64(found))
		}
	}
	return out
}

// jsSpace reports whether r is JavaScript's \s.
func jsSpace(r rune) bool {
	switch r {
	case '\t', '\n', '\v', '\f', '\r', ' ', 0xa0, 0x1680, 0x2028, 0x2029, 0x202f, 0x205f, 0x3000, 0xfeff:
		return true
	}
	return r >= 0x2000 && r <= 0x200a
}

// encodeSubwords is the model's subwords of text: a word's letters as its subwords, after <cap> or <allcap> for a
// capital first letter or capitals throughout (not after an apostrophe, where a suffix goes on), a number as <num>,
// and any other character as its token, as the TS's /[letters]+|[0-9]+|[^\s]/g matches them. That pattern has no u
// flag: a character outside the BMP is two UTF-16 units there, each a token of its own, each <unk>.
func encodeSubwords(text string) []int64 {
	idOf, _ := subVocab()
	norm := []rune(strings.ReplaceAll(text, "’", "'"))
	var out []int64
	for i := 0; i < len(norm); {
		r := norm[i]
		switch {
		case tr.IsLetter(r):
			j := i
			for j < len(norm) && tr.IsLetter(norm[j]) {
				j++
			}
			tok := string(norm[i:j])
			afterApos := i > 0 && norm[i-1] == '\''
			if !afterApos {
				if tr.IsAllUpper(tok) {
					out = append(out, subAllCapID)
				} else if tr.IsUpperInitial(tok) {
					out = append(out, subCapID)
				}
			}
			out = encodeSubwordPiece([]rune(tr.Lower(tok)), !afterApos, out)
			i = j
		case r >= '0' && r <= '9':
			for i < len(norm) && norm[i] >= '0' && norm[i] <= '9' {
				i++
			}
			out = append(out, subNumID)
		case jsSpace(r):
			i++
		case r > 0xffff:
			out = append(out, subUnkID, subUnkID)
			i++
		case unicode.IsLetter(r): // a letter not of Turkish words: a word of its own, one character long
			afterApos := i > 0 && norm[i-1] == '\''
			tok := string(r)
			if !afterApos {
				if tr.IsAllUpper(tok) {
					out = append(out, subAllCapID)
				} else if tr.IsUpperInitial(tok) {
					out = append(out, subCapID)
				}
			}
			out = encodeSubwordPiece([]rune(tr.Lower(tok)), !afterApos, out)
			i++
		default:
			if id, ok := idOf[string(r)]; ok {
				out = append(out, int64(id))
			} else {
				out = append(out, subUnkID)
			}
			i++
		}
	}
	return out
}

// packedSubwords are the model's inputs for a batch: n rows of seqLen subwords, their segments (1 the left, 2 the
// candidate, 3 the right, 0 padding), and FeatureDim features each.
type packedSubwords struct {
	subwords, segments []int64
	features           []float32
	seqLen, n          int
}

// packSubwords packs each candidate of each query into a row: [CLS], the left (at most its last leftSub
// subwords), the candidate (its first spanSub), the right (its first rightSub), padded to the longest row (at
// least 4, at most maxSubLen), as the TS's packSubwords.
func packSubwords(queries []ContextQuery) packedSubwords {
	type row struct {
		left, span, right []int64
		feat              []float64
	}
	var rows []row
	maxLen := 4
	for _, q := range queries {
		left := encodeSubwords(q.Left)
		if len(left) > leftSub {
			left = left[len(left)-leftSub:]
		}
		right := encodeSubwords(q.Right)
		if len(right) > rightSub {
			right = right[:rightSub]
		}
		for c, cand := range q.Candidates {
			span := encodeSubwords(cand)
			if len(span) > spanSub {
				span = span[:spanSub]
			}
			maxLen = max(maxLen, 1+len(left)+len(span)+len(right))
			var feat []float64
			if c < len(q.Features) {
				feat = q.Features[c]
			}
			rows = append(rows, row{left, span, right, feat})
		}
	}
	p := packedSubwords{n: len(rows), seqLen: min(maxLen, maxSubLen)}
	p.subwords = make([]int64, p.n*p.seqLen)
	p.segments = make([]int64, p.n*p.seqLen)
	p.features = make([]float32, p.n*FeatureDim)
	for i, r := range rows {
		pos, limit := i*p.seqLen, (i+1)*p.seqLen
		p.subwords[pos], p.segments[pos] = subClsID, 1
		pos++
		for _, part := range []struct {
			ids []int64
			seg int64
		}{{r.left, 1}, {r.span, 2}, {r.right, 3}} {
			for _, id := range part.ids {
				if pos >= limit {
					break
				}
				p.subwords[pos], p.segments[pos] = id, part.seg
				pos++
			}
		}
		for f := 0; f < FeatureDim && f < len(r.feat); f++ {
			p.features[i*FeatureDim+f] = float32(r.feat[f])
		}
	}
	return p
}

// onnxContext is a ContextModel that runs a v3 context.onnx: the queries' candidates packed into one batch, the
// network run once, its scores read back by query.
type onnxContext struct {
	model *onnx.Model
}

// LoadContextModel reads context.onnx (its bytes) as the engine's context model. Only a v3 subword model (inputs
// subwords, sub_segments, features; output scores) is read: the earlier character models are not the npm package's.
func LoadContextModel(b []byte) (ContextModel, error) {
	m, err := onnx.Load(b)
	if err != nil {
		return nil, err
	}
	want := map[string]bool{"subwords": true, "sub_segments": true, "features": true}
	for _, in := range m.Inputs {
		delete(want, in)
	}
	if len(want) > 0 || len(m.Outputs) == 0 {
		return nil, fmt.Errorf("context model: not a v3 subword model (inputs %v)", m.Inputs)
	}
	return &onnxContext{m}, nil
}

// SupportsNameContext reports that a v3 model judges ambiguous proper names too.
func (c *onnxContext) SupportsNameContext() bool { return true }

// Score runs the network on every candidate of every query at once.
func (c *onnxContext) Score(queries []ContextQuery) ([][]float64, error) {
	if len(queries) == 0 {
		return nil, nil
	}
	p := packSubwords(queries)
	if p.n == 0 {
		out := make([][]float64, len(queries))
		for q := range queries {
			out[q] = []float64{}
		}
		return out, nil
	}
	flat, err := c.run(p)
	if err != nil {
		return nil, err
	}
	out := make([][]float64, len(queries))
	i := 0
	for q, query := range queries {
		out[q] = make([]float64, len(query.Candidates))
		for k := range query.Candidates {
			out[q][k] = float64(flat[i])
			i++
		}
	}
	return out, nil
}

// run runs the network on a packed batch and returns its n scores.
func (c *onnxContext) run(p packedSubwords) ([]float32, error) {
	res, err := c.model.Run(map[string]*onnx.Tensor{
		"subwords":     {Type: onnx.Int64, Shape: []int{p.n, p.seqLen}, I: p.subwords},
		"sub_segments": {Type: onnx.Int64, Shape: []int{p.n, p.seqLen}, I: p.segments},
		"features":     {Type: onnx.Float32, Shape: []int{p.n, FeatureDim}, F: p.features},
	})
	if err != nil {
		return nil, err
	}
	scores := res["scores"]
	if scores == nil {
		scores = res[c.model.Outputs[0]]
	}
	if scores == nil || scores.Type != onnx.Float32 || len(scores.F) != p.n {
		return nil, fmt.Errorf("context model: no float32 scores, one a row")
	}
	return scores.F, nil
}
