# 19 — Otomatik güncelleme (K): ne var, ne denenmedi

## Çalışan akış (`update.go`, `updateui.go`, testlerle)

1. Sürüm derlemeye gömülür: `-X main.version=v1.2.3` (`scripts/build.sh`, release iş akışı). `dev` derlemesi kendini eski saymaz.
2. Açılışta ve `UpdateHours` (öntanımlı 24) aralığıyla, ayar açıksa, **manifest** sorgulanır: `GET <adres>`, yalnız `User-Agent: celestine/<sürüm>`;
   başka hiçbir şey gönderilmez (test). Yalnız HTTPS.
3. Manifest (`Release`): sürüm, tarih, notlar (Markdown), kanal (`beta`), her `os-arch` için `url`, `sha256`, `sig`, `size`, `kind`.
4. Yeni sürüm varsa iletişim kutusu: notlar, **Şimdi güncelle**, **3 gün sonra hatırlat**, **Bu sürümü atla**. Ayarlar ▸ Güncelleme: denetimi kapat
   (çevrimdışı mod), sessiz mod (yalnız ayarlarda göster), deneme sürümleri, kendiliğinden kur, sıklık. ⌘P: "Yeni sürümü denetle", "Bu sürümde neler yeni".
5. Güncelle: önce kitaplığın yedeği (`backup.go`), sonra indir; **boyut, sha256 ve ed25519 imzası** doğrulanır (imza `sürüm + "\n" + sha256` üstüne,
   yani başka sürüme ya da dosyaya taşınamaz); açık anahtar programın içinde (`-X main.updateKey=`), gizli anahtar yalnız CI'da. İmzasız, hatalı
   ya da başka anahtarla imzalı dosya **kurulmaz** (testler).
6. Putting it in place: the file is downloaded into a folder of this user alone beside what it replaces (`updateDir`), so the swap is a
   rename on one filesystem. On Linux and Windows the running file is moved aside as `celestine.old` and the new one takes its name; on
   macOS the `.app` zip is unpacked beside the bundle and the two are swapped (a zip escaping with `../` is refused, tested). Then
   "Restart": the new start gets `--restarted=<pid>` and waits for the old copy to be gone before it takes the one-instance lock (`restart.go`).
7. **Rollback:** an `update-pending` mark is written. The new version's first start stamps it (`started`) right after the lock, before
   anything can fail. A start that finds the stamp and no proof follows one that died: the old file is put back, the failed one kept
   as `.failed`, and the old version started. Twenty seconds of running, or a clean quit, is the proof: the mark and the old file go.
8. Güncellemeden sonra ilk açılışta "Bu sürümde neler yeni" bir kez çıkar. Kitaplık verisi hiç değişmez.

## The Cloudflare side and the release

- `cloudflare/update/worker.js` + `wrangler.toml`: the Worker `celestine-updates` at **https://dl.celestine.halid.dev**, over the R2 bucket
  `celestine-releases`. It serves `update.json` and `<version>/<file>`; `GET /download/<os-arch>` sends a browser to the latest release's
  file (the site's buttons). It records nothing. The address is in every release: it does not change.
- `scripts/release`: `keygen` (the key pair), `manifest` (hashes and signs the files, writes `update.json`).
- `.github/workflows/release.yml`: a `v*` tag → the gate, `scripts/build.sh` (address and key as `LDFLAGS`, budget 30 MB), packaging,
  `SHA256SUMS`, the signed manifest, the GitHub release, then R2 with `update.json` last.

### Setting it up (once)

1. Keys, kept out of the repository; `release.key` also goes into the password manager: lost, no installed app takes another update.
   `go run ./scripts/release keygen` in a private folder writes `release.key` and prints the public key.
2. GitHub secrets: `RELEASE_KEY` (the content of `release.key`), `UPDATE_PUBLIC_KEY` (the printed key),
   `UPDATE_URL` = `https://dl.celestine.halid.dev/update.json`.
3. Cloudflare: `wrangler r2 bucket create celestine-releases`, then `wrangler deploy` in `cloudflare/update/` (the custom domain comes with it).
4. An R2 API token (Object Read & Write on `celestine-releases`) in the dashboard; GitHub secrets `R2_ACCESS_KEY_ID`, `R2_SECRET_ACCESS_KEY`,
   `CLOUDFLARE_ACCOUNT_ID`.
5. A tag. Copies built before this (v0.0.1-v0.0.4) have no address in them: they are updated by hand once.

- **Open:** macOS builds are not signed with a Developer ID nor notarised. An update the app downloads itself carries no quarantine,
  but the first, hand-downloaded copy is "from an unidentified developer": right-click ▸ Open the first time (README).

Neden Cloudflare: depo özel; belirteç uygulamaya gömülemez; R2 ve Worker'la indirme sayacı ve hız elde, belirteç gerekmez.

## Denenmedi / yapılmadı (dürüstçe)

- **Gerçek bir güncelleme** üç işletim sisteminde, canlı sunucuyla. Mantık birim testlerle (sahte sunucu, geçici klasörler) doğrulandı; macOS ve Windows'ta
  gerçek `.app`/`.exe` değişimi bu makinede denenemedi.
- **macOS notarization ve Windows imzası**: gerekmiyor çünkü uygulamanın kendi indirdiği dosyada karantina işareti olmaz; ama ilk kurulum (kullanıcının
  elle indirdiği) için Apple Developer hesabı ve Authenticode gerekir: **açık soru 8, hâlâ cevapsız**.
- **Fark güncellemesi (bsdiff)**: yapılmadı; docs/17'de nedeni.
- **Kurumsal mod**: denetimi kapatmak (`UpdateOff`) yeter; ağ ilkesi dosyası yok.
