# Teknik denetim: bulgular ve aksiyon planı

Kaynak: Antigravity denetim raporu (`audit_report.md`, 2026-11-08). Rapordaki her önemli bulgu bu depodaki koda karşı kontrol edildi (çalışma ağacı, `main` 457f2fa + commitsiz değişiklikler). Rapor kimi yerde doğru, kimi yerde yanlış ya da abartılı çıktı; bu belge hangisinin hangisi olduğunu ve ne yapılacağını ayırır.

Durum etiketleri:
- **DOĞRU**: kodda doğrulandı.
- **KISMEN**: sorun var ama raporun anlattığı kadar ağır ya da tam o biçimde değil.
- **YANLIŞ**: kodda öyle bir sorun yok.
- **DOĞRULANMADI**: makul görünüyor, ama kodu okumak yetmedi; önce yeniden üretmek gerek.

## Durum (2026-11-08, sonra)

Yapıldı, testli: A1 (+ yetim nesne, toplu silme sigortası, senkron silmeleri 30 gün Çöp'te), A2, A3, A4 (gerçekti: bölme etkinleşince `resyncDoc`), B1, B2 (gerçekti: senkron, geçişte yazılan dosyanın üzerine yazmıyor), B3, B4 (`where()` çizimde bir şey oluşturmuyor, kilit altında okuyor), B5 (kilitler), C1 (MCP komut izin listesi), C2, C3 (`safeName`, `safeSubpath`, `within`), C4 (Worker: kota rezervasyonu, tek sorgulu `allow()`, mezar taşı toplu silme, yalnız yanlış kodlar global sayaçta), C5 (`0o600`), D1 (gerçek Markdown dışa aktarma, `exportmd.go`), D2, D3, D4 (en çok 40 belge metni, `touchDoc`), E3 (içe aktarma ve senkron hash önbelleği).

Gerçek çıkmadı, kod değişmedi: A5 (`checkDisk` zaten güncelliyor; `TestResolveConflictUpdatesOpenPage` bekçi olarak kaldı), E1 (arama sorgusu karede 0,2-0,5 ms), E2 (`ReadNamed` 8000 sayfada 7 ms, 60 MB resimle bile).

Bilerek yapılmadı: F1 (paket bölme); `time.Tick` (süreç ömrü boyunca çalışan döngüler, etkisi yok); `store_js.go` IndexedDB'yi dosya başına anahtara bölmek (WASM yapısı HEAD'de derlenmiyordu, `snapshot`/`endSnapshot`/`ReadNamed` eklenip derlenir hale getirildi, bölme çalıştırılıp denenemediği için yapılmadı); Worker'ın cron tetikleyicisi `wrangler.toml`'da, `deploy.sh`'ın ürettiği yapılandırmada yok (liste isteği artık süresi dolmuş 1000 mezar taşını tek çağrıyla siliyor, yani canlıda bir şey eksilmedi).

Worker değişiklikleri canlıya DEPLOY EDİLMEDİ: yerel `wrangler dev` ile denendi (kota yarışı: altı eşzamanlı yazıdan yalnız biri geçti).

---

## 1. Önce yapılacaklar (P0: veri kaybı)

### A1. E2EE senkronda `_index` yazılamazsa yerel dosyaların silinmesi — DOĞRU, YAPILDI
- Durum (2026-11-08): `Flush` sırası ve başarısızlıkta "gönderildi" kaydının unutulması `dc50230`'da; yetim nesnenin üzerine yazma (`encRemote.Put`) ve toplu silme sigortası (`massRemovals`: tombstone'suz ≥20 dosya ve kitaplığın %10'u silinmez, ayarlarda "Silmeleri uygula" düğmesi) sonradan eklendi. Testler: `TestSyncIndexFailureKeepsLocalFiles`, `TestSyncIndexFailureThenEdited`, `TestSyncHoldsMassRemoval`, `TestSyncMassRemovalNotHeldWhenDeletedThere`. Aşağıdaki açıklama özgün bulgudur.
- Yer: `sync.go` `syncOnce` (defer bloğu), `SyncOnce`, `encRemote.List/Put/Flush`; `syncapp.go` `syncNow`.
- Doğrulama: `encRemote.List()` yalnızca bellekteki `e.idx`'te olan adları döndürür. `Put` adı `e.idx`'e ekler ama `_index`'i yalnızca her 200 yeni adda bir ve `SyncOnce` sonunda yazar. `syncOnce`'ın `defer`'i `Flush`'tan **önce** çalışıp `st.Files[n].Remote` değerini yazar. `Flush` ağ hatasıyla düşerse `syncNow` yine de `saveSyncState` çağırır. Sonraki turda sunucu `_index`'inde ad yoktur: `rh == ""`, `base.Remote != ""`, yerel değişmemiş → `fsys.Remove`. Yani kullanıcının yerel dosyası "uzakta silindi" sanılıp silinir.
- Aynı yol, 200'lük ara `Flush` başarısız olursa da açılır.
- Yapılacak:
  1. `SyncOnce` içinde `Flush`'ı, `Remote` hash'lerinin `st.Files`'a yazılmasından önce çalıştır. `Flush` hata verirse `pushed` listesindeki dosyalar için `st.Files[n]`'i **kaydetme** (ya da `Remote`'u boş bırak; sonraki tur "iki taraf da değişti, aynı bayt" yoluna girer).
  2. Silme dalına emniyet ekle: `rh == ""` yüzünden yerel silmeden önce `inner.List()` ham listesinde opak ad gerçekten yok mu bak; `encRemote` için "indekste yok ama ham listede var" durumunu silme sayma.
  3. Toplu silme sigortası: bir turda yerel dosyaların %X'inden fazlası silinecekse (ör. >%10 ve >20 dosya) turu durdur, kullanıcıya sor.
  4. Test: `encRemote`'u `saveIndex` hata verecek sahte bir `Remote` ile sarıp iki tur çalıştır; yerel dosyaların durduğunu doğrula.
- Zorluk: orta. Önem: en yüksek.

### A2. Hata yutulunca sayfanın `"[]"`/boş ile ezilmesi — KISMEN
- Yer: `pagefile.go` `splitPage`, `cover.go` (`setCover`, `setCoverGradient`, `removeCover`, `restoreAutoCover`: `body, _ := p.Body()`).
- Doğrulama: `splitPage` bozuk JSON'da `"[]"` döndürür; `Page.Body()` bunu hata olarak iletmez. `cover.go` `Body()` hatasını yok sayıp `Save(body)` çağırır; okuma hatasında `body == ""` ile kaydeder. Gerçek ama tetiklenmesi nadir (dosya okunamaz ya da bozuk olmalı). Geri alınabilirlik: `versions` tablosu var, tamamen geri dönüşsüz değil.
- Yapılacak:
  1. `splitPage`'in `error` döndüren bir sürümü; `Page.Body()` bozuk JSON'da hata döndürsün (okuyan, gösteren yerler eski davranışı koruyabilir).
  2. `cover.go` içindeki dört yerde `Body()` hatası varsa kaydı iptal et, `a.saveErr` ile göster.
  3. `SetTitle`/`renumber` yollarında da aynı kontrol.
- Zorluk: düşük. Önem: orta.

### A3. Pasif bölmelerde kaydedilmemiş sayfa, silinmiş sayfa, nil belge — DOĞRU (kısmen doğrulandı)
- Yer: `main.go` `flush`, `panes.go` `restorePanes`, `sidebar.go` `trashPage`, `mcp.go` `writeBody`.
- Doğrulama: `flush()` yalnızca `a.page` ve `a.dirty` için çalışır, pasif bölmeler yok. `restorePanes` `st.doc = a.docs[p]` yazar; `a.docs[p]` yoksa `nil` kalır. `withPane` ile `a.paneState` takası yapısı gerçek.
- Yapılacak:
  1. `flush`'ı `a.ps.states` üzerinde dönen bir sürüme çevir (pasif bölmelerin `dirty` durumu); çıkışta ve senkron öncesinde bunu çağır.
  2. `restorePanes`: `a.docs[p] == nil` ise belgeyi yükle ya da `st.page = nil` yap. `pageBody` yolunda nil koruması.
  3. `trashPage`, MCP `writeBody`/`update_page`, `takeOutside`: tüm bölmeleri tara.
  4. Test: iki bölmede aynı sayfa, pasifte yazı, çıkış; sayfayı çöpe at, pasif bölmenin boşaldığını doğrula.
- Zorluk: orta. Önem: yüksek (kayıt kaybı).

### A4. Aynı sayfa iki bölmede: sürüm/imleç durumu paylaşımı — DOĞRULANMADI
- Yer: `main.go` `track`, `panes.go` `splitPane`.
- Not: `track` `a.version`, `a.caretAt`, `a.length` gibi bölmeye ait alanları kullanıyor ve `a.docs[p]` tek örnek; raporun senaryosu makul. Ancak bölme sayısı en fazla 2 ve durum takası `withPane` ile yapılıyor: gerçek bozulmanın oluşup oluşmadığı çalıştırarak görülmeli.
- Yapılacak: önce yeniden üretim (aynı sayfa iki bölmede, birinde yaz, ötekine tıkla; zincirleme dönüşümler: tablo/çip/kısayol). Üretilirse `version/length/prevText` durumlarını belge düzeyine (`docState`) taşı.
- Zorluk: yüksek. Önem: yeniden üretime bağlı.

### A5. `resolveConflict` sonrası bellekteki belge eski kalıyor — DOĞRULANMADI
- Yer: `syncapp.go` `resolveConflict`, `watch.go` `reloadLibrary`.
- Yapılacak: önce yeniden üret (çakışan sayfa açıkken "çakışmayı koru"). Üretilirse `resolveConflict` başında `flush`, sonra ilgili `a.docs[p]` kaydını düşür / `takeOutside(p)`.
- Zorluk: düşük-orta. Önem: orta.

---

## 2. Eşzamanlılık (P1)

### B1. `Store.snap` için kilitsiz erişim — DOĞRU
- Yer: `store_sql.go`: `snap` alanı, 7 yerde `s.snap = nil`, `snapshot/endSnapshot`, okuyucular (`ReadFile`, `List`...).
- Doğrulama: `Store`'da mutex yok. `db.SetMaxOpenConns(1)` SQL çağrılarını sıraya koyar, ama `s.snap` işaretçisi ve çok adımlı `WriteFile`/`Remove` (SELECT + INSERT/DELETE) kilit/transaction dışında. Arka planda `syncNow`, `lookForConflicts`, `prewarm` aynı `Store`'u kullanıyor.
- Yapılacak:
  1. `snap` işaretçisini `atomic.Pointer[storeSnap]` yap (en ucuz ve yeterli çözüm), ya da `sync.RWMutex`.
  2. `WriteFile`/`Remove` içindeki çok adımlı SQL'i `db.Begin()` transaction'ına al.
  3. `go test -race` ile ilgili testleri çalıştır, bulgunun kanıtını ve düzeltmeyi ölç (MyGo/cgo nedeniyle bütün uygulamayı değil, `Store` + `SyncOnce` testlerini).
- Zorluk: düşük-orta. Önem: yüksek.

### B2. Senkron arka planda çalışırken kullanıcı kaydı — DOĞRULANMADI
- Rapor, `syncNow` içinde `SyncOnce`'ın `a.mu` kilidi olmadan çalıştığını ve kullanıcı kaydını eski uzak sürümle ezebileceğini söylüyor. Motor "iki taraf da değişti → çakışma kopyası" mantığıyla çalıştığı için kullanıcı verisi ezilmez, çakışma kopyasına düşer; ama yerel hash `localFiles` okuma anına göre alınır, araya giren yazma olası. Önce testle (`SyncOnce` sırasında eşzamanlı `WriteFile`) bak, sonra karar ver.
- Zorluk: orta. Önem: orta.

### B3. `prewarm` işçileri `*Page` alanlarını okuyor — DOĞRU
- Yer: `prewarm.go` (işçi `readPageIdxPlain(j.p, ...)` çağırıyor), `backlinks.go` `readPageIdxPlain` (`q.Modified`, `q.writes`, `tasksOf(q, ...)`).
- Doğrulama: `job` içine `Modified/writes` kopyalanıyor ama işçi yine `*Page` üzerinden okuyor; ana iş parçacığı `Page.Save`'de aynı alanları yazıyor.
- Yapılacak: `readPageIdxPlain`'e `*Page` yerine `mod/writes/dir` gibi değerleri ver (`tasksOf`'un `Page`'in hangi alanlarını okuduğuna bak), işçiler `*Page`'e hiç dokunmasın.
- Zorluk: düşük. Önem: orta (yarış zararsız görünse de Go bellek modelinde tanımsız).

### B4. `quickNote.view` kilitsiz `app` erişimi — DOĞRULANMADI
- Yer: `quick.go` `view`, `where`, `quickCollection`, `journal`.
- Not: MyGo'nun ikinci pencerelerinin aynı UI iş parçacığında çizilip çizilmediği belirleyici. Aynı iş parçacığındaysa yarış yok; ama `where()` her karede `journal()`/`quickCollection()` çağırıyor ve bunlar koleksiyon yoksa **oluşturuyor** olabilir (bir çizim fonksiyonunda yan etki, o zaman yarıştan bağımsız bir hata). Önce `journal()` ve `quickCollection()` içeriğini oku.
- Yapılacak: `where()` sonucunu pencere açılışında hesapla/önbelleğe al; çizimde yan etkiyi kaldır.
- Zorluk: düşük. Önem: düşük-orta.

### B5. `guard`/`noteCrash` alan erişimi, `mcpHandler` token okuma, `time.Tick` — DOĞRU ama küçük
- `noteCrash` `a.crashMsg/crashText/crashOpen` alanlarını kilitsiz yazıyor (`crash.go`). `mcpHandler` `a.settings.Token`'ı kilitsiz okuyor. Yapılacak: `noteCrash` değişikliklerini `a.do(...)` içine al; token'ı başlangıçta bir `atomic.Value`'ya kopyala. `time.Tick` (`watch.go`, `images.go`) için `NewTicker` + kapanışta `Stop`: uygulama ömrü boyunca çalıştıkları için gerçek etkisi çok az.
- Zorluk: düşük. Önem: düşük.

---

## 3. Güvenlik (P1/P2)

### C1. MCP `run_command` her paletteki komutu açıyor — DOĞRU
- Yer: `mcp_server.go` `run_command`.
- Doğrulama: `a.runCommand(in.Command)` palet komutlarının tamamını çalıştırır ("Çöpü boşalt", "Hepsini varsayılana döndür", dosya seçici açanlar dahil). Token sahibi bunların hepsini tetikleyebilir.
- Yapılacak: izin listesi (allowlist) tanımla: yalnızca görünüm/gezinme gibi yıkıcı olmayan komutlar; `list_commands` da aynı listeyi göstersin. Yıkıcı komutlar için MCP üzerinden hiçbir zaman izin verme ya da kullanıcı onayı iste.
- Zorluk: düşük. Önem: yüksek (yerel jeton ele geçerse kalıcı veri kaybı).

### C2. MCP token karşılaştırması `!=` — DOĞRU ama düşük risk
- Yer: `mcp_server.go:61` civarı. Sunucu yalnızca `127.0.0.1`'de ve `Host` denetimi (DNS yeniden bağlama önlemi) var; zamanlama saldırısı pratikte zor. Yapılacak: `subtle.ConstantTimeCompare`. Tek satır.
- Zorluk: çok düşük. Önem: düşük.

### C3. Dizin geçişi (path traversal) — KISMEN (tehdit modeline bağlı)
- Yer: `sync.go` `syncOnce` (`abs(n)`) ve `dirRemote.file`; `importmd.go` `importPictures`; `library.go` `Restore` (`item.From`).
- Doğrulama: `abs(n) = filepath.Join(root, filepath.FromSlash(n))`, `n` uzak adından geliyor, `..` denetimi yok. Klasör tabanlı kitaplıkta ele geçirilmiş/kötü niyetli senkron sunucusu `../..` ile dışarı yazabilir/silebilir. Yerel SQLite `Store`'da `fsys` sanal olduğu için etkisi yok. `importPictures` için `filepath.Join(dir, ref)` `..` ile vault dışına çıkıp başka bir dosyayı kitaplığa kopyalayabilir (okuma yönü: dosya sızdırma değil, kullanıcının kendi makinesinde içe aktarma). `Restore` `.trashed.json` yerel, güvenilir dosya.
- Yapılacak: merkezi `safeSubpath(root, rel)` (mutlak yol ve `..` segmentini reddet) yaz; `syncOnce`'ta her `n` için, `dirRemote.file`'da, `importPictures`'ta kullan. Sync için adı geçersiz olan dosyayı atlayıp `res.Errors`'a yaz.
- Zorluk: düşük. Önem: orta (E2EE'siz, üçüncü taraf sunucu kullanılırsa yüksek).

### C4. Cloudflare Worker — KISMEN
- Yer: `cloudflare/sync/worker.js`, `cloudflare/sync/auth.js`.
- (a) Global kayıt kilidi: rapor abartılı. `code:all` sayacı yalnızca IP başına `code:` sınırı (10/saat) geçildikten sonra artar (`||` kısa devre). Saldırganın 200/saat'e ulaşması için ~20 farklı IP gerekir. Yine de IPv6/botnet ile mümkün; sayaç yalnızca **geçersiz kod** denemelerinde artmalı ya da `code:all` sınırı gerçek kayıt akışını engellemeyecek kadar yükseltilmeli (örn. admin sayfasında uyarı). Önem: düşük-orta.
- (b) Kota yarışı: DOĞRU. `user.bytes` kontrolü yazımdan önce bellekteki değerle, `bytes()` güncellemesi yazımdan sonra; eşzamanlı PUT'lar kotayı aşabilir, D1 hata verirse sayaç kayar. Yapılacak: kota güncellemesini `UPDATE users SET bytes=bytes+? WHERE id=? AND bytes+?<=quota` biçiminde atomik yap; R2 yazımı başarısızsa geri al; arada bir sayaç yeniden hesaplama (reconcile) işi.
- (c) `GET /files` içinde mezar taşı silme: DOĞRU. Çok sayıda süresi dolmuş mezar taşında istek yavaşlar ve alt istek limitine takılır. Yapılacak: silmeyi Cron Trigger (`scheduled`) içine taşı; listelemede yalnızca süresi dolanları atla.
- (d) `allow()` iki sorgu: `INSERT ... ON CONFLICT ... RETURNING n` ile tek sorgu (D1 RETURNING destekler).
- Zorluk: orta. Önem: orta.

### C5. Gizli anahtarlar düz metin, ayarlar `0o644` — KISMEN
- Rapor "ayar dışa aktarmada sızıyor" diyor: **YANLIŞ**. `settingsx.go` `secretKeys` listesi `token`, `syncToken`, `syncKey`, `accountUser`, `syncDir`, `syncUrl`... alanlarını dışa aktarmadan önce siliyor (ve içe aktarırken dokunmuyor).
- Doğru olan kısım: `settings.json` `osFS.WriteFile` ile `0o644`; `SyncKey`/`SyncToken`/MCP `Token` yerel diskte düz metin ve herkesçe okunabilir izinle. Ayrıca dışa aktarılan dosya da `0o644` (içinde gizli anahtar yok ama kullanıcı ayarları var).
- Yapılacak:
  1. Ayar dosyasını `0o600` ile yaz (yalnızca ayar dosyası; kitaplık dosyalarının izni ayrı bir konu).
  2. Orta vadede: `SyncKey`, `SyncToken`, `Token` için işletim sistemi anahtarlığı (macOS Keychain). Kapsam: MyGo/platform desteğine bağlı, ayrı iş.
  3. `exportSettingsTo`'yu `0o600` yap; dışa aktarma testine "gizli anahtarlar çıkmaz" kontrolü ekle (kapsama sağlam kalsın).
- Zorluk: düşük (1), yüksek (2). Önem: orta.

---

## 4. Veri bütünlüğü ve ürün hataları (P2)

### D1. "Markdown olarak dışa aktar" aslında `page.json` — DOĞRU
- Yer: `settings.go` `exportLibrary`, `backup.go` `exportZip`, `store.go` `copyTree`.
- Doğrulama: `copyTree(store, storeRoot, osFS{}, dir)` yapıyor; sonuç kitaplığın klasör biçimi (`page.json`), `.md` değil. Dosya adı `kitaplik-md-...` ve arayüz etiketi yanıltıcı. Kodun yorumu "ryudit'in komut satırından açabildiği klasör kitaplığı" diyor: yani yedek biçimi olarak mantıklı, ama etiket yanlış.
- Karar gerekir: (1) etiketi düzelt ("Klasör olarak dışa aktar") **ve** (2) ayrı gerçek bir "Markdown olarak dışa aktar" ekle (`formatFrontMatter` + `bodyMarkdown(decodeBody(...))`, ekler yanında).
- Zorluk: orta. Önem: orta (kullanıcıya verilen söz tutulmuyor).

### D2. Uzak silmede yetim klasör düğümü ve ölü kod — DOĞRU
- Yer: `sync.go` `syncOnce` (`fsys.Remove(abs(n))` sonrası üst klasör kalıyor), `sync.go:608` civarı `strings.Contains(n, "(çakışma ") && false`.
- Yapılacak: uzakta silinen dosyanın boş kalan sayfa klasörünü de kaldır (`Store`'da boş klasörü `Remove`); `uniqueDir` boş klasörü yeniden kullanabilsin. `&& false` satırını kaldır, çakışma dosyalarının senkron ve görünürlük davranışını belgeleyip (şu an kenar çubuğunda görünmüyor mu? kontrol et) ona göre karar ver.
- Zorluk: düşük. Önem: düşük-orta.

### D3. Görsel önbelleği `nil`'i kalıcı tutuyor — DOĞRU
- Yer: `images.go` `loadImage`.
- Doğrulama: okunamayan/henüz senkronlanmamış görsel için `a.images[file] = nil` kaydı kalıyor ve yeniden denenmiyor. Senkron sonrası gelen görsel yeniden başlatmaya kadar görünmez.
- Yapılacak: okuma/çözme hatasında önbelleğe yazma (ya da kısa süreli negatif önbellek + `reloadLibrary`/`checkDisk` sonrası temizleme).
- Zorluk: çok düşük. Önem: orta.

### D4. `a.docs` büyüyor (LRU yok) — DOĞRULANMADI (düşük)
- ~7.9k sayfalı kitaplıkta tüm belgeler açılmadığı için gerçek büyüme açılan sayfa sayısı kadar; ağır kullanımda oturum boyunca birikir. Önce bellek ölçümü (`docs/17-boyut-ve-performans.md` ile birlikte); gerekirse yalnızca açık bölmeler dışındaki belgeleri çıkaran basit bir sınır.
- Zorluk: orta. Önem: düşük.

---

## 5. Performans (P2/P3)

### E1. Arama ekranı her karede sorgu, görev satırı her karede tarama — DOĞRULANMADI
- Yer: `search.go` `searchView`, `trash.go` `tasksRow`. Önce profil: `docs/17` ile birlikte ölçülmeli. Doğrulanırsa `(searchQ, pagesGen, links.built)` anahtarıyla sonuç önbelleği; geciken/bugünkü görev sayıları `links` kurulurken hesaplansın.
- Zorluk: düşük-orta. Önem: ölçüme bağlı.

### E2. `ReadNamed` `LIKE '%/page.json'` — KISMEN
- Doğrulama: sorgu gerçekten tam tablo taraması yapar; ama tek bir sorgu, kitaplık açılışında (ve zaten #603'te 730→140 ms'ye indirildi). Raporun "BLOB'lar B-Tree yaprağında" iddiası SQLite için abartılı (büyük BLOB'lar taşma sayfalarına gider). Düşük öncelik; yapılacaksa şemaya `name` sütunu eklemek bir şema geçişi (migration) demek, riskine değmeyebilir.
- Zorluk: orta. Önem: düşük.

### E3. İçe aktarmada `WalkDir` döngüsü, senkronda her turda SHA-256 — DOĞRU
- `importPictures` her gömülü resim için tüm vault'u tarıyor (O(N×M)); `localFiles`/`dirRemote.List` her turda her dosyayı baştan hash'liyor. Yapılacak: içe aktarma başında bir kez ad→yol haritası; senkronda `(mtime, size)` değişmediyse önceki hash'i kullan (`syncState`'e `mtime/size` ekle). WASM/IndexedDB tüm kitaplığı tek anahtara yazıyor: web hedefi canlıya çıkacaksa dosya başına anahtar.
- Zorluk: orta. Önem: orta (senkron).

---

## 6. Mimari ve depo (P3: istek olunca)

### F1. Tek `package main`, devasa `app`/`paneState` struct'ları, paket seviyesi global değişkenler — DOĞRU (tespit), önceliği düşük
- Doğru tespit; ama 200 dosyalık bir yeniden yapılandırma şu an en riskli ve en az değer üreten iş (çalışma ağacında 40+ commitsiz değişiklik varken özellikle). Önerilen sıra: (1) A3/A4 gibi gerçek hataları düzelt; (2) `withPane` takasını kaldırmak için `paneState`'i parametre olarak geçirme işini, A3/A4 ile bağlantılı olarak kademeli yap; (3) paket bölme (`internal/store`, `internal/sync`) yalnızca sınırı temiz olan `Store` ve `sync` motoru için düşünülebilir, UI'a dokunmadan.
- Paket seviyesi global değişkenler (`rowH`, `phone`, `radiusScale`, `longMemo`...) için gerçek sorun, ikinci pencereler (quickNote, panewin) bunları paylaşıyorsa. Önce hangilerinin gerçekten karelere göre değiştiğini listele.

### F2. Depo hijyeni — YANLIŞ
- `.gitignore` zaten `/ryudit`, `/kitaplik/`, `/kitaplik.db`, `node_modules/` içeriyor; `git ls-files` bunların hiçbirini izlemiyor. Aksiyon yok.

---

## Önerilen sıra

| # | Aksiyon | Önem | Zorluk |
|---|---------|------|--------|
| 1 | A1 E2EE senkron silme hatası + toplu silme sigortası + test | Kritik | Orta |
| 2 | C1 MCP `run_command` izin listesi, C2 sabit zamanlı karşılaştırma | Yüksek | Düşük |
| 3 | A3 pasif bölme `flush`/`trashPage`/`restorePanes` nil | Yüksek | Orta |
| 4 | B1 `Store.snap` atomik + `WriteFile`/`Remove` transaction | Yüksek | Düşük-orta |
| 5 | A2 hata yutma (cover.go, splitPage) | Orta | Düşük |
| 6 | D3 görsel önbelleği nil | Orta | Çok düşük |
| 7 | C3 `safeSubpath`, C5 ayar dosyası `0o600` | Orta | Düşük |
| 8 | B3 prewarm yarışı, B5 küçük kilitler | Orta/Düşük | Düşük |
| 9 | C4 Worker kota atomikliği + cron ile mezar taşı temizliği | Orta | Orta |
| 10 | D1 dışa aktarma etiketi ve gerçek `.md` dışa aktarma | Orta | Orta |
| 11 | D2 yetim klasör + ölü kod | Düşük-orta | Düşük |
| 12 | A4, A5, B2, B4 önce yeniden üret, sonra karar | ? | ? |
| 13 | E1, E3, D4 ölçerek | Düşük-orta | Orta |
| 14 | F1 mimari dilimleme | Düşük | Yüksek |

Raporun kendi "en kritik 3"ünden üçüncüsü (SQLite `Store` ve `quickNote` yarışları) tek bir madde değil: `Store` kısmı B1 (doğru), `quickNote` kısmı B4 (doğrulanmadı), `prewarm` kısmı B3 (doğru).

## Test notları
- Yeni testleri Go'nun `-race` bayrağıyla çalıştır (`Store`, `SyncOnce`, `prewarm` için).
- Bilinen kırık testler (bu denetimle ilgisiz): `TestWhiteSpaceMarks`, `TestStatsOut`.
- A1 için `sync_test.go`'ya, `saveIndex`'i başarısız eden bir `Remote` sarmalayıcısıyla, iki turluk senaryo.
