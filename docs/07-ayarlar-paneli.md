# 07 — Ayarlar paneli

## İki yoldan açılır [görüldü]
1. **Komut paleti** (⌘P) → **Settings →**: paletin içinde gruplu bir alt menü açar (← Back ile geri). Gruplar: kullanıcı
   (e-posta • Admin), **Workspace (almasada)**, **Account Admin**.
2. Bu listedeki bir maddeyi seçince **Settings penceresi** açılır: solda arama kutusu ve gruplu menü, sağda seçilen sayfa,
   sağ üstte kapatma.

## Pencerenin yapısı [görüldü]
```
┌ Settings ────────────────────────────────────────────────────────── ✕ ┐
│ [🔍 Search settings…]  │  Başlık                                       │
│                        │  Açıklama                                     │
│ Preferences            │                                               │
│   Display Name         │  …ayarlar…                                    │
│   Profile Picture      │                                               │
│   Password & Email     │                                               │
│   Recovery Key         │                                               │
│   Date & Time          │                                               │
│   App Icon             │                                               │
│   Theme                │                                               │
│   MCP (AI Agents)      │                                               │
│   Markdown Mirror      │                                               │
│   Keyboard Shortcuts   │                                               │
│   Check for Updates    │                                               │
│ Workspace              │                                               │
│   Invite Users, Manage Users, Workspace Name, Theme CSS,               │
│   Default Collection, Export, Plugins                                  │
│ Account                │                                  [Reset][Save]│
│   Account Name/URL, Create Workspace, Close Account                    │
└────────────────────────────────────────────────────────────────────────┘
```
Her sayfanın kendi **Save** düğmesi var (ör. "Save Theme", "Reset"); bir sayfadaki değişiklik o sayfada kaydedilir.

## Önemli sayfalar [görüldü]

### Theme
- **Appearance**: Auto / Light / Dark (Auto sistemin görünümünü izler).
- **Light Themes** ve **Dark Themes** ayrı ızgaralar: açık temalardan biri ve koyu temalardan biri ayrı ayrı seçilir; görünüm
  hangisini istiyorsa o gösterilir. Örnek açık temalar: Light Cold, Light Warm Sans, Light Paper, Light Sans, Leather, Folder,
  Desk, Binder, Notebook, Herbarium, Premium Light; koyular: Dark Default, Dark Sans, Dark Paper, Tokyo Techno, Basalt
  Bedrock, Early Ember…
- **Reset** / **Save Theme**.
- Workspace altında ayrıca **Theme CSS** (yöneticiler için, temaya ek CSS).

### MCP (AI Agents)
- Açıklama: "Thymer'i Claude Code, Gemini CLI gibi AI asistanlarına MCP ile bağla."
- **Kurulum adımları**: 1) thymer.com/mcp'den Thymer CLI yardımcı uygulamasını indir, 2) çalıştır, 3) **Connect**.
- Bağlantı durumu ("Not connected") ve **Connect** düğmesi.
- **Workspace Access**: her çalışma alanı için ayrı bir seçici — **Off / Read only / Read & write** (kodda `off|readonly|readwrite`).

### Markdown Mirror
- "Çalışma alanındaki her sayfayı bilgisayarında bir klasörde Markdown dosyasına çevirir, **koleksiyon başına bir klasör**.
  Herhangi bir araçta aç, betikle işle ya da bir AI agent'a üzerinde çalıştır. **Değişiklikler birkaç saniye içinde iki
  yönlü** geçer."
- Bilgisayarda bir klasöre erişmesi gerektiği için yalnız masaüstü uygulamasında çalışır.

### Diğerleri [kod]
Date & Time (tarih/saat biçimleri, haftanın ilk günü), App Icon, Keyboard Shortcuts (kısayolları değiştirme), Default
Collection (⌃N'nin gideceği koleksiyon), Export (çalışma alanını zip olarak dışarı ver), Plugins.

## Bizim editör için [öneri]

**Adım adım:**
1. Aynı iskelet: bir ayarlar penceresi, solda **aranabilir** gruplu menü, sağda sayfa. Ayarlar ayrıca komut paletinden tek
   tek de çağrılabilsin (her ayar sayfası bir komut).
2. İlk sürümün sayfaları: **Görünüm/Tema**, **Editör** (caret: Slime / Mizan, yazı tipi, satır aralığı), **Kısayollar**,
   **MCP**, **Dosyalar** (kayıt klasörü, Markdown yansıtma), **Varsayılan koleksiyon**, **Dışa aktar**.
3. **Tema** sayfası Thymer'deki gibi: Görünüm (Otomatik/Açık/Koyu) + açık tema seçimi + koyu tema seçimi. Temalar veri
   dosyası ([00](00-OKU-BENI.md)'daki karar); "Tema klasörünü aç" ve "Temayı yeniden yükle" düğmeleri olsun.
4. **MCP** sayfası: aç/kapat, bağlantı adresi ve token'ı kopyala, bağlı agent'lar, ve **koleksiyon/çalışma alanı başına
   erişim: Kapalı / Salt okunur / Okuma-yazma**. Thymer'den farklı olarak ayrı bir yardımcı uygulama gerekmesin: MCP sunucusu
   uygulamanın içinde.
5. **Markdown yansıtma**'yı biz ayar değil, temel özellik yapalım ([02](02-veri-modeli.md)): sayfalar zaten diskte Markdown
   olarak durur; ayar yalnız klasörü seçer.
6. Her sayfada açık **Kaydet/Sıfırla** ya da anında uygulama: ikisinden birini seçip her yerde aynı kullan. (Öneri: tema ve
   caret anında uygulansın — denemek kolay olsun — geri kalanlar Kaydet ile.)

## Durum çubuğu (Kasım 2026)

Ayarlar ▸ Durum çubuğu: kaydetme durumu, caret adı, kelime ve harf sayısı; ek olarak (hepsi kapalı başlar) okuma süresi
(dakikada 200 kelime), sayfa yolu, seçili harf sayısı, satır ve sütun (caret'in paragrafı ve içindeki yeri), son düzenleme,
geri bağlantı sayısı (tıklayınca yan panel açılır). MCP açıksa nabız atan nokta da burada.
