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
- **Limits and devices:** tries are counted per address first, and per name or code alone only at a far higher cap, which
  nobody can use up to keep the owner out (#158): a name's sign-ins 10 a quarter of an hour from one address and 100 from
  all; a recovery 5 an hour from one address and 50 from all; an invite's codes 10 an hour from one address, 20 wrong ones
  an hour for one invited address and 2000 from all. A device's token holds 90 days unseen, and an account keeps 20
  devices, the least seen put out by a new sign-in (#161).
- E-posta **Resend** ile (`Celestine <davet@resend.halid.dev>`, `MAIL_FROM`), adres `celestine.halid.dev`, yönetici `ADMIN_TOKEN` sırrı. Kod `/admin`'de de bir kez görünür.
- Sırlar depoda değil; dağıtım `cloudflare/sync/deploy.sh` (`CELESTINE_SYNC_SECRETS=… ./deploy.sh` sırlar değişince, `./deploy.sh` yalnız kod).

## Uygulamada

Hesap formu Ayarlar ▸ Senkron'da: davetle hesap aç (e-posta + erişim kodu → şifre → kurtarma anahtarı bir kez gösterilir) ya da giriş yap;
"No code: join the waitlist" `<sunucu>/#waitlist`'i açar. Profil (⌘P ▸ "Hesap profili"): kota, eşitleme, kurtarma anahtarı, şifre, cihazlar, hesabı sil, çıkış.
Site (`cloudflare/sync/landing`) önce indirme, bekleme listesi altta.
`/privacy` says what the server keeps and how to have it deleted. Deleting an account also deletes its invite and waitlist row and
puts `account#<id>` for its address in the audit; /admin exports the waitlist as CSV and deletes a row (one without an account).
The admin panel (`/admin`) is drawn with synth-ui on a canvas (`cloudflare/sync/admin`, built by `npm run build` into
`admin.gen.js` and `admin.bin`, served by the Worker with its WebAssembly; the page's CSP allows WebAssembly and no
other site). It needs WebGPU or WebGL2: a browser without them (or with the GPU turned off) shows nothing.

## Açık

- Worker 9 Kasım'da sürüm 15 olarak canlıda (8 Kasım değişiklikleri, değişiklik sayacı ve `0003_changes.sql` dahil; #46). Uygulama içinden
  canlı sunucuya karşı uçtan uca deneme ve Resend ile gerçek bir gönderim henüz yapılmadı (#59).
- DNS'te `resend._domainkey.halid.dev` altında ikinci, boş bir `v=DKIM1; p=` kaydı var: zararsız, silinebilir.
