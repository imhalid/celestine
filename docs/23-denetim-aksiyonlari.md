# Teknik denetim: kalanlar

Kaynak: Antigravity denetim raporu (`audit_report.md`, 2026-11-08), koda karşı kontrol edildi. Doğru çıkan bulguların hepsi (A1–A4, B1–B5, C1–C5, D1–D4, E3) düzeltildi ve testli; A5, E1, E2 gerçek çıkmadı; F2 yanlıştı. Burada yalnız açık kalanlar ve bilerek verilen kararlar var.

## Açık

- **Worker'ın günlük cron'u canlıda yok:** `wrangler.toml`'da `crons` var, ama `cloudflare/sync/deploy.sh`'ın ürettiği yapılandırmada tetikleyici yok. Süresi dolmuş mezar taşlarını liste isteği de siliyor (1000'er); ama sunucudaki sürümlerin 30 günlük budaması yalnız cron'da (yazma yalnız dosya başına 10'u aşanı siler).
- **C5 (orta vade):** `SyncKey`, `SyncToken`, MCP `Token` işletim sisteminin anahtarlığında değil, ayar dosyasında (`0o600`). MyGo/platform desteğine bağlı, ayrı iş.
- **C4 Worker değişiklikleri** (kota rezervasyonu, tek sorgulu `allow()`, mezar taşı toplu silme) yerelde `wrangler dev` ile denendi; canlıya gidip gitmediği doğrulanmalı.

## Bilerek yapılmadı

- **F1** (tek `package main`, büyük `app`/`paneState`): yalnız `internal/library` ayrıldı; UI'ı paketlere bölmek şu an riskli ve az değerli.
- `time.Tick` (`watch.go`, `images.go`): süreç ömrü boyunca çalışan döngüler, etkisi yok.
- `store_js.go`'da IndexedDB'yi dosya başına anahtara bölmek: web sürümü canlıya çıkacaksa gerekir; çalıştırılıp denenemediği için yapılmadı.
- E2 (`ReadNamed` `LIKE '%/page.json'`): 8000 sayfada 7 ms; `name` sütunu bir şema geçişi ister, değmez.

## Test notları

- `Store`, `SyncOnce`, `prewarm` testleri `-race` ile çalıştırılmalı (MyGo/cgo yüzünden bütün uygulama değil).
