# 20 — Senkronizasyon (L): karar ve ne yapıldı

## Karar

**Tek ana yol: dosya düzeyinde, üç yollu eşitleme; uzak taraf bir `Remote` arayüzü.** Kitaplık küçük dosyalardan oluşan bir ağaç (bir sayfa =
`page.json`'lu klasör); eşitleme birimi dosya. Uzak taraf iki türlü:

- **Klasör** (`dirRemote`): iCloud, Dropbox, Syncthing gibi araçlar klasörü taşır; celestine klasöre yazar/okur. Hesap gerekmez.
- **Sunucu** (`httpRemote`): `cloudflare/sync/worker.js` (Worker + R2). Cihaz başına bir jeton; `If-Match` ile yarış koşulu R2'de çözülür.

Git ve Durable Objects (gerçek zamanlı) seçilmedi: Git yazar kitlesi için ağır, DO tek yazarlı ama bedeli ve karmaşıklığı fazla; ikisi de
`Remote` arayüzüyle sonradan eklenebilir.

## Algoritma (`sync.go`)

Her dosya için son eşitlemedeki durum saklanır (yerel karma, uzak karma). Şimdikiyle karşılaştırılır:

| Yerel | Uzak | Sonuç |
|---|---|---|
| değişmedi | değişmedi | hiçbir şey |
| değişti | değişmedi | **yukarı** (silindiyse uzakta mezar taşı) |
| değişmedi | değişti | **aşağı** (uzakta silindiyse çöpe gider, aşağıya bak) |
| ikisi de, farklı | | **çakışma:** uzaktaki kalır, bizimki `ad (çakışma 2026-11-07 10-00-00).uzantı` olarak yanında saklanır ve bir sonraki geçişte o da gider. Veri kaybolmaz |
| biz değiştirdik, onlar sildi | | bizimki kalır, yeniden gider |

Çakışmalar Ayarlar ▸ Senkron'da listelenir: **Bunu kullan** (kopya asıl dosyanın yerine geçer) ya da **Sil**. Durum çubuğunda `⇅ Eşit 14:32 /
eşitleniyor… / çevrimdışı / çakışma: 2` rozeti. Çevrimdışıyken yazılan yerelde durur, bağlanınca gider (test). Silme çöp mantığıyla uyumlu:
`.trashed.json` notları da dosya olarak eşitlenir.

## Güvenlik

- **Uçtan uca şifreleme** (isteğe bağlı): parolayı PBKDF2 (200 bin tur, uzak taraftaki tuzla) AES-256 anahtarına çevirir; her dosya AES-GCM ile
  mühürlenir; **dosya adları** HMAC'lenir, adları geri çözen küçük bir mühürlü dizin (`_index`) tutulur. Uzak taraf yalnız dosya sayısı ve boyutları
  görür (test: uzaktaki hiçbir ad ya da baytta düz metin yok). Yanlış parola veri vermez. **Kurtarma anahtarı** (anahtarın base32 hâli) bir kez gösterilir;
  parolayı ve anahtarı kaybetmek veriyi kaybetmektir, ayarlarda yazılı.
- Yalnız HTTPS. Jeton ve anahtar kitaplık dosyasının içindeki ayarlarda durur (dosyayı paylaşan, onu da paylaşır).
- What the remote serves is checked too (#148). The index carries a generation (a counter per device), and a sealed
  `_hashes` object beside it keeps each file's hash and sealed hash under its own generation. A device keeps the sealed
  copies it synced of each file. Each put seals with a nonce of its own, so an honest write never repeats earlier bytes:
  an older index, a file served under another's name, an older copy, or a deleted file brought back is refused, and
  the sync settings offer "Trust the remote again". An index or hashes missing are waited for five minutes (a folder
  synced by another program may bring them late); a folder not there at all is offline. Since phase 2 each file is
  sealed under a label of its path, so a file served under another's name does not open, and is refused; a file the
  hashes do not vouch for is waited for five minutes, as another device puts a file before its hashes, then refused.
  A file sealed before, with no label, still opens, and is sealed with its label when it is next written.
- A pass lists the server only when another device wrote since this one's last pass (#151). The server's counter
  moves once for each write it answers "ok" (`/changes` says `exact`), and this device counts its own: when the
  counter is what its last pass left, the pass takes what the sync state knows for the listing. A write of another
  device, before or during a pass, a pass with an error, or a server that is not exact, makes the next pass list.
- A passphrase is checked when it is set: against the sealed index, and before there is one against `_check`, a short
  text sealed with the key and put beside the salt by the passphrase that made the salt (#562). A second device
  joining with a typo before the first one's index is there is refused instead of sealing under another key. A remote
  whose salt came before the check has none, and takes any passphrase until its index is there.

## Silinenler çöpe gider, toplu silme beklemeye alınır (8 Kasım 2026)

- Uzakta silinmiş bir dosya burada hemen silinmez: `.cop` çöpüne gider (`synctrash.go`), **30 gün** kalır, geri alınabilir (Çöp ekranı "N gün sonra silinir" der);
  süre dolunca uygulama açılışında ve her eşitlemeden sonra `PurgeTrash` siler. Aynı bayt zaten çöpteyse (başka cihazda çöpe atılan sayfa) iki kez tutulmaz.
- Bir geçişte uzakta yok görünen 20'den çok dosya (ve yerelin onda birinden çoğu) **silinmez, bekletilir**: sunucu ya da klasör listesini kaybetmiş olabilir.
  Ayarlar ▸ Senkron'da "N dosya uzakta yok" ve **Silmeleri uygula** çıkar; mezar taşı olanlar (gerçek silmeler) bu sınıra girmez.
- Şifreli dizin yazılamazsa (`_index`) kalan yetim nesne üzerine yazılır; uzaktan gelen güvensiz adlar (`..`, mutlak yol) süzülür; geçiş sırasında kaydedilen sayfa uzaktakiyle ezilmez.

## Sunucudaki sürümler (8 Kasım 2026)

Worker bir dosyanın üzerine yazılırken (ya da silinirken) önceki hâlini `v<hesap>/<ad>@<zaman>` altında saklar: en çok **10 dakikada bir**, dosya başına **10**, **30 gün**
(yaşa göre günlük cron budar, sayıya göre her yazma), yalnız **1 MB'a kadar** dosyalar (sayfa, resim değil); `_index` ve `_salt` hariç. Kopyalar kotadan düşer, silinince geri verilir; hesap silinince
gider. Bayt olarak mühürlüdür: sunucu okuyamaz, uygulama kendi anahtarıyla açar. `GET /versions/<ad>` zamanları, `?at=<ms>` baytları verir.

Uygulamada: sayfa menüsü ▸ **Sunucudaki sürümler…** (yalnız sunucu hesabıyla), yerel Sürüm geçmişi penceresinin aynısı (fark görünümü, **Bu sürüme dön**; dönünce
sayfa yerelde o olur ve bir sonraki eşitlemede gider). En yeni 10 kopya getirilir. Denendi: 15 yazma → 10 kopya (en eskisi 04, en yenisi 13), silinince son içerik kopya olur,
kota doğru artıp azalır. **Denenmedi:** 10 dakikalık aralığın gerçek zamanda (yerelde `VERSION_GAP_MS` değişkeniyle 0'a indirildi); 30 günlük süre dolumu (`scheduled`).

## Bağlantıyı dene

Ayarlar ▸ Senkron ve Profil'de **Bağlantıyı dene**: yerin cevap verdiğini, hesabın ve anahtarın geçerli olduğunu (şifreli dizin açılır) ve kaç dosya olduğunu ms ile söyler;
hiçbir şeyi değiştirmez.

## Seçmeli ve yedek

Koleksiyon başına eşitle aç/kapa (kapalıysa hiçbir şeyi yukarı çıkmaz, aşağı inmez). Her eşitlemeden önce kitaplığın yedeği alınır (`backup.go`).

## Yapılmadı (dürüstçe)

- **Sunucuda günlük anlık görüntü, paylaşım, Wi-Fi sınırı, ağ hızı ayarı, ek boyut sınırı.** (Sürüm geçmişi yapıldı, yukarıda.) **QR eşleştirme istenmedi.**
- **Anında itme**: sunucudan itme yok; uygulama pencere öndeyken hesabın değişiklik sayacını sorar (20 sn'den başlayıp değişiklik gelmedikçe
  1 dk'ya kadar seyrekleşir, 5 dk kimse yoksa durur; #44, #45). Gerçek itme (Durable Object/WebSocket) sonraya.
- **Canlı sunucuya karşı uygulama içinden uçtan uca elle deneme** (#59). Worker yerelde (`wrangler dev`, `sync_worker_test.go`) ve Cloudflare'da denendi; dağıtım `cloudflare/sync/deploy.sh`.

İki cihazda değişen sayfa blok blok birleşir (`internal/library/merge.go`); aynı blok iki yanda değiştiyse çakışma kopyası. Klasör kitaplığı da eşitlenir.
