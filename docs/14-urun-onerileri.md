# 14 — Ürün önerileri ve uyarılar (4 Kasım 2026)

[13](13-istekler.md) senin istediklerin. Bu sayfa benim önerdiklerim: Thymer, Tana, Capacities, Obsidian (Bases),
Notion, Bear, Craft gibi uygulamalarda işe yarayan, bizde eksik olan şeyler; ve **önce bilmen gereken riskler**.
Her önerinin yanında: neden, nereden, çaba (**K** küçük, **O** orta, **B** büyük) ve ne zaman.

---

## Önce uyarılar

> **Durum (5 Kasım 2026):** U1, U2 ve U3 yapıldı — `markdown.go` (ham bloklar, kod dili, gereksiz kaçışlar),
> `frontmatter.go` (YAML, yerinde değişiklik), `watch.go` (1,5 sn'de bir diskle karşılaştırma, çakışmada iki sürüm).
> Ayrıca 5 (⌘K), 6 (⌘F), 25 (geri/ileri) ve 28 (menü çubuğu) yapıldı.
> Kalan bilinen normalleştirmeler: tek satır sonları birleşir (`a\nb` → `a b`), `__kalın__` → `**kalın**`,
> `1)` → `1.`, bağlantı başlığı (`"title"`) düşer, alıntıdaki iki paragraf iki alıntı olur. Okunuşları aynı.

### U1 — Başka araçların Markdown'ını bozuyoruz (ciddi, veri kaybı)
"Kitaplık düz Markdown, her araç okur" diyoruz. Okuma doğru, ama **bir sayfayı ryudit'te düzenleyip kaydedince**
richtext'in tanımadığı her şey bozuluyor. Denedim (`parseBody` → `bodyMarkdown`):

| Girdi | Kaydedilince |
|---|---|
| `Merhaba!` | `Merhaba\!` |
| tablo `\| a \| b \|` | `**a⇥b**` ve `1⇥2`: **tablo yok oluyor** |
| `[[Sayfa bağlantısı]]` (Obsidian) | `\[\[Sayfa bağlantısı\]\]` |
| dipnot `[^1]` | `\[1\]`: dipnot bozuluyor |
| `<div>html</div>` | `html`: HTML gidiyor |
| satır sonu (iki boşluk) | iki ayrı paragraf |

Bir Obsidian kasası (vault) ya da GitHub README'si ryudit'le açılıp bir harf değiştirilirse dosya kalıcı olarak
değişir. **Öneri:** (1) tanımadığımız blokları (tablo, HTML, dipnot) **olduğu gibi koru**: "ham blok" olarak göster,
düzenleme, kaydederken aynen yaz; (2) `[[bağlantı]]`yı kendi sözdizimimiz yap (ileride zaten lazım); (3) gereksiz
kaçışları (`\!`) yazma; (4) bir sayfayı ilk kez kaydederken bir şey kaybolacaksa uyar. Bunu bir **gidip-gelme test
korpusuyla** (Obsidian, GitHub, Bear dosyaları) kilitle. *(K–O, **en önce**)*

### U2 — Dışarıdan değişiklikler görünmüyor, üstüne yazılabiliyor
Klasör izlenmiyor. Başka bir editör, bir betik, iCloud/Dropbox eşitlemesi ya da dosyaya doğrudan yazan bir ajan
açık sayfayı değiştirirse, ryudit kaydedince **onların değişikliği gider**. Yol haritasında Aşama C; bence U1 ile
birlikte en üstte. Çakışmada iki sürümü de koru (`index (çakışma 2026-11-04).md`). *(O)*

### U3 — Ön madde (front matter) ayrıştırıcısı çok basit
Yalnız `anahtar: değer` satırlarını okuyor. YAML listesi (`tags: [a, b]` ya da `- a` satırları), çok satırlı değer,
iç içe alan **okunamıyor ve kaydederken bozuluyor**. Etiket özelliği ve Obsidian uyumu için gerçek bir YAML
kitaplığı (ör. `goccy/go-yaml`) şart. *(K)*

### U4 — Blok editör hedefi richtext'in sınırına çarpacak
richtext paragraf düzeyinde iyi; ama **içi başka bloklar olan bloklar** (içinde liste olan bilgi kutusu, açılır/kapanır
liste, sütunlar), **tablolar**, **satır içi tarih/kişi çipleri** ve **bloklara kalıcı kimlik** (blok bağlantısı,
transklüzyon) onun modelinde yok. Tutamak, sürükle-bırak, katlama, `/` menüsü üstüne kurulabilir; kutular ve sütunlar
kurulamaz. **Öneri:** Aşama B'ye başlamadan 2–3 günlük bir **deneme (spike)**: kutuyu ve katlamayı richtext üstünde
dene; olmazsa karar ver: richtext'i paragraf çizici olarak tutup blok modelini biz mi yazıyoruz, yoksa kutu/sütundan
vaz mı geçiyoruz. Bu karar ertelenirse sonra pahalıya patlar. *(karar)*

### U5 — Blok kimliği Markdown'ı kirletir
Blok bağlantısı ve transklüzyon (Thymer, Tana, Logseq) her bloğa bir kimlik ister. Markdown'da yeri yok; Obsidian
`^abc123` ekler, Logseq `id::` satırı. Dosyalar başka araçta çirkin görünür. **Öneri:** sayfa bağlantısı (`[[ ]]`)
evet; blok bağlantısını şimdilik **yapma**, ya da yalnız başlıklara bağlan (`[[Sayfa#Başlık]]`). *(karar)*

### U6 — Ajanın yaptığı geri alınamıyor
MCP ile bir ajan bir sayfanın metnini değiştirebiliyor; ⌘Z o değişikliği geri almıyor, sürüm geçmişi yok. Aşama 5'in
"bitti sayılır" ölçütünde "⌘Z ile geri alabiliyorum" var, karşılanmadı. **Öneri:** her ajan yazmasından önce
anlık görüntü (aşağıda Sürüm geçmişi) + durum çubuğunda "Ajan 3 sayfayı değiştirdi — geri al". Koleksiyon bazında
izin (yalnız okuma / yazma) da eklenmeli. *(O)*

### U7 — Büyük kitaplıkta yavaşlayacak
Arama (MCP `search` dahil) her çağrıda **bütün dosyaları diskten okuyor**; yüzlerce sayfada sorun yok, 10.000'de olur.
Açılışta bütün ön maddeler okunuyor. **Öneri:** bellekte bir dizin (başlık, özellikler, düz metin), değişince
güncellenen; dosya izlemeyle (U2) aynı altyapı. 10.000 sayfalık bir deneme kitaplığıyla ölç. *(O)*

### U8 — Arayüz metinleri kodun içine dağılmış
Bütün yazılar Türkçe ve fonksiyonların içinde. İngilizce (ya da başka dil) sonra eklenirse yüzlerce yeri gezmek
gerekir. **Öneri:** şimdiden tek bir yerde topla (`tr.go`), sonra dil eklemek bir dosya olur. *(K, ne kadar erken o kadar ucuz)*

### U9 — Kapsam: Thymer'in hepsi bir projeyi boğar
Thymer, Notion, Tana büyük ekiplerin yıllarıdır. Hepsini istersek hiçbiri iyi olmaz. **Kimliğimizi** seçelim:
*"yerel, düz Markdown, çok hızlı, yazmaktan keyif alınan, koleksiyonlu ve ajanlarla konuşan bir not defteri."*
Buna hizmet etmeyen şeye "sonra" ya da "hayır" diyelim (aşağıda "Yapmayalım").

### U10 — Bağımlılıklar
`ryu-go` `main` dalından sözde bir sürüme (`v0.1.1-0.2026…`) bağlı; MyGo 1.0 öncesi (v0.7.4), API değişebilir.
**Öneri:** ryu-go'ya etiket (`v0.2.0`) at ve ona bağlan; MyGo güncellemelerini tek tek, testlerle al. *(K)*

---

## Öneriler

### Yakalama — düşünceyi kaybetmeden yazmak
1. **Hızlı yakalama penceresi.** Her yerden bir kısayol (⌥Space) küçük bir pencere açar; yazılan "Gelen kutusu"na
   ya da seçilen koleksiyona gider. Bear, Things, Capacities'te en çok kullanılan özellik. *(O, yakında)*
2. **Günlük (Journal).** Her gün kendiliğinden bir sayfa; ⌘D bugüne gider, takvimden geçmiş günler. Capacities ve
   Thymer'in merkezi. Görevlere tarih verince o günün sayfasında da görünür. *(O, yakında)*
3. **Şablonlar.** Koleksiyonun "yeni sayfa" şablonu (ör. Toplantı: Katılanlar, Kararlar, Yapılacaklar başlıkları,
   özellik varsayılanları). Koleksiyon ayarlarına bir sekme. *(K, A ile)*
4. **Bağlantı yapıştırınca kart.** URL yapıştırınca başlığı ve sitesi çekilir (isteğe bağlı, ağ gerekir; ayarla
   kapatılabilir). *(O, sonra)*

### Bağlamak ve bulmak
5. **⌘K hızlı geçiş + komut paleti.** Bulanık arama ile sayfaya atla, komut çalıştır, son açılanlar. Bütün
   uygulamaların bir numaralı kısayolu; bizde yok. *(O, **hemen sonra**)*
6. **Sayfada bul ve değiştir (⌘F).** Şu an sayfa içinde arama yok; her editörde var, eksikliği hemen fark edilir. *(K–O, yakında)*
7. **Sayfa bağlantısı `[[ ]]` + geri bağlantılar.** `[[` yazınca sayfa önerileri; sayfanın altında "Bu sayfaya
   bağlananlar"; ad değişince bağlantılar güncellenir; bağlanmamış anmalar ("bu sayfanın adı geçen yerler").
   Obsidian/Logseq/Thymer'in kalbi. U1 ile birlikte. *(O, A'dan sonra)*
8. **Canlı sorgu (dinamik koleksiyon ve sorgu bloğu).** Tana'nın "live search"ü gibi: koşullardan oluşan bir sorgu,
   bütün koleksiyonlarda eşleşenleri canlı listeler; görünümleri var. Hem koleksiyon olarak ([13](13-istekler.md) 12)
   hem sayfa içinde blok olarak. Tana'da sonuçlar yerden bağımsız, etiket ve alanla toplanıyor: bizde özellikler
   koleksiyonlar arası paylaşılırsa ([13](13-istekler.md) 8) aynısı olur. *(B, arama dizini ile)*
9. **Etiketler.** `#etiket` satır içinde, ve "Etiket" özelliği (çoklu seçim). Kenar çubuğunda etiket listesi. *(O)*

### Görevler
10. **Görevler bir arada.** "Bugün / Gecikmiş / Yaklaşan": bütün sayfalardaki açık görevler, tarihlerine göre.
    Thymer ve Tana'da en çok kullanılan görünümlerden. Görev durumları (başladı, beklemede, önemli). *(O)*
11. **Satır içi tarih.** `@yarın`, `@cuma 14:00` yazınca tarih çipi; göreve tarih verir; günlükte görünür. *(O, B ile)*

### Görünümler
12. **Takvim görünümü.** Tarih özelliği olan koleksiyonlar takvimde; sürükleyerek tarih değiştirme. *(O)*
13. **Galeri görünümü.** Kartlar banner ya da sayfanın ilk görseliyle (kitaplar, tarifler, projeler için). *(O, banner ile)*
14. **Filtreler ve özet satırı.** Her görünümde kalıcı filtre ("Durum ≠ Bitti"), tabloda alt satırda sayı/toplam/ortalama. *(O)*
15. **Gruplama her görünümde.** Listede ve tabloda herhangi bir seçim özelliğine göre gruplar, katlanabilir. *(K)*

### Yazmak
16. **Odak modu ve daktilo kaydırması.** Kenar çubuğu ve her şey kaybolur, caret hep ekranın ortasında kalır;
    etkin paragraf dışındakiler soluklaşır. Mizan caret'inin yazma hızıyla çok iyi gider — bizim farkımız. *(K, yakında)*
17. **Yazma hedefi ve istatistik.** Sayfa ya da gün için kelime hedefi; durum çubuğunda ilerleme; günlük yazma
    serisi. Yazana odaklanan kimliğimize uyar. *(K)*
18. **Başlık ana hatları (outline).** Sağda ya da açılır bir panelde sayfanın başlıkları; tıklayınca oraya kayar;
    uzun sayfada şart. *(K)*
19. **Kod blokları için dil ve renklendirme.** ```` ```go ```` ile dil, renkli gösterim, kopyala düğmesi. *(O)*
20. **Görsel ve dosya.** Sürükle-bırak ya da yapıştır: sayfanın klasörüne kopyalanır, Markdown `![]()` olarak
    yazılır; boyutlandırma. Banner'ın da altyapısı. *(O)*
21. **Yazım denetimi.** macOS'un denetimi (kırmızı alt çizgi, öneriler); MyGo'da var mı bakılmalı. *(K–O)*

### Güvenlik ve geçmiş
22. **Sürüm geçmişi.** Her kaydedişte (en çok dakikada bir) anlık görüntü `.ryudit/gecmis/` altına; sayfa menüsünde
    "Sürüm geçmişi…": tarih listesi, fark, geri yükle. Ajan değişikliklerini geri almanın da yolu (U6). İstenirse
    arka planda git. *(O, U6 ile)*
23. **Çöpün kendiliğinden boşalması.** 30 gün sonra (ayarlanabilir). *(K)*
24. **Yedek.** Ayarlarda "kitaplığı şuraya günlük yedekle" (zip). *(K)*

### Gezinme ve pencere
25. **Geri / ileri (⌘[ / ⌘]) ve son açılanlar.** Tarayıcı gibi; şu an yok. *(K, hemen)*
26. **Sabitlenenler (Favoriler).** Kenar çubuğunun üstünde sık kullanılan sayfalar. *(K)*
27. **Yan panel ve sekmeler.** [13](13-istekler.md) 17; sonra ayrı pencerede açma. *(O)*
28. **Menü çubuğu.** Dosya / Düzen / Görünüm / Git menüleri, kısayollarıyla; macOS kullanıcısının ilk baktığı yer.
    Şu an yalnız uygulama ve düzen menüsü var. *(K)*

### Ajanlar (MCP)
29. **Ajan etkinliği görünür olsun.** Ajanın değiştirdiği satırlar kısa bir süre parlar; durum çubuğunda etkinlik;
    "Ajan geçmişi" listesi (ne zaman, hangi sayfa, ne yaptı, geri al). *(O)*
30. **Koleksiyon izinleri.** Her koleksiyon için: kapalı / yalnız okuma / yazma. *(K)*
31. **Daha akıllı araçlar.** `insert_markdown` (bir başlığın altına, bir görevin yanına ekle), `query` (özelliklere
    göre bul), `list_tasks`, `get_daily_note`; MCP "resources" (sayfalar okunabilir kaynak olarak) ve "prompts"
    (ör. "bu haftayı özetle"). *(O)*
32. **Uygulama kapalıyken de MCP.** `ryudit mcp` komutu (stdio): pencere açık değilken ajanlar yine yazabilsin;
    açılınca uygulama dışarıdan değişikliği görür (U2 gerekli). *(K, U2'den sonra)*

### Dışarıyla
33. **İçe aktar.** Obsidian kasası, Notion dışa aktarması, Bear, Apple Notlar, Thymer. Geçiş kolaylığı yeni kullanıcı
    getirir. U1 ve U3 çözülmeden yapma. *(O)*
34. **Dışa aktar.** Sayfa/koleksiyon: HTML, PDF (yazdır), tek Markdown dosyası. *(O)*

### Cila
35. **İlk açılış.** Boş kitaplıkta "Hoş geldin" yerine kısa bir tur: koleksiyon oluştur, özellik ekle, `/` dene;
    örnek bir koleksiyon (Okuma listesi) isteğe bağlı. *(K)*
36. **Boş durumlar.** Her boş ekranda ne yapılacağını söyleyen bir cümle ve bir düğme (boş koleksiyon, boş çöp,
    aramada sonuç yok). Kısmen var. *(K)*
37. **Uygulama simgesi, `.app` paketi, kendiliğinden güncelleme.** Günlük kullanılacaksa şart. *(O)*
38. **Erişilebilirlik.** VoiceOver etiketleri (testlerde zaten kullandığımız `Label`'lar iyi bir başlangıç), klavyeyle
    her yere ulaşım, büyük yazı desteği. *(K–O)*

---

## Yapmayalım (en azından şimdi)
- **Gerçek zamanlı ortak çalışma ve eşitleme sunucusu.** Yerel, düz dosya kimliğiyle çelişir; iCloud/Dropbox/git
  ile eşitleme yeter (U2 çözülünce güvenli olur).
- **Uygulamanın içine yapay zekâ.** MCP bunu dışarıdaki ajanlara bırakıyor; içeride bir AI özelliği hem bakım hem
  gizlilik yükü. (Karar: eklenti yok, ajan dışarıda.)
- **Grafik görünümü.** Güzel görünür, az kullanılır.
- **Blok bağlantısı ve transklüzyon** (U5) — en azından başlık bağlantısıyla yetinene kadar.
- **Sütunlar** — U4 kararı verilene kadar.

---

## Önerdiğim sıra

1. **Güvenlik temeli:** U1 (Markdown'ı koruma) + U3 (YAML) + U2 (dosya izleme). Bunlar olmadan kimseye
   "dosyalarını aç" diyemeyiz.
2. **[13](13-istekler.md)'teki küçük düzeltmeler** (caret, arka plan, yollar, kenar çubuğu düğmesi, MCP göstergesi)
   + 25 (geri/ileri) + 28 (menü çubuğu).
3. **Gezinme:** 5 (⌘K) + 6 (⌘F).
4. **Koleksiyon ayarları** ([13](13-istekler.md) 10) + şablonlar (3) + görünüm ayarları ve filtreler (14, 15).
5. **U4 denemesi**, sonra Aşama B (bloklar) + `/` ve `@` menüleri.
6. **Bağlantılar ve sorgu:** 7, 9, 8; günlük (2) ve görevler (10, 11).
7. **Geçmiş ve ajan güveni:** 22 + U6 + 29, 30.
8. **Yazmak:** 16, 17, 18; sonra görsel (20), takvim ve galeri (12, 13).

## Kaynaklar
- [Tana — Live search](https://help.tana.inc/live-search.html), [Tana Learn: live search and views](https://tana.inc/articles/tana-learn-live-live-search-and-views)
- [Capacities — günlük ve takip](https://capacities.io/use-cases/daily-tracking), [Capacities incelemesi 2026](https://blog.saner.ai/capacities-review)
- [Obsidian Bases özeti](https://skillselion.com/skills/agricidaniel/claude-obsidian/obsidian-bases) (ikincil kaynak; resmî: help.obsidian.md/bases)
- Thymer: [01](01-thymer-genel-bakis.md)–[10](10-mcp.md)'daki kendi incelememiz (almasada.thymer.com).
