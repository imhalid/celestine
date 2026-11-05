# 09 — Arama, görevler, günlük

## Arama sorgu dili [kod: MCP belgesi]
Aynı dil arama kutusunda, canlı sorgu bloklarında, koleksiyonun "Related Section"ında ve dinamik koleksiyonlarda kullanılır.

- **Metin**: yan yana kelimeler VE; tırnakla ifade: `"release notes"`.
- **Mantık**: `AND`, `OR`, `NOT` (`&&`, `||`, `!`), parantez.
- **Tarih**: `@today`, `@tomorrow`, `@monday`, `@"next monday"`, `@thisweek`, `@lastweek`, `@thismonth`, `@"week 10"`,
  `@2025-01-01`, karşılaştırma: `@due <= @today`.
- **Görevler**: `@task`, `@todo`, `@done`, `@due`, `@overdue`, `@assigned`, `@unassigned`, `@scheduled`, bayraklar
  (`@important`, `@inprogress`, `@waiting`…).
- **Kişiler**: `@me`, `@alice`, `@mention`, `@scheduled=@me`.
- **Etiketler**: `#tag`, önek: `#projects/`.
- **Tipler**: `@document`, `@heading`, `@text`, `@quote`, `@list`, `@image`, `@file`.
- **Zaman damgaları**: `@created_at`, `@modified_at`. **Bağlantılar**: `@link`, `@link="github"`.
- **Koleksiyon alanları**: `@collection="Projects"`, `@Project.status="In progress"`, operatörler `= === != < <= > >=`.
- `@due` (görevin kendi tarihi) ile `@scheduled` (birinin günlüğünde olması) farklıdır.

## Görevler [görüldü + kod]
- Kenar çubuğunda **Tasks**: hazır filtreler — All tasks, Due today, Overdue, Important, In progress, Assigned to me,
  Unassigned, Due this week/month… ("Tasks due today, overdue or important").
- Görev durumları: done, started (in progress), blocked (waiting), billable, important, discuss, alert, starred, canceled.
- Görev satırına doğal dilde tarih yazılabilir ("tomorrow 3pm"); "Schedule for today/tomorrow" görevi günlüğe koyar.

## Günlük (Journal) [görüldü]
- Kişi başına her gün bir sayfa ("Wed Nov 4"); başlığın altında **‹ Today ›** ve takvim düğmesi.
- Kenar çubuğunda Today / Yesterday / Previous 7 days / Previous 30 days grupları.
- **Quick Add** bugünün günlüğüne saat damgalı bir satır ekler ([05](05-not-ekleme-akislari.md)).
- Komutlar: Go to Today's Journal, Go to Journal date…, next/previous day, Switch Journal User….

## Bizim editör için [öneri]

**Adım adım:**
1. **İndeks**: her satırın metni, tipi, etiketleri, tarihleri ve bağlantıları için bellekte bir ters indeks; değişen satır
   yalnız kendini günceller. Kalıcı depoda SQLite FTS5 kullanılabilir (saf Go sürücü: `modernc.org/sqlite`, cgo yok —
   mizan-go'nun cgo'suz kapısıyla uyumlu).
2. **Sorgu dili**: önce metin + `#etiket` + `@todo/@done` + `@today`; sonra koleksiyon alanları ve karşılaştırmalar.
   Tek bir ayrıştırıcı, her yerde aynı.
3. **Canlı sorgu bloğu** ve **dinamik koleksiyon** bu motorun üstüne gelir.
4. Günlük ilk sürüm için isteğe bağlı; koleksiyonlar ve alt sayfalar önce.

## Görev görünümü (Kasım 2026)

Kenar çubuğunun altındaki **Görevler** satırı (açık görev sayısı rozetli) ya da komut paleti: kitaplıktaki bütün `- [ ] …`
satırları sayfalarına göre gruplanır (son değişen sayfa önde). Üstte Açık / Tamamlanan / Hepsi süzgeci. Onay kutusuna
tıklamak görevi sayfaya gitmeden işaretler: sayfanın dosyası yazılır, sayfa açıksa editördeki metin de güncellenir.
Sayfanın adına tıklamak sayfayı açar. Kod bloklarındaki görevler sayılmaz; görevdeki `[[bağlantı]]` adıyla gösterilir.
Geri/ileri (⌘[ ⌘]) bu görünümü de bilir. Görevler bağlantı dizininin okumasıyla toplanır; bir kutu işaretlenince liste hemen
yenilenir, başka yerdeki değişiklikler en çok 4 saniye geriden gelir.

Sonra eklenenler: görevin metnine tıklamak sayfayı açıp imleci o satıra koyar; görevdeki `[[2026-11-10]]` günü o görevin günü
olur (hap olarak yanında, gecikmişse kırmızı) ve açık görevler Bugün / Bu hafta / Gecikmiş diye süzülür.

## Kitaplıkta ara (Kasım 2026)

⇧⌘F, Git ▸ Kitaplıkta ara… ya da paletteki "Kitaplıkta ara": bütün sayfaların başlığında ve **metninde** arar. Her kelime
sayfada (başlıkta ya da metinde) bulunmalı; Türkçe harfler ASCII karşılığı gibi sayılır (`cay` ≈ `çay`); başlıkta geçenler ve
çok geçenler önde. Her sonuç sayfanın adını, koleksiyonunu ve bulunduğu yerin bir parçasını gösterir, aranan kelimeler
işaretli. Birine tıklamak sayfayı açar, imleç kelimenin üstüne gelir. Komut paleti (⌘K) de en az 3 harflik aramada, başlık
eşleşmelerinin ardına "Metinde · …" satırları ve "Kitaplıkta ara: …" komutunu ekler.

Sayfaların metni bağlantı dizininin okumasında toplanır (düz, tek satır; Markdown işaretleri ve `[[bağlantılar]]` sadeleştirilir);
arama görünümü açılırken dizin yenilenir, başka yerlerde en çok 4 saniye geriden gelir.

Aramada ayrıca: `"tam ifade"` (tırnak içi bütün olarak geçmeli), `başlık:kelime` (başlıkta), `koleksiyon:ad` (o koleksiyonda),
`#etiket` ya da `etiket:ad` (o etiketi taşıyan). Hepsi birbiriyle ve düz kelimelerle birleşir.

## Bağlantı grafiği (Kasım 2026)

Kenar çubuğunda "Bağlantı grafiği", ⇧⌘G ya da paletten: kitaplığın sayfaları nokta, `[[bağlantılar]]` çizgi. Noktalar yay gibi
kendiliğinden yerleşir (bağlı olanlar yaklaşır, hepsi birbirini iter; 320 adımda durur) ve görüntü pencereye sığdırılır. Rengi sayfanın
koleksiyonu verir, büyüklüğü bağlantı sayısı; açık sayfanın noktası halkalı. Tekerlek yakınlaştırır, boşluğu sürüklemek kaydırır,
bir noktayı sürüklemek onu yerine sabitler (diğerleri çevresinde yeniden yerleşir), tıklamak sayfayı açar; imleç bir noktadayken onun
komşuları ve çizgileri öne çıkar. Üstte arama (eşleşen noktalar parlar), "Bağlantısız sayfalar" anahtarı (kapalıyken hiçbir yere
bağlı olmayanlar çizilmez) ve "Yeniden düzenle". En çok bağlantılı 700 sayfa çizilir.
