# 02 — Veri modeli

Thymer'in en önemli fikri: **çalışma alanı tek bir ağaçtır**. Koleksiyon, kayıt (not), satır — hatta eklentiler — aynı
ağacın düğümleridir. Bağlantılar (referans, gömme, bahsetme) ağacın üstünde zayıf bağlardır; doğruluk kaynağı ağaçtır. [kod]

## Kavramlar [kod]

| Kavram | Ne | Örnek |
|---|---|---|
| **Workspace** (çalışma alanı) | Bütün ağacın kökü; bir ekip ya da kişi | `almasada` |
| **Collection** (koleksiyon) | Kayıtlardan oluşan bir veritabanı; görünümleri, özellik şeması ve ayarları var | Clips, Docs, C Dersleri |
| **Record / Page** (kayıt, sayfa, not) | Koleksiyondaki bir sayfa: başlık, özellikler ve satırlar | "SSH Bağlantı Rehberi" |
| **Line item** (satır) | Sayfanın içindeki bir blok: metin, görev, başlık, liste… İç içe sonsuz derinlikte olabilir | bir görev, bir başlık |
| **Property** (özellik) | Her düğümün kendi özellikleri olabilir; koleksiyonun şeması tablo sütunlarını tanımlar | Status, Due date |
| **Journal** (günlük) | Kişi başına günlük sayfaları olan özel bir koleksiyon | "Wed Nov 4" |

### Bir satırın saklanan hâli [kod]
```
guid    26 karakter (Crockford base32 benzeri)
type    text | task | heading | ulist | olist | quote | block | ref | transclusion | query | image | file | hr | table…
pguid   üst düğüm
cguids  çocukların sırası
ts      metin segmentleri: ["text","merhaba", "bold","dünya", "ref",<guid>, …]
kv, mp  özellikler ve meta (başlık boyutu, blok stili, dil…)
c_at/u_at/c_by/u_by   oluşturma/değiştirme zamanı ve kişisi
dlt     silinme bayrakları (yumuşak silme: çöp kutusu)
```

### Kayıtların yerleşik özellikleri [kod]
`title`, `icon` (`ti-star` gibi bir ikon sınıfı), `banner`, `created_at`, `updated_at` ve alt sayfalar açıksa
**`Sub-page of`** (üst sayfa). [görüldü: Clips koleksiyonunun Properties sekmesinde]

### Satır tipleri [kod]
`text`, `task`, `heading` (1–6), `ascii-banner`, `quote`, `block` (stiller: yok, quote, warning, note, row = sütunlar),
`olist`, `ulist`, `hr`, `image`, `file`, `ref` (başka kayda bağlantı), `table`/`table-row`/`table-cell`,
`transclusion` (başka yerden gömülü içerik), `query` (canlı sorgu). `hr`, `image`, `file`, `transclusion`, `query`
yaprak tiplerdir (çocukları olamaz).

### Metin segmentleri (zengin metin) [kod]
`text`, `bold`, `italic`, `code`, `link`, `linkobj` ({link, title}), `hashtag`, `icon`, `mention` (kullanıcı),
`ref` (kayıt), `datetime` ("tomorrow 3pm", "monday to friday", "Q1 2024" gibi doğal dilde tarih).

### Özellik tipleri [görüldü + kod]
Arayüzdeki "Add property" menüsü: **Choice, Number, Text, File, Image, Date & Time, URL, Page Link, User, Tag**.
Kod tarafında bunlara ek olarak `banner` ve `dynamic` (formülle hesaplanan) var.

### Görev durumları [kod]
`none`, `done`, `started`, `blocked` (waiting), `billable`, `important`, `discuss`, `alert`, `starred`, `canceled`.

### Atama örtüktür [kod]
Görevlerin tek bir "atanan" alanı yok. Bir görevde birinden bahsedilirse (`@kişi`) o kişinin; birinin günlüğündeyse onun;
bir kaydın "user" özelliği varsa içindeki görevler de o kişinin sayılır.

## Bizim editör için [öneri]

Aynı fikri alıyoruz, iki fark ile:

1. **Dosya da birinci sınıf.** Thymer'in "Markdown Mirror"ı (bkz. [07](07-ayarlar-paneli.md)) her sayfayı bir Markdown dosyasına,
   her koleksiyonu bir klasöre yansıtıyor. Biz bunu en baştan modelin parçası yapalım: depolama, bir veritabanı (hız ve
   sorgu için) ile bir klasör/Markdown görünümü arasında iki yönlü eşitlenir.
2. **Önce yerel, tek kullanıcı.** Thymer'in eşitleme ve şifreleme motoru çok büyük bir iş. Model, ileride eklenebilecek
   şekilde kurulur (her değişiklik küçük, geri alınabilir bir işlem: `insmov`, `update`, `set_kv`) ama ilk sürüm yereldir.

Önerilen Go tipleri (taslak):
```go
type GUID string

type Node struct {           // ağaçtaki her şey
    ID       GUID
    Kind     NodeKind        // Workspace, Collection, Record, Line
    Parent   GUID
    Children []GUID
    Type     LineType        // yalnız satırlar için: Text, Task, Heading…
    Segments []Segment       // zengin metin
    Props    map[string]Value
    Meta     map[string]any  // başlık boyutu, blok stili, dil…
    Created, Updated time.Time
    Trashed  bool
}

type Op interface{ Apply(*Tree) (undo Op) }   // her değişiklik bir işlem; geri alma = ters işlem
```

---

## Kitaplık tek dosyada (Kasım 2026)

Kitaplık artık klasör değil, uygulamanın yanındaki **tek bir SQLite dosyası**: `kitaplik.db`. Sayfalar, koleksiyonlar,
çöp, ayarlar ve MCP anahtarı hepsi içinde; başka hiçbir program klasörde `.md` göremez, uygulamayı taşımak dosyayı da taşımaktır.

- Kod aynı kaldı: kitaplık hâlâ klasör/dosya ağacı gibi davranıyor, ama `FS` arayüzünün (`store.go`) arkasında. İki zemini var:
  `osFS` (bir klasör) ve `Store` (tek dosya). Komut satırında bir **klasör** verilirse eskisi gibi düz dosyalarla çalışır.
- İlk açılışta `kitaplik.db` yoksa ve yanında (ya da çalışılan yerde) bir `kitaplik/` klasörü varsa, olduğu gibi dosyaya alınır;
  klasöre dokunulmaz, yedek olarak kalır.
- Ayarlar ▸ Kitaplık: dosyanın yolu ve **Markdown olarak dışa aktar** (dosyanın yanına `kitaplik-md-…` klasörü yazar; bu klasör
  `ryudit <klasör>` ile yine açılır).
- Dosya şifreli değil. Dışarıdan okunamaması, düz dosya olmaması kadardır; parola istenirse sonraki iş (SQLCipher ya da
  alan şifreleme).
- Dışarıdan düzenleme izlemesi (`watch.go`) dosya modunda kapalı: dosyayı uygulamadan başka kimse yazmaz.

## Resimler

Bir resim, sayfanın klasöründe `resim-20261105-135012.png` gibi bir dosyadır (kitaplık dosyasının içinde) ve sayfanın
Markdown'unda `![alt](resim-….png)` satırıdır; başka Markdown araçları da böyle okur. Editörde kendi paragrafında, en çok
640 px genişliğinde, köşeleri yuvarlak görünür. Dosya yoksa ya da resim değilse satır korunur, yerinde etiketli bir kutu çizilir.

- **Eklemek:** metne dosya sürükleyip bırakmak; `/` ▸ Ekle… ▸ Resim…; panoda yalnız resim varken ⌘V (pano 0,5 sn'de bir
  biçimlerine bakılır: resim var, metin yoksa ⌘V uygulamanındır, yoksa editörün).
- **Büyütmek:** resme tıklayınca pencereye sığacak boyda, karartılmış bir sayfanın üstünde açılır; tıklama ya da Esc kapatır.
- **Silmek:** resmin satırını silmek dosyayı silmez; dosya sayfanın klasöründe kalır (sayfa çöpe gidince birlikte gider).

## Sürüm geçmişi

Tek dosya kitaplığı her sayfanın önceki hallerini tutar (`versions` tablosu): bir sayfanın dosyası üzerine yazılırken, son kopyadan
5 dakikadan fazla geçmişse eski hali kopyalanır; sayfa başına en çok 100 kopya (eskisi gider). Metinsiz (yalnız ön madde) halleri
tutulmaz. Kopyalar sayfa taşınınca ya da çöpe gidince onunla gider, kalıcı silinince silinir. Klasör kitaplığı kopya tutmaz.

Sayfa menüsü ▸ **Sürüm geçmişi…**: solda kopyalar ("12 dk önce", saat, boyut), sağda seçilenin sayfanın şimdiki halinden farkı
(geri gelen satırlar yeşil, giden satırlar kırmızı üstü çizili; uzun aynı bölümler kısaltılır). **Bu sürüme dön** sayfayı o hale
getirir; dönmeden önceki hal de kopya olarak saklanır, yani dönüş de geri alınabilir. Başlık ve özellikler şimdiki halinde kalır.

Sürüm geçmişine sonradan: değişen satırların yanında, bir satır gidip bir satır gelirken **hangi kelimelerin** değiştiği de
işaretlenir (kelime farkı); pencerenin altındaki alandan o anki hâl **adla saklanır** ("Yayına giden"): adlı kopya 5 dakika
kuralına bakmaz ve 100 kopya sınırı yüzünden hiç silinmez. `versions` tablosuna `label` sütunu eklendi; eski dosyalarda ilk
açılışta kendiliğinden eklenir.
