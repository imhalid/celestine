# 18 — Web uygulaması (J)

celestine tarayıcıda çalışır (7 Kasım 2026'da derlendi, tarayıcıda yazıldı, kaydedildi, yenileyince yerindeydi; çevrimdışı da açılıyor).

## Nasıl

- `modernc.org/sqlite` js/wasm için derlenmediğinden `Store` ikiye ayrıldı: `store_sql.go` (`!js`, SQLite dosyası) ve `store_js.go` (`js`, aynı tür:
  bellekte bir dosya ağacı). Sürüm geçmişi (`history_sql.go`) ve dosya yedeği (`backup_sql.go`) yalnız masaüstünde; web'de yedek yerine dışa aktarma ve senkron var.
- `scripts/web.sh`: `mygo build -platform web` ile `dist/web/` (index.html, wasm, wasm_exec.js, PWA dosyaları, `web/touch.js`) ve `:8765`'te sunar.
- **PWA:** `web/manifest.webmanifest`, `web/sw.js`, simgeler. Servis çalışanı gerçek Chromium'da kaydoldu; sunucu durunca sayfa ve wasm önbellekten geldi.
  Sayfa önce ağdan (yoksa önbellekten), adı derlemeye bağlı dosyalar önce önbellekten gelir: yoksa yeni derleme hiç alınmaz.
- Dar pencere (telefon) telefon düzeninde açılır ve parmakla kaydırılır: ayrıntı docs/21.

## Boyut

wasm ≈ 28,5 MB (brotli ≈ 5,3 MB): MCP sunucusu web derlemesinde yok (docs/17). İlk yükleme bu kadar, ikincisi önbellekten.

## Sınırlar

- **Depolama:** kitaplık **IndexedDB**'de (`store_js.go`): başlangıçta bir kez okunur, değişikliklerden 300 ms sonra ve sayfa gizlenirken yazılır; IndexedDB
  yoksa localStorage'a (≈5 MB) düşer. Tüm kitaplık tek anahtarda (docs/23). File System Access API ve OPFS yok.
- **Klavye modu:** Esc ile normal kip açılıyor, ama o kipte harf tuşları (`f`, `?`) otomasyon aracından gönderilince çalışmadı: gerçek klavyeyle denenmeli.
- **Web'de olmayanlar:** MCP (tarayıcı yerel sunucu açamaz), pencere sarsıntısı, hızlı not, dosya izleme, güncelleme (servis çalışanı yapar), yedek dosyası,
  pencere düğmeleri, bölmeyi ayrı pencereye alma.
- **Kısayollar:** ⌘P (yazdır), ⌘T, ⌘W, ⌘L tarayıcıya ait: komut paleti için ⇧⌘P, normal kip için Esc.

## Kalan

"Uygulamayı yükle" düğmesi, Safari ve Firefox, tarayıcıda IME ve pano denenmedi; klavye harf tuşlarının eşlemesi.
