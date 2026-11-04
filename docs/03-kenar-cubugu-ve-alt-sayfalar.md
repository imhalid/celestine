# 03 — Kenar çubuğu ve alt sayfalar

Senin beğendiğin yapı: bir notun yanında `+` var, ona basınca o notun **altına** yeni bir not ekleniyor. Thymer'de bunun
adı **alt sayfa (sub-page)**.

## Alt sayfalar nasıl çalışıyor [görüldü]

### 1. Koleksiyon bazında açılır
Alt sayfa her koleksiyonda yoktur; koleksiyon ayarlarında açılır:

> Koleksiyon `…` → **Collection Settings** → **Pages** sekmesi → **Enable Sub-Pages**
> "Allow pages to be nested under other pages in this collection"

Bu ayar açık olan koleksiyonların sayfaları kenar çubuğunda **iç içe konabilir** (kodda `sidebar-item-nestable`) ve
yanlarında `+` belirir. Örneğin Clips, Docs, C Dersleri ve Dreams'te açık, Notes'ta kapalı: Notes'un sayfalarında `+` yok.

### 2. Alt sayfa ilişkisi bir özelliktir
Açıldığında koleksiyona yerleşik bir özellik eklenir: **`Sub-page of`** (üst sayfa). Yani alt sayfa ayrı bir yapı değil,
bir sayfanın "üstüm şu sayfa" demesidir. Bu iki şey sağlar:
- Sayfayı başka bir üstün altına taşımak, sadece bu özelliği değiştirmektir ("Make sub-page of…").
- Aynı koleksiyonun içinde kalır; koleksiyonun görünümleri (tablo, pano…) alt sayfaları da normal kayıt gibi gösterir.

### 3. Alt sayfa oluşturmanın yolları
| Nerede | Ne | Kısayol |
|---|---|---|
| Kenar çubuğu, sayfanın yanındaki `+` | "New Page…" — bu sayfanın altına | — |
| Kenar çubuğu, sayfanın `…` menüsü | **Create sub-page** | — |
| Sayfanın başlığının yanındaki `…` menüsü | **Create sub-page** | — |
| Komut paleti | "New sub-page of …", "Make sub-page of…" | ⌘P |

Kenar çubuğundaki sayfa menüsünün tamamı [görüldü]: **Open**, **Open Aside**, **Set Title & Icon** (F2), **Version
History…**, **Change Banner…**, **Move…**, **Move to Trash** (⌦), **Create sub-page**, **Pin**.

## Kenar çubuğunda nasıl gösterileceği [görüldü]

Aynı **Pages** sekmesinde **Display in Sidebar** — "Bu koleksiyon ve sayfaları kenar çubuğunda nasıl gösterilsin":

| Mod | Ne yapar |
|---|---|
| Don't show pages | Yalnız koleksiyon görünür |
| List | Sayfalar düz liste |
| **Sub-pages** | Sayfalar üst–alt ağacı olarak (alt sayfa açıkken) |
| By date created | Oluşturma tarihine göre gruplar |
| By date modified | Değiştirme tarihine göre gruplar |
| Group by Property… | Bir özelliğe göre başlıklar altında (ör. Status) |
| Nest by Property… | Bir özelliğe göre iç içe (ör. "Proje" → altında işler) |
| Custom | Kodla (eklenti) |
| Hide completely | Koleksiyon kenar çubuğunda hiç görünmez |

Aynı sekmede ayrıca:
- **Show the pages in this collection in the Command Palette?** — sayfalar ⌘K "Jump to"da çıksın mı.
- **Sort in Sidebar** — hangi özelliğe göre ve hangi yönde sıralansın (ör. Modified ↓).
- **Related Section** — her sayfanın tepesinde ilgili öğeleri gösteren bir sorgu ("@" ile seçenekler).

## "Bir notu koleksiyon yapmak" [görüldü + kod]

Thymer'de bir not doğrudan koleksiyona dönüştürülmüyor. Bu ihtiyacı karşılayan üç şey var:

1. **Alt sayfalar**: Bir not, altındaki notların kabı olur. Kenar çubuğunda klasör gibi açılır/kapanır; "notu koleksiyon gibi
   kullanmak" çoğu zaman bu demek.
2. **Nest by Property**: "Sayfa bağlantısı" tipinde bir özellik (ör. "Proje") ile sayfalar, başka bir koleksiyonun
   sayfalarının altında kenar çubuğunda iç içe gösterilebilir. Kod tarafında "By linked page" ve "By link collection"
   adlarıyla geçiyor.
3. **Dynamic Collection** (dinamik koleksiyon): Kendine ait kaydı olmayan, başka koleksiyonlardan **sorguyla** sayfa
   toplayan sanal bir koleksiyon (ör. "tüm koleksiyonlardaki #okunacak sayfalar"). Kenar çubuğunda Collections `…` →
   "New Dynamic Collection". Görünümleri `source_collections: ["*"]` gibi bir kaynak listesi taşır. Ayrıntı [06](06-koleksiyonlar-ve-gorunumler.md)'da.

## Bizim editör için [öneri]

**Adım adım:**
1. Kayda yerleşik bir `parent` alanı ekle (Thymer'deki `Sub-page of`). Koleksiyon ayarında `subPages bool`.
2. Kenar çubuğunu bir **ağaç görünümü** olarak yaz: düğüm = koleksiyon ya da sayfa; çocuklar `parent` alanından gelir.
   Satır yüksekliği sabit, yalnız görünen satırlar çizilir (sanallaştırma; mizan-go Table'daki gibi).
3. Her sayfa satırına üzerine gelince görünen iki düğme: `+` (altına yeni sayfa) ve `…` (yukarıdaki menü). Sürükleyip
   başka sayfanın üstüne bırakınca o sayfanın altına girsin (`parent` değişir); arasına bırakınca sıra değişir.
4. "Display in Sidebar" modlarından ilk sürümde üçü yeter: **Sub-pages**, **List**, **By date modified**. Diğerleri
   (Group/Nest by Property, gizle) sonra.
5. "Notu koleksiyona çevir" komutunu biz ekleyebiliriz (Thymer'de yok): seçilen notun alt sayfalarını yeni bir koleksiyona
   taşır, not da koleksiyonun açıklama sayfası olur. Ters yönü de: "Koleksiyonu bir sayfanın altına topla".
6. Hareket: bir alt sayfa eklenince üst düğüm yayla açılır, yeni satır yerinden büyüyerek gelir; sürüklerken komşular yer
   açar (mizan-go `dnd.go`'daki dil).
