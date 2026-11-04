# 01 — Thymer'e genel bakış

Thymer bir not ve görev uygulaması: her şey tek bir ağaçta durur (bkz. [02](02-veri-modeli.md)), notlar koleksiyonlarda
toplanır, sayfalar yan yana panellerde açılır. Masaüstü uygulaması, web uygulamasının (almasada.thymer.com) ince bir
Electron kabuğudur.

## Ekranın parçaları [görüldü]

```
┌──────────────┬──────────────────────────────────────┬──────────────────────────┐
│ KENAR ÇUBUĞU │ PANEL 1 (odaktaki)                   │ PANEL 2 (yanda açılan)   │
│              │  ← →            ⌘1  🖼  🔒  ◫  ✕       │  ← →  Clips / Settings   │
│ çalışma alanı│  [Koleksiyon kapsülü]                │                          │
│ Search  Jump │  Büyük başlık  …                     │                          │
│ Tasks   Home │  özellik kapsülleri  +  ⚙            │                          │
│ Pinned       │  içerik (satırlar)                   │                          │
│ Collections  │                                      │                          │
│ Journal      │                                      │                          │
│ Tags / Trash │                                      │                          │
├──────────────┴──────────────────────────────────────┴──────────────────────────┤
│ DURUM ÇUBUĞU: ⌘P Commands  ⌘K Jump To  ⌃N New page…  ⌘⇧F Search…  … Ln 1, Col 1 │
└────────────────────────────────────────────────────────────────────────────────┘
```

### Kenar çubuğu
Yukarıdan aşağı:
1. **Profil kartı** (avatar ya da görsel) ve **çalışma alanı seçici** (`almasada ⌃⌄`).
2. **Kısa yollar**: Quick Add (⚡), Search, Jump to, Tasks, Home.
3. **Pinned**: sabitlenmiş sayfalar ve koleksiyonlar.
4. **Collections**: her koleksiyon bir satır; altında sayfaları (koleksiyonun ayarına göre, bkz. [03](03-kenar-cubugu-ve-alt-sayfalar.md)).
   Başlığın yanında `…` (Yeni koleksiyon / Yeni dinamik koleksiyon) ve `+` (Yeni koleksiyon).
5. **Journal**: Bugün, Dün, Önceki 7 gün, Önceki 30 gün diye gruplanmış günlük sayfaları.
6. **Tags** ve **Trash**.

Her öğe sürüklenebilir (`nav-draggable`), koleksiyonlara renk verilebilir (`color-green`, `color-cyan`…).

### Paneller
- Sayfalar paneller içinde açılır. Bir panelin üst çubuğunda: geri/ileri, panel numarası (`⌘1`), banner, kilit,
  **bölme** (◫, yanına yeni panel açar) ve kapatma.
- **Open Aside** (`⌘↵`) bir şeyi yandaki panelde açar. Koleksiyon ayarları da yandaki panelde açılır.
- Panel komutları: Split panel, Close panel, Close all panels, Go to Next/Previous Panel, Swap with next panel, Focus Mode.

### Durum çubuğu
Bağlama göre değişir:
- Bir sayfada: `⌘P Commands`, `⌘K Jump To`, `⌃N New page…`, `⌘⇧F Search…`, `@ Insert…`, `Aa` (biçim), sağda `Ln 1, Col 1`,
  kullanıcılar, eşitleme durumu.
- Komut paleti açıkken: `↕ Navigate`, `↵ Select`, `⌘↵ Open Aside, if applicable`.
- Bir koleksiyonda: öğe sayısı (`1 items`).

## Bizim editör için [öneri]
- Bu üç bölgeli düzen (kenar çubuğu + bölünebilir paneller + bağlama duyarlı durum çubuğu) olduğu gibi alınabilir.
  mizan-go'daki **Dock** çalışması (alan bölme, sekmeler, ayrı pencere) panellerin daha güçlü bir hâli.
- Durum çubuğunun bağlama göre kısayol göstermesi çok iyi bir öğretici: yeni kullanıcı kısayolları oradan öğrenir.
