# 19 — Otomatik güncelleme (K): ne var, ne denenmedi

## Çalışan akış (`update.go`, `updateui.go`, testlerle)

1. Sürüm derlemeye gömülür: `-X main.version=v1.2.3` (`scripts/build.sh`, release iş akışı). `dev` derlemesi kendini eski saymaz.
2. Açılışta ve `UpdateHours` (öntanımlı 24) aralığıyla, ayar açıksa, **manifest** sorgulanır: `GET <adres>`, yalnız `User-Agent: celestine/<sürüm>`;
   başka hiçbir şey gönderilmez (test). Yalnız HTTPS.
3. Manifest (`Release`): the version, date, notes (Markdown), channel (`beta`), and for each `os-arch` the file's `url`, `sha256`,
   `sig`, `sig2`, `size` and `kind`.
4. Yeni sürüm varsa iletişim kutusu: notlar, **Şimdi güncelle**, **3 gün sonra hatırlat**, **Bu sürümü atla**. Ayarlar ▸ Güncelleme: denetimi kapat
   (çevrimdışı mod), sessiz mod (yalnız ayarlarda göster), deneme sürümleri, kendiliğinden kur, sıklık. ⌘P: "Yeni sürümü denetle", "Bu sürümde neler yeni".
5. Update: first a backup of the library (`backup.go`); then the **signature** is checked, before anything is downloaded; then the file is
   downloaded and its **size and sha256** checked. The public key is in the program (`-X main.updateKey=`), the secret key only in the CI. A
   file unsigned, wrongly signed or signed by another key is **not installed** (tests).
   - `sig2` (#455) is what this app requires: the ed25519 signature of the lines `celestine-update-2`, the version, the `os-arch` the asset
     is listed under, the file's name (the last part of its `url`), its `kind` (`binary` when none) and its sha256 (`sigText`). A signature
     cannot be moved to another version, file, system, name or kind: a darwin-arm64 build, rightly signed, served as linux-amd64's update
     is refused before it is downloaded, not installed to fail and be rolled back.
   - `sig` is the old signature, of `version + "\n" + sha256` alone (`legacySigText`): all that copies released before #455 check, so
     `scripts/release` keeps writing it beside `sig2`. This app does not accept it alone: a manifest stripped of `sig2` would open the hole
     again, and every release made since carries `sig2` (an older one is never newer than this app, so it is never offered). Both texts
     are held to one literal in the app's test and in `scripts/release`'s.
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
- `scripts/release`: `keygen` (the key pair), `manifest` (hashes the files, signs each with `sig` and `sig2`, writes `update.json`).
- `.github/workflows/release.yml`: a `v*` tag → the gate, `scripts/build.sh` (address and key as `LDFLAGS`, budget 30 MB), packaging,
  `SHA256SUMS`, the signed manifest, the GitHub release, then R2 with `update.json` last.

### Setting it up (once)

1. Keys, kept out of the repository; `release.key` also goes into the password manager: lost, no installed app takes another update.
   `go run ./scripts/release keygen` in a private folder writes `release.key` and prints the public key.
2. GitHub secrets: `RELEASE_KEY` (the content of `release.key`), `UPDATE_PUBLIC_KEY` (the printed key),
   `UPDATE_URL` = `https://dl.celestine.halid.dev/update.json`.
3. Cloudflare: `cf r2 buckets create --name celestine-releases`, then `cloudflare/update/deploy.sh` (`cf deploy --prebuilt`; the custom domain comes with it). Done on 2026-11-11.
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
