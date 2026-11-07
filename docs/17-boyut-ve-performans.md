# 17 — Boyut ve performans (I, N6)

7 Kasım 2026, bu makinede ölçüldü (Linux amd64, Go 1.27). Her sayı bir komutla yeniden üretilir.

## Boyut

| Derleme | Boyut |
|---|---|
| `go build` (sembollü) | 33,2 MB |
| `-ldflags="-s -w"` | 22,8 MB |
| `-s -w -trimpath -buildvcs=false` | 22,7 MB |
| beş hedef, `scripts/build.sh` | Linux amd64 22,2 MB, Windows amd64 22,7 MB |
| gzip -9 / xz -9 ile indirilen | 9,6 MB / 7,5 MB |

- **Uygulanan:** `scripts/build.sh`: beş hedef, `-s -w -trimpath -buildvcs=false`, sürüm gömülü, her hedefin boyutu bir öncekiyle
  karşılaştırılıp yazılır, bütçe (`BUDGET_MB`, öntanımlı 25) aşılırsa çıkış kodu 1. Release iş akışı aynı bayrakları kullanır.
- **Kazanç:** sembolleri atmak üçte bir (-10 MB). `-trimpath` ve `-buildvcs=false` ancak 90 KB: ücretsiz ama küçük.
- **Ne yer tutuyor** (kod, `go tool nm`): SQLite (`modernc.org/sqlite/lib`) ≈2 MB, uygulamanın kendisi ≈1,5 MB, MyGo arayüzü ≈0,8 MB,
  `crypto/tls` + `net/http` ≈0,5 MB (MCP sunucusu, güncelleme ve senkron istemcisi için gerekli), MCP SDK ≈0,25 MB. Görüntü
  çözücüler (`x/image`: webp, vp8) MyGo'dan gelir. Çalıştırılabilir dosyanın çoğu Go'nun tür ve işlev tabloları.
- **Ek bayrak ölçümü** (Linux amd64, `-s -w -trimpath` üstüne): `-gcflags=all=-l` (satır içi açmayı kapat) 23,5 → **21,9 MB** (−1,65 MB, %7), ama
  yerleşim ve çizim döngülerini yavaşlatır; `SMALL=1 scripts/build.sh` ile seçilir, öntanımlı değil. `GOAMD64=v3` −45 KB (yalnız yeni işlemcilerde çalışır:
  taşınabilirliği bozar), `-tags netgo,osusergo` fark etmedi.
- **Yapılmadı ve nedeni:** UPX (macOS imza ve notarization'ı bozar, Windows virüs yazılımları sever değil); MCP SDK yerine elle JSON-RPC
  (kazanç ≈0,25 MB, özellik riski: 9 aracın şeması SDK'dan geliyor); gömülü varlıklar yalnız 512 px'lik bir simge, sıkıştırılacak bir şey yok.
- **Dağıtım:** indirilen boyut 7,5–9,6 MB (xz/gzip); güncelleme paketi (K) zaten sıkıştırılmış indirilebilir. Fark güncellemesi (bsdiff)
  yapılmadı: tek ikili dosya için kazanç ≈%80 olurdu, ama imza ve geri dönüş yolunu karmaşıklaştırır.

## Boyut araştırması (7 Kasım, akşam)

Asıl ağır olan web ve Android'in indirdiği **wasm**; masaüstü ikilisi zaten 24 MB ve sıkıştırılmış 7,5–9,6 MB iniyor. Ölçümler bu makinede,
`go build -trimpath -ldflags="-s -w"`, brotli `-q 11` (Cloudflare'in sunduğu), gzip `-9`.

| wasm | ham | gzip | brotli |
|---|---|---|---|
| önce (MCP içinde) | 34,3 MB | 8,2 MB | 5,9 MB |
| **MCP web derlemesinden çıkınca (uygulandı)** | **29,6 MB** | **7,3 MB** | **5,3 MB** |
| üstüne satır içi açma kapalı (`-gcflags=all=-l`) | 29,8 MB | 6,3 MB | 4,6 MB |

- **Uygulanan: MCP sunucusu web'de yok.** Tarayıcı bir port dinleyemez, ama MCP SDK'sı, şemaları (jsonschema), JSON'u (segmentio), `encoding/gob` ve
  sunucu kodu wasm'a giriyordu. Sunucu `mcp_server.go`'ya (`//go:build !js`) taşındı, web'de `mcp_js.go` yalnız açılamadığını söyler. wasm −4,6 MB
  (%14), brotli −0,6 MB; `mygo build` çıktısı 33,0 → 28,5 MB, APK 35,8 → 31,0 MB. Masaüstünde hiçbir şey değişmedi (bütün testler geçiyor).
- **Önerilmeyen: satır içi açmayı kapatmak.** Brotli'de −0,7 MB daha kazandırır ama hız bedeli büyük (1 500 sayfa, `-gcflags=all=-l`): arama 5,3 →
  9,1 ms (+%72), palet 6,2 → 10,8 ms (+%75), bir tuş vuruşu karesi 38 → 51 ms (+%33). Web ve telefon zaten masaüstünden yavaş.
- **APK'da wasm'ı sıkıştırmak:** `noCompress += "wasm"` kaldırılınca APK **11,4 MB** (31 MB yerine). Bedeli açılışta wasm'ın her seferinde açılması
  (≈30 MB inflate); telefonun ekranı kilitli olduğundan açılış süresi ölçülemedi. Play Store indirmeyi zaten sıkıştırır; doğrudan dağıtımda (APK
  dosyası) anlamlı. Ölçülmeden açılmadı.
- **`wasm-opt -Oz` (binaryen):** Go wasm'ında genellikle %10–20; bu makinede kurulu değil, denenmedi. Bir sonraki adım: `binaryen` paketini kurup
  `scripts/web.sh`'e isteğe bağlı eklemek, sonra hızı yeniden ölçmek.
- **Masaüstünde ne yer tutuyor** (sembollü ikilide kod ve veri, toplam 13,9 MB; kalanı Go'nun işlev ve tür tabloları): SQLite 2,1 MB (depo: şart),
  uygulama 1,7 MB, MyGo arayüzü 0,85 MB, Ryugo 0,4 MB, MCP SDK 0,33 MB (+ jsonschema 0,1, segmentio JSON 0,12, gob 0,12: ajan bağlantısı),
  `crypto/tls` + `net/http` + http2 0,7 MB (senkron, güncelleme, MCP), YAML 0,14 MB (dışarıdan gelen Markdown'ın ön maddesi), `x/net/html` 0,08 MB
  (HTML yapıştırma). Hepsinin bir işi var; çıkarılacak ölü bağımlılık bulunmadı.

## Büyük kitaplık (N6)

`perf_test.go`: sentetik kitaplık (5 koleksiyon, her sayfada metin, bağlantı, etiket, görev). `PERF_PAGES=10000 go test -run TestPerfBudget -v`.

| 10 000 sayfa | süre / bellek |
|---|---|
| kitaplığı aç | 0,52 sn |
| ilk kare (başsız, yazılım çizici) | 3,7 sn |
| metinde ara | 0,48 sn |
| bağlantı dizini | 3 ms (önbellekte) |
| palet sorgusu | 22 ms |
| kitaplık rakamları (istatistik sayfası) | 0,54 sn |
| bellek | +421 MB (sayfa başına ≈42 KB) |

Varsayılan test 1 500 sayfayla çalışır (1,3 sn) ve bütçeyi (açılış 1,5 sn, ilk kare 3 sn, arama 2 sn, palet 0,6 sn, bellek 200 MB / 1000 sayfa)
bin sayfa başına ölçekler: iki-üç kat yavaşlayan bir regresyonu yakalar, yavaş makineyi yakalamaz.

**7 Kasım, öğlen: giderildi.** Aynı test, aynı makine:

| 10 000 sayfa | önce | sonra |
|---|---|---|
| ilk kare | 4,1 sn | **0,53 sn** |
| bellek | +424 MB | **+15 MB** |
| bir tuş ve karesi (başsız, yazılım çizici) | ölçülmüyordu; ağaç her karede 2 000+ satır kuruyordu (kare ≈1,5 sn) | **≈45 ms**, 1 500 sayfadakiyle aynı (≈40 ms) |

Ne yapıldı: (1) kenar çubuğu ağacı **sanal**: satırlar tek yükseklikte olduğundan bir satırın yeri listedeki sırasıdır; görünenin iki ekran üstü ve altı
dışındaki satırlar kurulmaz, her ardışık öbek için onların yüksekliğinde bir boşluk kurulur. Satır listesi (`a.rows`: oklar ve sürükleme için) yine
eksiksiz. 200 satırdan kısa ağaç, sürükleme, yeniden adlandırılan ya da odak verilecek satır olduğu gibi kurulur. (2) Geri bağlantı dizini ilk açılışta
bütün dosyaları ilk karede okuyordu: artık kare başına 6 ms okur, kaldığı yerden sürer, bitince dizini bir kerede kurar. (3) Metindeki her
`[[bağlantı]]` çipi her karede bütün sayfalarda başlık arıyordu: başlıklar bir önbellekte, sayfa eklenince/adlanınca/taşınınca/silinince
(`pagesGen`) yeniden kurulur. (4) Ağacın satır sayısı ve açık dalların imzası kare başına tek yürüyüşte sayılır. Testte yeni ölçü: bir tuş vuruşu
(bütçe 60 ms, kitaplıkla ölçeklenmez) ve `BenchmarkKeyBigLibrary`. Gerçek GPU'da ölçülmedi.

**Önceki bulgular:** (1) ilk kare 10 bin sayfada 3,7 sn: kenar çubuğu ağacı ve sayfa listeleri sanal değil; gerçek GPU'da bu daha düşük
olabilir ama ölçülmedi. (2) 42 KB/sayfa bellek çok: sayfa nesneleri, dizinler (bağlantı, düz metin, görev) ve yazı düzeni önbellekleri.
Yapılacak: ağaç satırlarını sanallaştırmak, düz metin dizinini sayfa açılana kadar bellekte tutmamak. İkisi de ölçüyle başlar (yukarıdaki
test) ama yapılmadı.
