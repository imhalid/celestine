# 16 — Yapılacaklar (6 Kasım 2026)

Kullanıcının 6 Kasım'da söylediği değişikliklerin tamamı, önem ve zorluğa göre sıralı. Hiçbir madde atlanmadı:
sonda "Mesajdaki her cümle nerede?" tablosu var. **7 Kasım:** kullanıcı, listelediklerine eksik ya da daha iyi yan eklememi söyledi; her maddenin altında "Eklenen öneriler" ve sonda "Eklenen yeni maddeler" (N1–N6) bunlar. Kodda gördüğüm mevcut durum ve belirsiz kalan yerler her
maddenin altında ve en sonda yazılı. Araştırma gereken maddelerde sonuç yazmadım, yalnız ilk adımları yazdım.

## Sıralı özet

Önem: **K** kritik (bozuk ya da istenen temel şey), **Y** yüksek, **O** orta, **D** düşük.
Zorluk: 1 çok kolay · 2 kolay · 3 orta · 4 zor · 5 çok zor / belirsiz. Sıra, önce önem sonra zorluk (kolaydan zora)
ve bağımlılıklara göre verildi.

| Sıra | # | İş | Önem | Zorluk | Bağımlı olduğu |
|---|---|---|---|---|---|
| 1 | A | Durum çubuğundan 7 göstergeyi, ayarlarını ve kodunu sil | Y | 2 | — |
| 2 | B | ⌘P: Ayarlar açılmıyor, tema değişmiyor (hata) ve temaya önizleme | K | 2–3 | — |
| 3 | C | Ayarlar diyalog olmasın: normal ekran olarak açılsın | Y | 3 | A (daha kısa liste) |
| 4 | D | Editör ayarları: yazı boyutu, satır, aralık, otomatik kayıt, boşluğu göster | Y | 3 | C |
| 5 | E | ⌘P ile uygulamadaki bütün ayarlar | Y | 3 | C, D (ayar listesi oturunca) |
| 6 | O | Bölünmüş görünüm (split view): sürükle-bırak, "yan panelde aç", her ekran bir bölmede | Y | 5 | C |
| 7 | F | Yazı tipi: uygulama ve editör fontu, bilgisayardaki fontlardan | O | 3–4 | C |
| 8 | G | Bütün uygulama klavyeden (Vim gibi), fare olmadan | K | 5 | B, C (sayfalar bitince) |
| 9 | H | İstatistik sayfası: yazım ve kullanım analizi | O | 3–4 | A |
| 10 | I | Uygulama boyutunu küçült (özellik kısmadan) | O | 3 | — |
| 11 | J | Web uygulaması: araştırma ve deneme | O | 4 | I (boyut) |
| 12 | K | Otomatik güncelleme ve yeni sürüm uyarısı | O | 4 | — |
| 13 | L | Senkronizasyon (Cloudflare dahil değerlendirme) | O | 5 | — |
| 14 | M | Android uygulaması: araştırma ve deneme | D | 5 | J, L |

## 7 Kasım, öğlen

Bu turun işleri: kendi kaydırma çubuğu, dört bölgeli pencere ve üst çubuktaki kayma (docs/22 madde 4, 5, 9), bölmeyi ayrı pencereye alma
(madde 11), tablo hücrelerinde oklar (G), menü ve MCP'nin ortak komut listesi (E), performans (N6), çeviri (N4), özel yazı tipinin açılıştaki kayması
(docs/22 madde 19), kapanmayan uygulama (madde 20), web ve APK'nın gerçek ortamda denenmesi (J, M). Editör ayarları (D) ve yazı tipindeki Türkçe karakter
uyarısı (F) kullanıcının isteğiyle olduğu gibi bırakıldı. Ayrıntı aşağıdaki tabloda ve docs/22'de.

## Durum özeti (7 Kasım, gece 3) — ne yapıldı, ne yapılmadı

Hedef "bütün maddeler ve bütün öneriler" idi. Aşağıdaki liste dürüst bir defter: **yapıldı** (testli), **kısmen**, **yapılmadı** (neden). Belirsiz kalan
açık sorular (1–10) için bu turda varsayılanla ilerlendi; cevap gelirse yön değişebilir.

| # | Durum | Yapılanlar | Yapılmayanlar ve nedeni |
|---|---|---|---|
| A | yapıldı | 7 gösterge ve kodu silindi; durum çubuğunu gizleme; yan panel "Bilgi"de kelime, harf, okuma süresi, geri bağlantı | — |
| B | yapıldı | tema önizlemesi, ✓, geri al bildirimi, argümanlı komutlar, son kullanılanlar, takma adlar, ⌘⇧P, modlar `> @ # : / ! ?`, sessiz hata yok, bağlam öne çıkarma (sayfa, koleksiyon, seçili metnin biçim komutları) | — |
| C | yapıldı | ayarlar ekran oldu; ayar araması (Türkçe harf duyarsız, İngilizce takma adlar), "Değiştirilenler" ve varsayılana dön (tek tek, topluca), profiller (Yazar, Kod, Sıkı, Okuma ve kendi kaydettiklerin), ayarları dosyaya yaz / dosyadan oku (gizli bilgiler yazılmaz ve okunmaz), her sonuçta komut adı | — |
| D | yapıldı | yazı boyutu ⌘= ⌘− ⌘0, paragraf aralığı, ilk satır girintisi, Tab uzunluğu, satır uzunluğu (en çok N karakterlik sütun), otomatik kayıt (aç/kapa, gecikme, ⌘S, "kaydedildi 14:32", çıkışta ve sayfa değişirken kayıt), boşluk/sekme/¶/kırılmaz boşluk/artık boşluk gösterimi (kod bloğunda ayrı), odak modu (satır, paragraf), daktilo kaydırması, caret uykusu, hafif mod, "son kayıttan önceki hâle dön", ayar ön-kümeleri (profiller), Vim hareketleri | harf ve kelime aralığı (imleç konumu düzenleyici düzeniyle hizalı kalmaz), yazım denetimi (işletim sistemi sözlüğüne MyGo'dan yol yok), sağdan sola yazı, imleç yanıp sönme hızı, boşlukların yalnız seçimde gösterilmesi; "length size" cevapsız (açık soru 4), ama satır uzunluğu (karakter olarak sütun sınırı) yapıldı |
| E | yapıldı | ayar listesi tek kayıt (`cmdreg.go`), her ayar bir komut (testle denetlenir), önizleme, geri al, argüman, son komutlar, kendi kısayolların (çakışma denetimiyle), makrolar | — **7 Kasım, öğlen: yapıldı.** Uygulamanın komutları tek listede (`appcmds.go`): menü çubuğu, menü çubuğu yokken pencerenin kısayolları, paletteki satırları ve kendi kısayolunun alamayacağı tuşlar ondan üretilir; MCP `list_commands` ve `run_command` ile paletin bütün listesini (ayarlar dahil) görür ve çalıştırır. `TestAppCommandsAreOneList`, `TestAppCommandsInEnglish` |
| F | yapıldı | sistem fontlarından uygulama/editör/kod yazı tipi, kendi harfleriyle liste, canlı önizleme, arama, favori, son kullanılan, ön-kümeler, yedek zinciri, ligatür, aynı genişlik rakam, kalınlık, arka planda tarama | Türkçe karakter eksikliği uyarısı ve sahte italik uyarısı (MyGo font dosyasını okutmuyor), optik boyut |
| G | yapıldı (çekirdek) | normal kip (Esc/i), j k ve sayı öneki, gg G, Enter/l/h, `f` harf etiketleri, Space leader, işaretler, `.` tekrar, ⌃O/Tab ile yer listesi, `gf`, `:` komut satırı, `?` kısayol sayfası, ⌘1/⌘2/⌘3, Vim hareketleri (ayar), liste/tablo/pano/takvim/arama/etiket/çöp/görev satırlarında klavye, sürüklemenin komut karşılıkları (sayfayı yukarı/aşağı/başka koleksiyona taşı, koleksiyonu taşı), kısayol değiştirme, sistem kısayoluyla çakışma reddi, odak halkası ayarı, N3 test altyapısı ve fare olmadan kabul senaryosu | ayar kontrollerinin (←→) klavyeyle gezilmesi, VoiceOver etiketlerinin denetimi, web'de normal kipin açılması. **Tablo hücreleri (7 Kasım, öğlen): yapıldı:** metinden ↓ (tablonun hemen önündeyken) başlığın ilk hücresini, ↑ (hemen arkasındayken) son satırı açar; hücrede metnin başında ← ve sonunda → yandaki hücreye geçer (imleç gidiş yönündeki uca konur, basılı tutunca hücre hücre gider), ↓ son satırdan ve ↑ başlıktan tablonun altına/üstüne, metne çıkar (satır eklemez; Tab eklemeye devam eder). `TestTableArrowKeys` |
| H | yapıldı | istatistik ekranı: yazım, saat×gün ve yıllık ısı haritası, oturumlar ve hız, hedef ve seri, kullanım süresi ve sayaçlar, koleksiyon/etiket/sayfa dağılımı, sayfa başına süre, komut ve sayfa listeleri, kullanılmayan özellikler, kitaplık sağlığı, CSV/JSON, haftalık özet, sıfırla, tek anahtar | — |
| I | ölçüldü | `scripts/build.sh`, bayraklar, bütçe, rapor (`17-boyut-ve-performans.md`) | bağımlılık sadeleştirme, tembel varlıklar, iki kip, fark güncellemesi: kazanç az ya da özellik riski, gerekçesi raporda |
| J | yapıldı (deneme) | wasm derlendi ve tarayıcıda çalıştı: açıldı, yazıldı, IndexedDB'ye kaydedildi, yenilenince kaldı; PWA dosyaları (manifest, servis çalışanı, simgeler); Esc ile normal kip açılıyor (`store_js.go`, `scripts/web.sh`, `web/`, `18-web.md`) | OPFS/klasör erişimi, Safari/Firefox testleri, web'de harf tuşlarıyla klavye komutlarının doğrulanması. **7 Kasım, öğlen:** servis çalışanı tarayıcıda kaydoldu ve sayfayı yönetti; sunucu kapalıyken sayfa, wasm ve betik önbellekten geldi (çevrimdışı açılış). Bulunan hata düzeltildi: sayfa da önbellekten veriliyordu, yeni bir derleme hiç alınmazdı; sayfa artık önce ağdan. Program kurulumda önbelleğe alınıyor (manifest, simgeler dahil). Web'de pencere düğmeleri, hızlı not paneli ve ayrı pencere yok |
| K | yapıldı, canlı denenmedi | manifest, imza, indirme, yerine koyma, geri dönüş, notlar, kanal, atla/ertele, çevrimdışı mod, Cloudflare Worker, release iş akışı (`19-guncelleme.md`) | üç işletim sisteminde gerçek güncelleme, fark güncellemesi, Apple/Windows imzası (açık soru 8) |
| L | yapıldı (ilk sürüm) | dosya düzeyinde üç yollu eşitleme, çakışma kopyası, silme, klasör ve sunucu uzağı, uçtan uca şifreleme, kurtarma anahtarı, seçmeli koleksiyon, rozet, çakışma çözümü, Worker (`20-senkron.md`) | cihaz yönetimi/QR, sunucuda geçmiş ve yedek, paylaşım, gerçek zamanlı, metin birleştirme, Worker'ın canlı denemesi |
| M | yapıldı (derlendi), çalıştırılamadı | `android/` WebView sarmalayıcısı, `scripts/android.sh`, 34,7 MB'lık APK derlendi (`21-android.md`) | Dokunmatik uyarlama ve Android'e özgü özellikler yok. **7 Kasım, öğlen: APK emülatörde çalıştı** (`-gpu host`; swiftshader çöküyordu): açılış, dokunarak ve ekran klavyesiyle yazma, kaydetme, yeniden açınca yerinde. Bulunan hata düzeltildi: içerik Android'in durum ve gezinme çubuklarının altına giriyordu (kenardan kenara çizim); boşluklar artık dolgu, ekran klavyesi açıkken sayfa onun üstünde biter. Temiz APK 35,8 MB |
| N1 | yapıldı | çökme günlüğü, "son oturum çöktü" iletişim kutusu, kare panik kurtarma | — |
| N2 | yapıldı | günlük/öncesi/elle yedek, budama, geri yükleme, zip dışa aktarma | — |
| N3 | yapıldı | `kbd` test yardımcısı ve senaryolar | — |
| N4 | yapıldı | **7 Kasım, öğlen:** `T()` ile sarılmadan ekrana giden 89 metin (düğme ve alan etiketleri, yer tutucular, boş ekranlar, bildirimler) çevrildi; `TestEnglishCoversScreen` artık bunları da denetler. Önceden: bildirimler (toast), araç ipuçları ve etiketler dahil; `TestEnglishCoversMessages` `T`/`Tf` ile yazılan her metnin İngilizcesini denetler; dil ayarı ve sistem dili, `T()`, her ayar satırının başlığı ve açıklaması, ayar adları, menüler, palet, komutlar, istatistik, yardım, tur (hepsi testle denetlenir: eksik çeviri testi bozar) | `T()` ile sarılmamış tek tük metin Türkçe kalabilir (testle yakalanmaz); sürücü hataları işletim sisteminin kendi metni; menü dil değişince yeniden başlatmada |
| N5 | yapıldı | `?` kısayol sayfası, ilk açılış turu, boş durumlarda ipucu | — |
| N6 | yapıldı | `perf_test.go`: 10 bin sayfa ölçümü ve bütçe testi. **7 Kasım, öğlen: sorunlar giderildi:** kenar çubuğu ağacı sanal (görünenin iki ekran yakınındaki satırlar kurulur), geri bağlantı dizini ilk açılışta karelere yayılır, `[[bağlantı]]` çiplerinin başlık araması önbellekli, ağaç kare başına bir kez sayılır. 10 bin sayfada ilk kare 4,1 sn → 0,53 sn, bellek +424 MB → +15 MB; bir tuş vuruşu bütçede (`a key`, ≤60 ms başsız çizimle) ve kitaplıkla büyümüyor (docs/17) | gerçek GPU'da ölçüm |
| O | yapıldı | (önceki tur) bölünmüş görünüm | — |

Neden bu sıra:

- **A ve B önce**: ikisi küçük, görünür, biri açık hata. C ve G'nin önünü açıyorlar.
- **C → D → E**: ayarlar sayfası yerine oturmadan hangi ayarın nerede olduğu belli olmaz. Palet (E) bu listeden
  üretilir, elle ayrı bir liste tutulmaz.
- **G (klavye)** en büyük iş ve uygulamanın her yerine dokunuyor. Başlık ve ayarlar sayfası son hâline gelmeden
  yapılırsa iki kez yapılır. Ama sürekli çalışılacak bir iş: kural (aşağıda) şimdiden konur, her yeni ekran kurala
  uysun.
- **I (boyut)** önce ölçülür, sonra küçültülür. Web (J) için de gerekli, çünkü wasm dosyası indirilecek.
- **K, L, M** en son: dışarıya bağımlı (GitHub, Cloudflare, Android araçları), geri alması zor ve araştırması uzun.

---

## A — Durum çubuğundan göstergeleri ve ayarlarını sil, kodu da sil

> **Durum (7 Kasım): yapıldı.** Yedi ayar anahtarı, ayarlar satırları, durum çubuğu çizimi ve ölü yardımcılar silindi
> (`statusbar.go` yalnız satır/sütun gösterir; `main.go`daki kelime sayısı gitti, harf sayısı `ShowChars` kaldı).
> Eski ayar dosyaları hatasız açılır (test). Kalan: durum çubuğu gizleme, sayfa bilgisi kutusu (öneriler).

**İstek:** "Ayarlar panelindeki sayfa yolu, seçili harf sayısı, okuma süresi, bugün yazılan harf sayısı, kelime
sayısı, son düzenleme ve geri bağlantılar butonları ve ayarları kaldırılsın. Koddan da silinsin."

**Silinecekler** (ayar anahtarı → ayarlardaki satır → durum çubuğundaki gösterge):

| Ayar (`mcp.go`) | Ayarlar sayfasında | Durum çubuğunda |
|---|---|---|
| `ShowPath` | Sayfa yolu | Koleksiyon › üst sayfa › sayfa |
| `ShowSel` | Seçili harf sayısı | "42 harf seçili" |
| `ShowReading` | Okuma süresi | "3 dk okuma" |
| `ShowToday` | Bugün yazılan | bugünün kelime sayısı (tıklayınca istatistik açılıyordu) |
| `ShowWords` | Kelime sayısı | açık sayfanın kelimeleri |
| `ShowEdited` | Son düzenleme | "3 dk önce düzenlendi" |
| `ShowLinks` | Geri bağlantılar | bu sayfaya bağlanan sayfa sayısı, tıklayınca yan panel |

**Yapılacaklar**
- Yedi ayar anahtarını `settings` yapısından, ayarlar sayfasından (`settingsStatus`, `settings.go`) ve durum
  çubuğu çiziminden (`statusbar.go`) sil. Kullanılmayan yardımcılar da gitsin (okuma süresi hesabı,
  seçim sayacı, `edited` metni…).
- Eski ayar dosyalarında bu anahtarlar kalmış olabilir: okurken yok sayılsın (JSON zaten yok sayar), bir sonraki
  kayıtta düşer.
- Testlerden ve ayar tile'larından (`quicksettings.go`, palet) ilgili satırlar da silinsin.
- Durum çubuğunun geri kalanı (kaydetme durumu, satır/sütun, harf sayısı, imleç konumu) ne olacak? **Açık soru 2.**

**Dikkat:** "Bugün yazılan" göstergesi istatistik penceresini açıyordu (`statusbar.go`). Bu kalkınca istatistiğe
yalnız ⌘P ve ayarlar kalır; H maddesi onu kendi sayfasına taşıyacak. Geri bağlantılar yan panelde ayrıca bir
bölüm olarak duruyor (`sidepanel.go`): düğme ve ayar silinince o bölüm kalsın mı? **Açık soru 3.**

**Kabul ölçütü:** Kodda `ShowPath|ShowSel|ShowReading|ShowToday|ShowWords|ShowEdited|ShowLinks` geçmiyor; tüm
testler geçiyor; durum çubuğunda yalnız kalan göstergeler var.

**Eklenen öneriler (7 Kasım)**
- Durum çubuğu tamamen **gizlenebilsin** (odak modu ile birlikte, ⌘P "Durum çubuğunu gizle").
- Kaldırılan gösterge verileri boşa gitmesin: kelime, okuma süresi, son düzenleme, geri bağlantı sayısı H'deki
  sayfa bilgisi kutusunda (yan panel "Bilgi" bölümü) kalsın, durum çubuğunda değil.
- Eski ayar dosyalarındaki anahtarlar sessizce düşer; bir test eski ayar dosyasını okuyup hatasız açıldığını denetler.

---

## B — ⌘P: Ayarlar açılmıyor, tema değişmiyor; temaya önizleme

> **Durum (7 Kasım): temel hata düzeltildi, önizleme eklendi.** Hata yeniden üretildi: palet listesi süzülünce imleç
> (fare) o sırada hangi satırın üstündeyse o satır seçiliyordu, ↵ de onu çalıştırıyordu (örn. fare ikinci satırın
> üstündeyken "ayarlar" yazıp ↵ "Koleksiyon ayarları"nı açıyordu). Artık satırı yalnız fare **hareket edince**
> seçiyor (regresyon testi eski kodda başarısız, yenide geçer). Eklendi: `Tema: Sistem/Açık/Koyu`, `Renk teması` ve
> `Caret: …` komutları **canlı önizlemeli** (gezerken uygulanır, Esc geri alır, ↵ kaydeder), geçerli olanın yanında ✓.
> Palette mod artık alandaki bir harf değil, soldaki **işaret**: `>` komutlar, `@` sayfalar (arama simgesinin yerine);
> alanın başına `>` ya da `@` yazınca mod değişir, alanda kalmaz.
> Kalan: argümanlı komutlar, son kullanılanlar, geri al bildirimi, bütün ayarlar (E).

**İstek:** "En basitinden Comment P ile tema değiştirme yapamıyorum. Ayarlar modalını açamıyorum. Tema
değiştirebilmem lazım. Mesela o ekrandan ön izleme olarak görebilmem lazım."

**Kodda gördüğüm** (`palette.go`): komut listesinde "Ayarlar" (`settingsOpen = true`) ve "Renk teması: …"
(`setAppTheme`) var. Yani komut var ama kullanıcıda çalışmıyor. Önce **yeniden üretilecek**: palet açıkken
"Ayarlar" seçilince diyalog neden açılmıyor (palet kapanırken `settingsOpen`i sıfırlıyor olabilir, ya da
`modalOpen` kontrolü araya giriyor: `blockui.go:432` palet açıkken başka diyaloğu sayıyor). Ayrıca:

- "Renk teması" yalnız renk paletlerini değiştiriyor; **açık / koyu / sistem** temasını değiştiren komut yok.
- Temalar arasında gezerken **canlı önizleme** yok.

**Yapılacaklar**
1. Hatayı yeniden üret (test yaz), "Ayarlar" komutunu çalıştır.
2. Komutlar: `Tema: Sistem`, `Tema: Açık`, `Tema: Koyu`, `Renk teması: <ad>` (var), hepsi paletten.
3. **Önizleme:** palette bir tema satırında ↑↓ ile gezerken uygulama o temaya geçici geçsin; ↵ kalıcı yapsın,
   Esc eski temaya döndürsün. (Tema değişimi tek renk anahtarı; yeni bir "önizleme durumu" yeterli.)
4. Palet Esc ile kapanınca hiçbir şey kalıcı değişmesin (önizleme geri alınır).

**Kabul ölçütü:** ⌘P → "tema" yazınca açık/koyu/sistem ve renk temaları listelenir; ok tuşlarıyla gezerken
uygulama o temaya döner; ↵ kaydeder, Esc geri alır. ⌘P → "ayarlar" ayarları açar. Testler var.

**Eklenen öneriler (7 Kasım)**
- Palet satırlarında **işaret**: açık olan tema/caret/kip yanında ✓ (kaç seçenek olursa olsun neyin açık olduğu
  görünür).
- Her değişiklikte kısa **geri al bildirimi**: "Tema: Koyu · Geri al" (palet kapandıktan sonra da).
- **Argümanlı komutlar:** palette "yazı boyutu 18", "satır aralığı 1,6", "tema koyu", "genişlik 720" yazınca
  doğrudan uygulanır (komut + değer, sıfır tıklama).
- **Son kullanılanlar** üstte, sık kullanılanlar öğrenilir (yerelde sayaç); boş sorguda "Son komutlar".
- **Bağlam öne çıkarma:** sayfa açıkken sayfa komutları, tabloyken tablo komutları, metin seçiliyken biçim komutları.
- Takma adlar ve Türkçe/İngilizce eşanlamlar: "tema"/"theme"/"görünüm", "ayar"/"settings"/"tercih".
- ⌘⇧P (VS Code alışkanlığı) ⌘P ile aynı palet; palet açıkken `>` komut, `@` sayfa, `#` etiket, `:` satıra git.
- Palet hataları sessiz kalmasın: komut çalışmazsa nedeni palet içinde ya da bildirimde yazsın (şimdi sessiz).

---

## C — Ayarlar diyalog olmasın: sağ panelde ayrı sayfa

> **Durum (7 Kasım, akşam): yapıldı, yön değişti.** Önce sağ panelde sayfa olarak yapılmıştı; kullanıcı panel yerine
> **normal ekran gibi** açılmasını istedi. Şimdi ayarlar, çöp ve görevler gibi ana alanda bir ekran: solda bölümler,
> sağda seçilen bölüm (`settingsScreen`, `settingsNav`). Geçmişe girer (⌘[ geri döner); ⌘, ve menü açar, açıkken
> ⌘, ya da Esc açıldığı yere döndürür; ⌘P "Ayarlar", ayar menüsü ve MCP rozeti de açar. Modal değil. Diyalog kodu ve
> panel kodu silindi. **Kalan (öneriler):** ayar araması, "değiştirilenler", varsayılana dön, profiller, dışa/içe aktar.
> Ayarların başka bir bölmede açılması split view'a (O) bağlı.

**İstek:** "Settings diyalog penceresi artık diyalog olmasın, onun yerine ayrı bir sayfa olarak açılsın sağ
panelde."

**Mevcut:** `ryugo.Dialog` ile açılan pencere (`settings.go`: Görünüm, Editör, Durum çubuğu, MCP, Kitaplık
sekmeleri; `settingsOpen`). Sağ panel (`sidepanel.go`, 280 px) var: anahat, geri bağlantılar, bilgi.

**Yapılacaklar**
- Ayarlar, yan panelin içinde **bir sayfa** olarak açılır: üstte geri / kapat, altında sekmeler ya da bölüm
  listesi. Dar panelde sekmeler yerine tek sütun, başlıklı bölümler.
- Panel genişliği ayar için yetmez (tile'lar 128 px): panel ayarlar açıkken genişlesin (örn. 420–480 px) ve
  fare ile yeniden boyutlanabilsin (kalıcı).
- `settingsOpen` diyalog bayrağını panel durumuna çevir (`blockui.go:432` modal sayımından çıkar, çünkü artık
  modal değil: yazmaya devam edilebilmeli).
- ⌘, ve menü "Ayarlar…" paneli açıp kapatsın; Esc kapatsın; sayfa açıkken ayar değişince **canlı** uygulansın.
- Kullanılmayan diyalog kodu silinsin.
- Ayar sayfasında arama (Thymer'deki gibi) — ileride E'nin temeli: ayar listesi tek kayıttan gelsin.

**Kabul ölçütü:** Ayarlar sağ panelde açılır, diyalog yok, metin yazmak ve ayarı değiştirmek aynı anda
mümkün; ⌘, açar/kapatır; mevcut tüm ayarlar orada.

**Eklenen öneriler (7 Kasım)**
- **Ayar araması**: `/` ile yazılan sözcükle sayfadaki tüm ayarlar süzülür (Türkçe harf duyarsız), eşleşen
  bölüme atlar.
- **Değiştirilenler** görünümü: varsayılandan farklı ayarlar ayrı listelenir, tek tıkla **varsayılana dön** (ayar
  başına ve topluca).
- **Ayar profilleri / paketler:** tema + renk + yazı tipi + caret + boyut bir "profil" olarak kaydedilir
  (Yazı, Kod, Okuma gibi), tek komutla geçilir (⌘P → "Profil: Okuma").
- **Ayarları dışa/içe aktar** (JSON): başka bilgisayara taşımak ve senkron (L) için de gerekli.
- **Canlı önizleme:** ayar değişirken örnek metin kutusu ya da doğrudan açık sayfa anında değişir; panelin üstünde
  küçük bir örnek paragraf.
- Panelde açıklayıcı alt satır ve her ayarın yanında **komut adı/kısayol** (öğretir, E ile birleşir).
- Panel **yeniden boyutlanır** ve genişliği hatırlanır; dar ekranda tek sütuna iner.

---

## D — Editör ayarları: yazı boyutu, satır, aralık, otomatik kayıt, boşlukları göster

**İstek:** "font size, length size, spacing, autosave, show white space, bunların ayarlarının eklenmesi
gerekiyor."

**Mevcut:** yazı boyutu (`FontSize`, slider 13–24), satır aralığı (`LineHeight`), sayfa genişliği (`PageWidth`)
ayarlar var. Otomatik kayıt sabit (son düzenlemeden 0,7 sn sonra, `autosave`, `main.go`) ve kapatılamıyor. Boşluk gösterimi yok.

| Ayar | Durum | Yapılacak |
|---|---|---|
| Yazı boyutu | var | boyutu paletten de artır/azalt (⌘+ ⌘−), sıfırla (⌘0) |
| "Length size" | **belirsiz — açık soru 4** | satır yüksekliği (var) ve/veya satır uzunluğu = sayfa genişliği (var) |
| Spacing | yok | harf aralığı, paragraf aralığı (ve kelime aralığı) |
| Otomatik kayıt | yok (sabit) | aç/kapat, bekleme süresi (örn. 0,7 / 2 / 5 sn), kapalıyken ⌘S ile kayıt ve "kaydedilmedi" göstergesi |
| Boşlukları göster | yok | boşluk `·`, sekme `→`, satır sonu `¶` (ayrı ayrı açılabilir), ince gri, imleç ve seçimi bozmadan |

**Notlar:** Boşluk gösterimi editör çizimine (`richtext` paint) girer: MyGo `richtext`i buna seçenek vermiyorsa
bizim `Draw` katmanımızda (caret katmanı gibi) metnin üstüne çizilir. Otomatik kayıt kapatılınca pencere
kapanırken ve sayfa değişirken kayıt yine de sorulmalı: **veri kaybı olmaz** kuralı.

**Kabul ölçütü:** Beş ayar sayfada, anında etkili, yeniden açılışta korunuyor; otomatik kayıt kapalıyken yazılan
metin ⌘S ya da çıkışta kaydedilir.

**Eklenen öneriler (7 Kasım)**
- **Sekme (tab) genişliği**: Tab'ın eklediği boşluk (2/4/8 ya da boşluk karakteri olarak), şimdi sabit.
- **Odak modları:** daktilo kaydırması (imleç satırı ekranın ortasında kalır), **satır/paragraf vurgusu** (öteki
  silikleşir), tam ekran/sakin yazma (her şey gizli, yalnız metin).
- **Yazım denetimi** (işletim sisteminin sözlüğü), dil seçimi ve ayrı kelimeleri ekleme.
- **Sarma ve yön:** sözcük sarma, "sert sarma" sütunu (örn. 80), sağdan sola yazı (Arapça/İbranice) desteği.
- **Paragraf girintisi** (ilk satır), paragraflar arası boşluk ve **kenar boşlukları**.
- **İmleç ayarları:** caret hareket hızı, yanıp sönme, "uyku" süresi (şimdi 5 sn sabit) ve kapatma; caret
  efektlerinin **hepsini kapatan** anahtar (hafif mod).
- **Ayar ön-kümeleri:** Yazar (geniş, 1,7 satır), Kod (monospace, boşluklar açık), Sıkı (küçük, yoğun).
- Otomatik kayıt: **kayıt göstergesi** ("Kaydediliyor… / Kaydedildi 14:32") ve kapalıyken kapanışta sorma;
  `history.go`'daki sürüm geçmişiyle birlikte "kayıttan önceki hâle dön".
- Boşluk gösterimi: ayrı ayrı (boşluk, sekme, satır sonu, **kırılmaz boşluk**, satır sonunda artık boşluk
  uyarı rengiyle); seçimde ve kod bloğunda ayrı ayrı açılabilir.

---

## E — ⌘P ile uygulamadaki bütün ayarlar ve komutlar

**İstek:** "Comment P fonksiyonlarının ve komutlarının güncellenmesi ve geliştirilmesi... Mümkün olduğu kadar
uygulamanın hepsini Comment P ile yapabilmem lazım. Uygulamadaki ayarların hepsini."

**Mevcut:** komut listesi `palette.go` `commands()` içinde elle yazılmış (yeni sayfa, ayarlar, görünümler, tema,
MCP, caret, çöp …). Menü (`menu.go`) ve kısayollar ayrı yazılmış. Ayarların çoğu paletten değişmiyor.

**Yapılacaklar**
1. **Tek komut kaydı** ([08](08-komut-paleti-ve-kisayollar.md) önerisi): her komut `{ID, Ad, Kısayol, Bağlam,
   Çalıştır}`; menü, palet, kısayol ve (ileride) MCP aynı listeyi okur. Elle üç yerde tutmak biter.
2. **Her ayar bir komut:** aç/kapa ayarları ("Otomatik kayıt: aç/kapat"), seçmeli ayarlar için alt liste (palette
   "Caret →" Slime, Mizan, CRT…; "Köşeler →", "Yazı boyutu →"…) ve **← Geri**.
3. B'deki önizleme mantığı seçmeli ayarların hepsine: caret, tema, köşe, yazı tipi.
4. Eksik komutlar: kenar çubuğu, yan panel, ayarlar, odak modu, tablo/pano/liste görünümü, çöp, dışa/içe
   aktarma, MCP, senkronizasyon (L), güncelleme denetimi (K), istatistik (H), sayfa taşı, çoğalt, katla…
5. Bağlama göre sırala (sayfa açıkken sayfa komutları önde), son kullanılanlar üstte.
6. Her komutun kısayolu varsa gösterilsin; kısayolu olmayanlara atama (ileride ayar sayfasında).

**Kabul ölçütü:** Ayarlar sayfasındaki her ayar için palette bir komut var (testle denetlenir: ayar listesi ↔
komut listesi eşleşmesi); kaydı olmayan komut yok.

**Eklenen öneriler (7 Kasım)**
- **Palet modları:** `>` komutlar, `@` sayfa/koleksiyon, `#` etiket, `:` satıra/satır sonuna git, `?` yardım,
  `/` bu sayfada ara, `!` son çalıştırılanlar. Başlangıç modu ayarlanabilir.
- **Komut kaydından otomatik:** menü çubuğu, bağlam menüleri, kısayol yardımı, MCP araç listesi ve palet **aynı
  listeden** üretilir; yeni komut tek yerde eklenir.
- **Komutların yetkisi/durumu:** koşullu (örn. "Çöpten geri al" yalnız çöp doluyken), pasif komutlar silik ve
  nedenini söyler.
- **Eylemin sonucu:** komut sonrası bildirim + **Geri al** (silme, taşıma, tema, ayar).
- **Kısayol atama:** palette bir komutun yanında `⌘K ⌘S`/"Kısayol ata" ile doğrudan kısayol belirleme, çakışma
  uyarısı.
- **Komut zincirleri / makrolar** (ileride): iki üç komutu art arda çalıştıran kullanıcı komutu ("Günlüğü aç +
  yeni görev"), ayar dosyasında JSON.
- **Palet verisinin dili:** komut adları tek dilde tutulur, arama Türkçe/İngilizce eşanlamlıyı bulur.

---

## F — Yazı tipi: uygulama ve editör, bilgisayardaki fontlardan

**İstek:** "Uygulama font ayarları eklenebilmesi gerekiyor. Uygulama dili ve editör dilinin bilgisayardaki
fontlardan alınması gerekiyor. Çekilmeli." (Cümledeki "dil" kelimesi **açık soru 1**; font olarak anladım.)

**Yapılacaklar**
- İki ayrı seçim: **uygulama yazı tipi** (arayüz) ve **editör yazı tipi** (metin), ayrıca kod için **monospace**.
- Seçenekler **bilgisayardaki yüklü fontlardan listelenir** (sistem fontları taranıp çekilir). macOS, Windows,
  Linux'ta ayrı yol: önce MyGo'nun font API'sine bakılır (`a.font` zaten bir yazı tipi tutuyor); yoksa
  işletim sistemi listeleri (macOS CoreText, Windows DirectWrite/registry, Linux fontconfig) çalışma zamanında
  okunur, cgo'suz kalması için `purego` ya da dosya taraması (`~/Library/Fonts`, `/Library/Fonts`,
  `C:\Windows\Fonts`, `/usr/share/fonts`).
- Her satır **kendi fontuyla** yazılsın ve ↑↓ ile **canlı önizleme** (B ile aynı mantık). Arama kutusu.
- Yedek: seçilen font bulunamazsa (başka bilgisayar, senkronizasyon) gömülü varsayılana dönülür, hata vermez.
- Ayar adla saklanır (caret'te yaptığımız gibi), sıra değişince bozulmaz.
- Font dosyası boyutu: gömülü fontları azaltmak I maddesine yarar (varsayılan dışındakiler sistemden gelir).

**Kabul ölçütü:** Ayarlardan uygulama ve editör yazı tipi sistemdeki fontlardan seçilir, önizlenir, kalıcıdır;
olmayan font seçilirse varsayılan açılır.

**Eklenen öneriler (7 Kasım)**
- **Türkçe karakter denetimi:** seçilen fontta ı, İ, ş, ğ, ç, ö, ü eksikse uyarı ("Bu yazı tipi Türkçe
  harflerin bir kısmını içermiyor"), eksik harf yedekten gelir.
- **Font ayrıntıları:** ağırlık (kalın/normal), **ligatür** aç/kapa, **tabular rakamlar**, optik boyut, italik
  kesimi yoksa sahte italik uyarısı.
- **Önizleme metni:** listede her font kendi adıyla ve örnek bir Türkçe cümleyle ("Pijamalı hasta yağız şoföre
  çabucak güvendi") görünür; **favoriler** ve "son kullanılanlar".
- **Yedek zinciri:** ana font → sistem yedeği → gömülü varsayılan; tek ayar olarak "Yazı tipi yığını".
- **Yazı tipi ön-kümeleri:** Okuma (serif), Yazı (sans), Kod (mono).
- **Performans:** font listesi ilk açılışta arka planda taranır, sonuç önbelleğe alınır (binlerce font olabilir).

---

## G — Bütün uygulama klavyeden, fare olmadan (Vim gibi)

**İstek:** "Bütün uygulama, yani yapılabilen yerler çoğunlukla mesela Vim gibi klavyeden kullanılabilmesi
gerekiyor... ayarlar mesela açılabilir. Ondan sonra yani bütün uygulama klavyeden, fare kullanmadan
kullanılabilmesi gerekiyor."

**Mevcut:** ⌘K sayfa atla, ⌘P komut, ⌘F bul, ⌘\ kenar çubuğu, ⌘, ayarlar (menüde), ⌘[ ⌘], ⌘J, ⌃N, Tab (artık
metinde sahipleniliyor). Kenar çubuğu ağacı, koleksiyon görünümleri (liste, tablo, pano, takvim), ayarlar,
menüler, çipler, tablolar, sürükle-bırak çoğunlukla fare.

Bu madde bir **ilke + liste**: ilke şimdiden konur, liste ekran ekran tamamlanır.

**İlke:** Her ekranda (1) odak nerede olduğu görünür, (2) Tab / ⇧Tab ile sıralı, ok / `hjkl` ile yönlü gezilir,
(3) ↵ etkinleştirir, Esc geri/kapatır, (4) fareyle yapılan her şeyin klavye karşılığı vardır, (5) komut paleti
her şeye son çaredir (E).

**Yapılacaklar (ekran ekran)**
1. **Odak modeli:** hangi bölge odakta (kenar çubuğu, sayfa/metin, yan panel, ayarlar, palet) ve bölgeler arası
   geçiş (örn. ⌃W + h/l ya da ⌘1/⌘2/⌘3). Odak halkası (editörde kapalı kalır, çünkü caret var).
2. **Kenar çubuğu:** ↑↓ / `j k` gezinti, `l`/→ aç, `h`/← kapat, ↵ aç, `a` alt sayfa, `r` yeniden adlandır, `d`
   çöpe, `x` sürükle yerine taşı (⌘⇧↑↓ gibi), `gg`/`G` baş/son.
3. **Koleksiyon görünümleri:** liste/tablo satırlarında ↑↓, ↵ aç, boşluk seç, `/` süz, `s` sırala (sütun seç),
   pano: kartlar arası ←→↑↓ ve ⇧← ⇧→ ile sütun değiştirerek taşıma, takvim: gün ok tuşlarıyla.
4. **Ayarlar sayfası (C):** ↑↓ bölümler, ↵ değiştir, ←→ kaydırıcı/seçenek, `/` arama. "Ayarlar açılabilir":
   ⌘, (var) ve paletten.
5. **Metin editörü:** Vim kipi **isteğe bağlı** (ayar): normal/ekle/görsel kip, `hjkl`, `w b e`, `0 $`, `dd`,
   `yy p`, `u`, `/`; kapalıyken her şey olduğu gibi. **Açık soru 5:** tam Vim mi, yalnız gezinme mi?
6. **Menüler, bağlam menüleri, sayfa menüsü, caret menüsü:** ok tuşları + ↵; `⌃Space` satır menüsü (var).
7. **Çipler / bağlantılar:** `[[…]]` ve `#etiket` üzerinde klavyeyle aç (⌘↵ ya da `gf`).
8. **Tablo hücreleri, özellikler (Durum, Etiket, Tarih):** Tab / ↵ düzenle, Esc iptal (tabloda var).
9. **Çöp, görevler, arama, hızlı not, istatistik:** hepsi ↑↓ ↵ Esc ile.
10. **Sürükle-bırak:** her sürükleme için klavye karşılığı (taşı komutu).
11. **Kısayol listesi:** `?` ya da ⌘/ ile yardım sayfası, bağlama göre.
12. **Kısayolları değiştirme:** E'deki komut kaydı üzerinden ayar sayfasında (Thymer de böyle).
13. **Erişilebilirlik:** MyGo erişilebilirlik ağacıyla uyumlu etiketler (VoiceOver); odak sırası mantıklı.

**Zorluk:** 5 — çünkü her ekrana dokunur, test gerektirir ve MyGo'nun odak sistemiyle (`rt.focused`, Tab ile odak
kaydırma, `Shortcut` sahiplenmesi: Tab'ta yaşadığımız sorun gibi) uğraşılır. Küçük adımlarla, ekran başına bir
kabul testiyle yürür: *"fare olmadan şu işi yap"* testi (`tt.Key(...)` dizisi).

**Kabul ölçütü:** Yeni sayfa aç, yaz, alt sayfa ekle, koleksiyon görünümünü değiştir, ayarı değiştir, temayı
değiştir, sayfayı çöpe at, geri al, ara, dışa aktar: hepsi yalnız klavyeyle, her biri için test.

**Eklenen öneriler (7 Kasım)**
- **İpucu etiketleri (Vimium gibi):** `f` basınca ekrandaki tıklanabilir her şeyin (düğme, satır, çip, menü)
  üstünde kısa harf etiketleri çıkar, harfi yazınca tıklanır. **Fareyi tamamen gereksiz kılan en güçlü yöntem**
  ve yeni ekran başına ek iş istemez (MyGo'nun etkileşimli düğüm listesinden üretilir).
- **Leader tuşu + which-key:** `Space` (ya da `,`) sonrası seçenekleri gösteren küçük bir menü (`Space w` →
  pencere, `Space s` → ayarlar…). Hangi tuşa basılacağını ezberlemek gerekmez.
- **Sayısal önek** (`5j`), **tekrar** (`.`), **geri al/ileri al** (`u`, `⌃R`), **işaretler** (`m` + harf, `'`
  + harf ile sayfa atlama), **atlama listesi** (⌃O / ⌃I: Vim'deki gibi önceki yerlere).
- **Vim komut satırı:** `:` ile açılan satır paletle birleşir (`:w` kaydet, `:q` kapat/geri, `:theme koyu`,
  `:set fontsize=18`, `:e Sayfa adı`).
- **Kısayol çakışma denetimi** ve sistem kısayollarıyla çakışma uyarısı (macOS ⌘Space, ⌘H…).
- **Öğrenme:** boş durumlarda ve palette kısayol ipuçları, `?` ile bağlamsal kısayol sayfası, ilk açılışta kısa tur
  (kapatılabilir).
- **Odak halkası** tutarlı: editör hariç her yerde görünür; ayar "Odak halkasını her zaman göster".
- Klavye ile **bölme/pencere** yönetimi: yan paneli odakla (⌘2), kenar çubuğuna odaklan (⌘1), metne dön (⌘3 /
  Esc).

---

## H — İstatistik sayfası: yazım ve kullanım analizi

**İstek:** "İstatistik sayfası olması gerekiyor. Bir analiz tutulması lazım. Yazım, kullanım vesaire."

**Mevcut:** `writing.go`: günlük eklenen kelime sayısı ayarlarla birlikte tutuluyor (400 gün), bir **diyalog**
bugünü, haftayı, ayı, en iyi günü, seri günleri ve son 30 günü çubuklarla gösteriyor. Kullanım (süre, komut,
sayfa) verisi **yok**.

**Yapılacaklar**
1. **Diyalog değil sayfa** (C ile aynı yön): yan panelde ya da ana alanda ayrı bir sayfa; ⌘P "İstatistikler".
2. **Yazım:** kelime, harf, karakter; gün/hafta/ay/yıl; saat dilimlerine göre (hangi saatte yazıyorum); en uzun
   seri; ortalama yazma hızı (Mizan'ın wpm'i), en verimli gün/saat; hedef (günlük kelime hedefi, ilerleme).
3. **Kullanım:** uygulamada geçirilen aktif süre (gün/hafta), açılan/oluşturulan/silinen sayfa sayısı, en çok
   kullanılan komutlar ve kısayollar, en çok açılan koleksiyon/sayfa, arama sayısı, sayfa başına yazılan.
4. **Kitaplık:** sayfa/koleksiyon/etiket/bağlantı sayıları, en büyük sayfalar, en eski/en yeni, yetim sayfalar,
   bağlantı grafiği özeti.
5. **Görselleştirme:** ısı haritası (GitHub gibi yıllık gün ızgarası), çubuk/çizgi grafik; Ryugo grafik bileşenleri.
6. **Veri nerede:** yalnız yerelde, kitaplık dosyasının içinde (`settings` şişmesin diye ayrı tablo), **dışarı
   gönderilmez**. Silme ve dışa aktarma (CSV/JSON) düğmesi. Senkronizasyon (L) açılırsa isteğe bağlı.
7. Toplama maliyeti sıfıra yakın olmalı (olay sayaçları, belleğe birikir, birkaç saniyede bir yazılır).
8. A ile kalkan "bugün yazılan" göstergesinin yerini bu sayfa alır.

**Kabul ölçütü:** İstatistik sayfası yazım ve kullanım bölümleriyle açılır, veri yeniden açılışta durur,
dışa aktarılabilir, hiçbir şey ağa gitmez.

**Eklenen öneriler (7 Kasım)**
- **Hedefler:** günlük/haftalık kelime hedefi, ilerleme halkası, seri koruma ("2 gün daha: 7 günlük seri").
- **Yıllık ısı haritası** (GitHub benzeri gün ızgarası), **saat × gün** ısı haritası ("pazartesi 22:00
  en verimli saatim").
- **Haftalık/aylık özet sayfası**: otomatik oluşan, istersen günlüğe düşen Markdown özeti ("Bu hafta 8.420 kelime,
  3 yeni koleksiyon").
- **Dağılım:** koleksiyon ve etiket bazında yazılan kelime, sayfa başına süre, en çok düzenlenen sayfalar.
- **Yazma oturumları:** kesintisiz yazma süresi, oturum başına kelime, ortalama hız (wpm), en uzun oturum.
- **Kitaplık sağlığı:** yetim sayfalar, kırık `[[bağlantı]]`lar, boş sayfalar, aynı başlıklı sayfalar (temizlik
  önerisi).
- **Kullanım:** komut kullanım sayacı → palette "sık kullanılan" sırası (B ile ortak veri), hiç kullanılmayan
  özellikler ("keşfet" önerisi).
- **Gizlilik ve denetim:** "İstatistik toplama" tek anahtarla kapanır; "Sıfırla"; veri dosyası düz JSON/CSV, her
  an dışa aktarılır; **senkron (L) açılsa bile** istatistik ayrı seçenek.
- Rakamlar **açıklanabilir** olsun: "kelime" nasıl sayıldı, silinenler sayılıyor mu belgesi sayfada.

---

## I — Uygulama boyutunu küçült, özellik kısmadan

**İstek:** "Optimizasyon olarak dosya boyutunun mümkün olduğu kadar küçük olması gerekiyor. Küçültebildiğimiz
kadar dosyayı, yani uygulama boyutunu küçültmemiz gerekiyor. Özellikleri kısmadan."

**Bugünkü ölçüm** (6 Kasım, `-ldflags="-s -w"`, `CGO_ENABLED=0`): macOS arm64 20,3 MB, macOS amd64 21,8 MB,
Linux amd64 21,4 MB, Linux arm64 19,9 MB, Windows amd64 21,9 MB. Sembolsüz değilken macOS arm64 29,7 MB.

**Yapılacaklar (önce ölç, sonra kes)**
1. **Neyin ne kadar yer tuttuğunu çıkar:** `go tool nm -size -sort size`, paket bazında dökümü (goweight benzeri),
   bağımlılık ağacı (`go mod graph`), gömülü varlıklar (`embed`: fontlar, ikonlar, kapaklar, `assets/`).
2. **Derleme bayrakları:** `-trimpath`, `-ldflags="-s -w"` (var), `-buildvcs=false`, gerekirse `-gcflags=-l`
   etkisi ölçülür; PGO ve inlining değişimleri boyutu hem artırıp hem azaltabilir, her biri ölçülür.
3. **Bağımlılıklar:** MCP SDK (`go-sdk`) oauth2, jsonschema, uritemplate, segmentio gibi paketleri çekiyor
   (`go.mod`). Gerçekten kullanılan kısım için daha hafif bir MCP uygulaması (yalnız streamable HTTP + JSON-RPC,
   elle) mümkün mü? **Özellik kısılmaz**: MCP'nin 9 aracı aynı kalır. SQLite sürücüsü ve `yaml` benzer şekilde
   ölçülür.
4. **Gömülü varlıklar:** SVG ikonlar ve fontlar sıkıştırılır (brotli/zstd gömüp açılışta açmak), kullanılmayan
   font ağırlıkları atılır. F ile sistem fontlarına geçilirse gömülü font azalır.
5. **Ölü kod:** kaldırılan caretler ve A'daki göstergeler zaten kodu azaltır; `deadcode` aracı ile erişilemeyen
   fonksiyonlar bulunur.
6. **Sıkıştırma:** UPX çalışır ama macOS imza/notarization ve Windows virüs yazılımı sorunları var; **önerilmez**,
   yalnız ölçüm için bakılır. Güncelleme paketi (K) zaten sıkıştırılmış iner.
7. **Boyut bütçesi:** CI'da ya da betikte her sürümde boyut yazılsın, belirlenen bütçeyi (örn. 15 MB) aşınca
   uyarsın. İlk hedef rakamı ölçümden sonra birlikte koyarız.
8. Çalışma zamanı belleği de ayrı: 7,7 bin sayfada RSS 690 MB ölçülmüştü; boyut işinin yanında bu da
   ölçülsün.

**Kabul ölçütü:** Dökümü çıkan boyut tablosu, uygulanan her adımın kazancı, özellik kaybı olmadığını gösteren
tam test seti geçmesi, yeni boyutlar (5 hedef için).

**Eklenen öneriler (7 Kasım)**
- **Boyut raporu otomatik:** her derlemede 5 hedefin boyutu tabloya yazılır, önceki sürümle farkı gösterilir
  (`git` etiketine bağlı); bütçe aşılınca derleme uyarır.
- **Gömülü varlıklar tembel açılsın:** yalnız gerektiğinde açılan sıkıştırılmış (zstd/brotli) varlıklar; hiç
  kullanılmayan temalar/ikonlar derlemeden çıkar (derleme etiketleriyle ya da ayrı paket).
- **İki kip:** "tam" ve "küçük"; küçük kipte yalnız bu oturumda lazım olmayan özellikler (örn. MCP, içe aktarma)
  ilk kullanımda indirilen eklentiden gelir. (Özellik kısmadan: yalnız dağıtım biçimi değişir. Karar sende:
  indirme gerektirir.)
- **Açılış süresi ve bellek de bütçeli:** ilk kare süresi, 10 bin sayfada bellek; regresyon testi.
- **Sürüm boyutu karşılaştırması** güncelleme paketi için de: `bsdiff` farkları (K) yalnız değişen baytları indirir.

---

## J — Web uygulaması: araştırma ve deneme

**İstek:** "Mobil uygulama ve web uygulaması yapılabilir mi? Bunun araştırılması ve test edilmesi gerekiyor."

**Bildiklerim:** MyGo belgeleri (`docs/cli.md`, `architecture.md`) tarayıcı hedefini destekliyor: `GOOS=js
GOARCH=wasm`, WebGL 2 tuvali üzerinde yerel arayüz, `build -platform web`. Yani arayüz kodu çoğunlukla taşınabilir.
**Bilmediklerim ve denenecekler:**

1. **Derleme denemesi:** projeyi `js/wasm` için derle, ne kırılıyor (dosya sistemi, SQLite, `purego`, pencere/menü
   çağrıları, MCP sunucusu, saat/dosya izleme).
2. **Depolama:** masaüstünde kitaplık `kitaplik.db` (SQLite) ve Markdown klasörü. Tarayıcıda dosya sistemi
   yok: IndexedDB / OPFS (Origin Private File System) / File System Access API (kullanıcının klasörü) ya da
   sunucu (L senkronizasyonu). Hangisi: deneyle seçilir.
3. **Boyut ve açılış:** wasm dosyası büyük olur (Go wasm tipik olarak masaüstü ikilisinden büyük/aynı); I
   maddesindeki küçültme ve brotli ile aktarım ölçülür; TinyGo mümkün mü (MyGo ile uyumlu değilse hayır).
4. **Girdi:** klavye kısayolları tarayıcıyla çakışır (⌘P yazdır, ⌘T, ⌘W, ⌘L…): G maddesi ile birlikte düşünülür;
   IME/metin girişi, pano, sürükle-bırak, dosya bırakma.
5. **MCP ve yerel işlemler:** tarayıcıda yerel MCP sunucusu açılamaz; web sürümünde bu özellik nasıl olur
   (sunucu üzerinden?).
6. **Yayın:** statik dosya olarak Cloudflare Pages/Workers (zaten `ui.halid.dev`'de benzer yayın yaptık).
7. **Çıktı:** "yapılabilir mi" sorusunun yazılı cevabı: ne çalışıyor, ne çalışmıyor, tahmini iş, boyut, öneri.

**Kabul ölçütü:** Çalışan bir deneme sürümü (yazma, sayfa açma, kaydetme) ve bir kısa rapor (`docs/`): sınırlar,
depolama seçimi, boyut, tahmini iş.

**Eklenen öneriler (7 Kasım)**
- **PWA:** kurulabilir (ana ekrana ekle), **çevrimdışı çalışır** (service worker + IndexedDB/OPFS), yerel bildirim.
- **Yayınla:** bir sayfayı ya da koleksiyonu **salt-okunur web sayfası** olarak Cloudflare'da yayınlama
  (kişisel site/not paylaşımı). Gizli bağlantı, istenince kapatma.
- **Dosya erişimi:** destekleyen tarayıcılarda File System Access API ile kullanıcının gerçek Markdown klasörünü
  açma (masaüstü sürümüyle aynı klasör).
- **Küçük ilk yükleme:** wasm brotli ile gelir, ilerleme çubuğu, ikinci açılışta önbellekten.
- **Test planı:** Chrome, Safari, Firefox; klavye kısayol çakışmaları; IME ve pano; dokunmatik/trackpad.

---

## K — Otomatik güncelleme ve yeni sürüm uyarısı

**İstek:** "GitHub'da release paylaşıldığında güncellemenin otomatik olarak yapılabilmesi ve uyarı verilebilmesi,
bunların araştırılması gerekiyor. Cloudflare üzerinden güncelleme yapılabilir veya veriler orada tutulabilir."

**Araştırılacaklar ve önerilen yol**
1. **Sürüm bilgisi:** derlemeye `-ldflags "-X main.version=…"` ile sürüm gömülür (şimdi yok); sürüm etiketi `vX.Y.Z`.
2. **Yeni sürüm denetimi:** açılışta ve belli aralıkla (ayarlanabilir, kapatılabilir) GitHub Releases API
   (`/repos/<sahip>/<repo>/releases/latest`) ya da kendi manifestimiz sorgulanır; sürüm karşılaştırılır; yeni
   varsa **uyarı** ("Sürüm 1.4 hazır · Notlar · Şimdi güncelle · Sonra"). Ayarlar: otomatik denetle / indir /
   kur, kanal (kararlı / önsürüm).
3. **Özel depo sorunu:** `imhalid/ryudit` **özel** (private). Özel depodan release varlığı indirmek için belirteç
   gerekir ve belirteç uygulamaya gömülemez. Seçenekler: (a) yalnız sürüm deposunu herkese açık yap (kaynak kod
   değil, yalnız sürüm dosyaları: ayrı `ryudit-releases` deposu), (b) **Cloudflare** üzerinden sun: GitHub Actions
   release'te dosyaları R2'ye yükler, Worker `update.json` manifestini verir; uygulama yalnız Cloudflare'a
   konuşur. Öneri (b): belirteç gerekmez, sürüm ve indirme sayacı kendi elimizde, hız iyi.
4. **Güvenlik:** indirilen dosyanın **imzası doğrulanmalı** (ed25519 / minisign: açık anahtar uygulamada, gizli
   anahtar yalnız CI'da); sağlama toplamı manifestte; yalnız HTTPS. İmzasız güncelleme kurulmaz.
5. **Kurulum biçimi (işletim sistemine göre):**
   - macOS: `.app` içindeki ikili ve paket güncellenir; **imza ve notarization** şart (Apple Developer hesabı),
     yoksa Gatekeeper engeller. Sparkle benzeri yaklaşım ya da kendi değiştirme + yeniden başlatma.
   - Windows: çalışan `.exe` kendini değiştiremez: yan dosyaya indir, kapanınca değiştiren küçük bir yardımcı ya da
     yeniden adlandırma hilesi; imza (Authenticode) yoksa SmartScreen uyarır.
   - Linux: AppImage / dağıtım paketi; kendi içinde değiştirme mümkün.
6. **Akış:** bildirim → kullanıcı onayı → arka planda indir → doğrula → "yeniden başlat ve güncelle". Başarısızlıkta
   eski sürüme dön (eski ikiliyi sakla). Kitaplık verisi **hiç değişmez**; veri şeması değişiyorsa sürüm içi
   geçiş + yedek.
7. **Gizlilik:** denetim yalnız sürüm sorar (kitaplık verisi gönderilmez); kapatılabilir.
8. İlk adım küçük: yalnız **uyarı** (indirme bağlantısı açar); otomatik kurulum ikinci adım.

**Kabul ölçütü:** Yeni sürüm yayınlanınca uygulama uyarı veriyor; imza doğrulanıyor; üç işletim sisteminde
güncelleme akışı denenmiş; başarısızlıkta geri dönüş var.

**Eklenen öneriler (7 Kasım)**
- **Sürüm notları** uygulama içinde (Markdown) uyarıyla birlikte gösterilir; "Bu sürümde neler yeni" sayfası
  güncellemeden sonra ilk açılışta çıkar.
- **Kanallar:** kararlı / deneme; "Bu sürümü atla"; "Beni 3 gün sonra hatırlat"; **sessiz mod** (yalnız
  ayarlarda göster).
- **Fark güncellemesi (delta):** yalnız değişen baytlar iner (bsdiff/zstd), uygulama boyutundan bağımsız küçük
  güncelleme.
- **Güvenli geri dönüş:** güncellemeden önce kitaplığın yedeği, açılış başarısız olursa eski sürüme otomatik dönüş
  ("güncelleme sonrası ilk açılışta çökerse geri al").
- **Yerel çökme günlüğü** (N1): güncelleme sonrası çökme yaşanırsa sürüm numarasıyla günlüğe yazılır.
- Denetim sıklığı ve zamanı ayarlanabilir; **kurumsal/çevrimdışı mod**: hiç ağa çıkma.

---

## L — Senkronizasyon

**İstek:** "Uygulama için senkronizasyon özelliğinin eklenmesi... Cloudflare üzerinden ... veriler orada
tutulabilir."

**Önce karar gerekenler (araştırma):**
- **Neyi senkronlamak:** kitaplık varsayılan olarak tek SQLite dosyası (`kitaplik.db`), istenirse düz Markdown
  klasörü. Senkron birimi: sayfa (dosya) bazında mı, olay günlüğü mü? Sayfa bazında ve sürüm numaralı en basiti.
- **Nerede:** (1) **Cloudflare**: Worker + **D1** (sayfa meta) + **R2** (ekler, görseller) ya da **Durable
  Objects** (hesap başına tek yazar, gerçek zamanlı); (2) kullanıcının kendi klasörü (iCloud, Syncthing, Dropbox:
  zaten Markdown ve dosya izleme var, `watch.go`); (3) Git. Üçü birlikte düşünülebilir ama **tek ana yol**
  seçilmeli.
- **Çakışma:** şimdiki yaklaşım "çakışma kopyası" (`index (çakışma …).md`). Senkronda sayfa başına
  `rev` + son değişiklik zamanı; çakışma olursa iki kopya tutulur ve kullanıcıya gösterilir; metin birleştirme
  (3 yollu) ikinci adım.
- **Güvenlik:** **uçtan uca şifreleme** (parola türetilmiş anahtar, sunucu düz metin görmez) mü, yalnız aktarım
  şifresi mi? Kimlik: hesap (e-posta bağlantısı / passkey) ya da cihaz anahtarı eşleştirme (QR). Cloudflare Access
  ya da kendi basit belirteç sistemi.
- **Çevrimdışı:** önce yerel yazılır, bağlanınca eşitlenir; kuyruk, yeniden deneme, kısmi indirme.
- **Silme:** çöp mantığı (var: `.trashed.json`) eşitlenir, kalıcı silme izlenir.
- **Dosya izleme ile ilişkisi:** `watch.go` diske dışarıdan yazılanı izliyor; senkron motoru da dosyaya yazan bir
  "dış süreç" gibi davranabilir mi (basit ama kırılgan), yoksa içerden mi.
- **Maliyet/limit:** D1, R2, Workers ücretsiz katman limitleri; kullanıcı başına depolama.
- **Web (J) ve Android (M) için ortak arka uç:** senkron sunucusu bu ikisinin de veri kaynağı olabilir.

**Yapılacaklar (önerilen aşamalar):** (1) tasarım belgesi + küçük deneme (iki cihaz, tek koleksiyon, tek yönlü) →
(2) çift yönlü sayfa senkronu, çakışma kopyası → (3) ekler, silme, şifreleme → (4) ayarlar sayfasında "Senkron"
bölümü, durum göstergesi, ⌘P komutları (E).

**Kabul ölçütü:** İki bilgisayar aynı hesapla bağlanıp bir sayfa yazar, diğerinde görür; çevrimdışı yazılan
sonradan eşitlenir; çakışma veri kaybettirmez.

**Eklenen öneriler (7 Kasım)**
- **Seçmeli senkron:** koleksiyon bazında aç/kapa (örn. "Günlük" eşitlensin, "Özel" yalnız bu cihazda).
- **Sürüm geçmişi sunucuda:** sayfa başına son N sürüm, belirli bir ana dönme (şimdiki `history.go`'nun uzantısı).
- **Cihaz yönetimi:** bağlı cihaz listesi, bir cihazı uzaktan çıkarma, yeni cihaz eşleştirme (QR/kod).
- **Durum rozeti:** durum çubuğunda/başlıkta eşitleniyor/eşit/çevrimdışı/çakışma; çakışma çözüm ekranı (iki
  sürüm yan yana, "ikisini de tut/biri").
- **Otomatik yedekleme:** günlük anlık görüntü (R2), "tarihe göre geri yükle"; dışa aktarma tek komut.
- **Paylaşım:** koleksiyonu bir başkasıyla salt-okunur ya da düzenlenebilir paylaşma (ikinci aşama).
- **Veri gizliliği:** uçtan uca şifreleme seçeneğinde parola kaybı = veri kaybı uyarısı ve kurtarma anahtarı.
- **Ağ kullanımı:** yalnız Wi-Fi'da eşitle, ekler için boyut sınırı, hız ayarı.

---

## M — Android uygulaması: araştırma ve deneme

**İstek:** "Mobil uygulama ve web uygulaması yapılabilir mi? ... Mobilde özellikle Android."

**Bildiklerim:** MyGo belgelerinde **Android hedefi geçmiyor** (masaüstü + web). Dolayısıyla bu, bir "port" değil
bir **araştırma**.

**Yollar (sırayla denenir)**
1. **MyGo Android'e çıkabiliyor mu?** MyGo geliştiricisine/belgeye bakılır; `GOOS=android` derlemesi, pencere ve
   girdi katmanı (`internal/…`) yok. Yoksa: bu yol kapanır ya da MyGo'ya katkı gerekir.
2. **Go çekirdeği + yerel arayüz:** Go kısmı (kitaplık, Markdown, arama) `gomobile bind` ile AAR yapılır; arayüz
   **Kotlin + Jetpack Compose** (önceki Android projelerimiz gibi). Editör (zengin metin) Compose'ta yeniden
   yazılır: büyük iş.
3. **Web sürümünü sarmak (J):** WebView/PWA içinde çalışan wasm sürümü; dokunmatik ve yazılım klavyesi için
   uyarlama gerekir. En ucuz yol; kalite ve performans ölçülür.
4. **Veri:** telefon tek başına çalışacaksa yerel kitaplık; asıl değer senkron (L) ile masaüstüyle eşitlemek.
5. **Dokunmatik arayüz:** kenar çubuğu çekmece, uzun basma menüleri, caretler (çoğu fareye/klavyeye göre
   tasarlı) mobilde anlamsız olabilir; G'deki klavye ilkeleri harici klavye için geçerli.
6. **Çıktı:** "yapılabilir mi" kararı + en makul yol + tahmini iş + deneme APK'sı (yazma + kaydetme).

**Kabul ölçütü:** Emülatörde çalışan bir deneme (sayfa listesi, sayfa açma, yazma, kaydetme) ve yazılı rapor.

**Eklenen öneriler (7 Kasım)**
- **Hızlı not:** bildirim çubuğu/ekran widget'ı ve **paylaş menüsünden** ("Ryudit'e ekle": web sayfası, metin,
  görsel) tek dokunuşla not.
- **Sesle yazma** (sistem klavyesi dikte) ve **kamera ile belge/görsel ekleme**.
- **Çevrimdışı öncelikli**, senkron (L) kuyruğu arka planda; pil dostu.
- Harici klavye için G'deki kısayollar; ekran klavyesi için **kısayol çubuğu** (Tab, ⌘, madde işareti, #, [[ ).
- **Tema ve yazı tipi** masaüstüyle aynı ayar profili (C) üzerinden gelir.

---

## O — Bölünmüş görünüm (split view)

> **Durum (7 Kasım, gece): temel yapıldı.** Yolu değiştirdim: editör durumunu `pane` yapısına taşımak yerine, bölme
> başına olan 77 alanı `paneState` yapısına toplayıp `app`e **gömdüm** (`panes.go`); `a.page`, `a.doc` gibi bütün
> kod olduğu gibi çalışıyor, kullanımdaki bölmenin durumunu görüyor. Diğer bölme çizilirken durumu içeri alınır, çizim
> bitince geri çıkarılır (build ve paint için ayrı). Her ekran bir bölmede açılabiliyor: sayfa, koleksiyon, ayarlar,
> çöp, görevler, arama, etiket. **Giriş yolları:** kenar çubuğundan sürükleyip içeriğin sağ kısmına bırakmak (parlayan
> "Yan bölmede aç" bölgesi; iki bölme varken her yarı kendi bölmesi), sayfa ve koleksiyon sağ tık menüsünde "Yan
> bölmede aç", menü ve ⌘P ("Yan bölmede aç: bu ekran / Ayarlar / Görevler / Çöp", "Bölmeler arası geç", "Bölmeyi
> kapat"), kısayollar ⌥⌘↵, ⌥⌘→, ⌥⌘W; her bölmede × düğmesi. Aynı sayfa iki bölmede tek metindir. Kütüphane diskten
> yeniden okununca diğer bölme de sayfasını bulur. **Bilinen sınırlar:** geri/ileri geçmişi bölmeler arasında ortak;
> ayırıcı sürüklenip boyutlanmıyor (eşit genişlik); en çok iki bölme; bölmeler yeniden açılışta hatırlanmıyor; ayarlar
> ekranı dar bölmede sıkışık (sekmeli düzen yapılabilir); `/` menüsü ve blok tutamağı tek (kullanımdaki bölmenin).
> Testler: iki farklı sayfa, aynı sayfa, ekran bölmesi, sürükle-bırak, menüler, yeniden okuma.

**İstek (7 Kasım, akşam):** "Split view'a geçebiliriz. Ayarlar sağdan açılıyor, sağdan açılmak yerine normal ekran gibi
açılmalı. Sol taraftan sürükleyip bırakınca split view yapabilmeliyim ya da sağ tıklayınca *open side panel* diye
açabilmeliyim."

**Anlaşılan:** (1) Ayarlar ana alanda normal bir ekran (C'de yapıldı). (2) Kenar çubuğundan bir sayfayı ya da
koleksiyonu sürükleyip pencerenin sağ yarısına bırakınca iki bölme yan yana. (3) Sayfanın/koleksiyonun sağ tık menüsünde
"Yan bölmede aç" (Thymer'deki ⌘↵ "yanda aç" ile aynı). (4) Her bölmede her ekran olabilir: sayfa, koleksiyon,
ayarlar, çöp, görevler, arama, etiket.

**Neden büyük iş:** uygulama (~31 bin satır) tek bir "açık sayfa" varsayıyor. Editörün durumu `app` yapısında tek
tek alanlar: `a.page`, `a.doc`, `a.version/length/caretAt`, `a.wrote/erased/typed`, `a.focused`, `a.pageScroll`,
`a.editorR`, `a.boxes`, kaydetme (`autosave`, `a.dirty`), `/` menüsü, çipler, tablolar, bul, seçim çubuğu, caret
katmanı. Görünümler de tek bayraklarla seçiliyor: `a.coll`, `a.trashOpen`, `a.tasksOpen`, `a.searchOpen`, `a.tag`,
`a.settingsOpen`. İkinci bir bölme için bunların **bölme başına** bir kopyası gerekir.

**Önerilen yol (aşamalı, her aşama testle yeşil kalır):**
1. **Bölme yapısı (davranış değişmez):** editör durumunu ve görünüm bayraklarını `type pane struct{…}` içine taşı;
   `a.main` tek bölme gibi çalışsın. En riskli aşama; ~43 `a.doc` ve ~16 `a.page` kullanımı ana dosyada, çoğu `slash.go`,
   `blockui.go`, `chips.go`, `tables.go`, `selbar.go`, `find.go`'da. Mevcut tüm testler bu aşamada aynen geçmeli.
2. **İkinci bölme, önce ekranlar:** ayarlar, çöp, görevler, arama gibi **editörsüz** görünümler sağ bölmede açılsın
   (düşük risk, hemen işe yarar: "ayarları sağda aç"). Ayırıcı çizgi, kapat düğmesi.
3. **İkinci bölmede sayfa editörü:** aynı sayfa iki bölmede açıksa aynı `richtext.Document` paylaşılır (canlı
   yansır); farklı sayfaysa kendi belgesi (`a.docs` önbelleği zaten var). Her bölmenin kendi kaydırması, imleci, odağı;
   caret efekti ve `/` menüsü yalnız odaktaki bölmede. Kaydetme sayfa başına tek yol (iki bölme aynı dosyayı ezmesin;
   dosya izleyici `watch.go` iki belgeyi de yenilemeli).
4. **Etkileşim:** kenar çubuğundan sürükle-bırak (sağ yarı bırakma bölgesi vurgusu; `drag.go`'daki satır sürükleme
   altyapısı kullanılır), sağ tık "Yan bölmede aç" (`pagemenu.go`, koleksiyon menüsü), ⌘↵, bölmeler arası odak
   (⌘⌥←/→), ayırıcıyı sürükleyerek boyut (yay hareketli, kalıcı), bölmeyi kapat (⌘W), yer değiştir, tek bölmeye dön.
5. **Kalıcılık:** açık bölmeler ve genişlikleri ayarlarda (G ile klavye kısayolları ve ⌘P komutları).

**Kabul ölçütü:** Bir sayfa kenar çubuğundan sağ yarıya sürüklenip bırakılınca iki bölme yan yana açılır; sağ tıkla "Yan
bölmede aç" aynısını yapar; ayarlar sağ bölmede de açılabilir; iki bölmede de yazılabilir; tüm testler geçer.

**Eklenen öneriler (7 Kasım)**
- Bölme sınırı: önce iki bölme; üç ve dikey bölme sonra.
- Bölmede sekme yok, ama **geri/ileri geçmişi bölme başına** (şimdiki `hist` bölme başına).
- Aynı sayfayı iki bölmede açmak **odak modu** olarak da işe yarar (üstte yazı, altta anahat gibi).
- Pencere dar olunca otomatik tek bölmeye düş (mobil/web J ve M için de gerekli).

---

## Eklenen yeni maddeler (7 Kasım)

Kullanıcı, listelediği özelliklere eksik gördüklerimi ve daha iyi yanlarını eklememi söyledi. Yukarıda her
maddenin altında "Eklenen öneriler" var. Aşağıdakiler hiçbir maddeye sığmayan ama listenin işini sağlamlaştıran
yeni maddeler.

| # | İş | Önem | Zorluk | Neden |
|---|---|---|---|---|
| N1 | Yerel çökme günlüğü ve "son oturum çöktü" bildirimi | Y | 2 | 6 Kasım'da tablo görünümü açılışta çöktü ve nedenini ancak çıktıyı okuyunca bulduk. Kullanıcı terminal açmadan da görebilmeli. |
| N2 | Otomatik yedek ve dışa aktarma | Y | 3 | Senkron (L) ve güncelleme (K) veriyi riske atar; önce güvenlik ağı. Kitaplık tek SQLite dosyası: bozulursa her şey gider. |
| N3 | "Fare olmadan" test altyapısı | Y | 3 | G'nin kabul ölçütü bu; her ekran için `tt.Key(...)` dizisiyle yazılan senaryo testleri, ortak yardımcılarla. |
| N4 | Arayüz dili (Türkçe/İngilizce) ve metinlerin tek yerde toplanması | O | 3 | Açık soru 1; belge ve metinler şimdi koda gömülü (Türkçe). Web ve Android'de ve açık kaynak olursa lazım. |
| N5 | Yardım sayfası, kısayol listesi ve ilk açılış turu | O | 2 | G ve E'nin yan ürünü: öğrenilebilirlik. |
| N6 | Büyük kitaplık performans ölçümü | O | 3 | 7,7 bin sayfada 690 MB bellek ve 3 sn MCP araması ölçülmüştü; I (boyut) yanında bellek ve hız bütçesi. |

**N1 — Çökme günlüğü.** `panic` yakalanır (`recover`), yığın izi ve sürüm numarası kullanıcı yapılandırma
klasörüne `crash-<tarih>.log` olarak yazılır; kitaplığa dokunulmaz. Bir sonraki açılışta "Son oturum beklenmedik
kapandı · Günlüğü göster · Kopyala" bildirimi. Günlük yalnız yerelde, hiçbir yere gönderilmez; son 20 dosya
tutulur. (Go'da arayüz iş parçacığındaki panik uygulamayı düşürür: `defer recover` çerçeve işlevinde de
denenmeli, kurtarılabiliyorsa o karede hata kutusu gösterip devam edilir.)

**N2 — Yedek.** Her gün ilk açılışta (ve büyük işlemlerden, örn. içe aktarma/senkron/güncelleme öncesi) kitaplığın
anlık görüntüsü (`.db` kopyası, sıkıştırılmış) yapılandırma klasörüne; son 7 gün + 4 hafta tutulur. "Yedekten
geri yükle" komutu (⌘P). Markdown klasörü olarak dışa aktarma (var olan içe aktarmanın tersi) ve zip.

**N3 — Test altyapısı.** `keyboardOnly(t, steps…)`: fare olayı göndermeyi yasaklayan test yardımcısı; senaryo
kitaplığı ("yeni sayfa aç, yaz, alt sayfa ekle, ayarı değiştir, temayı değiştir, çöpe at, geri al").

**N4 — Dil.** Tüm arayüz metinleri bir sözlüğe (`tr`, `en`); ayarlardan seçim, sistem diline göre açılış.
Editörün yazım denetimi dilinden ayrı bir ayar (açık soru 1'e bağlı).

**N5 — Yardım.** `?` ile bağlamsal kısayol sayfası (E'deki komut kaydından üretilir), ilk açılışta 4 adımlık tur
(kapatılabilir), boş durumlarda ipucu.

**N6 — Ölçüm.** Sentetik 10 bin sayfalık kitaplıkla açılış süresi, kare süresi, bellek ve arama süresi
ölçülür, bütçe testi olarak tutulur (`go test -bench`).

---

## Açık sorular (cevaplandı, 7 Kasım)

Cevaplar kullanıcıdan geldi; her birinin altında ne yapıldığı ya da ne kaldığı yazılı.

1. **Uygulama dili ve editör dili … fontlardan** (F). *Cevap: çözüldü.* Font olarak yapıldı; arayüz dili (Türkçe/İngilizce) ayrıca var (N4).
2. **Durum çubuğunda neler kalacak** (A). *Cevap: çözüldü.* Kaydetme durumu, satır/sütun, harf sayısı kalıyor; üstüne duruma göre kısayol ipuçları geldi (docs/22 madde 17).
3. **Geri bağlantılar** (A). *Cevap: "yan paneldeki ayarlar panel yerine küçük bir floating panel olsun"; sorulunca: sağ panel yüzen olsun.* Yapıldı: sağdaki
   yan panel (içindekiler, etiketler, geri bağlantılar, bilgi) artık yerleşimin bir sütunu değil, sayfanın üstünde yüzen küçük bir kart
   (`sidepanel.go`): başlık çubuğunun altında, sağda, içeriği kadar uzun, sağdan süzülerek gelir, kapatma düğmesi vardır, sol kenarından genişliği
   sürüklenir; metinden yer almaz. ⇧⌘\ ve ayar eskisi gibi açıp kapatır.
4. **"Length size"** (D). *Cevap: çözüldü.*
5. **Vim kipi** (G). *Cevap: tam Vim kipi değil; her aksiyonun kendi kısayolu olsun.* Böyle: palet komutlarının her birine kendi tuşu verilebiliyor
   (Ayarlar ▸ Kısayollar, `keys.go`), normal kipte tek harfli komutlar var (`kb.go`), Vim hareketleri isteğe bağlı kalıyor; tam Vim kipi yapılmayacak.
6. **⌘, düğmesi.** *Cevap: bilmiyorum.* Kodda ⌘\ kenar çubuğu, ⌘, ayarlar kaldı; çakışma görülürse söylenecek.
7. **Senkron altyapısı** (L). *Cevap: Cloudflare düşünülüyor, ama ikisi de (klasör ve sunucu) olmalı; şifreleme şart.* İkisi de var. **Şifreleme artık şart:**
   parola yazılmadan eşitleme başlamıyor (`syncRemote` anahtarsız uzak vermiyor; ayarlardaki "kapat" anahtarı kalktı). `TestSyncNeedsEncryption`.
8. **Güncelleme** (K). *Cevap: şimdilik yalnız kendisi için.* Apple/Windows imzası ve herkese açık depo gerekmiyor; kendi bilgisayarında, kendi sunucundan güncelleme
   yeter. İmza işleri ertelendi.
9. **Split view** (O). *Cevap: çözülüyor.* Bölmeler Ryugo'nun Dock'uyla yapıldı (docs/22 madde 11).
10. **Ayarlar ekranı** (C). *Cevap: çözüldü.*

---

## Mesajdaki her cümle nerede?

| Kullanıcının sözü | Madde |
|---|---|
| "bütün uygulama … Vim gibi klavyeden kullanılabilmesi" | G |
| "Command virgül … sidebar … ayarlar mesela açılabilir" | G.4, açık soru 6 |
| "bütün uygulama klavyeden, fare kullanmadan" | G |
| "istatistik sayfası … analiz … yazım, kullanım vesaire" | H |
| "dosya boyutunun … küçük … özellikleri kısmadan" | I |
| "Mobil uygulama ve web uygulaması yapılabilir mi? … araştırılması ve test edilmesi … özellikle Android" | J (web), M (Android) |
| "Comment P fonksiyonlarının ve komutlarının güncellenmesi ve geliştirilmesi" | E |
| "Comment P ile tema değiştirme yapamıyorum" | B |
| "Ayarlar modalını açamıyorum" | B |
| "Tema değiştirebilmem lazım … ön izleme olarak görebilmem" | B |
| "uygulamanın hepsini Comment P ile … ayarların hepsini" | E |
| "Settings diyalog … ayrı bir sayfa … sağ panelde" | C |
| "sayfa yolu, seçili harf sayısı, okuma süresi, bugün yazılan harf sayısı, kelime sayısı, son düzenleme ve geri bağlantılar butonları ve ayarları kaldırılsın. Koddan da silinsin" | A |
| "senkronizasyon özelliği" | L |
| "GitHub'da release … güncelleme otomatik … uyarı … araştırılması" | K |
| "Cloudflare üzerinden güncelleme … veriler orada tutulabilir" | K.3, L |
| "Uygulama font ayarları" | F |
| "Uygulama dili ve editör dilinin bilgisayardaki fontlardan alınması … çekilmeli" | F, açık soru 1 |
| "font size" | D (yazı boyutu) |
| "length size" | D, açık soru 4 |
| "spacing" | D (harf/paragraf aralığı) |
| "autosave" | D (otomatik kayıt) |
| "show white space" | D (boşlukları göster) |
| "> işareti soru işareti (arama simgesi) yerine gelsin; > veya @ olsun" | B (palet işareti) |
| "ayarlar sağdan açılıyor … normal ekran gibi açılmalı" | C (ayarlar ekran oldu) |
| "split view'a geçebiliriz … sol taraftan sürükleyip bırakınca split view" | O |
| "sağ tıklayınca open side panel diye açabilmeliyim" | O (Yan bölmede aç) |
