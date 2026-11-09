# Editör dokümanları

Bu klasör iki şeyi bir araya getirir:

1. **Thymer'de ne var, nasıl çalışıyor** — almasada.thymer.com'da gezilerek, Thymer MCP'sinin veri modeli belgesinden ve
   daha önce yapılmış teknik analizden (`~/thymer-research/THYMER-TEKNIK-ANALIZ.md`) çıkarıldı.
2. **Bizim editörü adım adım nasıl kurarız** — MyGo üzerinde, koleksiyon bazlı, WYSIWYG bir Markdown editörü.

Alınmış kararlar (her belgede geçerlidir):

- Eklenti sistemi **yok**, dışarıdan kod (JS ya da script) **yok**. Dışarıdan değiştirilebilen tek şey **tema**dır
  (bir veri dosyası). Özellikleri biz ekleriz.
- Caret'ler uygulamanın içinde, Go ile yazılır: **Slime** ve **Mizan** hazır (`prototypes/slime-md` demosunda). Tema yalnızca hangisinin
  açık olduğunu ve ayarlarını seçer.
- **MCP** olacak: kullanıcı ayarlardan açar, bir agent dosya ve koleksiyon oluşturabilir. Bu bir eklenti değil, uygulamanın
  kendi işlemlerinin dışarıya açılmasıdır.
- Şok dalgası efekti şimdilik **kapalı** (kodu duruyor).

## Okuma sırası

Belgelerde yalnız henüz yapılmamış işler, hâlâ geçerli kararlar ve açık sorunlar durur; yapılanlar koddadır.
Kodun bugünkü hâli için [architecture.md](architecture.md).

| # | Belge | Ne anlatıyor |
|---|---|---|
| 02 | [Veri modeli](02-veri-modeli.md) | Henüz olmayan satır ve özellik tipleri |
| 03 | [Kenar çubuğu ve alt sayfalar](03-kenar-cubugu-ve-alt-sayfalar.md) | Kenar çubuğunda eksik gösterim biçimleri, sayfa ikonu ve sabitleme |
| 04 | [Not sayfası](04-not-sayfasi.md) | Eksik satır tipleri ve satır işlemleri, yapıştırma/kopyalama biçimleri |
| 05 | [Not ekleme akışları](05-not-ekleme-akislari.md) | ⇧↵, "Yeni sayfa…" seçicisi, varsayılan koleksiyon |
| 06 | [Koleksiyonlar ve görünümler](06-koleksiyonlar-ve-gorunumler.md) | Galeri görünümü, alana göre gruplama, elle sıralama |
| 07 | [Ayarlar paneli](07-ayarlar-paneli.md) | Veri dosyası olarak temalar, tarih ve saat biçimleri |
| 09 | [Arama, görevler, günlük](09-arama-gorevler-gunluk.md) | Sorgu dili, görev filtreleri, canlı sorgu bloğu |
| 10 | [MCP](10-mcp.md) | Thymer'in MCP araçları; bizde eksik olanlar |
| 12 | [MyGo ekosistemi](12-mygo-ekosistemi.md) | İşimize yarayabilecek MyGo kütüphaneleri |
| 13 | [İstekler](13-istekler.md) | Kullanırken çıkan, henüz yapılmamış istekler |
| 14 | [Ürün önerileri ve uyarılar](14-urun-onerileri.md) | Açık kalan öneriler ve riskler |
| 15 | [Bloklar denemesi](15-bloklar-denemesi.md) | Bloklarla ilgili karar ve olmayacaklar |
| 16 | [Yapılacaklar](16-yapilacaklar.md) | Editör, yazı tipi, klavye, dil ve performansta kalan işler |
| 17 | [Boyut ve performans](17-boyut-ve-performans.md) | Uygulamanın boyutu, ölçümler, tuş süresi bütçesi |
| 18 | [Web](18-web.md) | Web sürümü ve denenmemiş kalanlar |
| 19 | [Güncelleme](19-guncelleme.md) | Otomatik güncelleme, yayın imzası ve açık işler |
| 20 | [Senkron](20-senkron.md) | Uçtan uca şifreli eşitleme ve eksikleri |
| 21 | [Android](21-android.md) | Android uygulaması ve telefonda eksikler |
| 22 | [Acil sorunlar](22-acil-sorunlar.md) | Kalan birkaç açık uç |
| 22 | [Hesap ve bekleme listesi](22-hesap-ve-bekleme-listesi.md) | Hesap, davet ve bekleme listesi: kararlar ve açıklar |
| 23 | [Denetim aksiyonları](23-denetim-aksiyonlari.md) | Denetimden kalan açık maddeler |

## Kaynak etiketleri

- **[görüldü]** canlı arayüzde gördüm.
- **[kod]** Thymer'in uygulama kodunda ya da MCP belgesinde var, arayüzde ayrıca bakmadım.
- **[öneri]** Thymer'de böyle değil ya da bilinmiyor; bizim editör için önerim.
