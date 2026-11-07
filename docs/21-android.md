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

## 7 Kasım, akşam: gerçek telefonda

- **Samsung Galaxy S21 (SM-G991B)**, USB ile: APK kuruldu ve açıldı; sayfa, üst çubuk ve durum çubuğu doğru yerde (sistem çubukları dolgu olarak),
  hata kaydı yok. Telefonun ekranı sonradan kilitlendiği için dokunarak yazma ve açılış süresi telefonda ölçülmedi (emülatörde yazma denenmişti).
- APK **31,0 MB** (MCP web derlemesinden çıktı). wasm APK içinde sıkıştırılırsa 11,4 MB olur; açılışa etkisi ölçülmeden açılmadı (docs/17).

## 7 Kasım, gece: telefona göre tasarım

Kullanıcı telefonda denedi: hiç kaydırılamıyordu ve çoğu yer sıkışıktı. İki ayrı sorun, ikisi de çözüldü.

**Dokunma** (`web/touch.js`, web sürümü ve uygulama ortak): MyGo'nun tuvali parmağı fare sayıyordu, parmakla çekmek metin seçiyor ya da satır
sürüklüyordu; tarayıcının kendi kaydırması kapalı (`touch-action: none`). Betik dokunuşları MyGo'dan önce (pencerede, yakalama evresinde) alır:

| hareket | ne olur |
|---|---|
| parmakla çekmek (9 px'ten fazla) | kaydırma, iki yönde; bırakınca süzülerek durur |
| kısa dokunuş | tıklama (iki dokunuş art arda çift tık) |
| uzun basış (0,48 sn) | sağ tık: satırın, metnin menüsü (kısa bir titreşimle) |
| ikinci parmak | yok sayılır (sayfa yakınlaşmasın) |

Ekran klavyesi: MyGo'nun gizli `textarea`'sı metin alanı odaklanana kadar `inputmode=none`; bir dokunuştan sonra metin istediğinde odak
dokunuşun sayıldığı süre içinde yeniden verilir, klavye açılır. Sayfa yakınlaşması kapalı (`user-scalable=no`). **Gerçek telefonda (S21, Samsung
klavyesi) klavye açılmıyordu:** betik `touchstart`'ı iptal ediyordu; bu, tarayıcının o dokunuştan ürettiği "tap" hareketini de iptal eder, Chrome
ise klavyeyi odaktaki metin için o harekette gösterir. Dokunma olaylarının kendisine artık dokunulmuyor (sayfanın kaymasını ve yakınlaşmasını
tuvalin `touch-action: none`'ı zaten engelliyor). Telefonda denendi: dokununca klavye açıldı (`dumpsys input_method`: `mInputShown=true`), yazı
geldi, ☰'e dokununca klavye açılmadı, kaydırma çalışıyor. Hata ayıklama derlemesinde sayfanın konsolu logcat'te (`ryudit` etiketi) ve sayfa
bilgisayardan incelenebilir.

**Telefon düzeni** (`mobile.go`, tarayıcıda 760 DIP'ten dar pencere):
- **Kenar çubuğu bir çekmece:** üst çubuktaki ☰ ile soldan kayarak gelir, arkası kararır; kararan yere, ✕'e dokunmak ya da bir sayfa/ekran açmak
  kapatır. Çekmecenin başında uygulamanın adı.
- **Üst çubuk** 50 pt: ☰, geri, başlık, sayfa arama, komutlar (⋯). Pencere düğmeleri, yan panel düğmesi yok.
- **Tek bölme:** bölünmüş görünüm, ayrı pencere, yüzen yan panel yok; "yan bölmede aç" burada açar, menülerde görünmez, komutları soluk ("Telefonda yok").
- **Dokunma boyutu:** ağaç satırları 42 pt, simge düğmeleri en az 38 pt, ipucu balonları yok; blok tutamağı ve oluk (fareyle çıkan) çizilmez;
  durum çubuğu kısayol listelemez.
- **Ekranlar:** kenar boşlukları 16 pt, ekranların üstü 12 pt. Ayarlar: bölümler üstte yatay kayan çipler, her ayarın denetimi adının altında,
  seçim kutucukları satıra sarılır, arama kendi satırında. Tur kartı ekran genişliğinde.
- Masaüstü ve geniş tarayıcı penceresi değişmedi. `TestPhoneLayout`.

**Denendi (emülatör, gerçek WebView):** parmakla kaydırma, ☰ ile çekmece, çekmeceden ekran açınca kapanması, uzun basışla satır menüsü, metne
dokununca klavye ve yazma. Gerçek telefonda (S21) da denendi (aşağıda klavye). APK: `android/app/build/outputs/apk/debug/app-debug.apk`.

**Kalan:** kenardan kaydırarak çekmece açma, çimdikle yazı büyütme, metin seçme tutamakları (uzun basış şimdilik menü açıyor), sürükle-bırak
(dokunmada yok), pano kartını parmakla taşıma.

## Yollar (araştırma)

| Yol | İş | Artı | Eksi |
|---|---|---|---|
| **1. Web sürümü + WebView** (yapıldı, çalıştırılmadı) | dokunmatik uyarlama ≈1 hafta | tek kod tabanı; senkronla masaüstüyle eşit | WebGL 2 performansı ve ekran klavyesi ölçülmeli |
| **2. Go çekirdeği + Kotlin/Compose** | `gomobile bind` 3 gün + zengin metin düzenleyiciyi yeniden yazmak 4–8 hafta | yerel his, bildirim/widget/paylaşım menüsü | büyük iş |
| **3. MyGo'yu Android'e taşımak** | MyGo'ya katkı | tek arayüz kodu | MyGo'nun sahibine bağlı |

**Öneri:** 1'i gerçek cihazda dene; ölçüm kötüyse 2'ye geç.
