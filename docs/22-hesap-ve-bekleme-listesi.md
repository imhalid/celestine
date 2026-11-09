# 22 — Hesap, bekleme listesi ve davet kodu

Bekleme listesi → yönetici onayı → davet kodu e-postayla → uygulamada hesap. Hesap yalnız sunucu senkronu (`20-senkron.md`) içindir;
uygulama hesapsız açılır ve çalışır.

## Kararlar (geçerli)

- **Sıfır bilgi:** uygulama şifreden iki ayrı değer türetir: sunucuya giden giriş sırrı ve hiç çıkmayan şifreleme anahtarı. Sunucu veriyi okuyamaz;
  şifre ve kurtarma anahtarı birlikte kaybolursa veri kurtarılamaz, e-postayla sıfırlama veriyi geri getirmez.
- **Kurtarma:** sunucu yalnız kurtarma anahtarından türetilen `recoveryAuth`'un karmasını saklar (`users.recovery_hash`); "Şifremi unuttum" veri
  anahtarını aynı bırakır, yeni şifreyle yeniden mühürler, bütün cihazları çıkarır. Eski hesaplar karmayı bir sonraki girişte kaydeder; o zamana kadar sıfırlanamaz.
- **Hesabın adı e-posta** ve anahtar türetmenin tuzu da o: e-posta değiştirmek anahtarı yeniden mühürlemeyi gerektirir. Eski kullanıcı adlı hesaplar
  aynı alandan kullanıcı adıyla girer.
- **Davet kodu:** `RYU-` + 6 karakter (ayrımı zor harfler yok, ≈30 bit), yalnız karması saklanır, tek kullanımlık, 14 gün, onaylanan e-postaya bağlı;
  deneme sınırlı, yanlış adres / yanlış kod / süresi dolmuş aynı cevabı verir.
- E-posta **Resend** ile (`Ryudit <davet@resend.halid.dev>`, `MAIL_FROM`), adres `ryudit.halid.dev`, yönetici `ADMIN_TOKEN` sırrı. Kod `/admin`'de de bir kez görünür.
- Sırlar depoda değil; dağıtım `cloudflare/sync/deploy.sh` (`RYUDIT_SYNC_SECRETS=… ./deploy.sh` sırlar değişince, `./deploy.sh` yalnız kod).

## Uygulamada

Hesap formu Ayarlar ▸ Senkron'da: davetle hesap aç (e-posta + erişim kodu → şifre → kurtarma anahtarı bir kez gösterilir) ya da giriş yap;
"No code: join the waitlist" `<sunucu>/#waitlist`'i açar. Profil (⌘P ▸ "Hesap profili"): kota, eşitleme, kurtarma anahtarı, şifre, cihazlar, hesabı sil, çıkış.
Site (`cloudflare/sync/landing`) önce indirme, bekleme listesi altta.

## Açık

- 8 Kasım değişiklikleri (şifre sıfırlama, `0002_recovery.sql`, e-postalı davet akışı, sunucu sürümleri) yerelde `wrangler dev` ile denendi;
  canlıya gidip gitmediği doğrulanmalı (gitmediyse Sunucudaki sürümler 404 verir).
- Uygulama içinden canlı sunucuya karşı uçtan uca deneme yapılmadı.
- Bekleme listesi dışa aktarma yok.
- Gizlilik metni sayfada tek cümle; ayrı sayfa ve silme/dışa aktarma süreci yazılmadı.
- DNS'te `resend._domainkey.halid.dev` altında ikinci, boş bir `v=DKIM1; p=` kaydı var: zararsız, silinebilir.
