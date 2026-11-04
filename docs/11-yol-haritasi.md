# 11 — Yol haritası: bizim editörü adım adım kurmak

Her aşama kendi başına çalışan, denenebilir bir uygulama bırakır. Bir aşama bitmeden sonrakine geçilmez. Kararlar
[00](00-OKU-BENI.md)'da: eklenti yok, yalnız tema dışarıdan; caret'ler Go'da; MCP uygulamanın içinde.

## Güncel durum (4 Kasım 2026)

Aşağıdaki ilk aşamalar başka bir yoldan yapıldı: editör MyGo'nun `ui/richtext`'i (blok = paragraf), veri düz klasörler
ve Markdown, bileşenler Ryugo. **Bitenler:** işaretsiz Markdown, ``` ve `---`, Slime/Mizan caret'leri, koleksiyonlar ve
alt sayfalar, sürükle-bırak ağaç, liste/tablo/pano, çöp, MCP sunucusu ve ayarlar.

**Eksik kalan, Thymer'e göre:** özellikler herkese zorla verilmiş (Durum), ve "blok editör" dediğimiz hâlde bloklarla
ilgili hiçbir şey yok. Kullanırken çıkan istek ve düzeltmelerin tam listesi ve sırası:
[13 — İstekler](13-istekler.md). Sıradaki aşamalar bunlar:

### Aşama A — Özellikler, Thymer gibi isteğe bağlı
1. Koleksiyonun **özellik şeması**: varsayılan olarak boş. Sayfadaki **+ Özellik ekle** ile eklenir: Seçim, Metin,
   Sayı, Tarih, Bağlantı (sonra Etiket, Sayfa bağlantısı, Kişi). Ad, tür, seçenekler sonradan düzenlenir, silinir.
2. Değerler sayfanın ön maddesinde (`özellik-kimliği: değer`); tanımadığımız anahtarlar korunur.
3. Sayfada başlığın altında **özellikler bölümü**; yalnız şemada özellik varsa görünür.
4. Tablo her özelliği sütun yapar, ona göre sıralar. **Pano** yalnız bir Seçim özelliği varsa vardır ve ona göre
   gruplar; değeri olmayanlar "Boş" sütununda.
5. MCP özellikleri okur, yazar, şemaya ekler.

### Aşama B — Bloklar
Blok = bir paragraf ve altındakiler (girintisi daha derin olan liste öğeleri; bir başlığın altında sonraki aynı ya da
üst düzey başlığa kadar olanlar).
1. **Tutamak** (⋮⋮): imleç bir bloğun üstündeyken solunda belirir; tıklanınca **blok menüsü**: Dönüştür (metin,
   başlık 1–3, liste, görev, alıntı, kod), Çoğalt, Sil, Bağlantıyı kopyala.
2. **Sürükle-bırak**: tutamaktan tutulan blok, altındakilerle birlikte kalkar (Ryugo'nun kart efektleri); bloklar yer
   açar; bırakınca yerine süzülür. Girintiye göre içe ya da dışa bırakma.
3. **⌘⇧↑ / ⌘⇧↓** bloğu altındakilerle birlikte taşır; **⌘/** katlar.
4. **Katlama**: başlıklar ve alt öğesi olan liste öğeleri solundaki okla katlanır; katlı hâli sayfada hatırlanır.
5. **/ menüsü**: boş satırda `/` yazınca blok türleri listesi (klavyeyle seçilir).
6. **Blok seçimi**: Esc caret'in bloğunu seçer, ↑↓ ve ⇧ ile genişler; seçili bloklar birlikte taşınır, silinir,
   kopyalanır (Markdown).
7. **Kutular** (callout: bilgi, uyarı, not) ve sonra sütunlar.

### Aşama C — Dosyalar dışarıdan değişince
Kitaplık klasörü izlenir: başka bir editörün, betiğin ya da ajanın yaptığı değişiklik pencereye gelir; açık sayfada
çakışma olursa ikisi de kaybolmaz.

### Aşama D — Komut paleti (⌘P / ⌘K) ve kısayollar

### Aşama E — Temalar veri dosyası olarak

---

## Aşama 0 — İskelet
1. Yeni bir MyGo projesi; `mizan-go` bileşenleri ve tokenları bağımlılık olarak.
2. Üç bölgeli pencere: kenar çubuğu, panel alanı, durum çubuğu ([01](01-thymer-genel-bakis.md)).
3. Tema dosyası okuma (JSON/TOML → mizan-go tokenları) ve anında yeniden yükleme.
4. Kapı (mizan-go ile aynı): `gofmt`, `go vet`, `go test`, cgo'suz derleme (darwin, linux, windows).

**Bitti sayılır:** boş bir pencere açılıyor, tema dosyası değişince renkler anında değişiyor.

## Aşama 1 — Veri modeli ve depolama
1. Ağaç: `Node`, `Op` (ekle/taşı, güncelle, özellik yaz), geri alma/yineleme ([02](02-veri-modeli.md)).
2. Kalıcılık: SQLite (saf Go) + her koleksiyon bir klasör, her sayfa bir `.md` dosyası (ön madde = özellikler).
   Dosya değişince (dışarıdan düzenleme) model güncellenir; model değişince dosya yazılır.
3. Testler: işlem → geri al → aynı ağaç; dosya ↔ model gidip gelme.

**Bitti sayılır:** komut satırından sayfa ekleyip silebiliyorum, klasörde Markdown dosyaları oluşuyor.

## Aşama 2 — Editör çekirdeği
1. `slime-md` demosundaki metin alanını **satır bazlı** yap: her satır bir düğüm ve kendi `TextLayout`'u
   ([04](04-not-sayfasi.md)).
2. Satırlar arası gezinme, seçim, kopyala/yapıştır (Markdown).
3. WYSIWYG: `# `, `- `, `[] `, `> `, ```` ``` ```` yazarken dönüşür; işaretler caret çıkınca gizlenir.
4. Girinti, katlama, satır taşıma (⌘⇧↑↓).
5. Caret'ler: **Slime** ve **Mizan** (hazır), ayardan seçilir.
6. Sanallaştırma: yalnız görünen satırlar.

**Bitti sayılır:** 5.000 satırlık bir sayfada akıcı yazılıyor, Markdown dosyası doğru üretiliyor.

## Aşama 3 — Koleksiyonlar, sayfalar, kenar çubuğu
1. `NewPage`, `NewCollection` işlemleri ve bütün giriş noktaları ([05](05-not-ekleme-akislari.md)).
2. Kenar çubuğu ağacı: koleksiyonlar, sayfalar, **alt sayfalar** (`+`, `…`, sürükle-bırak) ([03](03-kenar-cubugu-ve-alt-sayfalar.md)).
3. Sayfa anatomisi: koleksiyon kapsülü, başlık, `…` menüsü, özellikler bölümü.
4. Koleksiyon ayarları: General / Properties / Views / Pages, taslak → önizleme → kaydet ([06](06-koleksiyonlar-ve-gorunumler.md)).
5. Görünümler: list, table; sonra board.

**Bitti sayılır:** "Projeler" koleksiyonu oluşturup içine alt sayfalı notlar ekleyebiliyor, tabloda görebiliyorum.

## Aşama 4 — Komutlar, arama, ayarlar
1. Komut kaydı, ⌘P, ⌘K, kısayol ayarı ([08](08-komut-paleti-ve-kisayollar.md)).
2. Arama indeksi ve sorgu dilinin ilk hâli ([09](09-arama-gorevler-gunluk.md)).
3. Ayarlar penceresi: Görünüm/Tema, Editör, Kısayollar, Dosyalar, Varsayılan koleksiyon ([07](07-ayarlar-paneli.md)).

## Aşama 5 — MCP
1. Uygulama içi MCP sunucusu, ayar sayfası, koleksiyon başına erişim ([10](10-mcp.md)).
2. İlk araç seti: help, koleksiyon/sayfa okuma-yazma, insert_markdown, search, tema.
3. Agent değişikliklerinin arayüzde görünmesi ve geçmişte işaretlenmesi.

**Bitti sayılır:** Claude Code'a "bir Okuma Listesi koleksiyonu oluştur, içine 3 kitap ekle" diyorum, uygulamada canlı
olarak beliriyor, ⌘Z ile geri alabiliyorum.

## Aşama 6 — Sonra
Board/gallery/calendar görünümleri, dinamik koleksiyon, canlı sorgu bloğu, günlük, sürüm geçmişi, dışa aktarma,
çoklu pencere (Dock), şok dalgası efektini tema ayarı olarak geri açmak.

---

## Thymer'den alınacaklar / alınmayacaklar

| Alınacak | Alınmayacak |
|---|---|
| Tek ağaç modeli, satır = blok | Eklenti sistemi, kod çalıştırma |
| Alt sayfalar (koleksiyon bazında açılan) | Sunucu, eşitleme, uçtan uca şifreleme (ilk sürümde) |
| Kenar çubuğu gösterim modları | Ayrı bir MCP yardımcı uygulaması |
| Koleksiyon ayarlarında taslak → önizleme → kaydet | Tek tıklamada satır oluşturan Quick Add |
| Bağlama duyarlı durum çubuğu ve palet | |
| Markdown Mirror (bizde temel özellik) | |
| Sorgu dili, `insert_markdown`, `data_help` | |
