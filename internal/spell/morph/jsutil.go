package morph

// This file has the JavaScript semantics the port needs and Go does not have as such: String.prototype.trim,
// Number.parseInt, Math.round, and Math.exp as V8 computes it.

import (
	"math"
	"unicode"
	"unicode/utf8"
)

// runeLen is the length of s in runes, the TypeScript's length for text in the Basic Multilingual Plane.
func runeLen(s string) int { return utf8.RuneCountInString(s) }

// firstRune is the first character of s, "" for an empty s.
func firstRune(s string) string {
	_, size := utf8.DecodeRuneInString(s)
	return s[:size]
}

// isJSWhitespace reports whether String.prototype.trim removes r: the white space and line terminators of
// ECMAScript, which are not quite unicode.IsSpace (U+FEFF is one, U+0085 is not).
func isJSWhitespace(r rune) bool {
	switch r {
	case '\t', '\n', '\v', '\f', '\r', ' ', 0xA0, 0xFEFF, 0x2028, 0x2029:
		return true
	}
	return unicode.Is(unicode.Zs, r)
}

// jsTrim is String.prototype.trim.
func jsTrim(s string) string {
	start, end := 0, len(s)
	for start < end {
		r, size := utf8.DecodeRuneInString(s[start:])
		if !isJSWhitespace(r) {
			break
		}
		start += size
	}
	for end > start {
		r, size := utf8.DecodeLastRuneInString(s[start:end])
		if !isJSWhitespace(r) {
			break
		}
		end -= size
	}
	return s[start:end]
}

// jsParseInt is Number.parseInt(s, 10) for the values that fit an int: the leading decimal digits of s after
// white space and a sign; ok is false where JavaScript gives NaN.
func jsParseInt(s string) (int, bool) {
	s = jsTrim(s)
	sign := 1
	if s != "" && (s[0] == '+' || s[0] == '-') {
		if s[0] == '-' {
			sign = -1
		}
		s = s[1:]
	}
	n, digits := 0, 0
	for digits < len(s) && s[digits] >= '0' && s[digits] <= '9' {
		n = n*10 + int(s[digits]-'0')
		digits++
	}
	if digits == 0 {
		return 0, false
	}
	return sign * n, true
}

// jsRound is Math.round: the nearest integer, a half rounded up.
func jsRound(x float64) float64 {
	if math.IsNaN(x) || math.IsInf(x, 0) {
		return x
	}
	r := math.Floor(x)
	if x-r >= 0.5 {
		r++
	}
	return r
}

// The constants of fdlibm's e_exp.c.
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

// jsExp is Math.exp as V8 computes it, with fdlibm's __ieee754_exp, so that the prior's numbers are the
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
func jsSliceEnd(end, n int) int {
	if end < 0 {
		end += n
		if end < 0 {
			end = 0
		}
	}
	return min(end, n)
}
