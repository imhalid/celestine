# 05 — Not ekleme akışları

Thymer'de yeni bir not açmanın çok yolu var; hepsi aynı işlemi (bir koleksiyona kayıt eklemek) farklı yerlerden çağırır.

## Yollar [görüldü]

| # | Nereden | Ne olur | Kısayol |
|---|---|---|---|
| 1 | Koleksiyon sayfası, sağ üstte **New Page** | Bu koleksiyona yeni sayfa | — |
| 2 | Koleksiyon listesinin altında **+ New Page** | Aynı; satırın yanında kısayolu yazar | ⇧↵ |
| 3 | Durum çubuğu **Create Page** / **New page…** | Bir koleksiyon seçtirir ("New Page in…") | ⇧↵ / ⌃N |
| 4 | Kenar çubuğu, koleksiyonun yanındaki `+` | O koleksiyona yeni sayfa | — |
| 5 | Kenar çubuğu, bir sayfanın yanındaki `+` | O sayfanın **alt sayfası** ([03](03-kenar-cubugu-ve-alt-sayfalar.md)) | — |
| 6 | Sayfa / kenar çubuğu `…` → **Create sub-page** | Alt sayfa | — |
| 7 | Koleksiyon `…` → **New Page** | Bu koleksiyona yeni sayfa | — |
| 8 | Metin içinde **Link to new page…** | Yeni sayfa oluşturur ve buraya bağlantısını koyar | — |
| 9 | **Quick Add** (⚡, kenar çubuğu) | Bugünün günlüğünün sonuna **saat damgalı yeni bir satır** ekler ve oraya odaklanır | — |
| 10 | Varsayılan koleksiyon | "New page in default collection"; ayarlardan **Default Collection** seçilir | ⌃N |

Notlar:
- Yeni sayfanın adı başta "Untitled Page" (koleksiyonun ayarındaki "What do you call a page in this collection?" ile
  "Untitled Recipe" gibi kişiselleşebilir).
- Quick Add bir not değil, **günlüğe** hızlı bir giriş. İnceleme sırasında bu düğmeye basınca bugünün günlüğüne gerçekten
  "21:25" diye bir satır eklendi; yani "hızlı yakala" düğmesi tek tıklamada veri oluşturuyor.

## Koleksiyon oluşturma [görüldü + kod]
- Kenar çubuğu, **Collections** başlığının `+`'sı → **New Collection**.
- Collections `…` → **New Collection** ya da **New Dynamic Collection**.
- Komut paleti: "New Collection…", "New Dynamic Collection…".
- Yeni koleksiyon "Untitled Collection" adıyla ve tek bir görünümle (liste) başlar; ayarlar sonra yapılır.

## Bizim editör için [öneri]

**Tek bir "yeni sayfa" işlemi, birçok giriş noktası:**
```go
func (s *Store) NewPage(in GUID /* koleksiyon */, parent GUID /* alt sayfa ise */, title string) GUID
```
Bütün düğmeler, kısayollar ve MCP aracı bunu çağırır; geri alma, geçmiş ve kenar çubuğu güncellemesi tek yerde olur.

**Adım adım:**
1. `NewPage` ve `NewCollection` işlemlerini (geri alınabilir) yaz.
2. Giriş noktalarını sırayla ekle: koleksiyon sayfasındaki düğme ve listenin altındaki satır (⇧↵), kenar çubuğu `+`
   (koleksiyon ve sayfa), ⌃N (varsayılan koleksiyon + koleksiyon seçici), `…` menüleri.
3. Yeni sayfa açıldığında caret doğrudan başlıkta olsun; başlığa Enter basınca ilk satıra geçsin.
4. Hareket: listeye yeni satır yerinde büyüyerek gelir, kenar çubuğunda üst düğüm açılır ve yeni satır vurgulanıp söner.
5. Quick Add'i "hızlı yakala" olarak düşün: küçük bir giriş kutusu açsın, kullanıcı yazıp Enter'a basınca kaydetsin.
   Thymer'deki gibi tek tıklamada boş bir satır oluşturmasın (yanlışlıkla basınca çöp bırakıyor).

## Hızlı not (Kasım 2026)

Küçük bir panel, tüm pencerelerin üstünde, genel bir tuşla açılır: **⌘⌥N** (olmazsa **⌥⇧N**; Linux/Wayland'da masaüstü
tuşu portal üzerinden bağlar ve onay isteyebilir). Uygulamada Git ▸ Hızlı not… ve komut paletindeki "Hızlı not" da açar.
Panel dışına tıklayınca ya da Esc ile kapanır, yazılan taslak durur.

- **Günlüğe** (varsayılan): not, bugünün günlük sayfasının sonuna eklenir (sayfa yoksa yapılır). Her satır bir madde olur;
  `[]` ile başlayan satır görevdir (`- [ ]`), görev görünümünde çıkar. Sayfa açıksa editörde de görünür.
- **Yeni sayfa**: ilk satır başlık olur; sayfa gösterilen koleksiyona, yoksa ilk günlük-dışı koleksiyona yapılır.
- ⌘↵ (Ctrl↵) ekler, ↵ yeni satır açar.
