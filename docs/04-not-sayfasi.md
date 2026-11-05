# 04 — Not sayfası

## Anatomi [görüldü]

```
┌──────────────────────────────────────────────────────────────┐
│  (banner görseli — isteğe bağlı)                              │
│  [</> C Dersleri]          ← koleksiyon kapsülü (geri götürür) │
│  C Dersleri — Müfredat  …  ← başlık + sayfa menüsü              │
├──────────────────────────────────────────────────────────────┤
│  [Müfredat]  +  ⚙          ← özellik kapsülleri, özellik ekle,   │
│                              özellik görünüm ayarları          │
│  ⌄ # Başlık                ← başlıklar katlanabilir (sol ok)     │
│    paragraf…                                                   │
│  ⌄ ┌ blok ───────────────┐                                     │
│    │ iç içe içerik        │                                     │
│    └──────────────────────┘                                     │
│  1. numaralı liste…                                           │
└──────────────────────────────────────────────────────────────┘
Durum çubuğu: … @ Insert…  Aa     Ln 1, Col 1
```

- **Koleksiyon kapsülü**: sayfanın hangi koleksiyonda olduğu; tıklayınca koleksiyona gider. Koleksiyonun kendi sayfasında
  bu kapsül "Collection ⚙" olur ve ayarları açar.
- **Sayfa menüsü (`…`)** [görüldü]: Change Banner…, Version History…, Move to Trash, Move…, Set Title & Icon, Copy Page Link,
  Pin, Create sub-page, Hide Properties Section ve en altta "Edited 1 week ago by …".
- **Özellikler bölümü**: koleksiyonun şemasındaki özellikler kapsül olarak; `+` "Add property…" (Choice, Number, Text, File,
  Image, Date & Time, URL, Page Link, User, Tag), `⚙` "Property view options". Bölüm gizlenebilir.
- **Satırlar**: her satır bir düğüm; başlık ve blok satırlarının solunda katlama oku var. Satırlar girintiyle iç içe girer.

## Satır ekleme ve dönüştürme [görüldü: komut paleti]

Satır tipleri komut paletinden (⌘P) ya da Markdown kısayollarıyla:

| Tip | Ne |
|---|---|
| Text, Task (⌘L) | düz metin, görev |
| Heading 1–3 | başlıklar (model 1–6 destekliyor) |
| Bulleted / Numbered List | listeler |
| Line Quote | tek satır alıntı |
| Console / Flames / Neon Banner | animasyonlu ASCII başlık |
| Link Icon / Button | bağlantıyı düğme ya da ikon olarak göster |
| Plain / Quote / Warning / Info Block | iç içe içerik tutan kutular ("nest anything in a callout box") |
| Columns | blokları yan yana dizer |
| Code Block | sözdizimi renklendirmeli kod |
| Media, Attachment | görsel/video, dosya |
| Live Query | eşleşen sonuçları canlı gösteren sorgu |
| Divider | `---` |

Satır üzerinde işlemler: **Fold** (⌘/), **Zoom in** (⌥↓, satırı tek başına açar), **Open Aside** (⌘↵), **Pin Line**,
**Line Options Menu** (⌃Space), **Indent / Deindent** (⇥ / ⇧⇥), **Indent / Deindent tree** (⌘; / ⌘⇧;),
**Move item up / down** (⌘⇧↑ / ⌘⇧↓), **Bold** (⌘B), **Italic** (⌘I), **Inline Code** (⌘E), **Mention user** (@),
**Date or time**, **Link to…** (sayfalar, koleksiyonlar, satırlar), **Show line info**.

Yapıştırma: Paste as HTML / **Markdown** (⌘⌥V) / Text. Kopyalama: Copy as Markdown / HTML / Text, Copy Reference,
Copy Transclusion (satırı başka yere canlı gömmek için).

## Editörün kendisi [kod — teknik analizden]
- `contenteditable` yok. Gizli bir `textarea` klavyeyi ve giriş yöntemini alır; metin, seçim ve caret ayrı katmanlarda
  çizilir. Uzun sayfalarda yalnız görünen satırlar DOM'a takılır.
- Bizim MyGo demomuz (`slime-md`) da tam bu modeli kullanıyor: `HandleTextInput` ile metin bizde, caret ve seçimi biz
  çiziyoruz.

## Bizim editör için [öneri] — WYSIWYG adımları

1. **Satır = blok.** Belge, satırlardan oluşan bir ağaç. Her satır kendi `TextLayout`'unu tutar; bir düzenleme yalnız o
   satırı yeniden dizer.
2. **Markdown işaretleri yazarken dönüşür**: `# ` başlığa, `- ` listeye, `[] ` göreve, `> ` alıntıya, ```` ``` ```` koda.
   İşaret, caret satırdan çıkınca gizlenir; caret geri gelince görünür (Obsidian'ın "canlı önizleme"si gibi). Demo'daki
   soluk işaretler bunun başlangıcı.
3. **Çok satırlı seçim** satırların üstünden geçer; kopyalama Markdown üretir.
4. **Katlama** ve **zoom in** ağaçtan bedavaya gelir.
5. **Bloklar** (callout, sütunlar, kod) ilk sürümde: kod bloğu ve callout. Sütunlar sonra.
6. **Sanallaştırma**: satır yükseklikleri önbellekte, yalnız görünenler çizilir; 10.000 satırlık bir belge de akıcı olmalı.

## Yan panel (Kasım 2026)

Pencerenin sağ üstündeki panel düğmesi ya da ⇧⌘\ ile açılır, kapanışı hatırlanır (`panelOpen`). Üç bölüm: **İçindekiler**
(sayfanın başlıkları, tıklayınca imleç oraya gider), **Bağlananlar** (bu sayfaya `[[…]]` ile bağlanan sayfalar) ve **Bilgi**
(koleksiyon, oluşturulma ve değişme zamanı, kelime ve harf sayısı, alt/üst sayfa). Koleksiyon ve çöp görünümünde kapalı durur.

## Etiketler (Kasım 2026)

Metinde `#etiket` yazmak yeter: bir kelimenin başında `#`, ardından harfle başlayan harf, rakam, `-`, `_` ya da `/`
(`#proje/alt`). Dosyada başka bir şey tutulmaz; kod bloklarındaki ve `kod` içindeki `#` etiket sayılmaz, `# Başlık` da değil.

- Editörde kelimenin üstüne yumuşak bir hap çizilir (belgeye dokunmaz, geri alma kirlenmez); tıklayınca etiketin sayfası açılır.
- Etiket sayfası: etiketi taşıyan sayfalar (son değişen önde) ve üstte öteki etiketler, sayı ile. Geri/ileri (⌘[ ⌘]) onu da bilir.
- Yan panelde "Etiketler" bölümü sayfanınkileri sayar; komut paletinde (⌘K) her etiket `#ad` olarak aranır.
- Etiket listesi bağlantı dizini ile aynı okumada yapılır, en çok 4 saniye geriden gelir.
- Satırın ortasında `#` yazınca var olan etiketler menüde çıkar (en çok kullanılan önde, yazdıkça daralır; ↵ seçer, yeni ad
  yazılırsa "Yeni etiket"). Satır başındaki `#` başlığındır, menü açmaz; kitaplıkta hiç etiket yokken de açılmaz.

Etiket sayfasında **Yeniden adlandır**: bütün sayfalardaki `#etiket` yazıları değişir (kod içindekiler ve `#etiketler` gibi daha
uzun adlar değişmez); var olan bir etiketin adı yazılırsa ikisi birleşir.

Resimler: bir sayfa bu oturumda ilk kez açılırken, satırı artık hiçbir yerde geçmeyen `resim-…` dosyaları silinir (geri alma
geçmişi yokken, böylece silinen satır geri gelirse dosya yerinde olur). Elle konan başka adlı dosyalara dokunulmaz.

## Kapak resmi (Kasım 2026)

Sayfa menüsü ▸ "Kapak resmi ekle…" ya da başlığın altındaki "Kapak ekle": sayfanın **arka planında**, üstten tüm genişlikte bir
bant (232 px; resim kırpılarak sığar, altına doğru kararır) ve başlık beyaz olarak onun üstünde durur; özellikler ve metin bandın
altından başlar. Resim sayfanın klasöründe `kapak-….png` dosyasıdır ve sayfanın ön maddesinde `cover: kapak-….png` olarak
durur; değiştirilince eski dosya silinir. İmleç bandın üstündeyken sağ üstte "Kapağı değiştir" ve "Kapağı kaldır" düğmeleri çıkar.

Kapağı olan sayfanın koleksiyon görünümlerinde de resmi görünür: panoda kartın üstünde 84 px'lik bir şerit, listede satırın sağında
küçük bir resim.
