# 21 — Android (M): deneme sürümü ve rapor

**Soru:** Android uygulaması yapılabilir mi? **Yapılabilir; en ucuz yol (web sürümünü bir WebView'a sarmak) kuruldu ve APK derlendi. Emülatörde çalıştırılamadı.**

## Yapılan

- `android/`: küçük bir Gradle projesi (Kotlin, minSdk 26, compileSdk 36). `MainActivity` bir WebView açar; `assets/web/` içindeki web sürümünü
  (`index.html`, wasm, `wasm_exec.js`, PWA dosyaları) `https://appassets.ryudit.local/` kökeninden sunar (`shouldInterceptRequest`: wasm doğru MIME türüyle
  yüklenir, sayfanın IndexedDB için bir kökeni olur). "Paylaş" menüsünden gelen metin adrese `#share=` olarak eklenir. Geri tuşu uygulamanın kendi geçmişine
  (⌘[) gider.
- `scripts/android.sh`: web sürümünü derler, `android/app/src/main/assets/web`'e kopyalar, Gradle'ı (uygun JDK 17/21 seçilerek) çalıştırır; `install` ile
  adb'ye yükleyip başlatır. **Sonuç: `app-debug.apk` derlendi, 34,7 MB; `aapt2 dump badging` paket ve başlatıcı etkinliği doğruluyor, wasm APK'nın içinde.**
- Veri tarayıcının IndexedDB'sinde (WebView'ın kendi deposu) durur; masaüstüyle eşitlemek için senkron (L) kullanılır (`httpRemote` aynı Go kodu).

## Denenemeyen (dürüstçe)

- **APK'nın çalışması.** Bu makinedeki Android emülatörünün QEMU'su (`qemu-system-x86_64`, `Ocare_API_36` AVD'si, KVM açık) başlangıçta **SIGSEGV** ile
  düşüyor: üç denemede, pencereli ve pencresiz, `swiftshader_indirect` GPU'suyla (coredumpctl'de kayıtlı). Bir dördüncü denemede `-dns-server 8.8.8.8` ile (çökmenin kaydında `libnss_*` modülleri var, bir DNS çözümlemesi şüphesiyle) yine düştü. `-no-accel` ile çökmüyor ama o kipte WebGL 2
  çalışmaz. Bu, uygulamayla ilgili bir hata değil; emülatör sürümü ile bu sistem (Fedora, çekirdek 7.2) uyumsuz. **Gerçek bir telefonda ya da çalışan bir
  emülatörde `scripts/android.sh install` ile denenmeli.** WebView'da WebGL 2, yazılım klavyesi girişi ve dokunmatik sürükleme beklenen riskler.
- Dokunmatik uyarlama (kısayol çubuğu, çekmece kenar çubuğu, uzun basma menüleri), sesle yazma, kamera, bildirimden hızlı not, pil dostu kuyruk: yapılmadı.
- Yerel (Kotlin/Compose) uygulama (yol 2) ve MyGo'yu Android'e taşımak (yol 3): yapılmadı; web sarmalayıcı yetersiz kalırsa.

## 7 Kasım, öğlen: emülatörde çalıştı

- Emülatör `-gpu host` ile açıldı (önceki çökme `swiftshader_indirect` ile oluyordu; emülatör 37.1.11). `scripts/android.sh install`: APK kuruldu ve açıldı.
- Denenen: açılış, kenar çubuğu ve sayfa, dokunarak imleç koyma, **ekran klavyesiyle yazma** (Gboard; “APKYAZI” sayfaya geçti), “Kaydedildi”,
  uygulamayı zorla kapatıp açınca yazı yerinde (WebView'ın IndexedDB'si).
- **Bulunan ve düzeltilen hata:** Android 15+ uygulamayı kenardan kenara çizdiği için sayfa durum çubuğunun ve gezinme çubuğunun altına giriyordu
  (saat kenar çubuğu düğmelerinin üstündeydi). `MainActivity` sistem çubuklarının, kesik ekranın ve **ekran klavyesinin** boşluğunu WebView'ı tutan
  görünümün dolgusu yapıyor: sayfa durum çubuğunun altında başlar, klavye açıkken onun üstünde biter.
- APK temiz derlemede **35,8 MB**; artımlı hata ayıklama derlemesi eski wasm'ların yerini boş bırakıp 69 MB'a çıkabiliyor (`./gradlew clean`).
- Hâlâ yok: dokunmatik uyarlama (dar ekranda kenar çubuğu yer kaplıyor), gerçek telefon denemesi.

## Yollar (araştırma)

| Yol | İş | Artı | Eksi |
|---|---|---|---|
| **1. Web sürümü + WebView** (yapıldı, çalıştırılmadı) | dokunmatik uyarlama ≈1 hafta | tek kod tabanı; senkronla masaüstüyle eşit | WebGL 2 performansı ve ekran klavyesi ölçülmeli |
| **2. Go çekirdeği + Kotlin/Compose** | `gomobile bind` 3 gün + zengin metin düzenleyiciyi yeniden yazmak 4–8 hafta | yerel his, bildirim/widget/paylaşım menüsü | büyük iş |
| **3. MyGo'yu Android'e taşımak** | MyGo'ya katkı | tek arayüz kodu | MyGo'nun sahibine bağlı |

**Öneri:** 1'i gerçek cihazda dene; ölçüm kötüyse 2'ye geç.
