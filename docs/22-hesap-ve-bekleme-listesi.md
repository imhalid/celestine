# 22 — Hesap, bekleme listesi ve davet kodu: ne gerekiyor

**İstek (2026-11-07):** Kullanıcılar bekleme listesine yazılır; ben onaylarım; onaylanana `RYU-A7YJK4` gibi bir davet kodu gider;
uygulamayı açıp **kullanıcı adı + şifre + davet kodu** girer ve hesap oluşur. Senkron (`20-senkron.md`) bu hesaba bağlanır.

Bugün: hesap yok, yalnız elle yazılan cihaz jetonları (`TOKENS` Worker sırrı). Bu belge onların yerini alacak sistemin ne olduğunu,
parçalarını ve sırasını yazar. İşaretli (**?**) yerler karar bekler.

## Akış

```
site (bekleme formu) ──► Worker /waitlist ──► D1: waitlist(pending)
                                                   │  (yönetici bildirimi: e-posta ya da yalnız yönetim sayfası)
yönetici sayfası /admin ── Onayla ──► davet kodu üretilir (RYU-XXXXXX) ──► e-posta (ya da elle iletilir)
uygulama ► Hesap aç: kullanıcı adı + şifre + kod ──► Worker /register ──► D1: users + devices ──► cihaz jetonu
uygulama ► (başka cihaz) Giriş: kullanıcı adı + şifre ──► /login ──► yeni cihaz jetonu
uygulama ► senkron /files (jeton → hesap → R2 ön eki)
```

## Parçalar

### 1. Sunucu (Worker + D1 + R2)
Tek Worker (`ryudit-sync`) büyür; R2 dosyalar için kalır, **D1** hesap verisi için gelir (`cf d1 create`).

| Tablo | Alanlar |
|---|---|
| `waitlist` | id, e-posta (tekil, küçük harf), durum (`bekliyor/onaylı/reddedildi`), tarih, not, IP karması |
| `invites` | kod **karması** (kodun kendisi saklanmaz), waitlist_id, son kullanma, kullanıldı mı / kim |
| `users` | id, kullanıcı adı (tekil, büyük/küçük harf duyarsız), şifre karması + tuz, kota, durum, tarih |
| `devices` | jeton **karması**, user_id, ad ("MacBook"), son görülme, iptal edildi mi |

Uç noktalar: `POST /waitlist`, `POST /register`, `POST /login`, `GET /me`, `GET/DELETE /devices`, `POST /password`, `DELETE /account`,
`/admin/*` (yönetici), `/files…` (var; jetonu D1'den çözer).

### 2. Davet kodu
- Biçim `RYU-` + 6 karakter, alfabe `ABCDEFGHJKMNPQRSTUVWXYZ23456789` (0/O, 1/I/L, ayrımı zor harfler yok) ≈ 30 bit.
- Yalnız karması saklanır, **tek kullanımlık**, **14 gün** geçerli, onaylanan e-postaya bağlı.
- 30 bit tek başına küçük: deneme sayısı sınırlanır (IP başına saatte 10, genel sınır; üst üste yanlışta bekletme). Yanlış kod ile süresi dolmuş kod
  aynı hatayı verir.

### 3. Bekleme listesi sayfası
Tek sayfa, e-posta alanı + (**?** ad). Cloudflare **Turnstile** (bot), Workers **rate limiting**, e-posta doğrulama (biçim, uzunluk), aynı e-posta
ikinci kez yazılırsa "zaten listede" (kaç kişi olduğunu sızdırmadan). KVKK/GDPR: ne için toplandığı, silme isteği, saklama süresi.
Yayın: aynı Worker'ın statik varlıkları ya da ayrı adres (**?** alan adı).

### 4. Yönetici (onay)
`/admin`: bekleyenler, **Onayla** (kod üretir ve gönderir), **Reddet**, kodu yeniden gönder/iptal et, hesaplar (kota, askıya al, sil).
Koruma: `ADMIN_TOKEN` sırrı ya da Cloudflare Access (**?**). Yeni kayıtta sana bildirim (e-posta) isteğe bağlı.

### 5. E-posta gönderimi
Cloudflare **Email Service**: alan adı bağlanır (SPF/DKIM). **Rastgele alıcıya göndermek Workers Paid (≈5 $/ay) ister**; ücretsizde yalnız
doğrulanmış adreslere gidebilir. İlk sürümde alternatif: kod yönetici sayfasında görünür, sen elle iletirsin; e-posta sonra eklenir (**?**).

### 6. Kimlik doğrulama ve şifre
- Sunucuda şifre **PBKDF2-SHA256** (Workers'ta WebCrypto en çok 100 bin tur) + kullanıcıya özel tuz; karşılaştırma sabit zamanlı; giriş denemesi sınırlı.
- Cihaz jetonu rastgele 192 bit, sunucuda karması; çıkışta/iptalde silinir.
- **Uçtan uca şifreleme ile ilişki (önemli, ?):** şifre sunucuya düz giderse yönetici (ben) şifreleme anahtarını da türetebilir. İki yol:
  1. **Sıfır bilgi:** uygulama şifreden iki ayrı değer türetir: biri *giriş sırrı* (sunucuya gider), biri *şifreleme anahtarı* (hiç çıkmaz).
     Sunucu veriyi okuyamaz. Bedeli: şifre unutulursa veri **kurtarılamaz** (kurtarma anahtarı var), şifre sıfırlama e-postası veriyi geri getirmez.
  2. **Klasik:** şifreleme ayrı parola; hesap şifresi sıfırlanabilir, ama veri parolasız açılmaz.

### 7. Uygulama
- İlk açılışta (ya da Ayarlar ▸ Hesap) **Hesap aç** (kullanıcı adı, şifre, davet kodu) ve **Giriş yap**; "Bekleme listesine katıl" bağlantısı.
- Başarıda `SyncMode=server`, adres ve jeton otomatik; şifreleme anahtarı 6'ya göre.
- Ayarlar ▸ Hesap: cihazlar (liste, çıkar), çıkış, şifre değiştir, hesabı sil (sunucudaki veri dahil), kota göstergesi.
- Hesapsız kullanım **çalışmaya devam eder** (yerel, klasör senkronu): hesap yalnız bulut senkronu için.
- Hata metinleri Türkçe/İngilizce (i18n var).

### 8. Güvenlik ve işletme
- Kota (ör. hesap başına 200 MB; `PUT`'ta denetlenir), dosya başına 50 MiB sınırı var.
- Yönetici işlemleri kayıt altına alınır (kim, ne zaman); kod ve şifre günlüğe yazılmaz.
- D1 yedeği (`cf d1 export`), R2'de sürüm geçmişi yok (sunucuda geçmiş eksikti).
- Mevcut jetonlar: `halit` hesabı D1'e taşınır; `TOKENS` sırrı kaldırılır.

### 9. Testler
Worker için kayıt/giriş/davet akışının canlı testi (`sync_worker_test.go` kalıbında): kod bir kez çalışır, süresi dolan reddedilir, yanlış kod sınırlanır,
iptal edilen cihaz 401, kota aşımı 413, hesap silince dosyalar gider.

## Sıra
1. Karar (aşağıdaki **?**'ler) → 2. D1 şeması + `/register` `/login` + jeton çözümü (`/files` yeni yoldan) + Go istemci + uygulama ekranı
(kod elle üretilir) → 3. bekleme listesi sayfası + `/admin` → 4. e-posta → 5. cihaz yönetimi, şifre değiştirme, hesap silme, kota → 6. `TOKENS`'ı kaldırma.
