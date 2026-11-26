package spell

import (
	"math"
	"regexp"
	"strconv"
	"strings"
)

// The helpers in this file give the port JavaScript's own arithmetic, number parsing and string measures where the
// TypeScript engine (tinyfix-tr) leans on them, so that the Go answers are the engine's (#623).

// jsRound is JavaScript's Math.round: the nearest integer, with halves rounded up (-2.5 is -2, 2.5 is 3).
func jsRound(x float64) float64 {
	if math.IsNaN(x) || math.IsInf(x, 0) {
		return x
	}
	f := math.Floor(x)
	if x-f >= 0.5 {
		return f + 1
	}
	return f
}

// toUint32 is JavaScript's ToUint32, what storing a number into a Uint32Array or `>>> 0` does: NaN and the
// infinities are 0, anything else is truncated and taken modulo 2^32.
func toUint32(x float64) uint32 {
	if math.IsNaN(x) || math.IsInf(x, 0) {
		return 0
	}
	t := math.Trunc(x)
	m := math.Mod(t, 4294967296)
	if m < 0 {
		m += 4294967296
	}
	return uint32(m)
}

// toUint8 is what storing a number into a Uint8Array does.
func toUint8(x float64) uint8 { return uint8(toUint32(x)) }

// toUint16 is what DataView.setUint16 does to its value.
func toUint16(x float64) uint16 { return uint16(toUint32(x)) }

// jsWhitespace reports whether r is white space to JavaScript's String.prototype.trim and Number(): the WhiteSpace
// and LineTerminator code points of ECMAScript, which are not quite Go's unicode.IsSpace (U+0085 is not one, U+FEFF
// is).
func jsWhitespace(r rune) bool {
	switch r {
	case '\t', '\n', '\v', '\f', '\r', ' ', 0xA0, 0x1680, 0x2028, 0x2029, 0x202F, 0x205F, 0x3000, 0xFEFF:
		return true
	}
	return r >= 0x2000 && r <= 0x200A
}

// jsTrim is JavaScript's String.prototype.trim.
func jsTrim(s string) string { return strings.TrimFunc(s, jsWhitespace) }

// jsNumber is JavaScript's Number(text) for a string: white space around it is ignored, an empty string is 0,
// "0x", "0o" and "0b" prefixes are integers in those bases, "Infinity" is infinite, and anything else that is not
// a decimal literal is NaN.
func jsNumber(text string) float64 {
	s := jsTrim(text)
	if s == "" {
		return 0
	}
	if len(s) > 2 && s[0] == '0' {
		base := 0
		switch s[1] {
		case 'x', 'X':
			base = 16
		case 'o', 'O':
			base = 8
		case 'b', 'B':
			base = 2
		}
		if base != 0 {
			v := 0.0
			for _, c := range s[2:] {
				d := -1
				switch {
				case c >= '0' && c <= '9':
					d = int(c - '0')
				case c >= 'a' && c <= 'f':
					d = int(c-'a') + 10
				case c >= 'A' && c <= 'F':
					d = int(c-'A') + 10
				}
				if d < 0 || d >= base {
					return math.NaN()
				}
				v = v*float64(base) + float64(d)
			}
			return v
		}
	}
	sign, body := "", s
	if body[0] == '+' || body[0] == '-' {
		sign, body = body[:1], body[1:]
	}
	if body == "Infinity" {
		if sign == "-" {
			return math.Inf(-1)
		}
		return math.Inf(1)
	}
	if !isDecimalLiteral(body) {
		return math.NaN()
	}
	v, err := strconv.ParseFloat(sign+body, 64)
	if err != nil && !strings.Contains(err.Error(), "range") {
		return math.NaN()
	}
	return v
}

// isDecimalLiteral reports whether s is an unsigned StrDecimalLiteral of ECMAScript: digits with an optional
// fraction ("1.", ".5" and "1.5" are all one) and an optional exponent, no underscores.
func isDecimalLiteral(s string) bool {
	i, digits := 0, 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
		digits++
	}
	if i < len(s) && s[i] == '.' {
		i++
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			i++
			digits++
		}
	}
	if digits == 0 {
		return false
	}
	if i < len(s) && (s[i] == 'e' || s[i] == 'E') {
		i++
		if i < len(s) && (s[i] == '+' || s[i] == '-') {
			i++
		}
		exp := 0
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			i++
			exp++
		}
		if exp == 0 {
			return false
		}
	}
	return i == len(s)
}

// jsNumberString is how JavaScript writes a number into a string (`${n}`): integers without a fraction, NaN and
// Infinity by name, and exponents for the very large and the very small.
func jsNumberString(x float64) string {
	switch {
	case math.IsNaN(x):
		return "NaN"
	case math.IsInf(x, 1):
		return "Infinity"
	case math.IsInf(x, -1):
		return "-Infinity"
	case x == 0:
		return "0"
	}
	if a := math.Abs(x); a >= 1e-6 && a < 1e21 {
		return strconv.FormatFloat(x, 'f', -1, 64)
	}
	s := strconv.FormatFloat(x, 'e', -1, 64)
	mant, exp, _ := strings.Cut(s, "e")
	sign := exp[:1]
	exp = strings.TrimLeft(exp[1:], "0")
	if exp == "" {
		exp = "0"
	}
	return mant + "e" + sign + exp
}

// runeLen is a string's length in runes, which for Turkish text (all of it in the Basic Multilingual Plane) is its
// JavaScript length.
func runeLen(s string) int { return len([]rune(s)) }

// sliceRunes is JavaScript's String.prototype.slice(start, end) over runes: negative bounds count from the end, and
// bounds outside the string are clamped.
func sliceRunes(s string, start, end int) string {
	rs := []rune(s)
	n := len(rs)
	clamp := func(i int) int {
		if i < 0 {
			i += n
			if i < 0 {
				i = 0
			}
		}
		if i > n {
			i = n
		}
		return i
	}
	start, end = clamp(start), clamp(end)
	if start >= end {
		return ""
	}
	return string(rs[start:end])
}

// sliceFrom is JavaScript's String.prototype.slice(start) over runes.
func sliceFrom(s string, start int) string { return sliceRunes(s, start, math.MaxInt32) }

// lastWord is `text.split(" ").at(-1)`: what follows the last space, or the whole text.
func lastWord(text string) string {
	if i := strings.LastIndexByte(text, ' '); i >= 0 {
		return text[i+1:]
	}
	return text
}

// setOf is a set of words for membership tests, the TypeScript's Set where its order never matters. A word
// listed twice is one member, as in a Set.
func setOf(words ...string) map[string]bool {
	set := make(map[string]bool, len(words))
	for _, w := range words {
		set[w] = true
	}
	return set
}

// WordSet is a set of words that keeps the order they were listed in, as a JavaScript Set iterates, for the sets
// whose first match wins.
type WordSet struct {
	words []string
	index map[string]bool
}

// NewWordSet is a WordSet of words, in their order; a word listed again keeps its first place.
func NewWordSet(words ...string) WordSet {
	s := WordSet{index: make(map[string]bool, len(words))}
	for _, w := range words {
		if !s.index[w] {
			s.index[w] = true
			s.words = append(s.words, w)
		}
	}
	return s
}

// Has reports whether word is in the set.
func (s WordSet) Has(word string) bool { return s.index[word] }

// Words are the set's words in order. The slice is the set's own; it must not be changed.
func (s WordSet) Words() []string { return s.words }

// Len is the number of words.
func (s WordSet) Len() int { return len(s.words) }

// jsSpaceClass is JavaScript's \s as the body of an RE2 character class: Go's \s is ASCII only, JavaScript's is the
// WhiteSpace and LineTerminator code points of ECMAScript.
const jsSpaceClass = `\t\n\v\f\r \x{a0}\x{1680}\x{2000}-\x{200a}\x{2028}\x{2029}\x{202f}\x{205f}\x{3000}\x{feff}`

// runeOffsets maps each byte offset of s that starts a rune (and len(s)) to its rune offset, for turning the byte
// offsets of Go's regular expressions into JavaScript's string indices.
func runeOffsets(s string) []int {
	offs := make([]int, len(s)+1)
	n := 0
	for i := range s {
		offs[i] = n
		n++
	}
	offs[len(s)] = n
	return offs
}

// findAllRunes is `text.matchAll(re)` with its indices in runes: each match's submatch bounds, -1 for a group that
// did not take part.
func findAllRunes(re *regexp.Regexp, text string) [][]int {
	matches := re.FindAllStringSubmatchIndex(text, -1)
	if len(matches) == 0 {
		return nil
	}
	offs := runeOffsets(text)
	for _, m := range matches {
		for i, b := range m {
			if b >= 0 {
				m[i] = offs[b]
			}
		}
	}
	return matches
}

const (
	logLn2Hi = 6.93147180369123816490e-01
	logLn2Lo = 1.90821492927058770002e-10
	logTwo54 = 1.80143985094819840000e+16
	logLg1   = 6.666666666666735130e-01
	logLg2   = 3.999999999940941908e-01
	logLg3   = 2.857142874366239149e-01
	logLg4   = 2.222219843214978396e-01
	logLg5   = 1.818357216161805012e-01
	logLg6   = 1.531383769920937332e-01
	logLg7   = 1.479819860511658591e-01
)

// jsLog is Math.log as V8 computes it, with fdlibm's __ieee754_log, so the corrector's scores are the TypeScript's
// to the last bit (Go's math.Log is another algorithm on amd64). Every product is converted explicitly so the
// compiler cannot fuse it with an addition.
func jsLog(x float64) float64 {
	bits := math.Float64bits(x)
	hx := int32(bits >> 32)
	lx := uint32(bits)
	k := int32(0)
	if hx < 0x00100000 { // x < 2**-1022
		if (hx&0x7fffffff)|int32(lx) == 0 {
			return math.Inf(-1)
		}
		if hx < 0 {
			return math.NaN()
		}
		k -= 54
		x = float64(x * logTwo54) // a subnormal number, scaled up
		hx = int32(math.Float64bits(x) >> 32)
	}
	if hx >= 0x7ff00000 {
		return x + x
	}
	k += (hx >> 20) - 1023
	hx &= 0x000fffff
	i := (hx + 0x95f64) & 0x100000
	x = math.Float64frombits(uint64(uint32(hx|(i^0x3ff00000)))<<32 | uint64(uint32(math.Float64bits(x)))) // normalize x or x/2
	k += i >> 20
	f := x - 1.0
	if (0x000fffff & (2 + hx)) < 3 { // -2**-20 <= f < 2**-20
		if f == 0 {
			if k == 0 {
				return 0
			}
			dk := float64(k)
			return float64(dk*logLn2Hi) + float64(dk*logLn2Lo)
		}
		R := float64(float64(f*f) * (0.5 - float64(0.33333333333333333*f)))
		if k == 0 {
			return f - R
		}
		dk := float64(k)
		return float64(dk*logLn2Hi) - ((R - float64(dk*logLn2Lo)) - f)
	}
	s := f / (2.0 + f)
	dk := float64(k)
	z := float64(s * s)
	i = hx - 0x6147a
	w := float64(z * z)
	j := 0x6b851 - hx
	t1 := float64(w * (logLg2 + float64(w*(logLg4+float64(w*logLg6)))))
	t2 := float64(z * (logLg1 + float64(w*(logLg3+float64(w*(logLg5+float64(w*logLg7)))))))
	i |= j
	R := t2 + t1
	if i > 0 {
		hfsq := float64(float64(0.5*f) * f)
		if k == 0 {
			return f - (hfsq - float64(s*(hfsq+R)))
		}
		return float64(dk*logLn2Hi) - ((hfsq - (float64(s*(hfsq+R)) + float64(dk*logLn2Lo))) - f)
	}
	if k == 0 {
		return f - float64(s*(f-R))
	}
	return float64(dk*logLn2Hi) - ((float64(s*(f-R)) - float64(dk*logLn2Lo)) - f)
}

var (
	powBp  = [2]float64{1.0, 1.5}
	powDpH = [2]float64{0.0, 5.84962487220764160156e-01}
	powDpL = [2]float64{0.0, 1.35003920212974897128e-08}
)

const (
	powTwo53 = 9007199254740992.0
	powL1    = 5.99999999999994648725e-01
	powL2    = 4.28571428578550184252e-01
	powL3    = 3.33333329818377432918e-01
	powL4    = 2.72728123808534006489e-01
	powL5    = 2.30660745775561754067e-01
	powL6    = 2.06975017800338417784e-01
	powP1    = 1.66666666666666019037e-01
	powP2    = -2.77777777770155933842e-03
	powP3    = 6.61375632143793436117e-05
	powP4    = -1.65339022054652515390e-06
	powP5    = 4.13813679705723846039e-08
	powLg2   = 6.93147180559945286227e-01
	powLg2H  = 6.93147182464599609375e-01
	powLg2L  = -1.90465429995776804525e-09
	powOvt   = 8.0085662595372944372e-17
	powCp    = 9.61796693925975554329e-01
	powCpH   = 9.61796700954437255859e-01
	powCpL   = -7.02846165095275826516e-09
)

// withLow is x with its low 32 bits set to low; withHigh with its high 32 bits set to high.
func withLow(x float64, low uint32) float64 {
	return math.Float64frombits(math.Float64bits(x)&^0xffffffff | uint64(low))
}

func withHigh(x float64, high int32) float64 {
	return math.Float64frombits(uint64(uint32(high))<<32 | math.Float64bits(x)&0xffffffff)
}

func highWord(x float64) int32 { return int32(math.Float64bits(x) >> 32) }

// jsPow is Math.pow as V8 computes it, for what the corrector raises: a positive finite base to a fractional power
// of modest size (the prior's counts to 0.78 and 0.82). V8's pow is fdlibm's __ieee754_pow with one difference,
// kept here: it divides z*t1 by (t1-2) - (w+z*w) where fdlibm takes (z*t1)/(t1-2) - (w+z*w), so neither Go's
// math.Pow nor fdlibm gives its last bit. Other arguments are left to math.Pow, which agrees with JavaScript on
// the special values.
func jsPow(x, y float64) float64 {
	hy := highWord(y)
	iy := hy & 0x7fffffff
	if !(x > 0) || math.IsInf(x, 0) || math.IsNaN(y) || math.IsInf(y, 0) || y == 0 || x == 1 || iy > 0x41e00000 ||
		y == 1 || y == -1 || y == 2 || y == 0.5 {
		return math.Pow(x, y)
	}
	ax := x
	ix := highWord(ax)
	n := int32(0)
	// Take care of subnormal numbers.
	if ix < 0x00100000 {
		ax = float64(ax * powTwo53)
		n -= 53
		ix = highWord(ax)
	}
	n += (ix >> 20) - 0x3ff
	j := ix & 0x000fffff
	// Determine the interval.
	ix = j | 0x3ff00000
	var k int32
	switch {
	case j <= 0x3988E: // |x| < sqrt(3/2)
		k = 0
	case j < 0xBB67A: // |x| < sqrt(3)
		k = 1
	default:
		k = 0
		n++
		ix -= 0x00100000
	}
	ax = withHigh(ax, ix)

	// Compute ss = s_h + s_l = (x-1)/(x+1) or (x-1.5)/(x+1.5).
	u := ax - powBp[k]
	v := 1.0 / (ax + powBp[k])
	ss := float64(u * v)
	sH := withLow(ss, 0)
	// t_h = ax + bp[k], high part.
	tH := withHigh(0, ((ix>>1)|0x20000000)+0x00080000+(k<<18))
	tL := ax - (tH - powBp[k])
	sL := float64(v * ((u - float64(sH*tH)) - float64(sH*tL)))
	// Compute log(ax).
	s2 := float64(ss * ss)
	r := float64(float64(s2*s2) * (powL1 + float64(s2*(powL2+float64(s2*(powL3+float64(s2*(powL4+float64(s2*(powL5+float64(s2*powL6)))))))))))
	r += float64(sL * (sH + ss))
	s2 = float64(sH * sH)
	tH = withLow(3.0+s2+r, 0)
	tL = r - ((tH - 3.0) - s2)
	// u + v = ss * (1 + ...).
	u = float64(sH * tH)
	v = float64(sL*tH) + float64(tL*ss)
	// 2/(3log2) * (ss + ...).
	pH := withLow(u+v, 0)
	pL := v - (pH - u)
	zH := float64(powCpH * pH)
	zL := float64(powCpL*pH) + float64(pL*powCp) + powDpL[k]
	// log2(ax) = (ss + ...) * 2/(3log2) = n + dp_h + z_h + z_l.
	t := float64(n)
	t1 := withLow(((zH+zL)+powDpH[k])+t, 0)
	t2 := zL - (((t1 - t) - powDpH[k]) - zH)

	// Split y into y1 + y2 and compute (y1 + y2) * (t1 + t2).
	y1 := withLow(y, 0)
	pL = float64((y-y1)*t1) + float64(y*t2)
	pH = float64(y1 * t1)
	z := pL + pH
	zb := math.Float64bits(z)
	jj := int32(zb >> 32)
	ii := int32(uint32(zb))
	if jj >= 0x40900000 { // z >= 1024
		if (jj-0x40900000)|ii != 0 || pL+powOvt > z-pH {
			return math.Inf(1)
		}
	} else if (jj & 0x7fffffff) >= 0x4090cc00 { // z <= -1075
		if int32(uint32(jj)-0xc090cc00)|ii != 0 || pL <= z-pH {
			return 0
		}
	}
	// Compute 2**(p_h + p_l).
	i := jj & 0x7fffffff
	k = (i >> 20) - 0x3ff
	n = 0
	if i > 0x3fe00000 { // |z| > 0.5: n = [z + 0.5]
		n = jj + (0x00100000 >> (k + 1))
		k = ((n & 0x7fffffff) >> 20) - 0x3ff // the new k for n
		t = withHigh(0, n&^(0x000fffff>>k))
		n = ((n & 0x000fffff) | 0x00100000) >> (20 - k)
		if jj < 0 {
			n = -n
		}
		pH -= t
	}
	t = withLow(pL+pH, 0)
	u = float64(t * powLg2H)
	v = float64((pL-(t-pH))*powLg2) + float64(t*powLg2L)
	z = u + v
	w := v - (z - u)
	t = float64(z * z)
	t1 = z - float64(t*(powP1+float64(t*(powP2+float64(t*(powP3+float64(t*(powP4+float64(t*powP5)))))))))
	r = float64(z*t1) / ((t1 - 2.0) - (w + float64(z*w)))
	z = 1.0 - (r - z)
	jz := highWord(z) + (n << 20)
	if (jz >> 20) <= 0 {
		return math.Ldexp(z, int(n)) // a subnormal result
	}
	return withHigh(z, jz)
}

// The constants of fdlibm's e_exp.c, as morph/jsutil.go has them.
const (
	expHalfLo   = 0.5
	expO        = 7.09782712893383973096e+02
	expU        = -7.45133219101941108420e+02
	expInvLn2   = 1.44269504088896338700e+00
	expP1       = 1.66666666666666019037e-01
	expP2       = -2.77777777770155933842e-03
	expP3       = 6.61375632143793436117e-05
	expP4       = -1.65339022054652515390e-06
	expP5       = 4.13813679705723846039e-08
	expTwom1000 = 9.33263618503218878990e-302
)

var (
	expLn2HI = [2]float64{6.93147180369123816490e-01, -6.93147180369123816490e-01}
	expLn2LO = [2]float64{1.90821492927058770002e-10, -1.90821492927058770002e-10}
)

// jsExp is Math.exp as V8 computes it (a copy of the morphology's, which is not exported), with fdlibm's __ieee754_exp, so that the prior's numbers are the
// TypeScript's to the last bit on every platform (Go's math.Exp is another algorithm on amd64, and the compiler
// may fuse a multiplication and an addition elsewhere). Every product is converted explicitly to keep it rounded.
func jsExp(x float64) float64 {
	hx := uint32(math.Float64bits(x) >> 32)
	xsb := int(hx >> 31)
	hx &= 0x7fffffff

	var hi, lo float64
	k := 0
	// Filter out non-finite arguments.
	if hx >= 0x40862E42 { // |x| >= 709.78...
		if hx >= 0x7ff00000 {
			lx := uint32(math.Float64bits(x))
			if (hx&0xfffff)|lx != 0 {
				return x + x // NaN
			}
			if xsb == 0 {
				return x
			}
			return 0
		}
		if x > expO {
			return math.Inf(1)
		}
		if x < expU {
			return 0
		}
	}

	// Argument reduction.
	if hx > 0x3fd62e42 { // |x| > 0.5 ln2
		if hx < 0x3FF0A2B2 { // |x| < 1.5 ln2
			if x == 1 {
				return math.E // V8 returns E itself, which the computation below misses by a bit
			}
			hi = x - expLn2HI[xsb]
			lo = expLn2LO[xsb]
			k = 1 - xsb - xsb
		} else {
			half := expHalfLo
			if xsb != 0 {
				half = -expHalfLo
			}
			k = int(float64(expInvLn2*x) + half)
			t := float64(k)
			hi = x - float64(t*expLn2HI[0])
			lo = float64(t * expLn2LO[0])
		}
		x = hi - lo
	} else if hx < 0x3e300000 { // |x| < 2**-28
		if 1.0e300+x > 1 {
			return 1 + x
		}
	} else {
		k = 0
	}

	t := float64(x * x)
	var twopk float64
	if k >= -1021 {
		twopk = math.Float64frombits(uint64(0x3ff+k) << 52)
	} else {
		twopk = math.Float64frombits(uint64(0x3ff+(k+1000)) << 52)
	}
	c := x - float64(t*float64(expP1+float64(t*float64(expP2+float64(t*float64(expP3+float64(t*float64(expP4+float64(t*expP5)))))))))
	if k == 0 {
		return 1 - (float64(float64(x*c)/(c-2.0)) - x)
	}
	y := 1 - ((lo - float64(float64(x*c)/(2.0-c))) - hi)
	if k >= -1021 {
		if k == 1024 {
			return float64(float64(y*2.0) * 8.98846567431157953865e307) // 0x1p1023
		}
		return float64(y * twopk)
	}
	return float64(float64(y*twopk) * expTwom1000)
}

// jsSliceEnd is the end Array.prototype.slice(0, end) takes for an array of length n.
