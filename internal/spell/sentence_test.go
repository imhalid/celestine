package spell

import (
	"slices"
	"strings"
	"testing"
)

// These tests are tinyfix-tr's src/sentence.test.ts.

var sentenceTestWords = setOf(
	"merhaba", "ben", "geldim", "sonra", "gittik", "bugün", "çok", "güzel", "nasılsın", "iyiyim", "ahmet", "sınıf",
	"kez", "dosya", "site", "şeyler", "bir", "ne", "oldu", "dedi", "evet", "hayır", "gel", "hemen", "dün", "ilk",
	"istanbul", "uygulama", "inanılmaz", "ile", "bu", "aynı", "onlar",
)

var sentenceTestNames = map[string]string{
	"bell": "Bell", "hatay": "Hatay", "cola": "Cola", "ahmet": "Ahmet", "istanbul": "İstanbul", "türkçe": "Türkçe",
}

var sentenceTestStems = map[string][]string{
	"istanbula": {"istanbul"},
	"ahmetle":   {"ahmet"},
	"türkçeyi":  {"türkçe"},
	"belli":     {"bel", "belli"},
	"hatayı":    {"hata"},
}

// applyCorrections writes corrections into text.
func applyCorrections(text string, corrections []Correction) string {
	rs := []rune(text)
	var out strings.Builder
	cursor := 0
	for _, c := range corrections {
		out.WriteString(runeSlice(rs, cursor, c.Start) + c.Replacement)
		cursor = c.End
	}
	return out.String() + runeSlice(rs, cursor, len(rs))
}

// sentenceFix runs the minimal sentence rules, as the TypeScript test's fix does.
func sentenceFix(text string, corrections ...Correction) string {
	isWord := func(lower string) bool { return sentenceTestWords[lower] }
	nameFor := func(lower string, _ bool) (string, bool) { n, ok := sentenceTestNames[lower]; return n, ok }
	stemsOf := func(lower string) ([]string, bool) {
		if s, ok := sentenceTestStems[lower]; ok {
			return s, true
		}
		return []string{}, true
	}
	return applyCorrections(text, applySentenceRules(text, corrections, isWord, nameFor, stemsOf, nil))
}

func checkCases(t *testing.T, run func(string) string, cases [][2]string) {
	t.Helper()
	for _, c := range cases {
		if got := run(c[0]); got != c[1] {
			t.Errorf("%q: got %q, want %q", c[0], got, c[1])
		}
	}
}

func TestSentenceMissingSpaceAndCapitals(t *testing.T) {
	checkCases(t, func(s string) string { return sentenceFix(s) }, [][2]string{
		{"merhaba.ben geldim", "Merhaba. Ben geldim"},
		{"nasılsın?iyiyim", "Nasılsın? İyiyim"},
		{"gel!hemen", "Gel! Hemen"},
		{"dün geldim. sonra gittik.", "Dün geldim. Sonra gittik."},
		{"evet. ilk kez", "Evet. İlk kez"},
		{"Bugün çok güzel.", "Bugün çok güzel."},
		{"“ne oldu?” dedi.", "“Ne oldu?” dedi."},
	})
}

func TestSentenceLeavesAbbreviationsAndAddresses(t *testing.T) {
	checkCases(t, func(s string) string { return sentenceFix(s) }, [][2]string{
		// "Dr." is not a sentence end, but "ahmet" is a name: it gets a capital anyway.
		{"Dr. ahmet geldi", "Dr. Ahmet geldi"},
		{"Bir dr. lazım", "Bir dr. lazım"},
		{"Dr. ali geldi", "Dr. ali geldi"},
		{"Kalem, defter vb. şeyler aldık", "Kalem, defter vb. şeyler aldık"},
		{"Bu 3. sınıf, ilk kez geldim.", "Bu 3. sınıf, ilk kez geldim."},
		{"Değer 3.5 oldu", "Değer 3.5 oldu"},
		{"Bilmiyorum... belki gelir", "Bilmiyorum... belki gelir"},
		{"Bilmiyorum… belki gelir", "Bilmiyorum… belki gelir"},
		{"Siteye tdkai.halid.dev üzerinden girin", "Siteye tdkai.halid.dev üzerinden girin"},
		{"Bunu dosya.pdf olarak gönder", "Bunu dosya.pdf olarak gönder"},
		{"Bkz. site.com adresi", "Bkz. site.com adresi"},
		{"ahmet@site.com yazdı", "ahmet@site.com yazdı"},
		{"A. Yılmaz geldi. sonra gittik", "A. Yılmaz geldi. Sonra gittik"},
		{"iPhone aldım", "iPhone aldım"},
		{"XVIII. yüzyıl boyunca", "XVIII. yüzyıl boyunca"},
		{"Hızı 217 km. yaptı", "Hızı 217 km. yaptı"},
		{"Frekans 5.75 MHz. tir", "Frekans 5.75 MHz. tir"},
		{"Ruiz & Pav. ex Dunal", "Ruiz & Pav. ex Dunal"},
		{"Filipina Hola! magazin dergisi", "Filipina Hola! magazin dergisi"},
		{"Merhaba! nasılsın", "Merhaba! Nasılsın"},
		{"Hey Ocean! ve diğerleri", "Hey Ocean! ve diğerleri"},
		{"ს. ყაუხჩიშვილი. ბერძნული", "ს. ყაუხჩიშვილი. ბერძნული"},
		// Only real words on both sides: "istanbul.xyz" and "abc.def" are not sentences.
		{"Adres abc.def olsun", "Adres abc.def olsun"},
	})
}

func TestSentenceProperNames(t *testing.T) {
	checkCases(t, func(s string) string { return sentenceFix(s) }, [][2]string{
		{"Dün ahmet ile istanbula gittik.", "Dün Ahmet ile İstanbul'a gittik."},
		{"Bunu ahmet'e sor, türkçe biliyor.", "Bunu Ahmet'e sor, Türkçe biliyor."},
		{"Bu ahmetle ilgili", "Bu Ahmet'le ilgili"},
		{"Bugün dr. ahmet geldi", "Bugün Dr. Ahmet geldi"},
		{"Onlar türkçeyi seviyor", "Onlar Türkçeyi seviyor"},
		// Ordinary words that only start like a name: "bel + li", "hata + yı".
		{"Bu belli değil", "Bu belli değil"},
		{"Aynı hatayı yaptım", "Aynı hatayı yaptım"},
		{"Bir pina colada istedim", "Bir pina colada istedim"},
		{"Adres istanbul.com olsun", "Adres istanbul.com olsun"},
		{"Jyutping: ahmet2 olur", "Jyutping: ahmet2 olur"},
		{`Bu "i" harfi`, `Bu "i" harfi`},
	})
}

func TestSentenceMergesWithAnotherFix(t *testing.T) {
	// "cok" -> "çok" by the marks rule; at a sentence start it becomes "Çok".
	if got := sentenceFix("cok güzel", Correction{Start: 0, End: 3, Original: "cok", Replacement: "çok"}); got != "Çok güzel" {
		t.Errorf("got %q", got)
	}
	// A missing space before a word another rule fixes: "merhaba.cok" -> "Merhaba. Çok".
	if got := sentenceFix("merhaba.cok güzel", Correction{Start: 8, End: 11, Original: "cok", Replacement: "çok"}); got != "Merhaba. Çok güzel" {
		t.Errorf("got %q", got)
	}
}

func TestSentenceProperNameHelpers(t *testing.T) {
	// Number suffixes: the TDK apostrophe and consonant voicing.
	for _, c := range []struct {
		num, suffix string
		hadApos     bool
		want        string // "" for null
	}{
		{"1985", "te", false, "te"},
		{"1985", "de", false, "te"},
		{"1985", "de", true, "te"},
		{"1985", "te", true, ""}, // already right
		{"2023", "de", false, "te"},
		{"80", "lere", false, "lere"},
		{"2", "nci", false, "nci"},
		{"30", "da", false, "da"},
		// Units and short codes.
		{"5", "cm", false, ""},
		{"3", "a", false, ""},
		{"20", "px", false, ""},
	} {
		got, ok := FixNumberSuffix(c.num, c.suffix, c.hadApos)
		if !ok {
			got = ""
		}
		if got != c.want {
			t.Errorf("FixNumberSuffix(%q, %q, %v) = %q, want %q", c.num, c.suffix, c.hadApos, got, c.want)
		}
	}

	// Softening when a wrong apostrophe is taken out.
	morphWords := setOf("kitabı", "ağaca", "tutarlılığı")
	isMorph := func(w string) bool { return morphWords[w] }
	for _, c := range [][3]string{
		{"Hristiyanlık", "ın", "Hristiyanlığın"},
		{"Atatürkçülük", "ün", "Atatürkçülüğün"},
		{"Başbakanlık", "a", "Başbakanlığa"},
		{"kitap", "ı", "kitabı"},
		{"ağaç", "a", "ağaca"},
		{"Türkler", "in", "Türklerin"},
	} {
		if got := JoinWithoutApostrophe(c[0], c[1], isMorph); got != c[2] {
			t.Errorf("JoinWithoutApostrophe(%q, %q) = %q, want %q", c[0], c[1], got, c[2])
		}
	}
}

func TestSentenceFullEngineGuards(t *testing.T) {
	known := KnownMisspellingsFromTSV(strings.Join([]string{
		"şili\tŞili\t0\t5000\tNAME",
		"dicle\tDicle\t0\t5000\tNAME",
		"sırp\tSırp\t0\t5000\tNAME",
		"yahudi\tYahudi\t0\t5000\tNAME",
		"londra\tLondra\t0\t10000\tNAME",
		"eskimo\tEskimo\t0\t2000\tNAME",
		"hacettepe\tHacettepe\t0\t5000\tNAME",
		"boğaziçi\tBoğaziçi\t0\t8000\tNAME",
		"fransızca\tFransızca\tnoapos\t10000\tNAME",
		"mustafa\tMustafa\t0\t30000\tNAME",
		"murat\tMurat\t0\t20000\tNAME",
		"volvo\tVolvo\t0\t700\tNAME",
		"ptt\tPTT\t0\t5000\tACRO",
		"mustafa\tMustafa\t0\t500\tGIVEN",
		"murat\tMurat\t0\t500\tGIVEN",
		"eren\tEren\t0\t500\tGIVEN",
		"okan\tOkan\t0\t500\tGIVEN",
		"barış\tBarış\t0\t500\tGIVEN",
		"savaş\tSavaş\t0\t500\tGIVEN",
		"güneş\tGüneş\t0\t500\tGIVEN",
		"okan\tOkan\t0\t500\tSURNAME",
		"manço\tManço\t0\t500\tSURNAME",
		"yıldız\tYıldız\t0\t0\tAMBIG",
		"savaş\tSavaş\t0\t0\tAMBIG",
		"güneş\tGüneş\t0\t0\tAMBIG",
		"eren\tEren\t0\t0\tAMBIG",
	}, "\n"))
	morphSet := setOf(
		"sunucuların", "eren", "sona", "savaşa", "savaş", "güneş", "sokağıdır", "kış", "uykusuna", "yattı",
		"eski", "çağ", "astronomlarının", "orta", "güney", "kulübü", "partisini", "komisyonu", "elçiliği",
		"müdürü", "başkanı", "odası", "köy", "iletişim", "bölümünden", "öğretmenler", "günü",
	)
	counts := map[string]float64{"murat'ın": 800, "muratın": 10, "sunucuların": 120, "volvo": 698, "weisse": 38, "sharks": 79}
	capShares := map[string]float64{"volvo": 0.5, "weisse": 0.78, "sharks": 0.82}
	pairs := map[string]float64{"sona eren": 400}
	fixFull := func(text string, collectAmbig *[]AmbigCandidateQuery) string {
		res := &SentenceResources{
			Known:        known,
			IsMorphWord:  func(w string) bool { return morphSet[w] },
			Count:        func(w string) float64 { return counts[w] },
			CapitalShare: func(w string) float64 { return capShares[w] },
			Pair:         func(a, b string) float64 { return pairs[a+" "+b] },
			CollectAmbig: collectAmbig,
		}
		isWord := func(w string) bool { return morphSet[w] }
		nameFor := func(w string, _ bool) (string, bool) { return known.NameFor(w, 0) }
		return applyCorrections(text, applySentenceRules(text, nil, isWord, nameFor, nil, res))
	}
	checkCases(t, func(s string) string { return fixFull(s, nil) }, [][2]string{
		// 1. A generic modifier of an era, and a historical period.
		{"Gezegen adı çünkü eski çağ astronomlarının kayıtlarından gelir.", "Gezegen adı çünkü eski çağ astronomlarının kayıtlarından gelir."},
		{"eski çağın uygarlıkları incelendi.", "Eski Çağ'ın uygarlıkları incelendi."},
		{"İlk çağ yazının icadıyla başladı.", "İlk Çağ yazının icadıyla başladı."},
		// 2. A job after an institution, and an official title.
		{"Annesi daha sonra Bremen Sanat Müzesi müdürü ressam Carl Horn ile evlendi.", "Annesi daha sonra Bremen Sanat Müzesi müdürü ressam Carl Horn ile evlendi."},
		{"Eski Azerbaycan Millî Meclisi başkanı Resul Guliyev konuştu.", "Eski Azerbaycan Millî Meclisi başkanı Resul Guliyev konuştu."},
		{"Türk Dil Kurumu başkanına dilekçe verdik.", "Türk Dil Kurumu Başkanı'na dilekçe verdik."},
		{"Türk Dil Kurumu başkanı açıklama yaptı.", "Türk Dil Kurumu Başkanı açıklama yaptı."},
		// 3. Directions as adjectives, and regions.
		{"Hayvancılık ise ağırlıkla orta Şili ve güney Şili’nin kuzey kısımlarında yapılır.", "Hayvancılık ise ağırlıkla orta Şili ve güney Şili’nin kuzey kısımlarında yapılır."},
		{"Uçak güneybatıdan güney Hint Okyanusu'na doğru ilerledi.", "Uçak güneybatıdan güney Hint Okyanusu'na doğru ilerledi."},
		{"batı anadolu kıyılarında zeytinlikler yaygın.", "Batı Anadolu kıyılarında zeytinlikler yaygın."},
		// 4. No capitals guessed for uncertain mixed-case institutions.
		{"Dicle Gençlik kulübü sezon sonunda ligden çekilmiştir.", "Dicle Gençlik kulübü sezon sonunda ligden çekilmiştir."},
		{"Parti, Sırp Listesi partisini eleştirdi.", "Parti, Sırp Listesi partisini eleştirdi."},
		{"Eike Geisel, Yahudi kültür Birliği hakkında belgeseller yayınladı.", "Eike Geisel, Yahudi kültür Birliği hakkında belgeseller yayınladı."},
		{"Millî Eğitim Özel İhtisas komisyonu raporu yayımlandı.", "Millî Eğitim Özel İhtisas komisyonu raporu yayımlandı."},
		{"Paşa, Londra Büyük elçiliği esnasında çalıştı.", "Paşa, Londra Büyük elçiliği esnasında çalıştı."},
		{"Sonraki 3 yıl ise PTT genel müdürlüğünde avukatlık yaptı.", "Sonraki 3 yıl ise PTT genel müdürlüğünde avukatlık yaptı."},
		{"Aslında Boston Üniversitesi iletişim bölümünden mezun oldum.", "Aslında Boston Üniversitesi iletişim bölümünden mezun oldum."},
		{"Şenlikler eskiden Eskimo köy odası içinde olurdu.", "Şenlikler eskiden Eskimo köy odası içinde olurdu."},
		// Real lowercase and canonical institutions still get capitals.
		{"Boğaziçi üniversitesinde yüksek lisans yapıyor.", "Boğaziçi Üniversitesinde yüksek lisans yapıyor."},
		{"Pastayı köşedeki yıldız pastanesinden aldım.", "Pastayı köşedeki Yıldız Pastanesinden aldım."},
		// 5. An inflected institution head does not make a predicate's copula a place.
		{"Mekân Atatürk Lisesi’nin sokağıdır.", "Mekân Atatürk Lisesinin sokağıdır."},
		// 6. A common participle before a full name already capitalized gets no capital.
		{"Sezon sonunda sözleşmesi sona eren Okan Buruk takımdan ayrıldı.", "Sezon sonunda sözleşmesi sona eren Okan Buruk takımdan ayrıldı."},
		{"Çocukken barış Manço'nun şarkılarını dinlerdik.", "Çocukken Barış Manço'nun şarkılarını dinlerdik."},
		// 7. Special days of two words and titles of everyday words, by context.
		{"Bugün öğretmenler günü, çiçek almam lazım.", "Bugün öğretmenler günü, çiçek almam lazım."},
		{"Öğretmenler günü için sınıfımızla küçük bir sürpriz hazırladık.", "Öğretmenler günü için sınıfımızla küçük bir sürpriz hazırladık."},
		{"Öğretmenler günü kutlu olsun!", "Öğretmenler Günü kutlu olsun!"},
		{"gaziler günü töreninde müdürümüz konuştu.", "Gaziler Günü töreninde müdürümüz konuştu."},
		{"Ayılar kış uykusuna yattı.", "Ayılar kış uykusuna yattı."},
		{"Dün akşam kış uykusu filmini izledim.", "Dün akşam Kış Uykusu filmini izledim."},
		// 8. English plurals and brands with a low capital share are kept; the TDK's fixes are made.
		{"Sharks'ların yapması gereken tek şey topu tutmak.", "Sharks'ların yapması gereken tek şey topu tutmak."},
		{"O aptal Volvo'ları bozulup duruyor.", "O aptal Volvo'ları bozulup duruyor."},
		{"Hacettepe Üniversitesi'nde görev yaptı.", "Hacettepe Üniversitesinde görev yaptı."},
		{"Eserleri Fransızca'ya çevrildi.", "Eserleri Fransızcaya çevrildi."},
		{"Modern Berliner Weisse'ler üretilir.", "Modern Berliner Weisseler üretilir."},
		{"Kaynak olabilecek sunucu'ların adresleri belli.", "Kaynak olabilecek sunucuların adresleri belli."},
		{"Yangından sonra Mustafa ağa'nın babası geldi.", "Yangından sonra Mustafa Ağa'nın babası geldi."},
		{"Muratın oğlu uzun süre hüküm sürmüştür.", "Murat'ın oğlu uzun süre hüküm sürmüştür."},
		// 9. A parenthesis after a full stop.
		{`Bu adıma "tick" adı verilir. (başka bir deyişle, dağılım sonucudur).`, `Bu adıma "tick" adı verilir. (başka bir deyişle, dağılım sonucudur).`},
	})

	// The context model is not asked about quoted glosses or stems also written lowercase elsewhere.
	var ambig1 []AmbigCandidateQuery
	fixFull(`Helyum (Yunanca "güneş" anlamına gelen helios), kimyasal elementtir.`, &ambig1)
	if slices.ContainsFunc(ambig1, func(q AmbigCandidateQuery) bool { return q.Base == "güneş" }) {
		t.Errorf("a quoted gloss was asked about: %+v", ambig1)
	}
	var ambig2 []AmbigCandidateQuery
	fixFull("Olay, savaşa neden olduğu için, savaş başladıktan sonra arttı.", &ambig2)
	if slices.ContainsFunc(ambig2, func(q AmbigCandidateQuery) bool { return q.Base == "savaşa" || q.Base == "savaş" }) {
		t.Errorf("a stem written lowercase elsewhere was asked about: %+v", ambig2)
	}
}
