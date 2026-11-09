# 19 — Otomatik güncelleme (K): ne var, ne denenmedi

## Çalışan akış (`update.go`, `updateui.go`, testlerle)

1. Sürüm derlemeye gömülür: `-X main.version=v1.2.3` (`scripts/build.sh`, release iş akışı). `dev` derlemesi kendini eski saymaz.
2. Açılışta ve `UpdateHours` (öntanımlı 24) aralığıyla, ayar açıksa, **manifest** sorgulanır: `GET <adres>`, yalnız `User-Agent: ryudit/<sürüm>`;
   başka hiçbir şey gönderilmez (test). Yalnız HTTPS.
3. Manifest (`Release`): sürüm, tarih, notlar (Markdown), kanal (`beta`), her `os-arch` için `url`, `sha256`, `sig`, `size`, `kind`.
4. Yeni sürüm varsa iletişim kutusu: notlar, **Şimdi güncelle**, **3 gün sonra hatırlat**, **Bu sürümü atla**. Ayarlar ▸ Güncelleme: denetimi kapat
   (çevrimdışı mod), sessiz mod (yalnız ayarlarda göster), deneme sürümleri, kendiliğinden kur, sıklık. ⌘P: "Yeni sürümü denetle", "Bu sürümde neler yeni".
5. Güncelle: önce kitaplığın yedeği (`backup.go`), sonra indir; **boyut, sha256 ve ed25519 imzası** doğrulanır (imza `sürüm + "\n" + sha256` üstüne,
   yani başka sürüme ya da dosyaya taşınamaz); açık anahtar programın içinde (`-X main.updateKey=`), gizli anahtar yalnız CI'da. İmzasız, hatalı
   ya da başka anahtarla imzalı dosya **kurulmaz** (testler).
6. Yerine koyma: Linux/Windows'ta çalışan dosya `ryudit.old` olarak kenara alınır (çalışan program yeniden adlandırılabilir), yenisi yerine geçer;
   macOS'ta `.app` zip'i açılıp paket değiştirilir (`../` ile kaçan zip reddedilir, test). Sonra "Yeniden başlat".
7. **Geri dönüş:** `update-pending` işareti yazılır. Yeni sürüm açılışta düşerse (önceki oturum "beklenmedik kapandı", `crash.go`), sonraki başlangıçta
   eski dosya yerine konur, düşen `.failed` olarak saklanır ve eski sürüm başlatılır. 20 sn sağlıklı çalışırsa işaret silinir, eski dosya bir gün sonra.
8. Güncellemeden sonra ilk açılışta "Bu sürümde neler yeni" bir kez çıkar. Kitaplık verisi hiç değişmez.

## Cloudflare tarafı ve yayın

- `cloudflare/update/worker.js` + `wrangler.toml`: R2 kovasından `update.json` ve `<sürüm>/<dosya>` sunar; `GET /download/<os-arch>` sitenin indirme
  düğmeleri için son sürümün dosyasına yönlendirir (sync Worker'ı adresi `DOWNLOAD_URL`'den okur). Hiçbir şey kaydetmez.
- `scripts/release`: `keygen` (anahtar çifti), `manifest` (dosyaları karma ve imzala, `update.json` yaz).
- `.github/workflows/release.yml`: `v*` etiketi → test, beş derleme (gömülü sürüm ve anahtarla), manifest, R2'ye yükleme, GitHub release.
  Gereken sırlar dosyanın başında yazılı. **Açık:** sırlar (`RELEASE_KEY`, `UPDATE_PUBLIC_KEY`, `UPDATE_URL`, R2 anahtarları; `DOWNLOAD_HOST` değişkeni)
  ayarlı değil: sürümler imzasız ve R2'ye bir şey gitmiyor, uygulamanın güncelleme denetimi bir şey bulamaz.
- **Açık:** macOS ve Linux derlemeleri çıplak ikili dosya (macOS'ta `.app` yok, çalıştırma izni yok); yukarıdaki macOS `.app` değişimi
  (`kind: app-zip`) yayında kullanılmıyor.

Neden Cloudflare: depo özel; belirteç uygulamaya gömülemez; R2 ve Worker'la indirme sayacı ve hız elde, belirteç gerekmez.

## Denenmedi / yapılmadı (dürüstçe)

- **Gerçek bir güncelleme** üç işletim sisteminde, canlı sunucuyla. Mantık birim testlerle (sahte sunucu, geçici klasörler) doğrulandı; macOS ve Windows'ta
  gerçek `.app`/`.exe` değişimi bu makinede denenemedi.
- **macOS notarization ve Windows imzası**: gerekmiyor çünkü uygulamanın kendi indirdiği dosyada karantina işareti olmaz; ama ilk kurulum (kullanıcının
  elle indirdiği) için Apple Developer hesabı ve Authenticode gerekir: **açık soru 8, hâlâ cevapsız**.
- **Fark güncellemesi (bsdiff)**: yapılmadı; docs/17'de nedeni.
- **Kurumsal mod**: denetimi kapatmak (`UpdateOff`) yeter; ağ ilkesi dosyası yok.
