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

**Bulgular:** (1) ilk kare 10 bin sayfada 3,7 sn: kenar çubuğu ağacı ve sayfa listeleri sanal değil; gerçek GPU'da bu daha düşük
olabilir ama ölçülmedi. (2) 42 KB/sayfa bellek çok: sayfa nesneleri, dizinler (bağlantı, düz metin, görev) ve yazı düzeni önbellekleri.
Yapılacak: ağaç satırlarını sanallaştırmak, düz metin dizinini sayfa açılana kadar bellekte tutmamak. İkisi de ölçüyle başlar (yukarıdaki
test) ama yapılmadı.
