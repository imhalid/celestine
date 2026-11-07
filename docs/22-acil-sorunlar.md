# 22 — Acil sorunlar (kullanıcı raporu)

**Öncelik: çok acil.** Kullanıcı kullanırken fark etti ve bildirdi. 7 Kasım 2026. "Beklenen" satırı kullanıcının istediğidir; "Not" satırı
bilinen bağlamdır; "Yapıldı" satırı sonradan eklendi ve neyin nasıl çözüldüğünü, neyin çözülmediğini söyler.

Sıra, kullanıcıyı en çok engelleyenden en az engelleyene doğrudur.

## Donma ve kullanılamazlık

### 1. İstatistik sayfasını açarken uygulama donuyor
- **Belirti:** İstatistik ekranı açılırken uygulama donuyor.
- **Beklenen:** Hemen açılır; büyük kitaplıkta bile arayüz takılmaz.
- **Not:** Ekran açılışta kitaplığın tamamını tarıyor olabilir (kırık bağlantı, aynı başlık, en büyük sayfalar, ısı haritası). `docs/17`'de
  10 000 sayfada "kitaplık rakamları" 0,54 sn ölçülmüştü, yani yalnız bu sayı değil başka bir şey kilitliyor olabilir. Önce bir profil al,
  sonra tarama arka plana ya da önbelleğe alınır.
- **Yapıldı (neden bulundu):** Ekran her 10 saniyede kitaplıktaki **bütün sayfaların metnini** UI iş parçacığında yeniden okuyup ayrıştırıyordu
  (`libraryStats`: `p.Text()`, `wordsOf`, `plainText`). Kelime sayısı ve "boş mu" bilgisi artık sayfa dizinine (`pageIdx.words`, `.empty`) bir kez,
  sayfa değişince yazılıyor; ekran dizini zorla yeniden kurmuyor. Ayrıca ekran önce çiziliyor ("Hesaplanıyor…"), rakamlar bir sonraki karede okunuyor:
  açma tıklaması beklemez. **Ölçülmedi:** gerçek büyük kitaplıkta süre; `perf_test.go` bütçesi aynı.

### 2. Komut paletinde (⌘P) ok tuşlarıyla inerken liste kaymıyor
- **Belirti:** Ok tuşlarıyla aşağı inince seçili satır görünür alanın dışına çıkıyor, liste onu izlemiyor.
- **Beklenen:** Seçili satır hep görünür kalır (aşağı ve yukarı, uçlarda da).
- **Not:** Palet kipleri ve öneriler eklendikten sonra çıkmış olabilir. Kaydırmanın seçime bağlandığı yer paletin liste kısmı.
- **Yapıldı:** ↑↓ ve yeni sorgu seçili satırı `ScrollIntoView` ile gösteriyor (`pal.reveal`). `TestPaletteScrollsWithTheArrows`: 24 satır aşağı, 24 yukarı;
  düzeltme çıkarılınca test düşüyor (doğrulandı).

## Yazı tipi

### 3. Yazı tipi değiştirirken canlı önizleme düzeni bozuyor
- **Belirti:** Listede gezinirken seçilen yazı tipi anında uygulamaya uygulanıyor; bu, düzeni çok kötü bozuyor.
- **Beklenen:** Canlı uygulama yok. Üzerine gelince (hover) hiçbir şey değişmez. Bunun yerine listedeki her yazı tipinin adı **kendi yazı tipiyle**
  yazılır; seçince uygulanır.
- **Not:** `fonts.go`'daki canlı önizleme kaldırılır; liste satırı zaten "kendi harfleriyle" çiziliyor (docs/16 F), o korunur.
- **Yapıldı:** Ayarlar listesinde üzerine gelince önizleme yok (`fonts.prev` kaldırıldı); palette de yazı tipi önizlemesi yok (`preview` alanları
  kaldırıldı). Liste satırı yazı tipini kendi harfleriyle çiziyor, tıklayınca uygulanıyor. Tema ve caret önizlemeleri paletteki gibi duruyor.

## Kaydırma çubuğu

### 4. İstatistik sayfasında kaydırma çubuğu en sağda olmalı
- **Belirti:** Kaydırma çubuğu içerik sütununun kenarında duruyor.
- **Beklenen:** Pencerenin en sağ kenarında.
- **Yapıldı:** Kaydırma alanı pencere genişliğinde, içerik sütunu (`viewMax`) onun içinde ortalı. Çubuk artık alanın en sağında.
  **Yalnız istatistik sayfasında**: koleksiyon, ara, etiket, görev, çöp ekranları hâlâ içerik sütununun içinde kaydırıyor.

### 5. Özel (custom) kaydırma çubuğu, uygulamanın tamamında
- **İstek:** Yapılabiliyorsa uygulama genelinde tek bir özel kaydırma çubuğu: aynı görünüm, tema ve köşe ayarına uyar.
- **Not:** Önce MyGo'nun kaydırma çubuğunu özelleştirmeye izin verip vermediğine bakılır; vermiyorsa Ryugo'da bir bileşen var mı, ona bakılır.
  Yapılamıyorsa bunun nedeni buraya yazılır.
- **Kısmen yapıldı:** MyGo kaydırma çubuğunu tema üzerinden boyatıyor (`Theme.Scrollbar`, `ScrollbarWidth`); uygulamanın her çubuğu, editörünki de,
  tek yerden (`main.go`) aynı görünüşte: 7 pt, temanın soluk rengi. **Yapılamayan:** MyGo ucu her zaman hap şeklinde çiziyor
  (`scrollThumbFill`, yarıçap = genişlik/2); köşe ayarı çubuğa ulaşmıyor. Bunun için 18 kaydırma yerinin hepsini `Scrollbars(false)` ile kapatıp
  kendi çubuğumuzu çizmek gerekir ya da MyGo'ya bir seçenek eklenir.

## Köşe yarıçapı (radius) 0 iken

### 6. Mizan'da yazma hızı göstergesi bozuk görünüyor
- **Belirti:** Köşeler 0 olunca Mizan caret'inin gösterdiği yazma hızı bozuk çiziliyor.
- **Beklenen:** Köşe 0 iken de düzgün görünür.
- **Yapıldı (neden bulundu):** Kutu `rad(H/2)` ile çiziliyordu ama kenar çizgisi, ışık hüzmesi ve gölge hâlâ tam hap (`pill()`, `H/2`). Köşe 0 iken içi kare,
  çizgisi yuvarlak kalıyordu. `pill()` ve gölge artık köşe ayarına uyuyor. **Gözle doğrulanmadı**: Mizan çizimi test ekran görüntüsüyle bakılmadı.

### 7. Aralık (range) seçiciler yuvarlak kalıyor
- **Belirti:** Köşeler 0 olduğu hâlde range seçiciler (kaydırıcılar) yuvarlak.
- **Beklenen:** Köşe ayarına uyar, 0 iken köşeli.
- **Yapıldı, Ryugo'da:** `Slider`ın topuzu `Radius(k/2)` ile sabit yuvarlaktı; `t.Pill(k)` yapıldı (`ryu-go/controls.go`). **Bu değişiklik Ryugo
  deposunda (`../ryu-go`), commitlenmedi ve yayınlanmadı**: ryudit şimdilik yerel `go.work` ile onu kullanıyor (commitlenmez). Ryugo yayınlanıp
  `go.mod` yükseltilmeden bu madde başka bir makinede görünmez.

## Ryugo bileşenleri

### 8. Odak kipi ve dil seçimi Ryugo'dan alınmamış
- **Belirti:** Odak kipi ayarı ve dil seçimi kendi yazdığımız arayüzle yapılmış.
- **Beklenen:** Ryugo'da bunların bileşeni varsa onu kullan (bu, tema ve köşe ayarına da otomatik uyar).
- **Not:** Önce Ryugo'da karşılığı olup olmadığına bak (`github.com/imhalid/ryu-go`); yoksa bunu Ryugo'ya eklemek de seçenek.
- **Yapıldı:** Odak kipi, dil ve senkron "Nereye" seçimi `ryugo.Segmented` (`choiceBar`), yazı tipi ön-kümeleri `ryugo.Button`. Elle çizilmiş çip
  satırı kalmadı.

## Yerleşim

### 9. Sekmeli açınca sağ üstteki düğmeler kayıyor
- **Belirti:** Sekmeler açıkken sağ üstteki düğmeler yerinden oynuyor.
- **Beklenen:** Düğmeler sekme sayısından bağımsız aynı yerde durur.
- **Kısmen:** Ne zaman ve hangi düğmelerin kaydığı tam anlaşılmadı; koleksiyon araç çubuğunda düğmeler görünüm değişince yerinde kalıyor (ölçüldü).
  Bulunan tek kayma nedeni: iki bölmede her bölmenin sağ üstüne mutlak konumlu "Bölmeyi kapat" düğmesi biniyordu. Bölmeler Dock'a geçince o düğme
  kalktı (kapatma sekmenin üstünde). Başka bir kayma görürsen hangi ekranda olduğunu söyle.

### 10. Küçük ekranda arama sıkışıyor, panelin boyutu değiştirilemiyor
- **Belirti:** Küçük pencerede arama alanı sıkışıyor. Yan panelin boyutunu değiştirme yok.
- **Beklenen:** Panel sürüklenerek boyutlanır; arama küçük ekranda da kullanılabilir kalır.
- **Yapıldı:** Kenar çubuğu ve sağ panel kenarlarından sürüklenerek boyutlanıyor (çift tık varsayılana döner), genişlik ayarlarda saklanıyor
  (`sidebarWidth`, `panelWidth`) ve pencerenin en çok %30/%32'sini alıyor; küçük pencerede metne yer kalıyor. Koleksiyon araç çubuğu dar pencerede
  sarıyor, arama alanı en az 140 pt. `TestSidebarEdgeDrag`.

### 11. Paneller: Ryugo'nun workspace'i kullanılsın, şimdi yalnız iki panel var
- **Belirti:** Panel (bölünmüş görünüm) şu an yalnız iki tanesi yan yana açılabiliyor.
- **Beklenen:** Ryugo'daki workspace bileşeni kullanılır: orada paneller çok iyi çalışıyordu (boyutlandırma, yerleşim). İkiden fazla panel olur.
- **Not:** Bu, `panes.go`'daki kendi yazdığımız bölmenin yerine geçer; 10. maddeyle birlikte ele alınmalı (panel boyutlandırma Ryugo'dan gelir).
- **Yapıldı (kısmen Ryugo'da):** İki ya da daha çok bölme Ryugo'nun `Dock`'uyla düzenleniyor: kenarlardan boyutlanır, köşeden bölünür, sekme olarak
  üst üste konur, kapatılır, ⌘Z gibi geri alınır. "Yeni bölme: bu ekran" komutu ve menü maddesi eklendi; sayfa sürüklenip bir bölmenin üstüne bırakılınca o
  bölmede açılır. Tek bölme eskisi gibi tam pencere. Ryugo'ya `Layout.AddPane/RemovePane/PaneIDs/FindPane` eklendi (`ryu-go/dock_api.go`):
  **commitlenmedi, yayınlanmadı** (madde 7 ile aynı). `TestManyPanes`. **Bölme düzeni kaydediliyor** (`Settings.Panes`: Dock'un ağacı, her bölmenin gösterdiği yer, kullanımdaki bölme); pencere açılırken aynen
  geri geliyor, tek bölmeye dönünce kayıt siliniyor (`TestPanesComeBack`). **Yapılmadı:** bölmeyi ayrı pencereye sürükleme: Dock'un `Host`'u
  (`ryu-go/dockwin`) verilmedi; gerçek pencerelerde denenmeden eklenmez (Dock'un kendi belgesi de "gerçek pencerede denenmedi" diyor, Wayland'de
  ayrı kural var). Dock başlığındaki "Bölme" menüsü tek türlü.

## Sonradan bildirilenler (aynı gün)

### 12. Bir checkbox'ı tamamlayınca sayfa en sona atıyor
- **Yapıldı (neden bulundu):** Görev sırası değişmiyordu; sayfa kayıyordu. Kutuyu işaretlemek metni değiştirir (`ToggleChecked`), `track` her
  değişimde "caret'i göster" (`revealWant`) diyordu ve caret sayfanın sonunda kalmıştı. Artık caret ve uzunluk değişmeyen bir düzenleme
  (kutu, biçim) sayfayı caret'e götürmüyor. `TestTickingABoxKeepsThePage`.

### 13. ⌘F çubuğu sayfayı aşağı itiyor
- **Yapıldı:** Çubuk sayfanın sağ üstünde, sayfanın üzerinde, yukarıdan aşağı süzülerek açılıyor (`findDrop`); altındaki hiçbir şey kaymıyor.
  `TestFindBarLiesOverThePage`.

### 14. Bulunan metin ekran dışındaysa ileri/geri tuşu o metne gidiyor ama kaydırmıyor
- **Yapıldı (neden bulundu):** Odak bul çubuğundayken editör odakta değil; caret'i gösteren kaydırma (`drawCaret` → `revealCaret`) odak yoksa hiç
  çalışmıyordu. Bul açıkken odak yokken de çalışıyor, `showMatch` bunu istiyor. `TestFindStepScrollsToTheMatch` (düzeltme çıkarılınca düşüyor).

### 15. `@` ile (sayfa) aramada çok yavaş
- **Yapıldı:** Arama indeksi (`searchindex.go`): her sayfanın başlık ve metni katlanıp harf çiftlerine bölünür, her çift için hangi sayfalarda
  olduğunu gösteren bir bit satırı tutulur; bir sözcük kendi çiftlerinin VE'idir (10 000 sayfa = 157 makine sözcüğü), yalnız oradan çıkan
  sayfalara bakılır. Metin sayfa yeni ya da yazılmışsa bir kez okunur; kitaplık açılınca sayfalar karede 3 ms ile indekse alınır.
  Sayfa adları için de katlanmış başlıklar ve harf maskesi saklanıyor (`placeIdx`): vuruş başına 1000 öğe yapılıp katlanmıyor.
  **Ölçüm** (`TestIndexedKeystrokes`, vuruş başına ortalama, 12 sorgu): 1 500 sayfada 0,17 ms, 10 000 sayfada **0,88 ms** (en kötü 1,55 ms);
  önceki durum 10 000 sayfada metin 480 ms + adlar 22 ms. Sonuçlar eski taramayla aynı sırada, aynı puanla (`TestIndexAgreesWithScan`).
  **Sınırlar:** çok sayıda sayfanın yanıtladığı geniş sorguda ilk 600 sayfa (başlığında geçenler, sonra yeniler) puanlanır; indeks ilk
  açılışta birkaç yüz ms'lik karelerle dolana kadar eski tarama çalışır; indeks bellekte ≈ birkaç MB.

### 16. `@` ile aranan metne gidince kapak üstte kalıp satırların üstüne biniyor (ekran görüntüsü)
- **Yapıldı (neden bulundu):** Çok uzun sayfa (800 paragraf ya da 60 000 harften uzun) kendi penceresinde kayar; metin kayınca başlık ve
  özellikler katlanıp gidiyor ama kapak aynı yerde çiziliyordu, penceredeki en üstte kalıp satırların arkasında/üstünde. Ekran görüntüsündeki
  "×" eski "Bölmeyi kapat" düğmesiydi (artık yok). Kapak o durumda çizilmiyor (ağaç değişmeden: yoksa pencere durumu her karede
  gidip geliyordu, ölçüldü). `TestHugePageCoverGoesWhenScrolled` (düzeltme çıkarılınca düşüyor).

### 17. Durum çubuğunda kullanılabilecek kısayollar yazsın
- **Yapıldı:** Solda, duruma göre tuşlar: yazarken (⌘P, ⌘K, ⌘F, /, [[, Esc), palet, bul, / menüsü, normal kip, ayarlar, koleksiyon ve diğer
  ekranlar. Sığmayanlar sıkışmadan kesilir. Ayarlar ▸ Durum çubuğu ▸ "Kısayol ipuçları" ile kapanır, komut paletinden de.

### 18. Sağ panel yüzen küçük bir panel olsun (docs/16 açık soru 3'ün cevabı)
- **Yapıldı:** Yan panel yüzen bir kart (`sidepanel.go`), metinden yer almıyor; ayrıntı docs/16'da.

## Önerilen sıra

1. 1 ve 2 (donma, kayma): kullanımı engelliyor.
2. 3 (yazı tipi): düzeni bozuyor.
3. 6 ve 7 (köşe 0), 9 (sekme düğmeleri): görsel hatalar, küçük.
4. 4 ve 5 (kaydırma çubuğu).
5. 8, 10, 11 (Ryugo bileşenleri ve workspace): daha büyük, birlikte yapılır.

Her madde bittiğinde bu belgede işaretlenir ve ilgili bir test ya da elle doğrulama notu eklenir (donma için süre ölçümü, kayma için `ui.NewTester` testi).
