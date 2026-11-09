# 21 — Android (M)

Yol 1 kuruldu: web sürümü bir WebView'da. Emülatörde (`-gpu host` ile; `swiftshader_indirect` ile emülatör çöküyor) ve gerçek telefonda
(Samsung Galaxy S21) çalıştı: açılış, sayfa, ekran klavyesiyle yazma, kaydetme, kapatıp açınca yazı yerinde.

## Yapılan

- `android/`: küçük bir Gradle projesi (Kotlin, minSdk 26, compileSdk 36). `MainActivity` bir WebView açar; `assets/web/` içindeki web sürümünü
  `https://appassets.ryudit.local/` kökeninden sunar (`shouldInterceptRequest`: wasm doğru MIME türüyle yüklenir, sayfanın IndexedDB için bir kökeni olur).
  "Paylaş" menüsünden gelen metin adrese `#share=` olarak eklenir. Geri tuşu uygulamanın kendi geçmişine (⌘[) gider. Sistem çubuklarının, kesik
  ekranın ve ekran klavyesinin boşluğu WebView'ı tutan görünümün dolgusu (Android 15+ kenardan kenara çizer).
- `scripts/android.sh`: web sürümünü derler, `android/app/src/main/assets/web`'e kopyalar, Gradle'ı (JDK 17/21) çalıştırır; `install` ile adb'ye yükleyip başlatır.
- Veri WebView'ın IndexedDB'sinde; masaüstüyle eşitlemek için senkron (L).
- APK temiz derlemede ≈31 MB; artımlı hata ayıklama derlemesi eski wasm'lar yüzünden 69 MB'a çıkabiliyor (`./gradlew clean`). wasm APK içinde
  sıkıştırılırsa 11,4 MB olur; açılışa etkisi ölçülmeden açılmadı (docs/17).

## Telefona göre tasarım

**Dokunma** (`web/touch.js`, web sürümü ve uygulama ortak): MyGo'nun tuvali parmağı fare sayıyordu, parmakla çekmek metin seçiyor ya da satır
sürüklüyordu; tarayıcının kendi kaydırması kapalı (`touch-action: none`). Betik dokunuşları MyGo'dan önce (pencerede, yakalama evresinde) alır:

| hareket | ne olur |
|---|---|
| parmakla çekmek (9 px'ten fazla) | kaydırma, iki yönde; bırakınca süzülerek durur |
| kısa dokunuş | tıklama (iki dokunuş art arda çift tık) |
| uzun basış (0,48 sn) | sağ tık: satırın, metnin menüsü (kısa bir titreşimle) |
| ikinci parmak | yok sayılır (sayfa yakınlaşmasın) |

Ekran klavyesi: MyGo'nun gizli `textarea`'sı metin alanı odaklanana kadar `inputmode=none`; bir dokunuştan sonra metin istediğinde odak
dokunuşun sayıldığı süre içinde yeniden verilir, klavye açılır. Sayfa yakınlaşması kapalı (`user-scalable=no`). Betik `touchstart`'ı **iptal etmez**: iptal, tarayıcının "tap" hareketini de
iptal eder ve Chrome klavyeyi o harekette gösterir (S21'de klavye bu yüzden açılmıyordu); kaymayı ve yakınlaşmayı tuvalin `touch-action: none`'ı
engeller. Hata ayıklama derlemesinde sayfanın konsolu logcat'te (`ryudit` etiketi).

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

**Denendi (emülatör ve S21):** parmakla kaydırma, ☰ ile çekmece, çekmeceden ekran açınca kapanması, uzun basışla satır menüsü, metne
dokununca klavye ve yazma. APK: `android/app/build/outputs/apk/debug/app-debug.apk`.

**Kalan:** kenardan kaydırarak çekmece açma, çimdikle yazı büyütme, metin seçme tutamakları (uzun basış şimdilik menü açıyor), sürükle-bırak
(dokunmada yok), pano kartını parmakla taşıma; sesle yazma, kamera, bildirimden hızlı not; açılış süresi telefonda ölçülmedi.

## Yollar

| Yol | İş | Artı | Eksi |
|---|---|---|---|
| **1. Web sürümü + WebView** (yapıldı) | | tek kod tabanı; senkronla masaüstüyle eşit | WebGL 2 performansı ve ekran klavyesi ölçülmeli |
| **2. Go çekirdeği + Kotlin/Compose** | `gomobile bind` 3 gün + zengin metin düzenleyiciyi yeniden yazmak 4–8 hafta | yerel his, bildirim/widget/paylaşım menüsü | büyük iş |
| **3. MyGo'yu Android'e taşımak** | MyGo'ya katkı | tek arayüz kodu | MyGo'nun sahibine bağlı |

2 ve 3 yapılmadı; web sarmalayıcı yetersiz kalırsa.
