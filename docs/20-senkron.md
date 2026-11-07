# 20 — Senkronizasyon (L): karar ve ne yapıldı

## Karar

**Tek ana yol: dosya düzeyinde, üç yollu eşitleme; uzak taraf bir `Remote` arayüzü.** Kitaplık küçük dosyalardan oluşan bir ağaç (bir sayfa =
`page.json`'lu klasör); eşitleme birimi dosya. Uzak taraf iki türlü:

- **Klasör** (`dirRemote`): iCloud, Dropbox, Syncthing gibi araçlar klasörü taşır; ryudit klasöre yazar/okur. Hesap gerekmez.
- **Sunucu** (`httpRemote`): `cloudflare/sync/worker.js` (Worker + R2). Cihaz başına bir jeton; `If-Match` ile yarış koşulu R2'de çözülür.

Git ve Durable Objects (gerçek zamanlı) seçilmedi: Git yazar kitlesi için ağır, DO tek yazarlı ama bedeli ve karmaşıklığı fazla; ikisi de
`Remote` arayüzüyle sonradan eklenebilir.

## Algoritma (`sync.go`)

Her dosya için son eşitlemedeki durum saklanır (yerel karma, uzak karma). Şimdikiyle karşılaştırılır:

| Yerel | Uzak | Sonuç |
|---|---|---|
| değişmedi | değişmedi | hiçbir şey |
| değişti | değişmedi | **yukarı** (silindiyse uzakta mezar taşı) |
| değişmedi | değişti | **aşağı** (uzakta silindiyse burada silinir) |
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

## Seçmeli ve yedek

Koleksiyon başına eşitle aç/kapa (kapalıysa hiçbir şeyi yukarı çıkmaz, aşağı inmez). Her eşitlemeden önce kitaplığın yedeği alınır (`backup.go`).

## Yapılmadı (dürüstçe)

- **Cihaz yönetimi** (liste, uzaktan çıkarma, QR eşleştirme): jetonlar elle Worker sırrına yazılır.
- **Sunucuda sürüm geçmişi, günlük anlık görüntü, paylaşım, Wi-Fi sınırı, ağ hızı ayarı, ek boyut sınırı.**
- **Gerçek zamanlı eşitleme**: aralıklı (öntanımlı 5 dk) ve elle; değişiklik anında gitmez.
- **Metin birleştirme** (3 yollu): çakışma kopyası var, otomatik birleştirme yok.
- **Canlı Cloudflare denemesi**: Worker yazıldı, protokol istemci tarafı sahte bir sunucuyla test edildi; Worker'ın kendisi dağıtılıp denenmedi.
- Eşitleme yalnız tek dosyalı (SQLite) kitaplıkta çalışır.
