# 18 — Web uygulaması (J): çalışan deneme sürümü

**Soru:** ryudit tarayıcıda çalışır mı? **Cevap: evet. 7 Kasım 2026'da derlendi, tarayıcıda açıldı, yazıldı, kaydedildi, sayfa yenilenince yazı yerinde durdu.**

## Ne yapıldı

- `modernc.org/sqlite` js/wasm için derlenmiyordu: tek engel buydu. `Store` ikiye ayrıldı: `store_sql.go` (`!js`, SQLite dosyası) ve
  `store_js.go` (`js`, aynı tür: bellekte bir klasör/dosya ağacı, her değişiklikte tarayıcının **localStorage**'ına yazılır). Sürüm geçmişi
  (`history_sql.go`) ve dosya yedeği (`backup_sql.go`) yalnız masaüstünde; web'de `historian` yok, yedek yerine dışa aktarma ve senkron var.
- `scripts/web.sh`: `mygo build -platform web` ile `dist/web/` (index.html, wasm, wasm_exec.js) ve `:8765`'te sunar.
- Denenen (uygulama içi tarayıcıda): karşılama sayfası açılır, kenar çubuğu, kapak, jöle caret, ilk açılış turu, yazma ("WEBTEST"), "Kaydediliyor…" →
  "Kaydedildi", sayfayı yenileyince yazı yerinde. Kitaplık `localStorage["ryudit:kitaplik.db"]` altında (3,3 KB).

## Boyut

wasm 34 MB (32 MB `mygo build` çıktısında), **brotli ≈ 6,9 MB, gzip ≈ 8,3 MB** inen. Cloudflare Pages brotli ile sunar. İlk yükleme bu kadar;
ikincisi önbellekten.

## Sınırlar (bilinen, hâlâ geçerli)

- **Depolama:** kitaplık **IndexedDB**'de (`store_js.go`): başlangıçta bir kez okunur, değişikliklerden 300 ms sonra ve sayfa gizlenirken yazılır; tarayıcının
  izin verdiği kadar büyür (tarayıcıda denendi: yazı IndexedDB'ye gitti, localStorage boş, yenileyince yerinde). IndexedDB yoksa localStorage'a (≈5 MB) düşer.
  File System Access API ile gerçek klasör (Chromium) ve OPFS yapılmadı.
- **Klavye modu:** web'de Esc ile normal kip açılıyor (köşede "NORMAL" rozeti çıktı), ama o kipte harf tuşları (`f`, `?`) otomasyon aracından
  gönderildiğinde çalışmadı: gerçek klavyeyle ve tarayıcı tuş eşlemesiyle denenmeli (masaüstünde testli). Yazma ve fare çalışıyor.
- **MCP:** tarayıcıda yerel sunucu açılamaz; web sürümünde ajan bağlantısı yok.
- **Pencere sarsıntısı, hızlı not (genel kısayol), dosya izleme, güncelleme (service worker yapar), yedek dosyası:** web'de yok.
- **Kısayollar:** ⌘P (yazdır), ⌘T, ⌘W, ⌘L tarayıcıya ait: komut paleti için ⇧⌘P, normal kip için Esc.
- Başka bir bilgisayarla eşitleme: `httpRemote` tarayıcıdan da çalışır (aynı Go kodu); CORS için `cloudflare/sync/worker.js`'e başlık eklenmeli.
- **PWA:** `web/manifest.webmanifest`, `web/sw.js` (programı önbelleğe alır: çevrimdışı açılır, ikinci açılış anında) ve simgeler `scripts/web.sh` ile
  `dist/web`'e konur, `index.html` bunlara bağlanır. Manifest geçerli ve sw.js sözdizimi doğru; **servis çalışanı bu ortamdaki uygulama içi tarayıcıda
  kaydolmadı** (tarayıcı kayıt isteğini reddetti), o yüzden çevrimdışı açılış ve "kur" düğmesi gerçek bir tarayıcıda denenmedi.

## Tahmini kalan iş

PWA'nın gerçek tarayıcıda denenmesi ve düzeltilmesi 1 gün, tarayıcı testleri (Chrome, Safari, Firefox; IME, pano, dokunmatik) 2 gün, klavye harf tuşlarının eşlemesi 1 gün.
