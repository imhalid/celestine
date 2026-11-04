# 06 — Koleksiyonlar ve görünümler

## Koleksiyon sayfası [görüldü]
```
[✂ Collection ⚙]
Clips  …
[≡ List] [+]                         New Page | ⚙ filtre | Modified ⌃⌄ | ↓
[🔍 Search in Clips                                                   ]
⌄ Today
   ✂ Untitled Page                                             5h ago
     Merhaba umarım iyisinizdir… (ilk satırların önizlemesi)
+ New Page ⇧↵
```
- Üstte görünüm sekmeleri (List…) ve `+` **Add view**.
- Sağda **New Page**, filtre, sıralama alanı (Modified) ve yönü.
- Koleksiyon içinde arama kutusu.
- Liste, tarihe göre gruplanmış (Today…), her satırda ikon, başlık, zaman ve içerik önizlemesi.

Koleksiyon `…` menüsü: **Collection Settings**, **Change Collection Banner…**, **New Page**, **Views →**.

## Koleksiyon ayarları [görüldü]
Ayarlar yandaki panelde açılır: `Clips / Settings`. Dört sekme ve **Edit as Code**. Altta **Cancel / Preview / Save**:
değişiklikler kaydedilene kadar uygulanmaz, **Preview** önce denemeyi sağlar.

| Sekme | İçerik |
|---|---|
| **General** | Collection Name (ikon seçici + ad), Description, "What do you call a page in this collection?" (ör. Recipe), Move Collection to Trash |
| **Properties** | "Sayfalara yazar, tarih, durum gibi alanlar ekle". Liste: yerleşikler (Title, Modified, Created, Banner, Collection, Sub-page of) ve özeller (ör. URL, Site, Author, Published, Description, Tags, rating), **Add Property**, **Advanced display options** |
| **Views** | "Sayfaları tablo, kanban, galeri gibi görünümlerde göster; istediğin kadar görünüm oluştur". Görünüm listesi + **Add View** |
| **Pages** | Enable Sub-Pages, Display in Sidebar, Command Palette'te göster, Sort in Sidebar, Related Section ([03](03-kenar-cubugu-ve-alt-sayfalar.md)) |
| **Edit as Code** | Koleksiyonun ham JSON yapılandırması |

### Ham yapılandırma (JSON) [kod]
```json
{
  "ver": 1, "name": "Clips", "icon": "ti-scissors", "item_name": "Page", "description": "Pages saved from the web",
  "fields": [{ "id": "…", "label": "Author", "type": "text", "icon": "…", "active": true, "many": false,
               "read_only": false, "choices": [], "number_format": "plain", "filter_colguid": null }],
  "views":  [{ "id": "…", "label": "List", "type": "list", "sort_field_id": "…", "sort_dir": "desc",
               "group_by_field_id": null, "field_ids": ["…"], "query": "" }],
  "show_sidebar_items": true, "show_cmdpal_items": true,
  "sidebar_record_sort_field_id": "…", "sidebar_record_sort_dir": "desc",
  "managed": { "fields": false, "views": false, "sidebar": false }
}
```

## Görünüm tipleri [kod]
| Tip | Ne | Gereken |
|---|---|---|
| `list` | Belge listesi; tarihe ya da sayılabilir bir alana göre gruplanabilir | — |
| `table` | Tablo/hesap tablosu | — |
| `board` | Kanban | bir gruplama alanı (genelde Choice) |
| `gallery` | Kart galerisi (kapak görselleri ya da sayfa önizlemesi) | — |
| `calendar` | Takvim | bir tarih alanı |
| `record` | "Page Shortcut": koleksiyonun içinde bir sayfayı sekme gibi açar | — |

Görünümlerde **Custom Order** var: kayıtlar elle sıralanabilir (tablo, pano sütunları, galeri). Bir kayıt pano görünümünde
yalnızca bir sütunda olabilir: "Allow multiple" açık bir alanla pano gruplanamaz. [kod]

## Dinamik koleksiyon [kod]
- Kendine ait kaydı yoktur; görünümleri `source_collections` (ör. `["*"]` hepsi) ve bir sorguyla başka koleksiyonların
  sayfalarını toplar.
- Varsayılan görünümü "All Pages" adında bir tablo.
- Özellikleri değiştirilemez (kaynak koleksiyonlardan gelir), bu yüzden o özelliklerle pano gruplanamaz.
- Masaüstünde oluşturulur ("Create and configure Dynamic Collections in Thymer on desktop").

## Bizim editör için [öneri]

**Adım adım:**
1. **Koleksiyon şeması**: `Collection{Name, Icon, ItemName, Description, Fields[], Views[], Pages{SubPages, Sidebar, InPalette, Sort}}`.
   JSON olarak saklanır ve dışarı verilebilir (Edit as Code'un karşılığı).
2. **Ayarlar ekranı**: aynı dört sekme. **Taslak → Önizleme → Kaydet** akışı çok iyi: ayar değişikliği önce bir taslakta
   tutulur, önizlemede canlı görünür, kaydedince işlenir. mizan-go'nun Tabs ve Form bileşenleri hazır.
3. **Görünümler**, sırasıyla: `list` (gruplu), `table` (mizan-go'daki sanallaştırılmış tablo hazır), `board` (mizan-go dnd hazır),
   `gallery`, `calendar`.
4. **Alan tipleri**, sırasıyla: Text, Number, Choice, Date, Page Link, URL, Tag; sonra File/Image, User.
5. **Dinamik koleksiyon** = kaydedilmiş bir sorgu + görünümler. Arama motoru ([09](09-arama-gorevler-gunluk.md)) hazır
   olduğunda neredeyse bedava gelir.
6. Her görünümde **elle sıralama** (Custom Order) kalıcı olsun; sürükle-bırak ile.
