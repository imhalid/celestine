# 15 — Bloklar denemesi: richtext üstünde ne olur, ne olmaz (5 Kasım 2026)

[14](14-urun-onerileri.md)'teki U4 uyarısının cevabı. Soru: Aşama B'yi (bloklar) MyGo'nun `ui/richtext` editörü
üstünde mi kuracağız, yoksa kendi blok modelimizi mi yazacağız?

**Karar: richtext üstünde devam.** Blokların çoğu kuruldu ve çalışıyor; olmayanların çoğunun da bir yolu var.
Gerçekten olmayan iki şey var (sütunlar, kutunun içinde kutu); onlar için kendi modelimizi yazmaya değmez.

## Yapıldı (bu denemede)

| Özellik | Nasıl | Dosya |
|---|---|---|
| **Blok** = paragraf + altındakiler | Liste öğesi + daha içerideki öğeler; başlık + bir sonraki aynı/üst başlığa kadar; kod ve alıntı dizisi | `blockops.go` |
| **Taşıma** ⌘⇧↑ ⌘⇧↓ | Blok bir önceki/sonraki blokla yer değiştirir, caret onunla gider; tek ⌘Z | `blockops.go` |
| **Tutamak** ⋮⋮ | İmlecin üstünde olduğu paragrafın solunda belirir | `blockui.go` |
| **Blok menüsü** | Dönüştür (metin, başlık 1–3, listeler, görev, alıntı, kod, ayırıcı), Çoğalt, Yukarı/Aşağı taşı, Sil | `blockui.go` |
| **Sürükle-bırak** | Tutamaktan tutulan blok kalkar, ilk satırının kopyası imleci izler ("+ altındakiler"), bırakılacak yerde çizgi | `blockui.go` |
| **`/` menüsü** | Satır başında `/`; yazdıkça süzülür; ↑↓ ↵ Esc | `slash.go` |
| **Blok menüsü çubukta** | Blok › Yukarı taşı ⌘⇧↑, Aşağı taşı ⌘⇧↓, Çoğalt ⌘D | `menu.go` |

Teknik not: editör ↑↓↵ tuşlarını kendisi alıyor; `/` menüsü bunları *sonucundan* okuyor (↵ satırı böldüyse geri
birleştirip seçimi uyguluyor, caret satırdan çıktıysa geri koyup seçimi kaydırıyor). Çalışıyor ve test edildi, ama
kırılgan bir yöntem: MyGo'ya "tuşu önce bana sor" kancası eklenirse sadeleşir.

## Sonra yapıldı

- **`/` ve `@` tek menü** (13 · 24–27): iki işaret aynı gruplu listeyi açar, taşmaz, işaret göstermez, ad önce gelir.
- **Çipler ve `@` menüsü** (`chips.go`, `slash.go`): `[[Sayfa]]`, `[[Sayfa|takma ad]]` ve `[[2026-11-05]]` metinde
  çip olarak görünür, dosyaya aynen yazılır; tıklayınca sayfa açılır, olmayan sayfa oluşturulur. `@` ile sayfa,
  tarih (bugün, yarın…, `2026-11-10`) ve sayfanın seçim özelliklerine değer.

## Yolu var (sıradakiler)

- **Ekler (attachment) — en önemli bulgu.** richtext metnin içine bizim çizdiğimiz, boyutunu bizim verdiğimiz kutular
  koyabiliyor (`richtext.Attachment{Width, Height, Draw, Data}`). Bununla:
  - **Satır içi çipler:** `@yarın` tarihi, `[[sayfa]]` bağlantısı, kişi, özellik değeri.
  - **Resimler:** `![](a.png)` gerçek resim olarak.
  - **Tablolar:** ham bloktaki tablo, bir ek olarak gerçek tablo şeklinde çizilir; tıklayınca ham Markdown'a döner.
  - **Katlanmış bölüm çipi:** "… 5 satır".

  Dikkat: ekler Markdown'a `![ad]()` olarak yazılıyor. Her çip türünün kendi Markdown'ını bizim yazmamız gerekir
  (`markdown.go`'daki ham blok gibi).
- **Katlama.** richtext'te gizli paragraf yok. Yol: katlanan bloğun alt satırları belgeden alınır, yerine bir çip
  konur (tek bir düzenleme, ⌘Z ile geri alınır). Kaydederken çip açılıp gizli satırlar yazılır. Katlı durum
  dosyaya değil uygulamaya kaydedilir.
- **Bilgi kutuları (callout).** İçinde liste olan bir kutu olmaz (aşağıya bak). Ama Obsidian'ın `> [!note]` kutusu
  olur: alıntı paragrafları + ilk paragrafa kimlik + üstten yarı saydam renk ve ikon (bulma vurgularındaki gibi).
  Kutunun içinde düz metin olur; madde listesi olmaz.
- **`@` menüsü.** `/` ile aynı yapı; içeriği tarih, sayfa bağlantısı, özellik değeri (yukarıdaki çiplerle).
- **Blok seçimi** (Esc ile bloğu seçmek, ⇧↑↓ ile genişletmek). Seçimi bizim tutmamız ve çizmemiz gerekir;
  taşıma, silme ve kopyalama işlemleri hazır.

## Olmuyor

- **Sütunlar** (blokları yan yana dizmek). richtext tek sütunlu bir paragraf akışı; Markdown'da da karşılığı yok.
  Kendi blok modelimiz olmadan olmaz.
- **İçinde blok olan blok:** içinde liste olan kutu, açılır/kapanır liste (toggle) gibi. Aynı sebep. Toggle'ın
  yerini katlama tutuyor.
- **Sürüklerken diğer satırların kayarak yer açması** (ağaçtaki Ryugo efekti). Editörün paragrafları ayrı öğeler
  değil, tek tek kaydıramıyoruz. Yerine çizgi ve kalkan kopya var.

## Öneri

1. Ekler: tarih ve `[[sayfa]]` çipleri, ardından `@` menüsü. Bağlantılar özelliğinin (14 · 7) temeli de bu.
2. Katlama (çiple).
3. Tablonun ve resmin ek olarak çizilmesi.
4. Obsidian not kutusu.
5. Sütunlar ve iç içe bloklar: yapmayalım. Kullanıcılar gerçekten isterse kendi blok modelimizi konuşuruz.
