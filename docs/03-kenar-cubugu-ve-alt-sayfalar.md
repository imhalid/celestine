# 03 — Kenar çubuğu ve alt sayfalar

Alt sayfalar, kenar çubuğu ağacı, `+` ve `…` düğmeleri ve sürükleyerek taşıma yapıldı. Kalanlar Thymer'deki karşılıklarıyla:

Kenar çubuğundaki sayfa menüsünden [görüldü] bizde olmayanlar: **Set Title & Icon** (sayfaya ikon), **Pin** (sayfa sabitleme;
bizde yalnız koleksiyon sabitlenir).

## Kenar çubuğunda nasıl gösterileceği [görüldü]

Koleksiyon ayarları ▸ **Pages** ▸ **Display in Sidebar** — "Bu koleksiyon ve sayfaları kenar çubuğunda nasıl gösterilsin".
Bizde olmayan modlar:

| Mod | Ne yapar |
|---|---|
| Don't show pages | Yalnız koleksiyon görünür |
| By date created | Oluşturma tarihine göre gruplar |
| By date modified | Değiştirme tarihine göre gruplar |
| Group by Property… | Bir özelliğe göre başlıklar altında (ör. Status) |
| Nest by Property… | Bir özelliğe göre iç içe (ör. "Proje" → altında işler) |
| Hide completely | Koleksiyon kenar çubuğunda hiç görünmez |

Aynı sekmede ayrıca (bizde yok):
- **Show the pages in this collection in the Command Palette?** — sayfalar ⌘K "Jump to"da çıksın mı.
- **Sort in Sidebar** — hangi özelliğe göre ve hangi yönde sıralansın (ör. Modified ↓).
- **Related Section** — her sayfanın tepesinde ilgili öğeleri gösteren bir sorgu ("@" ile seçenekler).

**Nest by Property**: "Sayfa bağlantısı" tipinde bir özellik (ör. "Proje") ile sayfalar, başka bir koleksiyonun sayfalarının
altında kenar çubuğunda iç içe gösterilebilir. Kod tarafında "By linked page" ve "By link collection" adlarıyla geçiyor.
Önce Page Link özellik tipi gerekir ([02](02-veri-modeli.md)).

## Bizim editör için [öneri]

1. "Display in Sidebar" modlarından sıradakiler: **By date modified**, sonra Group/Nest by Property ve gizle.
2. "Notu koleksiyona çevir" komutunu biz ekleyebiliriz (Thymer'de yok): seçilen notun alt sayfalarını yeni bir koleksiyona
   taşır, not da koleksiyonun açıklama sayfası olur. Ters yönü de: "Koleksiyonu bir sayfanın altına topla".
