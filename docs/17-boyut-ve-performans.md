# 17 — Boyut ve performans (I, N6)

7 Kasım 2026, bu makinede ölçüldü (Linux amd64, Go 1.27). Her sayı bir komutla yeniden üretilir.

## Boyut

| Derleme | Boyut |
|---|---|
| `go build` (sembollü) | 33,2 MB |
| `-s -w -trimpath -buildvcs=false` (`scripts/build.sh`) | Linux amd64 22,2 MB, Windows amd64 22,7 MB |
| gzip -9 / xz -9 ile indirilen | 9,6 MB / 7,5 MB |

- **Ne yer tutuyor** (sembollü ikilide kod ve veri 13,9 MB; kalanı Go'nun işlev ve tür tabloları): SQLite 2,1 MB (depo: şart), uygulama 1,7 MB,
  MyGo arayüzü 0,85 MB, Ryugo 0,4 MB, MCP SDK 0,33 MB (+ jsonschema, segmentio JSON, gob ≈0,34 MB: ajan bağlantısı), `crypto/tls` + `net/http` +
  http2 0,7 MB (senkron, güncelleme, MCP), YAML 0,14 MB, `x/net/html` 0,08 MB (HTML yapıştırma). Çıkarılacak ölü bağımlılık bulunmadı.
- **Satır içi açmayı kapatmak** (`-gcflags=all=-l`, `SMALL=1 scripts/build.sh`): −1,65 MB (%7), ama hız bedeli büyük (1 500 sayfa: arama +%72,
  palet +%75, tuş vuruşu karesi +%33). Öntanımlı değil, web ve telefon için de önerilmez. `GOAMD64=v3` −45 KB ama taşınabilirliği bozar.
- **Yapılmadı ve nedeni:** UPX (macOS imza ve notarization'ı bozar, Windows virüs yazılımları sever değil); MCP SDK yerine elle JSON-RPC
  (kazanç ≈0,25 MB, araçların şeması SDK'dan geliyor); fark güncellemesi (bsdiff: tek ikilide kazanç ≈%80 olurdu, ama imza ve geri dönüş yolunu
  karmaşıklaştırır).

### wasm (web ve Android)

Asıl ağır olan wasm. `go build -trimpath -ldflags="-s -w"`, MCP sunucusu web derlemesinde yok (`mcp_server.go` `!js`): 29,6 MB ham, 7,3 MB gzip,
5,3 MB brotli (Cloudflare'in sunduğu).

- **Açık: APK'da wasm'ı sıkıştırmak.** `android/app/build.gradle.kts`'teki `noCompress += "wasm"` kaldırılınca APK 31 MB yerine **11,4 MB**. Bedeli her
  açılışta ≈30 MB inflate; açılış süresi ölçülemedi, ölçülmeden açılmadı. Play Store indirmeyi zaten sıkıştırır; doğrudan APK dağıtımında anlamlı.
- **Açık: `wasm-opt -Oz` (binaryen).** Go wasm'ında genellikle %10–20; denenmedi. Adım: `binaryen`'i kurup `scripts/web.sh`'e isteğe bağlı eklemek,
  sonra hızı yeniden ölçmek.

## Büyük kitaplık (N6)

`perf_test.go`: sentetik kitaplık (5 koleksiyon, her sayfada metin, bağlantı, etiket, görev). `PERF_PAGES=10000 go test -run TestPerfBudget -v`.
Varsayılan 1 500 sayfa; bütçe (açılış 1,5 sn, ilk kare 3 sn, arama 2 sn, palet 0,6 sn, bellek 200 MB / 1000 sayfa) bin sayfa başına ölçeklenir,
bir tuş vuruşu 60 ms (kitaplıkla ölçeklenmez). İki-üç kat yavaşlayan bir regresyonu yakalar, yavaş makineyi yakalamaz.

| 10 000 sayfa (7 Kasım) | süre / bellek |
|---|---|
| kitaplığı aç | 0,52 sn |
| ilk kare (başsız, yazılım çizici) | 0,53 sn |
| metinde ara | 0,48 sn |
| palet sorgusu | 22 ms |
| kitaplık rakamları | 0,54 sn |
| bellek | +15 MB |

**Açık sorun: tuş vuruşu bütçeyi aşıyor.** `TestPerfBudget` tam test setiyle (`go test ./...`) koşunca bir tuş vuruşu 70–90 ms sürüyor ve
60 ms'lik bütçeyi aşıyor; tek başına koşunca 35–55 ms (9 Kasım). Ya tuş yolu yavaşladı ya da ölçü yük altında güvenilmez; ikisi ayrılmalı
(`BenchmarkKeyBigLibrary`). Gerçek GPU'da hiç ölçülmedi.
