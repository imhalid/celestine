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

| # | Belge | Ne anlatıyor |
|---|---|---|
| 01 | [Genel bakış](01-thymer-genel-bakis.md) | Thymer ekranının parçaları: kenar çubuğu, paneller, durum çubuğu |
| 02 | [Veri modeli](02-veri-modeli.md) | Tek ağaç: çalışma alanı, koleksiyon, kayıt (not), satır, özellik |
| 03 | [Kenar çubuğu ve alt sayfalar](03-kenar-cubugu-ve-alt-sayfalar.md) | "+" ile altına not ekleme, notu koleksiyon gibi kullanma |
| 04 | [Not sayfası](04-not-sayfasi.md) | Bir notun anatomisi, satır tipleri, bloklar |
| 05 | [Not ekleme akışları](05-not-ekleme-akislari.md) | Not oluşturmanın bütün yolları |
| 06 | [Koleksiyonlar ve görünümler](06-koleksiyonlar-ve-gorunumler.md) | Koleksiyon ayarları, özellikler, görünümler, dinamik koleksiyon |
| 07 | [Ayarlar paneli](07-ayarlar-paneli.md) | Ayarlar penceresinin yapısı, tema, MCP, Markdown Mirror |
| 08 | [Komut paleti ve kısayollar](08-komut-paleti-ve-kisayollar.md) | ⌘P, ⌘K, satır komutları |
| 09 | [Arama, görevler, günlük](09-arama-gorevler-gunluk.md) | Sorgu dili, görev durumları, günlük sayfaları |
| 10 | [MCP](10-mcp.md) | Thymer'in MCP araçları ve bizim editörün araç listesi |
| 11 | [Yol haritası](11-yol-haritasi.md) | Bizim editörü kurmanın adımları, sırasıyla |
| 12 | [MyGo ekosistemi](12-mygo-ekosistemi.md) | MyGo için yazılmış kütüphaneler, işimize yarayanlar |
| 13 | [İstekler](13-istekler.md) | Kullanırken çıkan istek ve düzeltmeler, önerilen sırasıyla |
| 14 | [Ürün önerileri ve uyarılar](14-urun-onerileri.md) | Benzer uygulamalardan öneriler, riskler (Markdown'ı bozma, dosya izleme…), sıra |

## Kaynak etiketleri

- **[görüldü]** canlı arayüzde gördüm.
- **[kod]** Thymer'in uygulama kodunda ya da MCP belgesinde var, arayüzde ayrıca bakmadım.
- **[öneri]** Thymer'de böyle değil ya da bilinmiyor; bizim editör için önerim.
