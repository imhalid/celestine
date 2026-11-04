# 12 — MyGo ekosistemi: işimize yarayanlar

Kaynak: [awesome-mygo](https://github.com/agnogad/awesome-mygo) (250 proje) ve MyGo v0.7.4'ün kendi paketleri.
İncelendi: 2026-11-04.

## En önemli bulgu: MyGo'nun kendi `ui/richtext` paketi

MyGo'nun içinde, NSTextView gibi çalışan tam bir zengin metin editörü var (`github.com/egoist/mygo/ui/richtext`,
~8.800 satır, testli, MIT). Bizim `model.go` + `rules.go` + `markdown.go` + `view.go`'nun yaptığının çoğunu, daha olgun
hâliyle yapıyor:

| Özellik | richtext | Bizim editör |
|---|---|---|
| Paragraf stilleri: başlık 1–6, madde/numara/görev listesi, girinti, alıntı, kod bloğu, hizalama | var | hizalama yok, başlık 1–3 |
| Biçimler: kalın, italik, altı çizili, üstü çizili, kod, **bağlantı**, renk, yazı tipi | var | bağlantı, renk yok |
| **Yazarken Markdown kısayolları** (`# `, `- `, `> `, `**x**`…), işaretler görünmez | var (`NoMarkdown` ile kapatılır) | var |
| Markdown'a yazma (CommonMark + GitHub eklentileri), Markdown/HTML/RTF okuma | var | yalnız Markdown |
| Zengin pano (HTML, RTF, Markdown) | var | yalnız Markdown |
| **Ekler**: satır içinde görsel ya da uygulamanın kendi çizdiği bir kutu (`Attachment.Draw`) | var | yok |
| **Tag**: bir aralığa uygulamanın kendi işareti (bahsetme, yorum, sayfa bağlantısı) | var | yok |
| **Kalıcı paragraf kimlikleri** (`ParagraphStyle.ID`, `IdentifyParagraphs`) | var | blok ID'si var ama kalıcı değil |
| Metin yapısı | halat (rope): büyük belgede hızlı | blok dilimi |
| Yalnız görünen paragrafları dizip çizme, geri alma, giriş yöntemleri | var | var (daha basit) |
| `VisibleParagraphs()`: görünen paragrafların kutuları (kenara işaret, tutamaç koymak için) | var | — |
| Caret | yalnız renk (`Theme.Caret`); konumu dışarı verilmiyor | Slime + Mizan |

**Tek eksik caret.** richtext caret'i kendisi, 1px'lik bir çizgi olarak çiziyor; konumunu öğrenmenin bir yolu yok. Slime ve
Mizan için caret'in ekrandaki kutusuna ihtiyacımız var.

Çözüm yolları:
1. **MyGo'ya küçük bir katkı**: `Document` için caret kutusunu veren bir yöntem (ör. `CaretRect()`) ve caret'i gizleme
   seçeneği. Paket MIT ve yazarı aktif.
2. O kabul edilene kadar **MyGo'nun bir çatalı** (fork) ve `go.mod`'da `replace` ile yalnız bu eki kullanmak. richtext,
   MyGo'nun `internal/` paketlerini (`rope`, `text`, `l10n`, `platform`) kullandığı için tek başına kopyalanamıyor;
   ama tüm modülü çatallayıp küçük bir yama yapmak kolay.

## Diğer adaylar

| Proje | Ne | Neden ilgili | Lisans |
|---|---|---|---|
| [sorrycc/Deckle](https://github.com/sorrycc/Deckle) | MyGo ile yerel Markdown editörü | Yaklaşımı farklı (kaynak Markdown tutar, işaretleri seçim yakınında gösterir), ama içinden alınacak çok şey var: Fenwick ağacıyla yükseklik tahmini (100.000 satır akıcı), bölüm bölüm artımlı ayrıştırma, tablo/matematik/Mermaid'i bir resim olarak satıra yerleştirme, klasör izleyen çalışma alanı dizini, `/` blok menüsü, 9 tema, baskı/PDF. Ölçülmüş performans belgesi var: 300 KB'lık notta tuş başına 0,74 ms | MIT |
| [oneo-me/locus](https://github.com/oneo-me/locus) | Yerel, dosya tabanlı Markdown not defteri | **Her not, `index.md` içeren bir klasör; iç içe klasörler not ağacı** — bizim alt sayfalarımızın dosya karşılığı. Dosya izleme, salt okunur MCP (`list/search/resolve_article`, Host/Origin denetimi), agent'lar için "skill" kurma | MIT |
| [auron-labs/mygo-mcp](https://github.com/auron-labs/mygo-mcp) | MyGo görünümlerini bir MCP istemcisine açan geliştirme aracı | Agent (ben dahil) uygulamanın arayüzünü pencere açmadan görebilir, tıklayabilir, yazabilir, ekran görüntüsü alabilir. Geliştirirken test için çok değerli | **lisanssız** — kod alınamaz, yalnız fikir |
| [auron-labs/mygo-framework](https://github.com/auron-labs/mygo-framework) | Yerel öncelikli MyGo uygulama iskeleti | SQLite, migration, ayarlar, gizli bilgiler, **aynı servisin hem arayüzden hem MCP'den kullanılması** — bizim MCP planımızın aynısı | MIT |
| [LZD121/my-gotion](https://github.com/LZD121/my-gotion) | Notion klonu (arayüzü webview + React) | Arayüzü bize uymaz; veri tasarımı uyar: sayfalar ve veritabanları ayrı JSON dosyaları (tek doğruluk kaynağı), SQLite FTS5 yalnız arama indeksi, WebDAV/GitHub eşitleme | MIT |
| [oneo-me/onevra](https://github.com/oneo-me/onevra) | MyGo bileşen kütüphanesi (komut paleti, Markdown, grafik) | Bizde mizan-go var; komut paleti fikir için bakılabilir | MIT |
| [yldm-tech/nodra](https://github.com/yldm-tech/nodra) | Geri al/yinele, kalıcılık destekli durum deposu | Gerekmez: richtext kendi geçmişini tutuyor | MIT |
| `ui/editor` (MyGo içinde) | Kod editörü | `LanguageFor`, 32 dil için hazır sözdizimi renklendirme: kod bloklarımız için doğrudan kullanılabilir | MIT |
| Bileşen kitleri (CrychicUI, uix, mycn, mayu…) | Görünüm kütüphaneleri | Gerekmez: mizan-go var | — |

## Karar (2026-11-04)

Çekirdek richtext oldu. Caret için MyGo'ya katkı ya da çatal yok: caret'in yeri, paragrafı richtext'in yaptığı gibi
yeniden dizerek bulunuyor (`caret.go`) ve `caret_test.go` bunu her karakterde richtext'in kendi caret'iyle
karşılaştırıyor. richtext v0.7.4'te bulunan bir hata (paragraf sonunda kapanan kalın, alttaki paragrafa `[] `
dönüşümünden sonra sızıyor) uygulamada önleniyor (`main.go`, `track`).

## Öneri

1. **Editör çekirdeğini MyGo'nun `richtext`'ine taşı.** Bizim yazdığımız çekirdek doğru tasarlanmış ama richtext aynı
   şeyi daha olgun yapıyor (bağlantılar, ekler, zengin pano, halat yapısı, testler) ve MyGo'yla birlikte bakımı sürüyor.
   Bizim ekleyeceğimiz değer üstteki katmanlarda: koleksiyonlar, alt sayfalar, kenar çubuğu, MCP, caret'ler.
2. **Caret için** MyGo'ya küçük bir katkı yap; beklerken çatal + `replace`.
3. **Depolama için Locus'un düzenini al**: her sayfa bir klasör ve içinde `index.md`, alt sayfalar alt klasörler, ekler
   yanında. Koleksiyon = en üstteki klasör + bir `collection.json` (şema, görünümler). Arama indeksi yalnız bir türev
   (my-gotion'daki gibi).
4. **Deckle'dan** performans tekniklerini ve `/` menüsünü, **ui/editor'dan** kod renklendirmeyi al.
5. **Geliştirme için mygo-mcp fikrini** kendi testlerimizde kullan (lisansı yok, kodunu almadan).
