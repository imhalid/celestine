# 15 — Bloklar denemesi: richtext üstünde ne olur, ne olmaz (5 Kasım 2026)

[14](14-urun-onerileri.md)'teki U4 uyarısının cevabı. Soru: Aşama B'yi (bloklar) MyGo'nun `ui/richtext` editörü
üstünde mi kuracağız, yoksa kendi blok modelimizi mi yazacağız?

**Karar: richtext üstünde devam.** Bloklar, taşıma, tutamak, `/` ve `@` menüsü, çipler, katlama, resimler, tablolar ve not
kutuları yapıldı. Gerçekten olmayan iki şey var (sütunlar, kutunun içinde kutu); onlar için kendi modelimizi yazmaya değmez.

Teknik not: editör ↑↓↵ tuşlarını kendisi alıyor; `/` menüsü bunları *sonucundan* okuyor (↵ satırı böldüyse geri
birleştirip seçimi uyguluyor, caret satırdan çıktıysa geri koyup seçimi kaydırıyor). Çalışıyor ve test edildi, ama
kırılgan bir yöntem: MyGo'ya "tuşu önce bana sor" kancası eklenirse sadeleşir.

## Yolu var (sıradakiler)

- **Blok seçimi** (birden çok bloğu seçmek, ⇧↑↓ ile genişletmek; Esc artık klavyenin normal kipine geçiyor, `kb.go`). Seçimi bizim tutmamız ve çizmemiz gerekir;
  taşıma, silme ve kopyalama işlemleri hazır.

## Olmuyor

- **Sütunlar** (blokları yan yana dizmek). richtext tek sütunlu bir paragraf akışı. Kendi blok modelimiz olmadan olmaz.
- **İçinde blok olan blok:** içinde liste olan kutu, açılır/kapanır liste (toggle) gibi. Aynı sebep. Toggle'ın
  yerini katlama tutuyor.
- **Sürüklerken diğer satırların kayarak yer açması** (ağaçtaki Ryugo efekti). Editörün paragrafları ayrı öğeler
  değil, tek tek kaydıramıyoruz. Yerine çizgi ve kalkan kopya var.

Sütunlar ve iç içe bloklar: yapmayalım. Kullanıcılar gerçekten isterse kendi blok modelimizi konuşuruz.
