package morph

// This file is morphotactics.ts: the Turkish morphotactic state machine. It is adapted from Zemberek's
// TurkishMorphotactics.java and Conditions.java, made stricter for formal spelling:
//   - the colloquial vowel-dropped locative and ablative contractions (burda, içerde, ilerde) are left out;
//   - the attached question particle (mi) and conjunctions (de/da/ki) are not suffixes;
//   - the productive -daş (>dAş) derivation and the deyince/yiyince transitions are added;
//   - the closed-class function adjectives (nasıl, öyle, böyle, şöyle, her, herhangi, birkaç, birçok, birtakım,
//     bazı, hiçbir, tüm, bütün) do not become nouns (Adj -> Zero -> Noun), and "daha" takes no locative -da, so
//     the clitic errors *nasılda, *öylemi, *dahada are rejected.

import "github.com/imhalid/celestine/internal/spell/tr"

// morphemeSet is the morphemes of the graph, one value each, compared by pointer as the TypeScript compares its
// objects; the graph's code calls them M.x, as the TypeScript does.
type morphemeSet struct {
	root, noun, adj, verb, pron, adv, conj, ques, postp, det, num, dup, interj *morpheme

	// Person and number agreement.
	a1sg, a2sg, a3sg, a1pl, a2pl, a3pl *morpheme

	// Possessive.
	pnon, p1sg, p2sg, p3sg, p1pl, p2pl, p3pl *morpheme

	// Case.
	nom, dat, acc, abl, loc, ins, gen, equ *morpheme

	// Derivational.
	dim, ness, with, without, related, justLike, rel, agt, dash, become, acquire, ly, caus, recip, reflex, able,
	pass, inf1, inf2, inf3, actOf, pastPart, narrPart, futPart, presPart, aorPart, notState, feelLike, everSince,
	repeat, almost, hastily, stay, start, asIf, while_, when, sinceDoingSo, asLongAs, byDoingSo, adamantly,
	afterDoingSo, withoutHavingDoneSo, withoutBeingAbleToHaveDoneSo, zero *morpheme

	// Verbal inflection.
	cop, neg, unable, pres, past, narr, cond, prog1, prog2, aor, fut, imp, opt, desr, neces *morpheme
}

// newMorpheme is a morpheme; pos is "" for none.
func newMorpheme(id string, pos PrimaryPos, derivational bool) *morpheme {
	return &morpheme{id: id, pos: pos, derivational: derivational}
}

// allMorphemes is the morphemes of the graph (MORPHEMES).
var allMorphemes = morphemeSet{
	root:   newMorpheme("Root", "", false),
	noun:   newMorpheme("Noun", "Noun", false),
	adj:    newMorpheme("Adj", "Adj", false),
	verb:   newMorpheme("Verb", "Verb", false),
	pron:   newMorpheme("Pron", "Pron", false),
	adv:    newMorpheme("Adv", "Adv", false),
	conj:   newMorpheme("Conj", "Conj", false),
	ques:   newMorpheme("Ques", "Ques", false),
	postp:  newMorpheme("Postp", "Postp", false),
	det:    newMorpheme("Det", "Det", false),
	num:    newMorpheme("Num", "Num", false),
	dup:    newMorpheme("Dup", "Dup", false),
	interj: newMorpheme("Interj", "Interj", false),

	a1sg: newMorpheme("A1sg", "", false),
	a2sg: newMorpheme("A2sg", "", false),
	a3sg: newMorpheme("A3sg", "", false),
	a1pl: newMorpheme("A1pl", "", false),
	a2pl: newMorpheme("A2pl", "", false),
	a3pl: newMorpheme("A3pl", "", false),

	pnon: newMorpheme("Pnon", "", false),
	p1sg: newMorpheme("P1sg", "", false),
	p2sg: newMorpheme("P2sg", "", false),
	p3sg: newMorpheme("P3sg", "", false),
	p1pl: newMorpheme("P1pl", "", false),
	p2pl: newMorpheme("P2pl", "", false),
	p3pl: newMorpheme("P3pl", "", false),

	nom: newMorpheme("Nom", "", false),
	dat: newMorpheme("Dat", "", false),
	acc: newMorpheme("Acc", "", false),
	abl: newMorpheme("Abl", "", false),
	loc: newMorpheme("Loc", "", false),
	ins: newMorpheme("Ins", "", false),
	gen: newMorpheme("Gen", "", false),
	equ: newMorpheme("Equ", "", false),

	dim:                          newMorpheme("Dim", "", true),
	ness:                         newMorpheme("Ness", "", true),
	with:                         newMorpheme("With", "", true),
	without:                      newMorpheme("Without", "", true),
	related:                      newMorpheme("Related", "", true),
	justLike:                     newMorpheme("JustLike", "", true),
	rel:                          newMorpheme("Rel", "", true),
	agt:                          newMorpheme("Agt", "", true),
	dash:                         newMorpheme("Dash", "", true),
	become:                       newMorpheme("Become", "", true),
	acquire:                      newMorpheme("Acquire", "", true),
	ly:                           newMorpheme("Ly", "", true),
	caus:                         newMorpheme("Caus", "", true),
	recip:                        newMorpheme("Recip", "", true),
	reflex:                       newMorpheme("Reflex", "", true),
	able:                         newMorpheme("Able", "", true),
	pass:                         newMorpheme("Pass", "", true),
	inf1:                         newMorpheme("Inf1", "", true),
	inf2:                         newMorpheme("Inf2", "", true),
	inf3:                         newMorpheme("Inf3", "", true),
	actOf:                        newMorpheme("ActOf", "", true),
	pastPart:                     newMorpheme("PastPart", "", true),
	narrPart:                     newMorpheme("NarrPart", "", true),
	futPart:                      newMorpheme("FutPart", "", true),
	presPart:                     newMorpheme("PresPart", "", true),
	aorPart:                      newMorpheme("AorPart", "", true),
	notState:                     newMorpheme("NotState", "", true),
	feelLike:                     newMorpheme("FeelLike", "", true),
	everSince:                    newMorpheme("EverSince", "", true),
	repeat:                       newMorpheme("Repeat", "", true),
	almost:                       newMorpheme("Almost", "", true),
	hastily:                      newMorpheme("Hastily", "", true),
	stay:                         newMorpheme("Stay", "", true),
	start:                        newMorpheme("Start", "", true),
	asIf:                         newMorpheme("AsIf", "", true),
	while_:                       newMorpheme("While", "", true),
	when:                         newMorpheme("When", "", true),
	sinceDoingSo:                 newMorpheme("SinceDoingSo", "", true),
	asLongAs:                     newMorpheme("AsLongAs", "", true),
	byDoingSo:                    newMorpheme("ByDoingSo", "", true),
	adamantly:                    newMorpheme("Adamantly", "", true),
	afterDoingSo:                 newMorpheme("AfterDoingSo", "", true),
	withoutHavingDoneSo:          newMorpheme("WithoutHavingDoneSo", "", true),
	withoutBeingAbleToHaveDoneSo: newMorpheme("WithoutBeingAbleToHaveDoneSo", "", true),
	zero:                         newMorpheme("Zero", "", true),

	cop:    newMorpheme("Cop", "", false),
	neg:    newMorpheme("Neg", "", false),
	unable: newMorpheme("Unable", "", false),
	pres:   newMorpheme("Pres", "", false),
	past:   newMorpheme("Past", "", false),
	narr:   newMorpheme("Narr", "", false),
	cond:   newMorpheme("Cond", "", false),
	prog1:  newMorpheme("Prog1", "", false),
	prog2:  newMorpheme("Prog2", "", false),
	aor:    newMorpheme("Aor", "", false),
	fut:    newMorpheme("Fut", "", false),
	imp:    newMorpheme("Imp", "", false),
	opt:    newMorpheme("Opt", "", false),
	desr:   newMorpheme("Desr", "", false),
	neces:  newMorpheme("Neces", "", false),
}

// The condition combinators.

var (
	hasTail           condition = func(p *pathNode) bool { return p.pos < p.inputLength }
	hasSurface        condition = func(p *pathNode) bool { return p.hasSurface }
	hasNoSurface      condition = func(p *pathNode) bool { return !p.hasSurface }
	currentGroupEmpty condition = func(p *pathNode) bool { return !p.currentGroupHasSurface }
)

// hasAttr holds when the root has one of the attributes of mask.
func hasAttr(mask int) condition {
	return func(p *pathNode) bool { return p.stemTransition.item.attributes&mask != 0 }
}

// notHaveAttr holds when the root has none of the attributes of mask.
func notHaveAttr(mask int) condition {
	return func(p *pathNode) bool { return p.stemTransition.item.attributes&mask == 0 }
}

// hasPhon holds when the path's phonetic bit mask has one of mask's bits.
func hasPhon(mask int) condition {
	return func(p *pathNode) bool { return p.phonetic&mask != 0 }
}

// notHavePhon holds when the path's phonetic bit mask has none of mask's bits.
func notHavePhon(mask int) condition {
	return func(p *pathNode) bool { return p.phonetic&mask == 0 }
}

// rootKeyIs holds when the root's key is key.
func rootKeyIs(key string) condition {
	return func(p *pathNode) bool { return p.stemTransition.item.key == key }
}

// rootKeyIsNot holds when the root's key is not key.
func rootKeyIsNot(key string) condition {
	return func(p *pathNode) bool { return p.stemTransition.item.key != key }
}

// stringSet is a set of strings.
func stringSet(items []string) map[string]bool {
	set := make(map[string]bool, len(items))
	for _, s := range items {
		set[s] = true
	}
	return set
}

// rootKeyIn holds when the root's key is one of keys.
func rootKeyIn(keys ...string) condition {
	set := stringSet(keys)
	return func(p *pathNode) bool { return set[p.stemTransition.item.key] }
}

// rootKeyNotIn holds when the root's key is none of keys.
func rootKeyNotIn(keys ...string) condition {
	set := stringSet(keys)
	return func(p *pathNode) bool { return !set[p.stemTransition.item.key] }
}

// rootSurfaceIs holds when the stem's surface is surface.
func rootSurfaceIs(surface string) condition {
	return func(p *pathNode) bool { return p.stemTransition.surface == surface }
}

// rootSurfaceIn holds when the stem's surface is one of surfaces.
func rootSurfaceIn(surfaces ...string) condition {
	set := stringSet(surfaces)
	return func(p *pathNode) bool { return set[p.stemTransition.surface] }
}

// secondaryPosIs holds when the root's secondary part of speech is pos.
func secondaryPosIs(pos SecondaryPos) condition {
	return func(p *pathNode) bool { return p.stemTransition.item.secondaryPos == pos }
}

// stateSet is a set of states.
func stateSet(states []*morphemeState) map[*morphemeState]bool {
	set := make(map[*morphemeState]bool, len(states))
	for _, s := range states {
		set[s] = true
	}
	return set
}

// morphemeSetOf is a set of morphemes.
func morphemeSetOf(ms []*morpheme) map[*morpheme]bool {
	set := make(map[*morpheme]bool, len(ms))
	for _, m := range ms {
		set[m] = true
	}
	return set
}

// prevStateIs holds when the previous step is in state.
func prevStateIs(state *morphemeState) condition {
	return func(p *pathNode) bool { return p.prev != nil && p.prev.state == state }
}

// prevStateIsNot holds when there is no previous step or it is not in state.
func prevStateIsNot(state *morphemeState) condition {
	return func(p *pathNode) bool { return p.prev == nil || p.prev.state != state }
}

// prevStateIn holds when the previous step is in one of states.
func prevStateIn(states ...*morphemeState) condition {
	set := stateSet(states)
	return func(p *pathNode) bool { return p.prev != nil && set[p.prev.state] }
}

// prevMorphemeIs holds when the previous step's morpheme is m.
func prevMorphemeIs(m *morpheme) condition {
	return func(p *pathNode) bool { return p.prev != nil && p.prev.state.morpheme == m }
}

// prevMorphemeIsNot holds when there is no previous step or its morpheme is not m.
func prevMorphemeIsNot(m *morpheme) condition {
	return func(p *pathNode) bool { return p.prev == nil || p.prev.state.morpheme != m }
}

// prevMorphemeIn holds when the previous step's morpheme is one of ms.
func prevMorphemeIn(ms ...*morpheme) condition {
	set := morphemeSetOf(ms)
	return func(p *pathNode) bool { return p.prev != nil && set[p.prev.state.morpheme] }
}

// lastDerivIs holds when the path's last derivation is state.
func lastDerivIs(state *morphemeState) condition {
	return func(p *pathNode) bool { return p.lastDerivation == state }
}

// lastDerivIn holds when the path's last derivation is one of states.
func lastDerivIn(states ...*morphemeState) condition {
	set := stateSet(states)
	return func(p *pathNode) bool { return p.lastDerivation != nil && set[p.lastDerivation] }
}

// containsMorpheme holds when a step of the path has one of ms.
func containsMorpheme(ms ...*morpheme) condition {
	if len(ms) == 1 {
		target := ms[0]
		return func(p *pathNode) bool {
			for cur := p; cur != nil; cur = cur.prev {
				if cur.state.morpheme == target {
					return true
				}
			}
			return false
		}
	}
	set := morphemeSetOf(ms)
	return func(p *pathNode) bool {
		for cur := p; cur != nil; cur = cur.prev {
			if set[cur.state.morpheme] {
				return true
			}
		}
		return false
	}
}

// containsMorphemeSeq holds when the path has the morphemes of seq in consecutive steps.
func containsMorphemeSeq(seq ...*morpheme) condition {
	n := len(seq)
	last := seq[n-1]
	return func(p *pathNode) bool {
		for cur := p; cur != nil; cur = cur.prev {
			if cur.state.morpheme == last {
				probe := cur
				idx := n - 1
				for idx >= 0 && probe != nil && probe.state.morpheme == seq[idx] {
					idx--
					probe = probe.prev
				}
				if idx < 0 {
					return true
				}
			}
		}
		return false
	}
}

// hasTailSeq holds when the path's last steps have the morphemes of seq.
func hasTailSeq(seq ...*morpheme) condition {
	return func(p *pathNode) bool {
		cur := p
		for idx := len(seq) - 1; idx >= 0; idx-- {
			if cur == nil || cur.state.morpheme != seq[idx] {
				return false
			}
			cur = cur.prev
		}
		return true
	}
}

// currentGroupContains holds when a step after the last derivation is in one of states.
func currentGroupContains(states ...*morphemeState) condition {
	set := stateSet(states)
	return func(p *pathNode) bool {
		for cur := p; cur != nil && cur.prev != nil; cur = cur.prev {
			if set[cur.state] {
				return true
			}
			if cur.state.derivative {
				return false
			}
		}
		return false
	}
}

// prevGroupContains holds when a step of the group before the last derivation is in one of states.
func prevGroupContains(states ...*morphemeState) condition {
	set := stateSet(states)
	return func(p *pathNode) bool {
		cur := p
		for cur != nil && !cur.state.derivative {
			cur = cur.prev
		}
		if cur == nil || cur.prev == nil {
			return false
		}
		for cur = cur.prev; cur != nil && cur.prev != nil; cur = cur.prev {
			if set[cur.state] {
				return true
			}
			if cur.state.derivative {
				return false
			}
		}
		return false
	}
}

// prevGroupContainsMorpheme holds when a step of the group before the last derivation has one of ms.
func prevGroupContainsMorpheme(ms ...*morpheme) condition {
	set := morphemeSetOf(ms)
	return func(p *pathNode) bool {
		cur := p
		for cur != nil && !cur.state.derivative {
			cur = cur.prev
		}
		if cur == nil || cur.prev == nil {
			return false
		}
		for cur = cur.prev; cur != nil && cur.prev != nil; cur = cur.prev {
			if set[cur.state.morpheme] {
				return true
			}
			if cur.state.derivative {
				return false
			}
		}
		return false
	}
}

// and holds when all of conds hold.
func and(conds ...condition) condition {
	return func(p *pathNode) bool {
		for _, c := range conds {
			if c != nil && !c(p) {
				return false
			}
		}
		return true
	}
}

// or holds when one of conds holds.
func or(conds ...condition) condition {
	return func(p *pathNode) bool {
		for _, c := range conds {
			if c != nil && c(p) {
				return true
			}
		}
		return false
	}
}

// not holds when cond does not.
func not(cond condition) condition {
	return func(p *pathNode) bool { return !cond(p) }
}

// The flags of makeState.
const (
	sTerminal = 1 << iota
	sDerivative
	sPosRoot
)

// makeState is a state with no transitions yet.
func makeState(id string, m *morpheme, flags ...int) *morphemeState {
	f := 0
	for _, x := range flags {
		f |= x
	}
	return &morphemeState{id: id, morpheme: m, terminal: f&sTerminal != 0, derivative: f&sDerivative != 0, posRoot: f&sPosRoot != 0}
}

// computeForbiddenPhonetic is the expectation a template's first sound rules out: a template starting with a
// consonant cannot follow a stem that expects a vowel, one starting with a vowel a stem that expects a consonant.
func computeForbiddenPhonetic(template string) int {
	if template == "" {
		return 0
	}
	lower := []rune(tr.Lower(template))
	first := lower[0]
	firstCharVowel := isVowel(first)
	forbidden := 0
	if first == '>' || !firstCharVowel {
		forbidden = paExpectsVowel
	}
	if first == '+' && len(lower) > 2 && isVowel(lower[2]) || firstCharVowel {
		forbidden = paExpectsConsonant
	}
	return forbidden
}

// addTransition adds a suffix transition from one state to another.
func addTransition(from, to *morphemeState, template string, cond ...condition) {
	tokens := tokenizeTemplate(template)
	t := &suffixTransition{
		from:              from,
		to:                to,
		template:          template,
		tokens:            tokens,
		hasSurface:        len(tokens) > 0,
		forbiddenPhonetic: computeForbiddenPhonetic(template),
		cache:             new(surfaceCache),
	}
	if len(tokens) > 0 {
		t.lastToken = &tokens[len(tokens)-1]
	}
	if len(cond) > 0 {
		t.condition = cond[0]
	}
	from.outgoing = append(from.outgoing, t)
}

// addEmpty adds a transition without a surface.
func addEmpty(from, to *morphemeState, cond ...condition) {
	addTransition(from, to, "", cond...)
}

// copyOutgoing gives target the transitions source has now, sharing their surface caches.
func copyOutgoing(target, source *morphemeState) {
	for _, t := range source.outgoing {
		c := *t
		c.from = target
		target.outgoing = append(target.outgoing, &c)
	}
}

// removeTransitionsToMorpheme removes a state's transitions to the states of morpheme m.
func removeTransitionsToMorpheme(state *morphemeState, m *morpheme) {
	for i := len(state.outgoing) - 1; i >= 0; i-- {
		if state.outgoing[i].to.morpheme == m {
			state.outgoing = append(state.outgoing[:i], state.outgoing[i+1:]...)
		}
	}
}

// nonNominalizableAdjLemmas are the closed-class function adjectives that take the copula (nasılsın, öyleydi)
// through adjZeroDeriv -> nVerb_S but no noun case or possessive (*nasılda, *öylemi, *herde, *birkaçta).
var nonNominalizableAdjLemmas = stringSet([]string{
	"nasıl", "öyle", "böyle", "şöyle", "her", "herhangi", "birkaç", "birçok", "birtakım", "bazı", "hiçbir", "tüm",
	"bütün",
})

// morphotacticsGraph is the root states of the graph and how a dictionary item finds its own.
type morphotacticsGraph struct {
	noun_S, nounCompoundRoot_S, nounSuRoot_S, nounProper_S, nounAbbrv_S, nounNoSuffix_S                  *morphemeState
	adjectiveRoot_ST, adjNoSuffix_ST, numeralRoot_ST, nVerbDegil_S                                       *morphemeState
	pronPers_S, pronPers_Mod_S, pronDemons_S, pronQuant_S, pronQuantModified_S, pronQues_S, pronReflex_S *morphemeState
	advRoot_ST, advForVerbDeriv_ST, conjRoot_ST, interjRoot_ST, detRoot_ST, dupRoot_ST, postpRoot_ST     *morphemeState
	verbRoot_S, verbRoot_VowelDrop_S, verbLastVowelDropModRoot_S, verbLastVowelDropUnmodRoot_S           *morphemeState
	vImplicitRecipRoot_S, vImplicitReflexRoot_S, vDeYeRoot_S, questionRoot_S, imekRoot_S                 *morphemeState
	getRootState                                                                                         func(item *dictionaryItem, phonetic int) *morphemeState
}

// newMorphotacticsGraph builds the graph. The TypeScript builds it once per process; here each morphology has
// its own, so the transitions' surface caches, which remember the first path that met each key, start empty for
// each as they do for a fresh TypeScript process.
func newMorphotacticsGraph() *morphotacticsGraph {
	M := &allMorphemes

	// --- Noun states ---
	noun_S := makeState("noun_S", M.noun, sPosRoot)
	nounCompoundRoot_S := makeState("nounCompoundRoot_S", M.noun, sPosRoot)
	nounSuRoot_S := makeState("nounSuRoot_S", M.noun, sPosRoot)
	nounInf1Root_S := makeState("nounInf1Root_S", M.noun, sPosRoot)
	nounActOfRoot_S := makeState("nounActOfRoot_S", M.noun, sPosRoot)
	nounProper_S := makeState("nounProper_S", M.noun, sPosRoot)
	nounAbbrv_S := makeState("nounAbbrv_S", M.noun, sPosRoot)
	nounNoSuffix_S := makeState("nounNoSuffix_S", M.noun, sPosRoot)
	nounA3sgNoSuffix_S := makeState("nounA3sgNoSuffix_S", M.a3sg)
	nounPnonNoSuffix_S := makeState("nounPnonNoSuffix_S", M.pnon)
	nounNomNoSuffix_ST := makeState("nounNomNoSuffix_ST", M.nom, sTerminal)

	a3sg_S := makeState("a3sg_S", M.a3sg)
	a3sgSu_S := makeState("a3sgSu_S", M.a3sg)
	a3sgCompound_S := makeState("a3sgCompound_S", M.a3sg)
	a3sgInf1_S := makeState("a3sgInf1_S", M.a3sg)
	a3sgActOf_S := makeState("a3sgActOf_S", M.a3sg)
	a3pl_S := makeState("a3pl_S", M.a3pl)
	a3plActOf_S := makeState("a3plActOf_S", M.a3pl)
	a3plCompound_S := makeState("a3plCompound_S", M.a3pl)
	a3plCompound2_S := makeState("a3plCompound2_S", M.a3pl)

	pnon_S := makeState("pnon_S", M.pnon)
	pnonCompound_S := makeState("pnonCompound_S", M.pnon)
	pnonCompound2_S := makeState("pnonCompound2_S", M.pnon)
	pnonInf1_S := makeState("pnonInf1_S", M.pnon)
	pnonActOf_S := makeState("pnonActOf_S", M.pnon)
	p1sg_S := makeState("p1sg_S", M.p1sg)
	p2sg_S := makeState("p2sg_S", M.p2sg)
	p3sg_S := makeState("p3sg_S", M.p3sg)
	p1pl_S := makeState("p1pl_S", M.p1pl)
	p2pl_S := makeState("p2pl_S", M.p2pl)
	p3pl_S := makeState("p3pl_S", M.p3pl)

	nom_ST := makeState("nom_ST", M.nom, sTerminal)
	nom_S := makeState("nom_S", M.nom)
	dat_ST := makeState("dat_ST", M.dat, sTerminal)
	abl_ST := makeState("abl_ST", M.abl, sTerminal)
	loc_ST := makeState("loc_ST", M.loc, sTerminal)
	ins_ST := makeState("ins_ST", M.ins, sTerminal)
	acc_ST := makeState("acc_ST", M.acc, sTerminal)
	gen_ST := makeState("gen_ST", M.gen, sTerminal)
	equ_ST := makeState("equ_ST", M.equ, sTerminal)

	dim_S := makeState("dim_S", M.dim, sDerivative)
	ness_S := makeState("ness_S", M.ness, sDerivative)
	agt_S := makeState("agt_S", M.agt, sDerivative)
	dash_S := makeState("dash_S", M.dash, sDerivative)
	related_S := makeState("related_S", M.related, sDerivative)
	rel_S := makeState("rel_S", M.rel, sDerivative)
	relToPron_S := makeState("relToPron_S", M.rel, sDerivative)
	with_S := makeState("with_S", M.with, sDerivative)
	without_S := makeState("without_S", M.without, sDerivative)
	justLike_S := makeState("justLike_S", M.justLike, sDerivative)
	nounZeroDeriv_S := makeState("nounZeroDeriv_S", M.zero, sDerivative)
	become_S := makeState("become_S", M.become, sDerivative)
	acquire_S := makeState("acquire_S", M.acquire, sDerivative)

	// --- Adjective states ---
	adjectiveRoot_ST := makeState("adjectiveRoot_ST", M.adj, sTerminal|sPosRoot)
	adjNoSuffix_ST := makeState("adjNoSuffix_ST", M.adj, sTerminal|sPosRoot)
	adjAfterVerb_S := makeState("adjAfterVerb_S", M.adj, sPosRoot)
	adjAfterVerb_ST := makeState("adjAfterVerb_ST", M.adj, sTerminal|sPosRoot)
	adjZeroDeriv_S := makeState("adjZeroDeriv_S", M.zero, sDerivative)

	aPnon_ST := makeState("aPnon_ST", M.pnon, sTerminal)
	aP1sg_ST := makeState("aP1sg_ST", M.p1sg, sTerminal)
	aP2sg_ST := makeState("aP2sg_ST", M.p2sg, sTerminal)
	aP3sg_ST := makeState("aP3sg_ST", M.p3sg, sTerminal)
	aP1pl_ST := makeState("aP1pl_ST", M.p1pl, sTerminal)
	aP2pl_ST := makeState("aP2pl_ST", M.p2pl, sTerminal)
	aP3pl_ST := makeState("aP3pl_ST", M.p3pl, sTerminal)

	aLy_S := makeState("aLy_S", M.ly, sDerivative)
	aAsIf_S := makeState("aAsIf_S", M.asIf, sDerivative)
	aAgt_S := makeState("aAgt_S", M.agt, sDerivative)

	aorPartNegZero_S := makeState("aorPartNegZero_S", M.zero, sDerivative)
	aorPartNegNoun_S := makeState("aorPartNegNoun_S", M.noun, sPosRoot)
	aorPartNegA3sg_S := makeState("aorPartNegA3sg_S", M.a3sg)
	aorPartNegPnon_S := makeState("aorPartNegPnon_S", M.pnon)
	aorPartNegAbl_ST := makeState("aorPartNegAbl_ST", M.abl, sTerminal)

	// --- Numeral states ---
	numeralRoot_ST := makeState("numeralRoot_ST", M.num, sTerminal|sPosRoot)
	numZeroDeriv_S := makeState("numZeroDeriv_S", M.zero, sDerivative)

	// --- Nominal predicate (ek-fiil) states ---
	nVerb_S := makeState("nVerb_S", M.verb, sPosRoot)
	nVerbDegil_S := makeState("nVerbDegil_S", M.verb, sPosRoot)
	nPresent_S := makeState("nPresent_S", M.pres)
	nPast_S := makeState("nPast_S", M.past)
	nNarr_S := makeState("nNarr_S", M.narr)
	nCond_S := makeState("nCond_S", M.cond)
	nA1sg_ST := makeState("nA1sg_ST", M.a1sg, sTerminal)
	nA2sg_ST := makeState("nA2sg_ST", M.a2sg, sTerminal)
	nA1pl_ST := makeState("nA1pl_ST", M.a1pl, sTerminal)
	nA2pl_ST := makeState("nA2pl_ST", M.a2pl, sTerminal)
	nA3sg_ST := makeState("nA3sg_ST", M.a3sg, sTerminal)
	nA3sg_S := makeState("nA3sg_S", M.a3sg)
	nA3pl_ST := makeState("nA3pl_ST", M.a3pl, sTerminal)
	nCop_ST := makeState("nCop_ST", M.cop, sTerminal)
	nCopBeforeA3pl_S := makeState("nCopBeforeA3pl_S", M.cop)
	nNeg_S := makeState("nNeg_S", M.neg)

	// --- Pronoun states ---
	pronPers_S := makeState("pronPers_S", M.pron, sPosRoot)
	pronPers_Mod_S := makeState("pronPers_Mod_S", M.pron, sPosRoot)
	pronDemons_S := makeState("pronDemons_S", M.pron, sPosRoot)
	pronQuant_S := makeState("pronQuant_S", M.pron, sPosRoot)
	pronQuantModified_S := makeState("pronQuantModified_S", M.pron, sPosRoot)
	pronQues_S := makeState("pronQues_S", M.pron, sPosRoot)
	pronReflex_S := makeState("pronReflex_S", M.pron, sPosRoot)
	pronAfterRel_S := makeState("pronAfterRel_S", M.pron, sPosRoot)

	pA1sg_S := makeState("pA1sg_S", M.a1sg)
	pA2sg_S := makeState("pA2sg_S", M.a2sg)
	pA1sgMod_S := makeState("pA1sgMod_S", M.a1sg)
	pA2sgMod_S := makeState("pA2sgMod_S", M.a2sg)
	pA3sg_S := makeState("pA3sg_S", M.a3sg)
	pA3sgRel_S := makeState("pA3sgRel_S", M.a3sg)
	pA1pl_S := makeState("pA1pl_S", M.a1pl)
	pA2pl_S := makeState("pA2pl_S", M.a2pl)
	pA3pl_S := makeState("pA3pl_S", M.a3pl)
	pA3plRel_S := makeState("pA3plRel_S", M.a3pl)

	pQuantA3sg_S := makeState("pQuantA3sg_S", M.a3sg)
	pQuantA3pl_S := makeState("pQuantA3pl_S", M.a3pl)
	pQuantModA3pl_S := makeState("pQuantModA3pl_S", M.a3pl)
	pQuantA1pl_S := makeState("pQuantA1pl_S", M.a1pl)
	pQuantA2pl_S := makeState("pQuantA2pl_S", M.a2pl)

	pQuesA3sg_S := makeState("pQuesA3sg_S", M.a3sg)
	pQuesA3pl_S := makeState("pQuesA3pl_S", M.a3pl)

	pReflexA1sg_S := makeState("pReflexA1sg_S", M.a1sg)
	pReflexA2sg_S := makeState("pReflexA2sg_S", M.a2sg)
	pReflexA3sg_S := makeState("pReflexA3sg_S", M.a3sg)
	pReflexA1pl_S := makeState("pReflexA1pl_S", M.a1pl)
	pReflexA2pl_S := makeState("pReflexA2pl_S", M.a2pl)
	pReflexA3pl_S := makeState("pReflexA3pl_S", M.a3pl)

	pPnon_S := makeState("pPnon_S", M.pnon)
	pPnonRel_S := makeState("pPnonRel_S", M.pnon)
	pPnonMod_S := makeState("pPnonMod_S", M.pnon)
	pP1sg_S := makeState("pP1sg_S", M.p1sg)
	pP2sg_S := makeState("pP2sg_S", M.p2sg)
	pP3sg_S := makeState("pP3sg_S", M.p3sg)
	pP1pl_S := makeState("pP1pl_S", M.p1pl)
	pP2pl_S := makeState("pP2pl_S", M.p2pl)
	pP3pl_S := makeState("pP3pl_S", M.p3pl)

	pNom_ST := makeState("pNom_ST", M.nom, sTerminal)
	pDat_ST := makeState("pDat_ST", M.dat, sTerminal)
	pAcc_ST := makeState("pAcc_ST", M.acc, sTerminal)
	pAbl_ST := makeState("pAbl_ST", M.abl, sTerminal)
	pLoc_ST := makeState("pLoc_ST", M.loc, sTerminal)
	pGen_ST := makeState("pGen_ST", M.gen, sTerminal)
	pIns_ST := makeState("pIns_ST", M.ins, sTerminal)
	pEqu_ST := makeState("pEqu_ST", M.equ, sTerminal)
	pronZeroDeriv_S := makeState("pronZeroDeriv_S", M.zero, sDerivative)

	pvVerbRoot_S := makeState("pvVerbRoot_S", M.verb, sPosRoot)
	pvPresent_S := makeState("pvPresent_S", M.pres)
	pvPast_S := makeState("pvPast_S", M.past)
	pvNarr_S := makeState("pvNarr_S", M.narr)
	pvCond_S := makeState("pvCond_S", M.cond)
	pvA1sg_ST := makeState("pvA1sg_ST", M.a1sg, sTerminal)
	pvA2sg_ST := makeState("pvA2sg_ST", M.a2sg, sTerminal)
	pvA3sg_ST := makeState("pvA3sg_ST", M.a3sg, sTerminal)
	pvA3sg_S := makeState("pvA3sg_S", M.a3sg)
	pvA1pl_ST := makeState("pvA1pl_ST", M.a1pl, sTerminal)
	pvA2pl_ST := makeState("pvA2pl_ST", M.a2pl, sTerminal)
	pvA3pl_ST := makeState("pvA3pl_ST", M.a3pl, sTerminal)
	pvCopBeforeA3pl_S := makeState("pvCopBeforeA3pl_S", M.cop)
	pvCop_ST := makeState("pvCop_ST", M.cop, sTerminal)

	// --- Adverb / Closed class / Postpositive states ---
	advRoot_ST := makeState("advRoot_ST", M.adv, sTerminal|sPosRoot)
	advNounRoot_ST := makeState("advNounRoot_ST", M.adv, sTerminal|sPosRoot)
	advForVerbDeriv_ST := makeState("advForVerbDeriv_ST", M.adv, sTerminal|sPosRoot)
	avNounAfterAdvRoot_S := makeState("avNounAfterAdvRoot_S", M.noun, sPosRoot)
	avA3sg_S := makeState("avA3sg_S", M.a3sg)
	avPnon_S := makeState("avPnon_S", M.pnon)
	avDat_ST := makeState("avDat_ST", M.dat, sTerminal)
	avZero_S := makeState("avZero_S", M.zero, sDerivative)
	avZeroToVerb_S := makeState("avZeroToVerb_S", M.zero, sDerivative)

	conjRoot_ST := makeState("conjRoot_ST", M.conj, sTerminal|sPosRoot)
	interjRoot_ST := makeState("interjRoot_ST", M.interj, sTerminal|sPosRoot)
	detRoot_ST := makeState("detRoot_ST", M.det, sTerminal|sPosRoot)
	dupRoot_ST := makeState("dupRoot_ST", M.dup, sTerminal|sPosRoot)

	postpRoot_ST := makeState("postpRoot_ST", M.postp, sTerminal|sPosRoot)
	postpZero_S := makeState("postpZero_S", M.zero, sDerivative)
	po2nRoot_S := makeState("po2nRoot_S", M.noun)
	po2nA3sg_S := makeState("po2nA3sg_S", M.a3sg)
	po2nA3pl_S := makeState("po2nA3pl_S", M.a3pl)
	po2nP3sg_S := makeState("po2nP3sg_S", M.p3sg)
	po2nP1sg_S := makeState("po2nP1sg_S", M.p1sg)
	po2nP2sg_S := makeState("po2nP2sg_S", M.p2sg)
	po2nP1pl_S := makeState("po2nP1pl_S", M.p1pl)
	po2nP2pl_S := makeState("po2nP2pl_S", M.p2pl)
	po2nPnon_S := makeState("po2nPnon_S", M.pnon)
	po2nNom_ST := makeState("po2nNom_ST", M.nom, sTerminal)
	po2nDat_ST := makeState("po2nDat_ST", M.dat, sTerminal)
	po2nAbl_ST := makeState("po2nAbl_ST", M.abl, sTerminal)
	po2nLoc_ST := makeState("po2nLoc_ST", M.loc, sTerminal)
	po2nIns_ST := makeState("po2nIns_ST", M.ins, sTerminal)
	po2nAcc_ST := makeState("po2nAcc_ST", M.acc, sTerminal)
	po2nGen_ST := makeState("po2nGen_ST", M.gen, sTerminal)
	po2nEqu_ST := makeState("po2nEqu_ST", M.equ, sTerminal)

	// --- Verb states ---
	verbRoot_S := makeState("verbRoot_S", M.verb, sPosRoot)
	verbRoot_VowelDrop_S := makeState("verbRoot_VowelDrop_S", M.verb, sPosRoot)
	verbLastVowelDropModRoot_S := makeState("verbLastVowelDropModRoot_S", M.verb, sPosRoot)
	verbLastVowelDropUnmodRoot_S := makeState("verbLastVowelDropUnmodRoot_S", M.verb, sPosRoot)
	vDeYeRoot_S := makeState("vDeYeRoot_S", M.verb, sPosRoot)

	vA1sg_ST := makeState("vA1sg_ST", M.a1sg, sTerminal)
	vA2sg_ST := makeState("vA2sg_ST", M.a2sg, sTerminal)
	vA3sg_ST := makeState("vA3sg_ST", M.a3sg, sTerminal)
	vA1pl_ST := makeState("vA1pl_ST", M.a1pl, sTerminal)
	vA2pl_ST := makeState("vA2pl_ST", M.a2pl, sTerminal)
	vA3pl_ST := makeState("vA3pl_ST", M.a3pl, sTerminal)

	vPast_S := makeState("vPast_S", M.past)
	vNarr_S := makeState("vNarr_S", M.narr)
	vCond_S := makeState("vCond_S", M.cond)
	vCondAfterPerson_ST := makeState("vCondAfterPerson_ST", M.cond, sTerminal)
	vPastAfterTense_S := makeState("vPastAfterTense_S", M.past)
	vNarrAfterTense_S := makeState("vNarrAfterTense_S", M.narr)
	vPastAfterTense_ST := makeState("vPastAfterTense_ST", M.past, sTerminal)
	vNarrAfterTense_ST := makeState("vNarrAfterTense_ST", M.narr, sTerminal)
	vCond_ST := makeState("vCond_ST", M.cond, sTerminal)

	vProgYor_S := makeState("vProgYor_S", M.prog1)
	vProgMakta_S := makeState("vProgMakta_S", M.prog2)
	vFut_S := makeState("vFut_S", M.fut)
	vCop_ST := makeState("vCop_ST", M.cop, sTerminal)
	vCopBeforeA3pl_S := makeState("vCopBeforeA3pl_S", M.cop)

	vNeg_S := makeState("vNeg_S", M.neg)
	vUnable_S := makeState("vUnable_S", M.unable)
	vNegProg1_S := makeState("vNegProg1_S", M.neg)
	vUnableProg1_S := makeState("vUnableProg1_S", M.unable)

	vImp_S := makeState("vImp_S", M.imp)
	vImpYemekYi_S := makeState("vImpYemekYi_S", M.imp)
	vImpYemekYe_S := makeState("vImpYemekYe_S", M.imp)

	vCausT_S := makeState("vCausT_S", M.caus, sDerivative)
	vCausTır_S := makeState("vCausTır_S", M.caus, sDerivative)
	vRecip_S := makeState("vRecip_S", M.recip, sDerivative)
	vImplicitRecipRoot_S := makeState("vImplicitRecipRoot_S", M.verb, sPosRoot)
	vReflex_S := makeState("vReflex_S", M.reflex, sDerivative)
	vImplicitReflexRoot_S := makeState("vImplicitReflexRoot_S", M.verb, sPosRoot)

	vAor_S := makeState("vAor_S", M.aor)
	vAorNeg_S := makeState("vAorNeg_S", M.aor)
	vAorNegEmpty_S := makeState("vAorNegEmpty_S", M.aor)
	vAorPartNeg_S := makeState("vAorPartNeg_S", M.aorPart, sDerivative)
	vAorPart_S := makeState("vAorPart_S", M.aorPart, sDerivative)

	vAble_S := makeState("vAble_S", M.able, sDerivative)
	vAbleNeg_S := makeState("vAbleNeg_S", M.able, sDerivative)
	vAbleNegDerivRoot_S := makeState("vAbleNegDerivRoot_S", M.verb, sPosRoot)
	vPass_S := makeState("vPass_S", M.pass, sDerivative)

	vOpt_S := makeState("vOpt_S", M.opt)
	vDesr_S := makeState("vDesr_S", M.desr)
	vNeces_S := makeState("vNeces_S", M.neces)

	vInf1_S := makeState("vInf1_S", M.inf1, sDerivative)
	vInf2_S := makeState("vInf2_S", M.inf2, sDerivative)
	vInf3_S := makeState("vInf3_S", M.inf3, sDerivative)
	vAgt_S := makeState("vAgt_S", M.agt, sDerivative)
	vActOf_S := makeState("vActOf_S", M.actOf, sDerivative)
	vPastPart_S := makeState("vPastPart_S", M.pastPart, sDerivative)
	vFutPart_S := makeState("vFutPart_S", M.futPart, sDerivative)
	vPresPart_S := makeState("vPresPart_S", M.presPart, sDerivative)
	vNarrPart_S := makeState("vNarrPart_S", M.narrPart, sDerivative)
	vFeelLike_S := makeState("vFeelLike_S", M.feelLike, sDerivative)
	vNotState_S := makeState("vNotState_S", M.notState, sDerivative)

	vEverSince_S := makeState("vEverSince_S", M.everSince, sDerivative)
	vRepeat_S := makeState("vRepeat_S", M.repeat, sDerivative)
	vAlmost_S := makeState("vAlmost_S", M.almost, sDerivative)
	vHastily_S := makeState("vHastily_S", M.hastily, sDerivative)
	vStay_S := makeState("vStay_S", M.stay, sDerivative)
	vStart_S := makeState("vStart_S", M.start, sDerivative)

	vWhile_S := makeState("vWhile_S", M.while_, sDerivative)
	vWhen_S := makeState("vWhen_S", M.when, sDerivative)
	vAsIf_S := makeState("vAsIf_S", M.asIf, sDerivative)
	vSinceDoingSo_S := makeState("vSinceDoingSo_S", M.sinceDoingSo, sDerivative)
	vAsLongAs_S := makeState("vAsLongAs_S", M.asLongAs, sDerivative)
	vByDoingSo_S := makeState("vByDoingSo_S", M.byDoingSo, sDerivative)
	vAdamantly_S := makeState("vAdamantly_S", M.adamantly, sDerivative)
	vAfterDoing_S := makeState("vAfterDoing_S", M.afterDoingSo, sDerivative)
	vWithoutHavingDoneSo_S := makeState("vWithoutHavingDoneSo_S", M.withoutHavingDoneSo, sDerivative)
	vWithoutBeingAbleToHaveDoneSo_S := makeState(
		"vWithoutBeingAbleToHaveDoneSo_S",
		M.withoutBeingAbleToHaveDoneSo,
		sDerivative,
	)

	// --- Standalone Question (mı/mi/mu/mü) states ---
	questionRoot_S := makeState("questionRoot_S", M.ques, sPosRoot)
	qPresent_S := makeState("qPresent_S", M.pres)
	qPast_S := makeState("qPast_S", M.past)
	qNarr_S := makeState("qNarr_S", M.narr)
	qA1sg_ST := makeState("qA1sg_ST", M.a1sg, sTerminal)
	qA2sg_ST := makeState("qA2sg_ST", M.a2sg, sTerminal)
	qA3sg_ST := makeState("qA3sg_ST", M.a3sg, sTerminal)
	qA1pl_ST := makeState("qA1pl_ST", M.a1pl, sTerminal)
	qA2pl_ST := makeState("qA2pl_ST", M.a2pl, sTerminal)
	qA3pl_ST := makeState("qA3pl_ST", M.a3pl, sTerminal)
	qCopBeforeA3pl_S := makeState("qCopBeforeA3pl_S", M.cop)
	qCop_ST := makeState("qCop_ST", M.cop, sTerminal)

	// --- Standalone Verb imek (idi, imiş, ise) states ---
	imekRoot_S := makeState("imekRoot_S", M.verb, sPosRoot)
	imekPast_S := makeState("imekPast_S", M.past)
	imekNarr_S := makeState("imekNarr_S", M.narr)
	imekCond_S := makeState("imekCond_S", M.cond)
	imekA1sg_ST := makeState("imekA1sg_ST", M.a1sg, sTerminal)
	imekA2sg_ST := makeState("imekA2sg_ST", M.a2sg, sTerminal)
	imekA3sg_ST := makeState("imekA3sg_ST", M.a3sg, sTerminal)
	imekA1pl_ST := makeState("imekA1pl_ST", M.a1pl, sTerminal)
	imekA2pl_ST := makeState("imekA2pl_ST", M.a2pl, sTerminal)
	imekA3pl_ST := makeState("imekA3pl_ST", M.a3pl, sTerminal)
	imekCop_ST := makeState("imekCop_ST", M.cop, sTerminal)

	// =========================================================================
	// 1. Noun States
	// =========================================================================

	isCompoundOrInsertN := hasAttr(raCompoundP3sg | raNounConsInsertN)
	notCompoundOrInsertN := notHaveAttr(raCompoundP3sg | raNounConsInsertN)
	rootIsAbbrv := secondaryPosIs("Abbreviation")
	notFamily := notHaveAttr(raFamilyMember)
	possessionCond := and(notFamily, not(rootIsAbbrv))

	addEmpty(noun_S, a3sg_S, notHaveAttr(raImplicitPlural))
	addTransition(noun_S, a3pl_S, "lAr", and(notHaveAttr(raImplicitPlural), notHaveAttr(raCompoundP3sg)))
	addEmpty(noun_S, a3pl_S, hasAttr(raImplicitPlural))

	// Compound P3sg roots (e.g. "zeytinyağ" from "zeytinyağı")
	addEmpty(nounCompoundRoot_S, a3sgCompound_S, hasAttr(raCompoundP3sgRoot))
	addEmpty(a3sgCompound_S, pnonCompound_S)
	addTransition(a3sgCompound_S, p3pl_S, "lArI")

	containsNess := containsMorpheme(M.ness)
	notWithOrWithout := not(containsMorpheme(M.with, M.without))

	addEmpty(pnonCompound_S, nom_S)
	addTransition(nom_S, become_S, "lAş")
	addTransition(nom_S, acquire_S, "lAn")
	addTransition(nom_S, with_S, "lI", notWithOrWithout)
	addTransition(nom_S, without_S, "sIz", notWithOrWithout)
	addTransition(nom_S, ness_S, "lI~k", not(containsNess))
	addTransition(nom_S, ness_S, "lI!ğ", not(containsNess))
	addTransition(nom_S, agt_S, ">cI", not(containsMorpheme(M.agt)))
	addTransition(nom_S, dash_S, ">dAş", not(containsMorpheme(M.dash)))
	addTransition(nom_S, justLike_S, "+msI", not(containsMorpheme(M.justLike)))
	addTransition(nom_S, dim_S, ">cI~k", and(hasNoSurface, not(containsMorpheme(M.dim))))
	addTransition(nom_S, dim_S, ">cI!ğ", and(hasNoSurface, not(containsMorpheme(M.dim))))
	addTransition(nom_S, dim_S, "cAğIz", hasNoSurface)

	addTransition(nounCompoundRoot_S, a3plCompound_S, "lAr", hasAttr(raCompoundP3sgRoot))
	addTransition(nounCompoundRoot_S, a3plCompound2_S, "lArI", hasAttr(raCompoundP3sgRoot))
	addTransition(a3plCompound_S, p3sg_S, "I")
	addTransition(a3plCompound_S, p2sg_S, "In")
	addTransition(a3plCompound_S, p1sg_S, "Im")
	addTransition(a3plCompound_S, p1pl_S, "ImIz")
	addTransition(a3plCompound_S, p2pl_S, "InIz")
	addTransition(a3plCompound_S, p3pl_S, "I")
	addEmpty(a3plCompound2_S, pnonCompound2_S)
	addEmpty(pnonCompound2_S, nom_ST)

	// Regular A3sg -> Possessive
	addEmpty(a3sg_S, pnon_S, notFamily)
	addTransition(a3sg_S, p1sg_S, "Im", possessionCond)
	addTransition(a3sg_S, p2sg_S, "In", and(possessionCond, not(prevGroupContainsMorpheme(M.justLike))))
	addTransition(a3sg_S, p3sg_S, "+sI", and(possessionCond, notHaveAttr(raCompoundP3sg)))
	addEmpty(a3sg_S, p3sg_S, hasAttr(raCompoundP3sg))
	addTransition(a3sg_S, p1pl_S, "ImIz", possessionCond)
	addTransition(a3sg_S, p2pl_S, "InIz", and(possessionCond, not(prevGroupContainsMorpheme(M.justLike))))
	addTransition(a3sg_S, p3pl_S, "lArI", and(possessionCond, notHaveAttr(raCompoundP3sg)))

	// Regular A3pl -> Possessive
	addEmpty(a3pl_S, pnon_S, notFamily)
	addTransition(a3pl_S, p1sg_S, "Im", possessionCond)
	addTransition(a3pl_S, p2sg_S, "In", possessionCond)
	addEmpty(a3pl_S, p1sg_S, hasAttr(raImplicitP1sg))
	addEmpty(a3pl_S, p2sg_S, hasAttr(raImplicitP2sg))
	addTransition(a3pl_S, p3sg_S, "I", possessionCond)
	addTransition(a3pl_S, p1pl_S, "ImIz", possessionCond)
	addTransition(a3pl_S, p2pl_S, "InIz", possessionCond)
	addTransition(a3pl_S, p3pl_S, "I", possessionCond)

	// Special "su" / "akarsu" roots
	addEmpty(nounSuRoot_S, a3sgSu_S)
	addTransition(nounSuRoot_S, a3pl_S, "lar")
	addEmpty(a3sgSu_S, pnon_S)
	addTransition(a3sgSu_S, p1sg_S, "yum")
	addTransition(a3sgSu_S, p2sg_S, "yun")
	addTransition(a3sgSu_S, p3sg_S, "yu")
	addTransition(a3sgSu_S, p1pl_S, "yumuz")
	addTransition(a3sgSu_S, p2pl_S, "yunuz")
	addTransition(a3sgSu_S, p3pl_S, "lArI")

	// Possessive -> Case
	equCond := or(
		prevMorphemeIs(M.a3pl),
		not(containsMorpheme(M.adj, M.futPart, M.presPart, M.narrPart, M.pastPart)),
		containsMorphemeSeq(M.able, M.verb, M.pastPart),
	)
	equWithPastPart := or(equCond, containsMorpheme(M.pastPart))

	addEmpty(pnon_S, nom_ST, notFamily)
	addTransition(pnon_S, dat_ST, "+yA", notCompoundOrInsertN)
	addTransition(pnon_S, abl_ST, ">dAn", notCompoundOrInsertN)
	// Prevent "daha + da" -> "*dahada" (clitic misspelling); "dahası" is lexicalized and "dahaki" uses nom_ST->rel_S.
	addTransition(pnon_S, loc_ST, ">dA", and(notCompoundOrInsertN, rootKeyIsNot("daha_Noun_Time")))
	addTransition(pnon_S, acc_ST, "+yI", notCompoundOrInsertN)
	addTransition(pnon_S, gen_ST, "+nIn", prevStateIsNot(a3sgSu_S))
	addTransition(pnon_S, gen_ST, "yIn", prevStateIs(a3sgSu_S))
	addTransition(pnon_S, equ_ST, ">cA", and(notCompoundOrInsertN, equCond))
	addTransition(pnon_S, ins_ST, "+ylA")

	addTransition(pnon_S, dat_ST, "+nA", isCompoundOrInsertN)
	addTransition(pnon_S, abl_ST, "+ndAn", isCompoundOrInsertN)
	addTransition(pnon_S, loc_ST, "+ndA", isCompoundOrInsertN)
	addTransition(pnon_S, equ_ST, "+ncA", and(isCompoundOrInsertN, equCond))
	addTransition(pnon_S, acc_ST, "+nI", isCompoundOrInsertN)
	addEmpty(pnon_S, dat_ST, hasAttr(raImplicitDative))

	for _, ps := range []*morphemeState{p1sg_S, p2sg_S, p1pl_S, p2pl_S} {
		addEmpty(ps, nom_ST)
		addTransition(ps, dat_ST, "A")
		addTransition(ps, loc_ST, "dA")
		addTransition(ps, abl_ST, "dAn")
		addTransition(ps, ins_ST, "lA")
		addTransition(ps, gen_ST, "In")
		addTransition(ps, equ_ST, "cA", equWithPastPart)
		addTransition(ps, acc_ST, "I")
	}

	addEmpty(p3sg_S, nom_ST)
	addTransition(p3sg_S, dat_ST, "nA")
	addTransition(p3sg_S, loc_ST, "ndA")
	addTransition(p3sg_S, abl_ST, "ndAn")
	addTransition(p3sg_S, ins_ST, "ylA")
	addTransition(p3sg_S, gen_ST, "nIn")
	addTransition(p3sg_S, equ_ST, "ncA", equWithPastPart)
	addTransition(p3sg_S, acc_ST, "nI")

	addEmpty(p3pl_S, nom_ST)
	addTransition(p3pl_S, dat_ST, "nA")
	addTransition(p3pl_S, loc_ST, "ndA")
	addTransition(p3pl_S, abl_ST, "ndAn")
	addTransition(p3pl_S, ins_ST, "ylA")
	addTransition(p3pl_S, gen_ST, "nIn")
	addTransition(p3pl_S, equ_ST, "+ncA")
	addTransition(p3pl_S, acc_ST, "nI")

	// Derivations from nom_ST
	notAbbrv := not(rootIsAbbrv)
	emptyAdjNounSeq := containsMorphemeSeq(M.adj, M.zero, M.noun, M.a3sg, M.pnon, M.nom)

	addTransition(nom_ST, dim_S, ">cI~k", and(hasNoSurface, notAbbrv))
	addTransition(nom_ST, dim_S, ">cI!ğ", and(hasNoSurface, notAbbrv))
	addTransition(nom_ST, dim_S, "cAğIz", and(hasNoSurface, notAbbrv))
	addEmpty(dim_S, noun_S)

	nessCond := and(currentGroupEmpty, not(containsNess), not(emptyAdjNounSeq), notAbbrv)
	addTransition(nom_ST, ness_S, "lI~k", nessCond)
	addTransition(nom_ST, ness_S, "lI!ğ", nessCond)
	addEmpty(ness_S, noun_S)

	addTransition(nom_ST, agt_S, ">cI", and(currentGroupEmpty, not(containsMorpheme(M.adj, M.agt))))
	addEmpty(agt_S, noun_S)

	verbDerivNoun := containsMorpheme(M.inf1, M.inf2, M.inf3, M.pastPart, M.futPart)
	addTransition(
		nom_ST,
		dash_S,
		">dAş",
		and(
			currentGroupEmpty,
			not(containsMorpheme(M.dash, M.adj, M.with, M.without, M.ness, M.agt, M.dim)),
			not(verbDerivNoun),
			notAbbrv,
		),
	)
	addEmpty(dash_S, noun_S)

	noun2VerbCond := and(hasTail, not(and(currentGroupEmpty, lastDerivIs(adjZeroDeriv_S))))
	for _, cs := range []*morphemeState{nom_ST, dat_ST, abl_ST, loc_ST, ins_ST, gen_ST} {
		addEmpty(cs, nounZeroDeriv_S, noun2VerbCond)
	}
	addEmpty(nounZeroDeriv_S, nVerb_S)

	addTransition(nom_ST, with_S, "lI", and(currentGroupEmpty, notWithOrWithout))
	addTransition(nom_ST, without_S, "sIz", and(currentGroupEmpty, not(containsMorpheme(M.with, M.without, M.inf1))))

	justLikeCond := and(
		currentGroupEmpty,
		not(containsMorpheme(M.justLike, M.futPart, M.pastPart, M.presPart, M.adj)),
		notAbbrv,
	)
	addTransition(nom_ST, justLike_S, "+msI", justLikeCond)
	addTransition(nom_ST, justLike_S, "ImsI", and(notHavePhon(paLastVowel), justLikeCond))

	addTransition(
		nom_ST,
		related_S,
		"sAl",
		and(currentGroupEmpty, not(containsMorpheme(M.with, M.without, M.related)), notAbbrv),
	)

	addEmpty(with_S, adjectiveRoot_ST)
	addEmpty(without_S, adjectiveRoot_ST)
	addEmpty(related_S, adjectiveRoot_ST)
	addEmpty(justLike_S, adjectiveRoot_ST)

	// -ki (Rel)
	notRelRepetition := not(hasTailSeq(M.rel, M.adj, M.zero, M.noun, M.a3sg, M.pnon, M.loc))
	addTransition(loc_ST, rel_S, "ki", notRelRepetition)
	addEmpty(rel_S, adjectiveRoot_ST)
	// Allow pronominal -ki after locative ("evdekini", "elindekine", "evdekinde", "evdekinden").
	addEmpty(rel_S, pronAfterRel_S, hasTail)

	isTime := and(currentGroupEmpty, secondaryPosIs("Time"))
	isDunGunBugun := rootKeyIn("dün_Noun_Time", "gün_Noun_Time", "bugün_Noun_Time")
	isDirKi := rootKeyIn("ileri_Noun", "geri_Noun", "öte_Noun", "beri_Noun")
	addTransition(nom_ST, rel_S, "ki", and(isTime, not(isDunGunBugun)))
	addTransition(nom_ST, rel_S, "ki", and(currentGroupEmpty, isDirKi))
	addTransition(nom_ST, rel_S, "kü", and(isTime, isDunGunBugun))

	addTransition(gen_ST, relToPron_S, "ki")
	addEmpty(relToPron_S, pronAfterRel_S)

	becomeAcquireCond := and(currentGroupEmpty, not(containsMorpheme(M.adj)), not(verbDerivNoun), notAbbrv)
	addTransition(nom_ST, become_S, "lAş", becomeAcquireCond)
	addEmpty(become_S, verbRoot_S)
	addTransition(nom_ST, acquire_S, "lAn", becomeAcquireCond)
	addEmpty(acquire_S, verbRoot_S)

	// Inf1 (-mAk) noun path
	addEmpty(nounInf1Root_S, a3sgInf1_S)
	addEmpty(a3sgInf1_S, pnonInf1_S)
	addEmpty(pnonInf1_S, nom_ST)
	addTransition(pnonInf1_S, abl_ST, "tAn")
	addTransition(pnonInf1_S, loc_ST, "tA")
	addTransition(pnonInf1_S, ins_ST, "lA")

	// ActOf (-mAcA) noun path
	addEmpty(nounActOfRoot_S, a3sgActOf_S)
	addTransition(nounActOfRoot_S, a3plActOf_S, "lar")
	addEmpty(a3sgActOf_S, pnonActOf_S)
	addEmpty(a3plActOf_S, pnonActOf_S)
	addEmpty(pnonActOf_S, nom_ST)

	// Proper nouns, abbreviations, and NoSuffix nouns
	addEmpty(nounProper_S, a3sg_S)
	addTransition(nounProper_S, a3pl_S, "lAr")
	addEmpty(nounAbbrv_S, a3sg_S)
	addTransition(nounAbbrv_S, a3pl_S, "lAr")
	addEmpty(nounNoSuffix_S, nounA3sgNoSuffix_S)
	addEmpty(nounA3sgNoSuffix_S, nounPnonNoSuffix_S)
	addEmpty(nounPnonNoSuffix_S, nounNomNoSuffix_ST)

	// =========================================================================
	// 2. Adjective States
	// =========================================================================

	isNominalizableAdj := func(p *pathNode) bool {
		return !(p.stemTransition.item.primaryPos == "Adj" && nonNominalizableAdjLemmas[p.stemTransition.item.root])
	}

	addEmpty(adjectiveRoot_ST, adjZeroDeriv_S, hasTail)
	// After -ki (rel_S), nominal case/plural inflections go through pronAfterRel_S (evdekini, evdekiler),
	// preventing ungrammatical non-buffer forms like *evdekiyi, *evdekide.
	addEmpty(adjZeroDeriv_S, noun_S, and(isNominalizableAdj, not(hasTailSeq(M.rel, M.adj, M.zero))))
	addEmpty(adjZeroDeriv_S, nVerb_S)

	addTransition(adjectiveRoot_ST, aLy_S, ">cA")
	addEmpty(aLy_S, advRoot_ST)

	notAdjDerivRepeat := not(containsMorpheme(M.asIf, M.ly, M.agt, M.with, M.justLike))
	addTransition(adjectiveRoot_ST, aAsIf_S, ">cA", notAdjDerivRepeat)
	addEmpty(aAsIf_S, adjectiveRoot_ST)

	addTransition(adjectiveRoot_ST, aAgt_S, ">cI", notAdjDerivRepeat)
	addEmpty(aAgt_S, noun_S)

	adjJustLikeCond := and(currentGroupEmpty, not(containsMorpheme(M.justLike)))
	addTransition(adjectiveRoot_ST, justLike_S, "+msI", adjJustLikeCond)
	addTransition(adjectiveRoot_ST, justLike_S, "ImsI", and(notHavePhon(paLastVowel), adjJustLikeCond))

	addTransition(adjectiveRoot_ST, become_S, "lAş", currentGroupEmpty)
	addTransition(adjectiveRoot_ST, acquire_S, "lAn", currentGroupEmpty)

	afterFutOrPastPart := prevMorphemeIn(M.futPart, M.pastPart)
	addEmpty(adjAfterVerb_S, aPnon_ST, afterFutOrPastPart)
	addTransition(adjAfterVerb_S, aP1sg_ST, "Im", afterFutOrPastPart)
	addTransition(adjAfterVerb_S, aP2sg_ST, "In", afterFutOrPastPart)
	addTransition(adjAfterVerb_S, aP3sg_ST, "I", afterFutOrPastPart)
	addTransition(adjAfterVerb_S, aP1pl_ST, "ImIz", afterFutOrPastPart)
	addTransition(adjAfterVerb_S, aP2pl_ST, "InIz", afterFutOrPastPart)
	addTransition(adjAfterVerb_S, aP3pl_ST, "lArI", afterFutOrPastPart)

	addTransition(adjectiveRoot_ST, ness_S, "lI~k")
	addTransition(adjectiveRoot_ST, ness_S, "lI!ğ")

	addTransition(adjAfterVerb_ST, ness_S, "lI~k", prevMorphemeIs(M.aorPart))
	addTransition(adjAfterVerb_ST, ness_S, "lI!ğ", prevMorphemeIs(M.aorPart))

	// bilmezden, görmezden
	addEmpty(adjAfterVerb_ST, aorPartNegZero_S, prevStateIs(vAorPartNeg_S))
	addEmpty(aorPartNegZero_S, aorPartNegNoun_S)
	addEmpty(aorPartNegNoun_S, aorPartNegA3sg_S)
	addEmpty(aorPartNegA3sg_S, aorPartNegPnon_S)
	addTransition(aorPartNegPnon_S, aorPartNegAbl_ST, "dAn")

	// =========================================================================
	// 3. Numeral States
	// =========================================================================

	addTransition(numeralRoot_ST, ness_S, "lI~k")
	addTransition(numeralRoot_ST, ness_S, "lI!ğ")
	addEmpty(numeralRoot_ST, numZeroDeriv_S, hasTail)
	addEmpty(numZeroDeriv_S, noun_S)
	addEmpty(numZeroDeriv_S, nVerb_S)
	addTransition(numeralRoot_ST, justLike_S, "+msI", adjJustLikeCond)
	addTransition(numeralRoot_ST, justLike_S, "ImsI", and(notHavePhon(paLastVowel), adjJustLikeCond))

	// =========================================================================
	// 4. Nominal Predicate (Ek-Fiil) States
	// =========================================================================

	addEmpty(nVerb_S, nPresent_S)
	addTransition(nVerb_S, nPast_S, "+y>dI")
	addTransition(nVerb_S, nNarr_S, "+ymIş")
	addTransition(nVerb_S, nCond_S, "+ysA")
	addTransition(nVerb_S, vWhile_S, "+yken", not(hasTailSeq(M.narrPart, M.adj, M.zero, M.verb)))

	isDegil := rootKeyIs("değil_Verb")
	addEmpty(nVerbDegil_S, nNeg_S, isDegil)
	copyOutgoing(nNeg_S, nVerb_S)

	nVerbBlockedDeriv := or(
		containsMorpheme(M.inf1),
		and(
			containsMorpheme(M.inf2, M.inf3, M.pastPart, M.futPart, M.feelLike),
			not(prevGroupContains(loc_ST, abl_ST, ins_ST)),
		),
	)
	allowA1sg := and(notFamily, not(containsMorphemeSeq(M.p1sg, M.nom)), not(nVerbBlockedDeriv))
	allowA2sg := and(notFamily, not(containsMorphemeSeq(M.p2sg, M.nom)), not(nVerbBlockedDeriv))
	allowA3pl := and(
		notFamily,
		not(prevGroupContains(a3pl_S)),
		not(containsMorphemeSeq(M.p3pl, M.nom)),
		not(nVerbBlockedDeriv),
	)
	allowA2pl := and(notFamily, not(containsMorphemeSeq(M.p2pl, M.nom)), not(nVerbBlockedDeriv))
	allowA1pl := and(
		notFamily,
		not(containsMorphemeSeq(M.p1sg, M.nom)),
		not(containsMorphemeSeq(M.p1pl, M.nom)),
		not(nVerbBlockedDeriv),
	)

	addTransition(nPresent_S, nA1sg_ST, "+yIm", allowA1sg)
	addTransition(nPresent_S, nA2sg_ST, "sIn", allowA2sg)
	addEmpty(nPresent_S, nA3sg_S)
	addEmpty(nPresent_S, nA3sg_ST, isDegil)
	addTransition(
		nPresent_S,
		nA3pl_ST,
		"lAr",
		and(notHaveAttr(raCompoundP3sg), not(prevGroupContainsMorpheme(M.inf1)), allowA3pl),
	)
	addTransition(nPresent_S, nA1pl_ST, "+yIz", allowA1pl)
	addTransition(nPresent_S, nA2pl_ST, "sInIz", allowA2pl)

	addTransition(nPast_S, nA1sg_ST, "m", allowA1sg)
	addTransition(nPast_S, nA2sg_ST, "n", allowA2sg)
	addTransition(nPast_S, nA1pl_ST, "k", allowA1pl)
	addTransition(nPast_S, nA2pl_ST, "InIz", allowA2pl)
	addTransition(nPast_S, nA3pl_ST, "lAr", and(notHaveAttr(raCompoundP3sg), allowA3pl))
	addEmpty(nPast_S, nA3sg_ST)

	addTransition(nNarr_S, nA1sg_ST, "Im", allowA1sg)
	addTransition(nNarr_S, nA2sg_ST, "sIn", allowA2sg)
	addTransition(nNarr_S, nA1pl_ST, "Iz", allowA1pl)
	addTransition(nNarr_S, nA2pl_ST, "sInIz", allowA2pl)
	addTransition(nNarr_S, nA3pl_ST, "lAr", and(notHaveAttr(raCompoundP3sg), allowA3pl))
	addEmpty(nNarr_S, nA3sg_ST)
	addTransition(nNarr_S, nCond_S, "sA")

	addTransition(nCond_S, nA1sg_ST, "m", allowA1sg)
	addTransition(nCond_S, nA2sg_ST, "n", allowA2sg)
	addTransition(nCond_S, nA1pl_ST, "k", allowA1pl)
	addTransition(nCond_S, nA2pl_ST, "nIz", allowA2pl)
	addEmpty(nCond_S, nA3sg_ST)
	addTransition(nCond_S, nA3pl_ST, "lAr")

	nRejectNoCopula := not(currentGroupContains(nPast_S, nCond_S, nCopBeforeA3pl_S))
	addTransition(nA1sg_ST, nCop_ST, "dIr", nRejectNoCopula)
	addTransition(nA2sg_ST, nCop_ST, "dIr", nRejectNoCopula)
	addTransition(nA1pl_ST, nCop_ST, "dIr", nRejectNoCopula)
	addTransition(nA2pl_ST, nCop_ST, "dIr", nRejectNoCopula)
	addTransition(nA3sg_S, nCop_ST, ">dIr", nRejectNoCopula)
	addTransition(nA3pl_ST, nCop_ST, "dIr", nRejectNoCopula)

	afterNarr := prevMorphemeIs(M.narr)
	for _, s := range []*morphemeState{nA3sg_ST, nA1sg_ST, nA2sg_ST, nA1pl_ST, nA2pl_ST, nA3pl_ST} {
		addTransition(s, vAsIf_S, ">cAsInA", afterNarr)
	}

	nA3plAfterCase := and(prevStateIs(nPresent_S), prevGroupContains(loc_ST, abl_ST, dat_ST, ins_ST))
	addTransition(nA3pl_ST, vPastAfterTense_ST, "dI", nA3plAfterCase)
	addTransition(nA3pl_ST, vNarrAfterTense_ST, "mIş", nA3plAfterCase)
	addTransition(nA3pl_ST, vCond_ST, "sA", nA3plAfterCase)
	addTransition(nA3pl_ST, vWhile_S, "ken", nA3plAfterCase)

	addTransition(nPresent_S, nCopBeforeA3pl_S, ">dIr")
	addTransition(nCopBeforeA3pl_S, nA3pl_ST, "lAr", and(notHaveAttr(raCompoundP3sg), allowA3pl))

	// =========================================================================
	// 5. Pronoun States
	// =========================================================================

	const BEN = "ben_Pron_PersonalPron"
	const SEN = "sen_Pron_PersonalPron"
	const O_PERS = "o_Pron_PersonalPron"
	const BIZ = "biz_Pron_PersonalPron"
	const SIZ = "siz_Pron_PersonalPron"
	const FALAN = "falan_Pron_PersonalPron"
	const FALANCA = "falanca_Pron_PersonalPron"

	const BU = "bu_Pron_DemonstrativePron"
	const SU = "şu_Pron_DemonstrativePron"
	const O_DEMONS = "o_Pron_DemonstrativePron"

	const BIRBIRI = "birbiri_Pron_QuantitivePron"
	const BIRI = "biri_Pron_QuantitivePron"
	const BAZI = "bazı_Pron_QuantitivePron"
	const BIRCOGU = "birçoğu_Pron_QuantitivePron"
	const BIRKACI = "birkaçı_Pron_QuantitivePron"
	const BERIKI = "beriki_Pron_QuantitivePron"
	const COGU = "çoğu_Pron_QuantitivePron"
	const CUMLESI = "cümlesi_Pron_QuantitivePron"
	const HEP = "hep_Pron_QuantitivePron"
	const HERBIRI = "herbiri_Pron_QuantitivePron"
	const HERKES = "herkes_Pron_QuantitivePron"
	const HICBIRI = "hiçbiri_Pron_QuantitivePron"
	const HEPSI = "hepsi_Pron_QuantitivePron"
	const KIMI = "kimi_Pron_QuantitivePron"
	const KIMSE = "kimse_Pron_QuantitivePron"
	const OBURKU = "öbürkü_Pron_QuantitivePron"
	const OBURU = "öbürü_Pron_QuantitivePron"
	const TUMU = "tümü_Pron_QuantitivePron"
	const TOPU = "topu_Pron_QuantitivePron"
	const UMUM_QUANT = "umum_Pron_QuantitivePron"
	const UMUM_PLAIN = "umum_Pron"

	const NE = "ne_Pron_QuestionPron"
	const NERE = "nere_Pron_QuestionPron"
	const KIM = "kim_Pron_QuestionPron"
	const HANGI = "hangi_Pron_QuestionPron"
	const KENDI = "kendi_Pron_ReflexivePron"

	addEmpty(pronPers_S, pA1sg_S, rootKeyIs(BEN))
	addEmpty(pronPers_S, pA2sg_S, rootKeyIs(SEN))
	addEmpty(pronPers_S, pA3sg_S, rootKeyIn(O_PERS, FALAN, FALANCA))
	addTransition(pronPers_S, pA3pl_S, "nlAr", rootKeyIs(O_PERS))
	addTransition(pronPers_S, pA3pl_S, "lAr", rootKeyIn(FALAN, FALANCA))
	addEmpty(pronPers_S, pA1pl_S, rootKeyIs(BIZ))
	addTransition(pronPers_S, pA1pl_S, "lAr", rootKeyIs(BIZ))
	addEmpty(pronPers_S, pA2pl_S, rootKeyIs(SIZ))
	addTransition(pronPers_S, pA2pl_S, "lAr", rootKeyIs(SIZ))

	// Modified ben/sen -> ban-a, san-a
	addEmpty(pronPers_Mod_S, pA1sgMod_S, rootKeyIs(BEN))
	addEmpty(pronPers_Mod_S, pA2sgMod_S, rootKeyIs(SEN))
	addEmpty(pA1sgMod_S, pPnonMod_S)
	addEmpty(pA2sgMod_S, pPnonMod_S)
	addTransition(pPnonMod_S, pDat_ST, "A")

	for _, s := range []*morphemeState{pA1sg_S, pA2sg_S, pA3sg_S, pA1pl_S, pA2pl_S, pA3pl_S} {
		addEmpty(s, pPnon_S)
	}

	// Noun -> Rel -> Pron (masanınki, evdekini, evdekiler)
	addEmpty(pronAfterRel_S, pA3sgRel_S)
	addTransition(pronAfterRel_S, pA3plRel_S, "lAr")
	addEmpty(pA3sgRel_S, pPnonRel_S)
	addEmpty(pA3plRel_S, pPnonRel_S)
	addEmpty(pPnonRel_S, pNom_ST)
	addTransition(pPnonRel_S, pDat_ST, "+nA")
	addTransition(pPnonRel_S, pAcc_ST, "+nI")
	addTransition(pPnonRel_S, pAbl_ST, "+ndAn")
	addTransition(pPnonRel_S, pLoc_ST, "+ndA")
	addTransition(pPnonRel_S, pIns_ST, "+ylA")
	addTransition(pPnonRel_S, pGen_ST, "+nIn")

	// Demonstrative pronouns (bu, şu, o)
	addEmpty(pronDemons_S, pA3sg_S)
	addTransition(pronDemons_S, pA3pl_S, "nlAr")

	// Quantitive pronouns
	addEmpty(
		pronQuant_S,
		pQuantA3sg_S,
		rootKeyNotIn(HERKES, UMUM_QUANT, UMUM_PLAIN, HEPSI, CUMLESI, HEP, TUMU, BIRKACI, TOPU),
	)
	addTransition(
		pronQuant_S,
		pQuantA3pl_S,
		"lAr",
		rootKeyNotIn(HEP, HEPSI, BIRKACI, UMUM_QUANT, UMUM_PLAIN, CUMLESI, COGU, BIRCOGU, HERBIRI, TUMU, HICBIRI, TOPU, OBURU),
	)
	addTransition(pronQuant_S, pQuantA1pl_S, "lAr", rootKeyIs(BAZI))
	addTransition(pronQuant_S, pQuantA2pl_S, "lAr", rootKeyIs(BAZI))
	addEmpty(
		pronQuant_S,
		pQuantA3pl_S,
		rootKeyIn(HERKES, UMUM_QUANT, UMUM_PLAIN, BIRKACI, HEPSI, CUMLESI, COGU, BIRCOGU, TUMU, TOPU),
	)
	addEmpty(pronQuant_S, a3sg_S, rootKeyIs(KIMSE))
	addTransition(pronQuant_S, a3pl_S, "lAr", rootKeyIs(KIMSE))

	quantPl12Roots := rootKeyIn(BIRI, BAZI, BIRBIRI, BIRKACI, HERBIRI, HEP, KIMI, COGU, BIRCOGU, TUMU, TOPU, HICBIRI)
	addEmpty(pronQuant_S, pQuantA1pl_S, quantPl12Roots)
	addEmpty(pronQuant_S, pQuantA2pl_S, quantPl12Roots)

	addEmpty(pronQuantModified_S, pQuantModA3pl_S)
	addTransition(pQuantModA3pl_S, pP3pl_S, "lArI")

	notModPron := notHavePhon(paModifiedPronoun)
	plainVowelPron := and(secondaryPosIs("None"), hasPhon(paLastVowel))
	addEmpty(
		pQuantA3sg_S,
		pP3sg_S,
		and(
			or(rootKeyIn(BIRI, BIRBIRI, KIMI, HERBIRI, HICBIRI, OBURU, OBURKU, BERIKI), plainVowelPron),
			notModPron,
		),
	)
	addTransition(
		pQuantA3sg_S,
		pP3sg_S,
		"sI",
		and(rootKeyIn(BIRI, BAZI, KIMI, BIRBIRI, HERBIRI, HICBIRI, OBURKU), notModPron),
	)

	addTransition(pQuantA3pl_S, pP3pl_S, "I", rootKeyIn(BIRI, BAZI, BIRBIRI, KIMI, OBURKU, BERIKI))
	addEmpty(pQuantA3pl_S, pP3pl_S, rootKeyIn(HEPSI, BIRKACI, CUMLESI, COGU, TUMU, TOPU, BIRCOGU))
	addEmpty(pQuantA3pl_S, pPnon_S, rootKeyIn(HERKES, UMUM_QUANT, UMUM_PLAIN, OBURKU, BERIKI))

	addTransition(pQuantA1pl_S, pP1pl_S, "ImIz")
	addTransition(pQuantA2pl_S, pP2pl_S, "InIz")

	// Question pronouns (kim, ne, nere, hangi)
	addEmpty(pronQues_S, pQuesA3sg_S)
	addTransition(pronQues_S, pQuesA3pl_S, "lAr")

	addEmpty(pQuesA3sg_S, pPnon_S)
	addTransition(pQuesA3sg_S, pP3sg_S, "+sI")
	addTransition(pQuesA3sg_S, pP1sg_S, "Im", rootKeyIsNot(NE))
	addTransition(pQuesA3sg_S, pP1sg_S, "yIm", rootKeyIs(NE))
	addTransition(pQuesA3sg_S, pP2sg_S, "In", rootKeyIsNot(NE))
	addTransition(pQuesA3sg_S, pP2sg_S, "yIn", rootKeyIs(NE))
	addTransition(pQuesA3sg_S, pP1pl_S, "ImIz", rootKeyIsNot(NE))
	addTransition(pQuesA3sg_S, pP1pl_S, "yImIz", rootKeyIs(NE))
	addTransition(pQuesA3sg_S, pP2pl_S, "InIz", rootKeyIsNot(NE))
	addTransition(pQuesA3sg_S, pP2pl_S, "yInIz", rootKeyIs(NE))

	addEmpty(pQuesA3pl_S, pPnon_S)
	addTransition(pQuesA3pl_S, pP3sg_S, "I")
	addTransition(pQuesA3pl_S, pP1sg_S, "Im")
	addTransition(pQuesA3pl_S, pP2sg_S, "In")
	addTransition(pQuesA3pl_S, pP1pl_S, "ImIz")
	addTransition(pQuesA3pl_S, pP2pl_S, "InIz")

	// Reflexive pronoun (kendi)
	addEmpty(pronReflex_S, pReflexA1sg_S)
	addEmpty(pronReflex_S, pReflexA2sg_S)
	addEmpty(pronReflex_S, pReflexA3sg_S)
	addEmpty(pronReflex_S, pReflexA1pl_S)
	addEmpty(pronReflex_S, pReflexA2pl_S)
	addEmpty(pronReflex_S, pReflexA3pl_S)

	addTransition(pReflexA1sg_S, pP1sg_S, "Im")
	addTransition(pReflexA2sg_S, pP2sg_S, "In")
	addTransition(pReflexA3sg_S, pP3sg_S, "+sI")
	addEmpty(pReflexA3sg_S, pP3sg_S)
	addTransition(pReflexA1pl_S, pP1pl_S, "ImIz")
	addTransition(pReflexA2pl_S, pP2pl_S, "InIz")
	addTransition(pReflexA3pl_S, pP3pl_S, "lArI")

	// Case transitions for pronouns
	yKeys := []string{NE, NERE, FALAN, FALANCA, HEP, HERKES}
	nGroup := rootKeyNotIn(yKeys...)
	yGroup := rootKeyIn(yKeys...)

	addEmpty(pPnon_S, pNom_ST)
	addTransition(pPnon_S, pDat_ST, "+nA", rootKeyNotIn(BEN, SEN, NE, NERE, FALAN, FALANCA, HERKES))
	addTransition(pPnon_S, pDat_ST, "+yA", yGroup)
	addTransition(pPnon_S, pAcc_ST, "+nI", nGroup)
	addTransition(pPnon_S, pAcc_ST, "+yI", yGroup)
	addTransition(pPnon_S, pLoc_ST, "+ndA", nGroup)
	addTransition(pPnon_S, pLoc_ST, ">dA", yGroup)
	addTransition(pPnon_S, pAbl_ST, "+ndAn", nGroup)
	addTransition(pPnon_S, pAbl_ST, ">dAn", yGroup)
	addTransition(pPnon_S, pGen_ST, "+nIn", and(nGroup, not(and(rootKeyIn(BIZ, BEN, SEN), hasNoSurface))))
	addTransition(pPnon_S, pGen_ST, "im", and(rootKeyIn(BEN, BIZ), hasNoSurface))
	addTransition(pPnon_S, pGen_ST, "in", and(rootKeyIs(SEN), hasNoSurface))
	addTransition(pPnon_S, pGen_ST, "+yIn", and(yGroup, rootKeyIsNot(BIZ)))
	addTransition(pPnon_S, pEqu_ST, ">cA")
	addTransition(pPnon_S, pIns_ST, "+ylA", yGroup)
	addTransition(pPnon_S, pIns_ST, "+nlA", nGroup)
	addTransition(pPnon_S, pIns_ST, "+nInlA", and(nGroup, rootKeyIn(BU, SU, O_PERS, O_DEMONS, SEN), hasNoSurface))
	addTransition(pPnon_S, pIns_ST, "inle", and(rootKeyIs(SIZ), hasNoSurface))
	addTransition(pPnon_S, pIns_ST, "imle", and(rootKeyIn(BIZ, BEN), hasNoSurface))

	p1sgPronCond := rootKeyIn(KIM, BEN, NE, NERE, KENDI)
	addEmpty(pP1sg_S, pNom_ST)
	addTransition(pP1sg_S, pDat_ST, "+nA", nGroup)
	addTransition(pP1sg_S, pAcc_ST, "+nI", nGroup)
	addTransition(pP1sg_S, pDat_ST, "+yA", yGroup)
	addTransition(pP1sg_S, pAcc_ST, "+yI", yGroup)
	addTransition(pP1sg_S, pLoc_ST, "+ndA", rootKeyIs(KENDI))
	addTransition(pP1sg_S, pAbl_ST, "+ndAn", rootKeyIs(KENDI))
	addTransition(pP1sg_S, pEqu_ST, "+ncA", rootKeyIs(KENDI))
	addTransition(pP1sg_S, pIns_ST, "+nlA", p1sgPronCond)
	addTransition(pP1sg_S, pGen_ST, "+nIn", p1sgPronCond)

	p2sgPronCond := rootKeyIn(KIM, SEN, NE, NERE, KENDI)
	addEmpty(pP2sg_S, pNom_ST)
	addTransition(pP2sg_S, pDat_ST, "+nA", nGroup)
	addTransition(pP2sg_S, pAcc_ST, "+nI", nGroup)
	addTransition(pP2sg_S, pDat_ST, "+yA", yGroup)
	addTransition(pP2sg_S, pAcc_ST, "+yI", yGroup)
	addTransition(pP2sg_S, pLoc_ST, "+ndA", rootKeyIs(KENDI))
	addTransition(pP2sg_S, pAbl_ST, "+ndAn", rootKeyIs(KENDI))
	addTransition(pP2sg_S, pEqu_ST, "+ncA", rootKeyIs(KENDI))
	addTransition(pP2sg_S, pIns_ST, "+nlA", p2sgPronCond)
	addTransition(pP2sg_S, pGen_ST, "+nIn", p2sgPronCond)

	p3sgPronCond := or(
		rootKeyIn(KENDI, KIM, NE, NERE, O_PERS, BAZI, BIRI, BIRBIRI, HERBIRI, HEP, KIMI, HICBIRI, HANGI),
		plainVowelPron,
	)
	addEmpty(pP3sg_S, pNom_ST)
	// After 3rd-person possessive (-sI/-I), all pronouns take buffer -n- (nesine, nesini, neresini, kendisine).
	addTransition(pP3sg_S, pDat_ST, "+nA")
	addTransition(pP3sg_S, pAcc_ST, "+nI")
	addTransition(pP3sg_S, pLoc_ST, "+ndA", p3sgPronCond)
	addTransition(pP3sg_S, pAbl_ST, "+ndAn", p3sgPronCond)
	addTransition(pP3sg_S, pGen_ST, "+nIn", p3sgPronCond)
	addTransition(pP3sg_S, pEqu_ST, "ncA", p3sgPronCond)
	addTransition(pP3sg_S, pIns_ST, "+ylA", p3sgPronCond)

	hepCond := rootKeyIn(
		KENDI, KIM, NE, NERE, BIZ, SIZ, BIRI, BIRBIRI, BIRKACI, HERBIRI, HEP, KIMI, COGU, BIRCOGU,
		TUMU, TOPU, BAZI, HICBIRI, HANGI,
	)
	for _, ps := range []*morphemeState{pP1pl_S, pP2pl_S} {
		addEmpty(ps, pNom_ST)
		addTransition(ps, pDat_ST, "+nA", nGroup)
		addTransition(ps, pAcc_ST, "+nI", nGroup)
		addTransition(ps, pDat_ST, "+yA", yGroup)
		addTransition(ps, pAcc_ST, "+yI", yGroup)
		addTransition(ps, pLoc_ST, "+ndA", hepCond)
		addTransition(ps, pAbl_ST, "+ndAn", hepCond)
		addTransition(ps, pGen_ST, "+nIn", hepCond)
		addTransition(ps, pEqu_ST, "+ncA", hepCond)
		addTransition(ps, pIns_ST, "+nlA", hepCond)
	}

	hepsiCond := rootKeyIn(
		KENDI, KIM, NE, NERE, O_PERS, BAZI, BIRI, HERKES, UMUM_QUANT, UMUM_PLAIN, BIRKACI, HEPSI,
		CUMLESI, COGU, BIRCOGU, BIRBIRI, TUMU, KIMI, TOPU, HANGI,
	)
	addEmpty(pP3pl_S, pNom_ST)
	addTransition(pP3pl_S, pDat_ST, "+nA")
	addTransition(pP3pl_S, pAcc_ST, "+nI")
	addTransition(pP3pl_S, pLoc_ST, "+ndA", hepsiCond)
	addTransition(pP3pl_S, pAbl_ST, "+ndAn", hepsiCond)
	addTransition(pP3pl_S, pGen_ST, "+nIn", or(hepsiCond, rootKeyIn(SEN, SIZ)))
	addTransition(pP3pl_S, pEqu_ST, "+ncA", hepsiCond)
	addTransition(pP3pl_S, pIns_ST, "+ylA", hepsiCond)

	addTransition(pNom_ST, with_S, "+nlI", rootKeyIn(BU, SU, O_DEMONS, BEN, SEN, O_PERS, BIZ, SIZ))
	addTransition(pNom_ST, with_S, "lI", rootKeyIs(NERE))
	addTransition(pNom_ST, with_S, "+ylI", rootKeyIs(NE))
	addTransition(pNom_ST, without_S, "+nsIz", rootKeyIn(NERE, BU, SU, O_DEMONS, BEN, SEN, O_PERS, BIZ, SIZ))
	addTransition(pNom_ST, without_S, "+ysIz", rootKeyIs(NE))
	addTransition(pGen_ST, relToPron_S, "ki", rootKeyIn(NERE, BU, SU, O_DEMONS, NE, BEN, SEN, O_PERS, BIZ, SIZ, KIM, KENDI))
	addTransition(pLoc_ST, rel_S, "ki", notRelRepetition)
	addTransition(pIns_ST, vWhile_S, "+yken")

	for _, ps := range []*morphemeState{pNom_ST, pDat_ST, pLoc_ST, pAbl_ST, pGen_ST, pIns_ST} {
		addEmpty(ps, pronZeroDeriv_S, hasTail)
	}
	addEmpty(pronZeroDeriv_S, pvVerbRoot_S)

	// Verb after pronoun
	addEmpty(pvVerbRoot_S, pvPresent_S)
	addTransition(pvVerbRoot_S, vWhile_S, "+yken")
	addTransition(pvVerbRoot_S, pvPast_S, "+ydI")
	addTransition(pvVerbRoot_S, pvNarr_S, "+ymIş")
	addTransition(pvVerbRoot_S, pvCond_S, "+ysA")

	pvAllowA1sg := not(prevGroupContains(pA1pl_S, pP1sg_S))
	pvAllowA1pl := not(prevGroupContains(pA1sg_S, pA2sg_S, pP1sg_S, pP2sg_S))
	pvAllowA2sg := not(prevGroupContains(pA2pl_S, pP2sg_S))
	pvAllowA2pl := not(prevGroupContains(pA2sg_S, pP2pl_S))

	addTransition(pvPresent_S, pvA1sg_ST, "+yIm", pvAllowA1sg)
	addTransition(pvPresent_S, pvA2sg_ST, "sIn", pvAllowA2sg)
	addEmpty(pvPresent_S, pvA3sg_S)
	addTransition(pvPresent_S, pvA1pl_ST, "+yIz", pvAllowA1pl)
	addTransition(pvPresent_S, pvA2pl_ST, "sInIz")
	addTransition(pvPresent_S, pvA3pl_ST, "lAr", prevGroupContains(pLoc_ST))

	addTransition(pvPast_S, pvA1sg_ST, "m", pvAllowA1sg)
	addTransition(pvPast_S, pvA2sg_ST, "n", pvAllowA2sg)
	addTransition(pvPast_S, pvA1pl_ST, "k", pvAllowA1pl)
	addTransition(pvPast_S, pvA2pl_ST, "InIz")
	addTransition(pvPast_S, pvA3pl_ST, "lAr")
	addEmpty(pvPast_S, pvA3sg_ST)

	addTransition(pvNarr_S, pvA1sg_ST, "Im", pvAllowA1sg)
	addTransition(pvNarr_S, pvA2sg_ST, "sIn", pvAllowA2sg)
	addTransition(pvNarr_S, pvA1pl_ST, "Iz", pvAllowA1pl)
	addTransition(pvNarr_S, pvA2pl_ST, "sInIz")
	addTransition(pvNarr_S, pvA3pl_ST, "lAr")
	addEmpty(pvNarr_S, pvA3sg_ST)
	addTransition(pvNarr_S, pvCond_S, "sA")

	addTransition(pvCond_S, pvA1sg_ST, "m", pvAllowA1sg)
	addTransition(pvCond_S, pvA2sg_ST, "n", pvAllowA2sg)
	addTransition(pvCond_S, pvA1pl_ST, "k", pvAllowA1pl)
	addTransition(pvCond_S, pvA2pl_ST, "nIz", pvAllowA2pl)
	addEmpty(pvCond_S, pvA3sg_ST)
	addTransition(pvCond_S, pvA3pl_ST, "lAr")

	pvRejectNoCopula := not(currentGroupContains(pvPast_S, pvCond_S, pvCopBeforeA3pl_S))
	addTransition(pvA1sg_ST, pvCop_ST, "dIr", pvRejectNoCopula)
	addTransition(pvA2sg_ST, pvCop_ST, "dIr", pvRejectNoCopula)
	addTransition(pvA1pl_ST, pvCop_ST, "dIr", pvRejectNoCopula)
	addTransition(pvA2pl_ST, pvCop_ST, "dIr", pvRejectNoCopula)
	addTransition(pvA3sg_S, pvCop_ST, ">dIr", pvRejectNoCopula)
	addTransition(pvA3pl_ST, pvCop_ST, "dIr", pvRejectNoCopula)

	pvA3plAfterLoc := prevStateIs(pvPresent_S)
	addTransition(pvA3pl_ST, vPastAfterTense_ST, "dI", pvA3plAfterLoc)
	addTransition(pvA3pl_ST, vNarrAfterTense_ST, "mIş", pvA3plAfterLoc)
	addTransition(pvA3pl_ST, vCond_ST, "sA", pvA3plAfterLoc)
	addTransition(pvA3pl_ST, vWhile_S, "ken", pvA3plAfterLoc)

	addTransition(pvPresent_S, pvCopBeforeA3pl_S, ">dIr")
	addTransition(pvCopBeforeA3pl_S, pvA3pl_ST, "lAr")

	// =========================================================================
	// 6. Adverbs & Postpositives
	// =========================================================================

	addEmpty(advNounRoot_ST, avZero_S)
	addEmpty(avZero_S, avNounAfterAdvRoot_S)
	addEmpty(avNounAfterAdvRoot_S, avA3sg_S)
	addEmpty(avA3sg_S, avPnon_S)
	addTransition(avPnon_S, avDat_ST, "+yA")

	addEmpty(advForVerbDeriv_ST, avZeroToVerb_S)
	addEmpty(avZeroToVerb_S, nVerb_S)

	addEmpty(postpRoot_ST, postpZero_S, hasTail)
	addEmpty(postpZero_S, nVerb_S)

	isGibi := rootKeyIn("gibi_Postp_PCGen", "gibi_Postp_PCNom")
	isGibiOrSonra := rootKeyIn("gibi_Postp_PCGen", "gibi_Postp_PCNom", "sonra_Postp_PCAbl")
	addEmpty(postpZero_S, po2nRoot_S, isGibiOrSonra)
	addEmpty(po2nRoot_S, po2nA3sg_S)
	addTransition(po2nRoot_S, po2nA3pl_S, "lAr")

	addTransition(po2nA3sg_S, po2nP3sg_S, "+sI")
	addTransition(po2nA3sg_S, po2nP1sg_S, "m", isGibi)
	addTransition(po2nA3sg_S, po2nP2sg_S, "n", isGibi)
	addTransition(po2nA3sg_S, po2nP1pl_S, "miz", isGibi)
	addTransition(po2nA3sg_S, po2nP2pl_S, "niz", isGibi)

	addTransition(po2nA3pl_S, po2nP3sg_S, "+sI")
	addEmpty(po2nA3pl_S, po2nPnon_S)

	addEmpty(po2nP3sg_S, po2nNom_ST)
	addTransition(po2nP3sg_S, po2nDat_ST, "nA")
	addTransition(po2nP3sg_S, po2nLoc_ST, "ndA")
	addTransition(po2nP3sg_S, po2nAbl_ST, "ndAn")
	addTransition(po2nP3sg_S, po2nIns_ST, "ylA")
	addTransition(po2nP3sg_S, po2nGen_ST, "nIn")
	addTransition(po2nP3sg_S, po2nAcc_ST, "nI")

	addEmpty(po2nPnon_S, po2nNom_ST)
	addTransition(po2nPnon_S, po2nDat_ST, "A")
	addTransition(po2nPnon_S, po2nLoc_ST, "dA")
	addTransition(po2nPnon_S, po2nAbl_ST, "dAn")
	addTransition(po2nPnon_S, po2nIns_ST, "lA")
	addTransition(po2nPnon_S, po2nGen_ST, "In")
	addTransition(po2nPnon_S, po2nEqu_ST, "cA")
	addTransition(po2nPnon_S, po2nAcc_ST, "I")

	addTransition(po2nP1sg_S, po2nDat_ST, "e")
	addTransition(po2nP2sg_S, po2nDat_ST, "e")
	addTransition(po2nP1pl_S, po2nDat_ST, "e")
	addTransition(po2nP2pl_S, po2nDat_ST, "e")

	// =========================================================================
	// 7. Verbs
	// =========================================================================

	// Imperative
	addEmpty(verbRoot_S, vImp_S)
	addEmpty(vImp_S, vA2sg_ST)
	addTransition(vImp_S, vA2sg_ST, "sAnA")
	addTransition(vImp_S, vA3sg_ST, "sIn")
	addTransition(vImp_S, vA2pl_ST, "+yIn")
	addTransition(vImp_S, vA2pl_ST, "+yInIz")
	addTransition(vImp_S, vA2pl_ST, "sAnIzA")
	addTransition(vImp_S, vA3pl_ST, "sInlAr")

	// Causative (-t, ->dIr) with chaining (e.g. araştır-t, oku-t-tur, yap-tır-t)
	noCausAfterCompound := not(
		lastDerivIn(vEverSince_S, vRepeat_S, vAlmost_S, vHastily_S, vStay_S, vStart_S),
	)
	addTransition(
		verbRoot_S,
		vCausT_S,
		"t",
		and(
			or(hasAttr(raCausativeT), lastDerivIs(vCausTır_S)),
			not(lastDerivIn(vCausT_S, vPass_S, vAble_S, become_S, acquire_S)),
			noCausAfterCompound,
		),
	)
	addTransition(
		verbRoot_S,
		vCausTır_S,
		">dIr",
		and(
			hasPhon(paLastConsonant),
			not(lastDerivIn(vCausTır_S, vPass_S, vAble_S)),
			noCausAfterCompound,
		),
	)
	addEmpty(vCausT_S, verbRoot_S)
	addEmpty(vCausTır_S, verbRoot_S)

	// Progressive 1 (-Iyor)
	addTransition(verbRoot_S, vProgYor_S, "Iyor", notHavePhon(paLastVowel))
	addTransition(verbRoot_VowelDrop_S, vProgYor_S, "Iyor")
	addTransition(vProgYor_S, vA1sg_ST, "um")
	addTransition(vProgYor_S, vA2sg_ST, "sun")
	addEmpty(vProgYor_S, vA3sg_ST)
	addTransition(vProgYor_S, vA1pl_ST, "uz")
	addTransition(vProgYor_S, vA2pl_ST, "sunuz")
	addTransition(vProgYor_S, vA3pl_ST, "lar")
	addTransition(vProgYor_S, vCond_S, "sa")
	addTransition(vProgYor_S, vPastAfterTense_S, "du")
	addTransition(vProgYor_S, vNarrAfterTense_S, "muş")
	addTransition(vProgYor_S, vCopBeforeA3pl_S, "dur")
	addTransition(vProgYor_S, vWhile_S, "ken")

	// Progressive 2 (-mAktA)
	addTransition(verbRoot_S, vProgMakta_S, "mAktA")
	addTransition(vProgMakta_S, vA1sg_ST, "yIm")
	addTransition(vProgMakta_S, vA2sg_ST, "sIn")
	addEmpty(vProgMakta_S, vA3sg_ST)
	addTransition(vProgMakta_S, vA1pl_ST, "yIz")
	addTransition(vProgMakta_S, vA2pl_ST, "sInIz")
	addTransition(vProgMakta_S, vA3pl_ST, "lAr")
	addTransition(vProgMakta_S, vCond_S, "ysA")
	addTransition(vProgMakta_S, vPastAfterTense_S, "ydI")
	addTransition(vProgMakta_S, vNarrAfterTense_S, "ymIş")
	addTransition(vProgMakta_S, vCopBeforeA3pl_S, "dIr")
	addTransition(vProgMakta_S, vWhile_S, "yken")

	// Positive Aorist (-Ir / -Ar)
	aorICond := or(hasAttr(raAoristI), hasSurface)
	aorACond := and(hasAttr(raAoristA), hasNoSurface)
	addTransition(verbRoot_S, vAor_S, "Ir", aorICond)
	addTransition(verbRoot_S, vAor_S, "Ar", aorACond)
	addTransition(vAor_S, vA1sg_ST, "Im")
	addTransition(vAor_S, vA2sg_ST, "sIn")
	addEmpty(vAor_S, vA3sg_ST)
	addTransition(vAor_S, vA1pl_ST, "Iz")
	addTransition(vAor_S, vA2pl_ST, "sInIz")
	addTransition(vAor_S, vA3pl_ST, "lAr")
	addTransition(vAor_S, vPastAfterTense_S, "dI")
	addTransition(vAor_S, vNarrAfterTense_S, "mIş")
	addTransition(vAor_S, vCond_S, "sA")
	addTransition(vAor_S, vCopBeforeA3pl_S, "dIr")
	addTransition(vAor_S, vWhile_S, "ken")

	// Negative (-mA)
	addTransition(verbRoot_S, vNeg_S, "mA", prevMorphemeIsNot(M.able))
	addEmpty(vNeg_S, vImp_S)
	addTransition(vNeg_S, vPast_S, "dI")
	addTransition(vNeg_S, vFut_S, "yAcA~k")
	addTransition(vNeg_S, vFut_S, "yAcA!ğ")
	addTransition(vNeg_S, vNarr_S, "mIş")
	addTransition(vNeg_S, vProgMakta_S, "mAktA")
	addTransition(vNeg_S, vOpt_S, "yA")
	addTransition(vNeg_S, vDesr_S, "sA")
	addTransition(vNeg_S, vNeces_S, "mAlI")
	addTransition(vNeg_S, vInf1_S, "mAk")
	addTransition(vNeg_S, vInf2_S, "mA")
	addTransition(vNeg_S, vInf3_S, "yIş")
	addTransition(vNeg_S, vActOf_S, "mAcA")
	addTransition(vNeg_S, vPastPart_S, "dI~k")
	addTransition(vNeg_S, vPastPart_S, "dI!ğ")
	addTransition(vNeg_S, vFutPart_S, "yAcA~k")
	addTransition(vNeg_S, vFutPart_S, "yAcA!ğ")
	addTransition(vNeg_S, vPresPart_S, "yAn")
	addTransition(vNeg_S, vNarrPart_S, "mIş")
	addTransition(vNeg_S, vSinceDoingSo_S, "yAlI")
	addTransition(vNeg_S, vByDoingSo_S, "yArAk")
	addTransition(vNeg_S, vHastily_S, "yIver")
	addTransition(vNeg_S, vEverSince_S, "yAgör")
	addTransition(vNeg_S, vAfterDoing_S, "yIp")
	addTransition(vNeg_S, vWhen_S, "yIncA")
	addTransition(vNeg_S, vAsLongAs_S, "dIkçA")
	addTransition(vNeg_S, vNotState_S, "mAzlI~k")
	addTransition(vNeg_S, vNotState_S, "mAzlI!ğ")
	addTransition(vNeg_S, vFeelLike_S, "yAsI")
	addTransition(vNeg_S, vAorNeg_S, "z")
	addEmpty(vNeg_S, vAorNegEmpty_S)
	addTransition(vNeg_S, vAorPartNeg_S, "z")
	addTransition(vNeg_S, vAble_S, "yAbil")

	// Negative before -Iyor (-m + Iyor)
	addTransition(verbRoot_S, vNegProg1_S, "m")
	addTransition(vNegProg1_S, vProgYor_S, "Iyor")

	// Negative Aorist
	addTransition(vAorNeg_S, vA2sg_ST, "sIn")
	addEmpty(vAorNeg_S, vA3sg_ST)
	addTransition(vAorNeg_S, vA2pl_ST, "sInIz")
	addTransition(vAorNeg_S, vA3pl_ST, "lAr")
	addTransition(vAorNeg_S, vPastAfterTense_S, "dI")
	addTransition(vAorNeg_S, vNarrAfterTense_S, "mIş")
	addTransition(vAorNeg_S, vCond_S, "sA")
	addTransition(vAorNeg_S, vCopBeforeA3pl_S, "dIr")
	addTransition(vAorNeg_S, vWhile_S, "ken")

	addTransition(vAorNegEmpty_S, vA1sg_ST, "m")
	addTransition(vAorNegEmpty_S, vA1pl_ST, "yIz")
	addEmpty(vAorPartNeg_S, adjAfterVerb_ST)

	// Ability (+yAbil) & Unable (+yAmA, +yAm)
	addTransition(verbRoot_S, vAble_S, "+yAbil", not(lastDerivIs(vAble_S)))
	addEmpty(vAble_S, verbRoot_S)

	addEmpty(vAbleNeg_S, vAbleNegDerivRoot_S)
	addTransition(vAbleNegDerivRoot_S, vNeg_S, "mA")
	addTransition(vAbleNegDerivRoot_S, vNegProg1_S, "m")

	addTransition(verbRoot_S, vUnable_S, "+yAmA", prevMorphemeIsNot(M.able))
	copyOutgoing(vUnable_S, vNeg_S)
	addTransition(verbRoot_S, vUnableProg1_S, "+yAm")
	addTransition(vUnableProg1_S, vProgYor_S, "Iyor")

	// Verbal nouns, participles, and adjectives
	addTransition(verbRoot_S, vInf1_S, "mA~k")
	addEmpty(vInf1_S, nounInf1Root_S)

	addTransition(verbRoot_S, vInf2_S, "mA")
	addEmpty(vInf2_S, noun_S)

	addTransition(verbRoot_S, vInf3_S, "+yIş")
	addEmpty(vInf3_S, noun_S)

	addTransition(verbRoot_S, vAgt_S, "+yIcI")
	addEmpty(vAgt_S, noun_S)
	addEmpty(vAgt_S, adjAfterVerb_ST)

	addTransition(verbRoot_S, vActOf_S, "mAcA")
	addEmpty(vActOf_S, nounActOfRoot_S)

	addTransition(verbRoot_S, vPastPart_S, ">dI~k")
	addTransition(verbRoot_S, vPastPart_S, ">dI!ğ")
	addEmpty(vPastPart_S, noun_S)
	addEmpty(vPastPart_S, adjAfterVerb_S)

	addTransition(verbRoot_S, vFutPart_S, "+yAcA~k")
	addTransition(verbRoot_S, vFutPart_S, "+yAcA!ğ")
	addEmpty(vFutPart_S, noun_S, hasTail)
	addEmpty(vFutPart_S, adjAfterVerb_S)

	addTransition(verbRoot_S, vNarrPart_S, "mIş")
	addEmpty(vNarrPart_S, adjectiveRoot_ST)

	addTransition(verbRoot_S, vAorPart_S, "Ir", aorICond)
	addTransition(verbRoot_S, vAorPart_S, "Ar", aorACond)
	addEmpty(vAorPart_S, adjAfterVerb_ST)

	addTransition(verbRoot_S, vPresPart_S, "+yAn")
	addEmpty(vPresPart_S, noun_S, hasTail)
	addEmpty(vPresPart_S, adjAfterVerb_ST)

	addTransition(verbRoot_S, vFeelLike_S, "+yAsI")
	addEmpty(vFeelLike_S, noun_S, hasTail)
	addEmpty(vFeelLike_S, adjAfterVerb_ST)

	addTransition(verbRoot_S, vNotState_S, "mAzlI~k")
	addTransition(verbRoot_S, vNotState_S, "mAzlI!ğ")
	addEmpty(vNotState_S, noun_S)

	// Implicit Reciprocal & Reflexive
	addEmpty(vImplicitRecipRoot_S, vRecip_S)
	addEmpty(vRecip_S, verbRoot_S)
	addEmpty(vImplicitReflexRoot_S, vReflex_S)
	addEmpty(vReflex_S, verbRoot_S)

	// Passive (-In, -InIl, +nIl)
	notPassOrAble := and(not(containsMorpheme(M.pass)), not(containsMorpheme(M.able)))
	prevIsCaus := prevStateIn(vCausT_S, vCausTır_S)
	notAfterDerivStem := not(
		prevStateIn(vCausT_S, vCausTır_S, vEverSince_S, vRepeat_S, vAlmost_S, vHastily_S, vStay_S, vStart_S),
	)
	addTransition(verbRoot_S, vPass_S, "In", and(hasAttr(raPassiveIn), notPassOrAble, notAfterDerivStem))
	addTransition(verbRoot_S, vPass_S, "InIl", and(hasAttr(raPassiveIn), notPassOrAble, notAfterDerivStem))
	addTransition(
		verbRoot_S,
		vPass_S,
		"+nIl",
		and(
			not(containsMorpheme(M.able)),
			or(prevIsCaus, and(notHaveAttr(raPassiveIn), not(containsMorpheme(M.pass)))),
		),
	)
	addEmpty(vPass_S, verbRoot_S)

	// Condition (-sA)
	addTransition(vCond_S, vA1sg_ST, "m")
	addTransition(vCond_S, vA2sg_ST, "n")
	addEmpty(vCond_S, vA3sg_ST)
	addTransition(vCond_S, vA1pl_ST, "k")
	addTransition(vCond_S, vA2pl_ST, "nIz")
	addTransition(vCond_S, vA3pl_ST, "lAr")

	// Past (>dI)
	addTransition(verbRoot_S, vPast_S, ">dI")
	addTransition(vPast_S, vA1sg_ST, "m")
	addTransition(vPast_S, vA2sg_ST, "n")
	addEmpty(vPast_S, vA3sg_ST)
	addTransition(vPast_S, vA1pl_ST, "k")
	addTransition(vPast_S, vA2pl_ST, "nIz")
	addTransition(vPast_S, vA3pl_ST, "lAr")
	addTransition(vPast_S, vCond_S, "ysA")

	// Narrative (-mIş)
	addTransition(verbRoot_S, vNarr_S, "mIş")
	addTransition(vNarr_S, vA1sg_ST, "Im")
	addTransition(vNarr_S, vA2sg_ST, "sIn")
	addEmpty(vNarr_S, vA3sg_ST)
	addTransition(vNarr_S, vA1pl_ST, "Iz")
	addTransition(vNarr_S, vA2pl_ST, "sInIz")
	addTransition(vNarr_S, vA3pl_ST, "lAr")
	addTransition(vNarr_S, vCond_S, "sA")
	addTransition(vNarr_S, vPastAfterTense_S, "tI")
	addTransition(vNarr_S, vCopBeforeA3pl_S, "tIr")
	addTransition(vNarr_S, vWhile_S, "ken")
	addTransition(vNarr_S, vNarrAfterTense_S, "mIş")

	// Past after tense
	addTransition(vPastAfterTense_S, vA1sg_ST, "m")
	addTransition(vPastAfterTense_S, vA2sg_ST, "n")
	addEmpty(vPastAfterTense_S, vA3sg_ST)
	addTransition(vPastAfterTense_S, vA1pl_ST, "k")
	addTransition(vPastAfterTense_S, vA2pl_ST, "nIz")
	addTransition(vPastAfterTense_S, vA3pl_ST, "lAr")

	// Narrative after tense
	addTransition(vNarrAfterTense_S, vA1sg_ST, "Im")
	addTransition(vNarrAfterTense_S, vA2sg_ST, "sIn")
	addEmpty(vNarrAfterTense_S, vA3sg_ST)
	addTransition(vNarrAfterTense_S, vA1pl_ST, "Iz")
	addTransition(vNarrAfterTense_S, vA2pl_ST, "sInIz")
	addTransition(vNarrAfterTense_S, vA3pl_ST, "lAr")
	addTransition(vNarrAfterTense_S, vWhile_S, "ken")
	addTransition(vNarrAfterTense_S, vCopBeforeA3pl_S, "tIr")

	// Future (+yAcA~k, +yAcA!ğ)
	addTransition(verbRoot_S, vFut_S, "+yAcA~k")
	addTransition(verbRoot_S, vFut_S, "+yAcA!ğ")
	addTransition(vFut_S, vA1sg_ST, "Im")
	addTransition(vFut_S, vA2sg_ST, "sIn")
	addEmpty(vFut_S, vA3sg_ST)
	addTransition(vFut_S, vA1pl_ST, "Iz")
	addTransition(vFut_S, vA2pl_ST, "sInIz")
	addTransition(vFut_S, vA3pl_ST, "lAr")
	addTransition(vFut_S, vCond_S, "sA")
	addTransition(vFut_S, vPastAfterTense_S, "tI")
	addTransition(vFut_S, vNarrAfterTense_S, "mIş")
	addTransition(vFut_S, vCopBeforeA3pl_S, "tIr")
	addTransition(vFut_S, vWhile_S, "ken")

	// Special irregular verbs: demek / yemek (de/di, ye/yi)
	diYiCond := rootSurfaceIn("di", "yi")
	deYeCond := rootSurfaceIn("de", "ye")
	cMultiVerb := not(prevMorphemeIn(M.everSince, M.repeat, M.almost, M.hastily, M.stay, M.start))

	addTransition(vDeYeRoot_S, vFut_S, "yece~k", diYiCond)
	addTransition(vDeYeRoot_S, vFut_S, "yece!ğ", diYiCond)
	addTransition(vDeYeRoot_S, vProgYor_S, "yor", diYiCond)
	addTransition(vDeYeRoot_S, vAble_S, "yebil", diYiCond)
	addTransition(vDeYeRoot_S, vAbleNeg_S, "ye", diYiCond)
	addTransition(vDeYeRoot_S, vInf3_S, "yiş", rootSurfaceIs("yi"))
	addTransition(vDeYeRoot_S, vFutPart_S, "yece~k", diYiCond)
	addTransition(vDeYeRoot_S, vFutPart_S, "yece!ğ", diYiCond)
	addTransition(vDeYeRoot_S, vPresPart_S, "yen", diYiCond)
	addTransition(vDeYeRoot_S, vEverSince_S, "yegel", and(diYiCond, cMultiVerb))
	addTransition(vDeYeRoot_S, vRepeat_S, "yedur", and(diYiCond, cMultiVerb))
	addTransition(vDeYeRoot_S, vRepeat_S, "yegör", and(diYiCond, cMultiVerb))
	addTransition(vDeYeRoot_S, vAlmost_S, "yeyaz", and(diYiCond, cMultiVerb))
	addTransition(vDeYeRoot_S, vStart_S, "yekoy", and(diYiCond, cMultiVerb))
	addTransition(vDeYeRoot_S, vHastily_S, "yiver", and(diYiCond, cMultiVerb))
	addTransition(vDeYeRoot_S, vSinceDoingSo_S, "yeli", diYiCond)
	addTransition(vDeYeRoot_S, vByDoingSo_S, "yerek", diYiCond)
	addTransition(vDeYeRoot_S, vFeelLike_S, "yesi", diYiCond)
	addTransition(vDeYeRoot_S, vAfterDoing_S, "yip", rootSurfaceIn("de", "yi"))
	addTransition(vDeYeRoot_S, vWhen_S, "yince", rootSurfaceIn("de", "yi"))
	addTransition(vDeYeRoot_S, vWithoutBeingAbleToHaveDoneSo_S, "yemeden", diYiCond)
	addTransition(vDeYeRoot_S, vOpt_S, "ye", diYiCond)

	addTransition(vDeYeRoot_S, vCausTır_S, "dir", deYeCond)
	addTransition(vDeYeRoot_S, vPass_S, "n", deYeCond)
	addTransition(vDeYeRoot_S, vPass_S, "nil", deYeCond)
	addTransition(vDeYeRoot_S, vPast_S, "di", deYeCond)
	addTransition(vDeYeRoot_S, vNarr_S, "miş", deYeCond)
	addTransition(vDeYeRoot_S, vAor_S, "r", deYeCond)
	addTransition(vDeYeRoot_S, vNeg_S, "me", deYeCond)
	addTransition(vDeYeRoot_S, vNegProg1_S, "m", deYeCond)
	addTransition(vDeYeRoot_S, vProgMakta_S, "mekte", deYeCond)
	addTransition(vDeYeRoot_S, vDesr_S, "se", deYeCond)
	addTransition(vDeYeRoot_S, vInf1_S, "mek", deYeCond)
	addTransition(vDeYeRoot_S, vInf2_S, "me", deYeCond)
	addTransition(vDeYeRoot_S, vInf3_S, "yiş", rootSurfaceIs("de"))
	addTransition(vDeYeRoot_S, vPastPart_S, "di~k", deYeCond)
	addTransition(vDeYeRoot_S, vPastPart_S, "di!ğ", deYeCond)
	addTransition(vDeYeRoot_S, vNarrPart_S, "miş", deYeCond)
	addTransition(vDeYeRoot_S, vAsLongAs_S, "dikçe", deYeCond)
	addTransition(vDeYeRoot_S, vWithoutHavingDoneSo_S, "meden", deYeCond)
	addTransition(vDeYeRoot_S, vWithoutHavingDoneSo_S, "meksizin", deYeCond)
	addTransition(vDeYeRoot_S, vNeces_S, "meli", deYeCond)
	addTransition(vDeYeRoot_S, vNotState_S, "mezli~k", deYeCond)
	addTransition(vDeYeRoot_S, vNotState_S, "mezli!ğ", deYeCond)
	addEmpty(vDeYeRoot_S, vImp_S, rootSurfaceIs("de"))
	addEmpty(vDeYeRoot_S, vImpYemekYe_S, rootSurfaceIs("ye"))
	addEmpty(vDeYeRoot_S, vImpYemekYi_S, rootSurfaceIs("yi"))

	addTransition(vImpYemekYi_S, vA2pl_ST, "yin")
	addTransition(vImpYemekYi_S, vA2pl_ST, "yiniz")
	addEmpty(vImpYemekYe_S, vA2sg_ST)
	addTransition(vImpYemekYe_S, vA2sg_ST, "sene")
	addTransition(vImpYemekYe_S, vA3sg_ST, "sin")
	addTransition(vImpYemekYe_S, vA2pl_ST, "senize")
	addTransition(vImpYemekYe_S, vA3pl_ST, "sinler")

	// Optative (+yA)
	addTransition(verbRoot_S, vOpt_S, "+yA")
	addTransition(vOpt_S, vA1sg_ST, "yIm")
	addTransition(vOpt_S, vA2sg_ST, "sIn")
	addEmpty(vOpt_S, vA3sg_ST)
	addTransition(vOpt_S, vA1pl_ST, "lIm")
	addTransition(vOpt_S, vA2pl_ST, "sInIz")
	addTransition(vOpt_S, vA3pl_ST, "lAr")
	addTransition(vOpt_S, vPastAfterTense_S, "ydI")
	addTransition(vOpt_S, vNarrAfterTense_S, "ymIş")

	// Desire (-sA)
	addTransition(verbRoot_S, vDesr_S, "sA")
	addTransition(vDesr_S, vA1sg_ST, "m")
	addTransition(vDesr_S, vA2sg_ST, "n")
	addEmpty(vDesr_S, vA3sg_ST)
	addTransition(vDesr_S, vA1pl_ST, "k")
	addTransition(vDesr_S, vA2pl_ST, "nIz")
	addTransition(vDesr_S, vA3pl_ST, "lAr")
	addTransition(vDesr_S, vPastAfterTense_S, "ydI")
	addTransition(vDesr_S, vNarrAfterTense_S, "ymIş")

	// Necessity (-mAlI)
	addTransition(verbRoot_S, vNeces_S, "mAlI")
	addTransition(vNeces_S, vA1sg_ST, "yIm")
	addTransition(vNeces_S, vA2sg_ST, "sIn")
	addEmpty(vNeces_S, vA3sg_ST)
	addTransition(vNeces_S, vA1pl_ST, "yIz")
	addTransition(vNeces_S, vA2pl_ST, "sInIz")
	addTransition(vNeces_S, vA3pl_ST, "lAr")
	addTransition(vNeces_S, vPastAfterTense_S, "ydI")
	addTransition(vNeces_S, vCond_S, "ysA")
	addTransition(vNeces_S, vNarrAfterTense_S, "ymIş")
	addTransition(vNeces_S, vCopBeforeA3pl_S, "dIr")
	addTransition(vNeces_S, vWhile_S, "yken")

	// A3pl before compound tense ("yapar-lar-dı"), Copula, and Past+Person+Cond ("geldinse")
	prevNotPastNarrCond := not(prevStateIn(vPastAfterTense_S, vNarrAfterTense_S, vCond_S))
	addTransition(vA3pl_ST, vPastAfterTense_ST, "dI", prevNotPastNarrCond)
	addTransition(vA3pl_ST, vNarrAfterTense_ST, "mIş", prevNotPastNarrCond)
	addTransition(vA3pl_ST, vCond_ST, "sA", prevNotPastNarrCond)

	copWhileTenses := prevMorphemeIn(M.prog1, M.prog2, M.neces, M.fut, M.narr, M.aor)
	addTransition(vA3pl_ST, vCop_ST, "dIr", copWhileTenses)
	addTransition(vA3pl_ST, vWhile_S, "ken", copWhileTenses)
	addTransition(vA1sg_ST, vCop_ST, "dIr", copWhileTenses)
	addTransition(vA2sg_ST, vCop_ST, "dIr", copWhileTenses)
	addTransition(vA3sg_ST, vCop_ST, ">dIr", copWhileTenses)
	addTransition(vA1pl_ST, vCop_ST, "dIr", copWhileTenses)
	addTransition(vA2pl_ST, vCop_ST, "dIr", copWhileTenses)
	addTransition(vCopBeforeA3pl_S, vA3pl_ST, "lAr")

	prevPastNoCond := and(prevMorphemeIs(M.past), not(containsMorpheme(M.cond, M.desr)))
	addTransition(vA1sg_ST, vCondAfterPerson_ST, "sA", prevPastNoCond)
	addTransition(vA2sg_ST, vCondAfterPerson_ST, "sA", prevPastNoCond)
	addTransition(vA1pl_ST, vCondAfterPerson_ST, "sA", prevPastNoCond)
	addTransition(vA2pl_ST, vCondAfterPerson_ST, "sA", prevPastNoCond)

	// Compound auxiliary verb derivations
	addTransition(verbRoot_S, vEverSince_S, "+yAgel", cMultiVerb)
	addTransition(verbRoot_S, vRepeat_S, "+yAdur", cMultiVerb)
	addTransition(verbRoot_S, vRepeat_S, "+yAgör", cMultiVerb)
	addTransition(verbRoot_S, vAlmost_S, "+yAyaz", cMultiVerb)
	addTransition(verbRoot_S, vHastily_S, "+yIver", cMultiVerb)
	addTransition(verbRoot_S, vStay_S, "+yAkal", cMultiVerb)
	addTransition(verbRoot_S, vStart_S, "+yAkoy", cMultiVerb)

	for _, s := range []*morphemeState{vEverSince_S, vRepeat_S, vAlmost_S, vHastily_S, vStay_S, vStart_S} {
		addEmpty(s, verbRoot_S)
	}

	// Adverbial verb derivations (converbs / zarf-fiiller)
	addTransition(vA3sg_ST, vAsIf_S, ">cAsInA", prevMorphemeIn(M.aor, M.narr))
	addTransition(verbRoot_S, vWhen_S, "+yIncA")
	addTransition(verbRoot_S, vSinceDoingSo_S, "+yAlI")
	addTransition(verbRoot_S, vByDoingSo_S, "+yArAk")
	addTransition(verbRoot_S, vAdamantly_S, "+yAsIyA")
	addTransition(verbRoot_S, vAfterDoing_S, "+yIp")
	addTransition(verbRoot_S, vWithoutBeingAbleToHaveDoneSo_S, "+yAmAdAn")
	addTransition(verbRoot_S, vAsLongAs_S, ">dIkçA")
	addTransition(verbRoot_S, vWithoutHavingDoneSo_S, "mAdAn")
	addTransition(verbRoot_S, vWithoutHavingDoneSo_S, "mAksIzIn")

	for _, s := range []*morphemeState{
		vAsIf_S,
		vSinceDoingSo_S,
		vByDoingSo_S,
		vAdamantly_S,
		vAfterDoing_S,
		vWithoutBeingAbleToHaveDoneSo_S,
		vAsLongAs_S,
		vWithoutHavingDoneSo_S,
		vWhile_S,
	} {
		addEmpty(s, advRoot_ST)
	}
	addEmpty(vWhen_S, advNounRoot_ST)

	// Post-processing: LastVowelDrop verbs like kavurmak -> kavr-ul (and block kavur-ul)
	addTransition(verbLastVowelDropModRoot_S, vPass_S, "Il")
	copyOutgoing(verbLastVowelDropUnmodRoot_S, verbRoot_S)
	removeTransitionsToMorpheme(verbLastVowelDropUnmodRoot_S, M.pass)

	// =========================================================================
	// 8. Standalone Question (mı/mi/mu/mü) & imek (idi/imiş/ise)
	// =========================================================================

	addEmpty(questionRoot_S, qPresent_S)
	addTransition(questionRoot_S, qPast_S, "ydI")
	addTransition(questionRoot_S, qNarr_S, "ymIş")

	addTransition(qPresent_S, qA1sg_ST, "yIm")
	addTransition(qPresent_S, qA2sg_ST, "sIn")
	addEmpty(qPresent_S, qA3sg_ST)
	addTransition(qPresent_S, qA1pl_ST, "+yIz")
	addTransition(qPresent_S, qA2pl_ST, "sInIz")

	addTransition(qPast_S, qA1sg_ST, "m")
	addTransition(qPast_S, qA2sg_ST, "n")
	addTransition(qPast_S, qA1pl_ST, "k")
	addTransition(qPast_S, qA2pl_ST, "InIz")
	addTransition(qPast_S, qA3pl_ST, "lAr")
	addEmpty(qPast_S, qA3sg_ST)

	addTransition(qNarr_S, qA1sg_ST, "Im")
	addTransition(qNarr_S, qA2sg_ST, "sIn")
	addTransition(qNarr_S, qA1pl_ST, "Iz")
	addTransition(qNarr_S, qA2pl_ST, "sInIz")
	addTransition(qNarr_S, qA3pl_ST, "lAr")
	addEmpty(qNarr_S, qA3sg_ST)

	qRejectNoCopula := not(currentGroupContains(qPast_S))
	addTransition(qA1sg_ST, qCop_ST, "dIr", qRejectNoCopula)
	addTransition(qA2sg_ST, qCop_ST, "dIr", qRejectNoCopula)
	addTransition(qA3sg_ST, qCop_ST, ">dIr", qRejectNoCopula)
	addTransition(qA1pl_ST, qCop_ST, "dIr", qRejectNoCopula)
	addTransition(qA2pl_ST, qCop_ST, "dIr", qRejectNoCopula)
	addTransition(qPresent_S, qCopBeforeA3pl_S, ">dIr")
	addTransition(qCopBeforeA3pl_S, qA3pl_ST, "lAr")

	// Verb imek
	addTransition(imekRoot_S, imekPast_S, "di")
	addTransition(imekRoot_S, imekNarr_S, "miş")
	addTransition(imekRoot_S, imekCond_S, "se")

	addTransition(imekPast_S, imekA1sg_ST, "m")
	addTransition(imekPast_S, imekA2sg_ST, "n")
	addEmpty(imekPast_S, imekA3sg_ST)
	addTransition(imekPast_S, imekA1pl_ST, "k")
	addTransition(imekPast_S, imekA2pl_ST, "niz")
	addTransition(imekPast_S, imekA3pl_ST, "ler")
	addTransition(imekPast_S, imekCond_S, "yse")

	addTransition(imekNarr_S, imekA1sg_ST, "im")
	addTransition(imekNarr_S, imekA2sg_ST, "sin")
	addEmpty(imekNarr_S, imekA3sg_ST)
	addTransition(imekNarr_S, imekA1pl_ST, "iz")
	addTransition(imekNarr_S, imekA2pl_ST, "siniz")
	addTransition(imekNarr_S, imekA3pl_ST, "ler")
	addTransition(imekNarr_S, imekCond_S, "se")

	addTransition(imekCond_S, imekA1sg_ST, "m")
	addTransition(imekCond_S, imekA2sg_ST, "n")
	addEmpty(imekCond_S, imekA3sg_ST)
	addTransition(imekCond_S, imekA1pl_ST, "k")
	addTransition(imekCond_S, imekA2pl_ST, "niz")
	addTransition(imekCond_S, imekA3pl_ST, "ler")

	imekRejectNoCopula := not(currentGroupContains(imekPast_S))
	addTransition(imekA1sg_ST, imekCop_ST, "dir", imekRejectNoCopula)
	addTransition(imekA2sg_ST, imekCop_ST, "dir", imekRejectNoCopula)
	addTransition(imekA3sg_ST, imekCop_ST, "tir", imekRejectNoCopula)
	addTransition(imekA1pl_ST, imekCop_ST, "dir", imekRejectNoCopula)
	addTransition(imekA2pl_ST, imekCop_ST, "dir", imekRejectNoCopula)
	addTransition(imekA3pl_ST, imekCop_ST, "dir", imekRejectNoCopula)

	// =========================================================================
	// Root state routing
	// =========================================================================

	// =========================================================================
	// Root state routing
	// =========================================================================

	specialRootStates := map[string]*morphemeState{
		"değil_Verb":  nVerbDegil_S,
		"imek_Verb":   imekRoot_S,
		"su_Noun":     nounSuRoot_S,
		"akarsu_Noun": nounSuRoot_S,
		"öyle_Adv":    advForVerbDeriv_ST,
		"böyle_Adv":   advForVerbDeriv_ST,
		"şöyle_Adv":   advForVerbDeriv_ST,
	}

	getRootState := func(item *dictionaryItem, phonetic int) *morphemeState {
		if special, ok := specialRootStates[item.key]; ok {
			return special
		}
		if phonetic&paLastLetterDropped != 0 {
			return verbRoot_VowelDrop_S
		}
		if item.attributes&raReciprocal != 0 {
			return vImplicitRecipRoot_S
		}
		if item.attributes&raReflexive != 0 {
			return vImplicitReflexRoot_S
		}
		switch item.primaryPos {
		case "Noun":
			switch {
			case item.attributes&raNoSuffix != 0:
				return nounNoSuffix_S
			case item.secondaryPos == "ProperNoun":
				return nounProper_S
			case item.secondaryPos == "Abbreviation":
				return nounAbbrv_S
			case item.attributes&raCompoundP3sgRoot != 0:
				return nounCompoundRoot_S
			}
			return noun_S
		case "Adj":
			if item.attributes&raNoSuffix != 0 {
				return adjNoSuffix_ST
			}
			return adjectiveRoot_ST
		case "Pron":
			switch item.secondaryPos {
			case "PersonalPron":
				return pronPers_S
			case "DemonstrativePron":
				return pronDemons_S
			case "QuestionPron":
				return pronQues_S
			case "ReflexivePron":
				return pronReflex_S
			}
			return pronQuant_S
		case "Adv":
			return advRoot_ST
		case "Conj":
			return conjRoot_ST
		case "Ques":
			return questionRoot_S
		case "Interj":
			return interjRoot_ST
		case "Verb":
			return verbRoot_S
		case "Det":
			return detRoot_ST
		case "Postp":
			return postpRoot_ST
		case "Num":
			return numeralRoot_ST
		case "Dup":
			return dupRoot_ST
		}
		return noun_S
	}

	return &morphotacticsGraph{
		noun_S:                       noun_S,
		nounCompoundRoot_S:           nounCompoundRoot_S,
		nounSuRoot_S:                 nounSuRoot_S,
		nounProper_S:                 nounProper_S,
		nounAbbrv_S:                  nounAbbrv_S,
		nounNoSuffix_S:               nounNoSuffix_S,
		adjectiveRoot_ST:             adjectiveRoot_ST,
		adjNoSuffix_ST:               adjNoSuffix_ST,
		numeralRoot_ST:               numeralRoot_ST,
		nVerbDegil_S:                 nVerbDegil_S,
		pronPers_S:                   pronPers_S,
		pronPers_Mod_S:               pronPers_Mod_S,
		pronDemons_S:                 pronDemons_S,
		pronQuant_S:                  pronQuant_S,
		pronQuantModified_S:          pronQuantModified_S,
		pronQues_S:                   pronQues_S,
		pronReflex_S:                 pronReflex_S,
		advRoot_ST:                   advRoot_ST,
		advForVerbDeriv_ST:           advForVerbDeriv_ST,
		conjRoot_ST:                  conjRoot_ST,
		interjRoot_ST:                interjRoot_ST,
		detRoot_ST:                   detRoot_ST,
		dupRoot_ST:                   dupRoot_ST,
		postpRoot_ST:                 postpRoot_ST,
		verbRoot_S:                   verbRoot_S,
		verbRoot_VowelDrop_S:         verbRoot_VowelDrop_S,
		verbLastVowelDropModRoot_S:   verbLastVowelDropModRoot_S,
		verbLastVowelDropUnmodRoot_S: verbLastVowelDropUnmodRoot_S,
		vImplicitRecipRoot_S:         vImplicitRecipRoot_S,
		vImplicitReflexRoot_S:        vImplicitReflexRoot_S,
		vDeYeRoot_S:                  vDeYeRoot_S,
		questionRoot_S:               questionRoot_S,
		imekRoot_S:                   imekRoot_S,
		getRootState:                 getRootState,
	}
}
