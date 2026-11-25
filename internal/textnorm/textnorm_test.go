package textnorm

import "testing"

func TestFoldAndTurkishASCII(t *testing.T) {
	if got := Fold("Çağrı İLE Şeker ÖlçÜ"); got != "cagri ile seker olcu" {
		t.Fatalf("Fold got %q", got)
	}
	if got := Fold("Plain ASCII"); got != "plain ascii" {
		t.Fatalf("Fold ASCII got %q", got)
	}
}

func TestPlainMarkdown(t *testing.T) {
	md := "# Başlık\n> alıntı **kalın** [[Hedef|görünen]]\n- [ ] görev bir | iki\n" + ListMarkStart + "dots-->\n```\nkod\n```\n"
	got := Plain(md)
	if got != "Başlık alıntı kalın görünen görev bir iki kod" {
		t.Fatalf("Plain got %q", got)
	}
}

func TestMatch(t *testing.T) {
	if Match("ypl", "Yapılacaklar") <= 0 {
		t.Fatal("expected subsequence match")
	}
	if Match("cilek", "Çilek") <= Match("cilek", "reçel çilek") {
		t.Fatal("expected prefix match to outscore later word match")
	}
	if Match("xyz", "abc") != -1 {
		t.Fatal("expected -1 on miss")
	}
}
