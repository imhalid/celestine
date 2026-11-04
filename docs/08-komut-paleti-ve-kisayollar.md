# 08 — Komut paleti ve kısayollar

## Üç palet [görüldü + kod]
| Kısayol | Ne | Not |
|---|---|---|
| **⌘P** Commands | Komutlar. Bağlama göre listeyi değiştirir: bir satırdayken önce satır komutları gelir | `>` ile yalnız komutlar |
| **⌘K** Jump To | Sayfa, koleksiyon, satır arasında atlama | `@` ile "jump to…" |
| **⌘⇧F** Search | Bütün çalışma alanında arama | sorgu dili [09](09-arama-gorevler-gunluk.md) |

Palet içi: ↕ gezin, ↵ seç, ⌘↵ yanda aç. Bazı maddeler alt menü açar (ör. **Settings →**, **Views →**) ve **← Back** ile dönülür.

## Önemli kısayollar [görüldü]
| Kısayol | İş |
|---|---|
| ⌃N | Yeni sayfa (varsayılan koleksiyon) |
| ⇧↵ | Koleksiyonda yeni sayfa |
| ⌘L | Görev satırı |
| ⌘/ | Katla |
| ⌥↓ | Satıra yakınlaş (zoom in) |
| ⌘↵ | Yanda aç |
| ⌃Space | Satır seçenekleri menüsü |
| ⇥ / ⇧⇥ | Girinti / geri girinti |
| ⌘; / ⌘⇧; | Ağacı girintile / geri al |
| ⌘⇧↑ / ⌘⇧↓ | Satırı yukarı / aşağı taşı |
| ⌘B, ⌘I, ⌘E | Kalın, italik, satır içi kod |
| ⌘⌥V | Markdown olarak yapıştır |
| @ | Kişi, tarih, sayfa ekle ("@ Insert…") |
| F2 | Başlık ve ikon |
| ⌦ | Çöpe at |

Ayrıca: Go to Today's Journal, Go to next/previous journal day, Navigate Back/Forward, Toggle Sidebar, Focus Mode,
Split panel, Help: Keyboard Shortcuts, Help: Markdown Mirror. Kısayollar ayarlardan değiştirilebilir.

## Bizim editör için [öneri]

**Adım adım:**
1. **Tek komut kaydı**: her komut `{ID, Ad, Kısayol, Bağlam, Çalıştır}`. Menüler, palet, kısayollar ve MCP aynı kayıttan okur.
   ```go
   type Command struct {
       ID, Label string
       Keys      ui.Shortcut        // değiştirilebilir
       When      func(Ctx) bool     // bağlam: satırda mı, koleksiyonda mı…
       Run       func(Ctx)
   }
   ```
2. **Palet** = bulanık arama + bağlama göre sıralama + alt menüler. mizan-go'daki Combobox ve popover dili hazır.
3. **⌘K** sayfa atlama: başlık indeksi üzerinde bulanık arama; son açılanlar üstte.
4. **Kısayol ayarı**: komut kaydından otomatik bir liste; çakışmaları göster.
5. Durum çubuğu, o bağlamda en sık kullanılan 3–4 kısayolu göstersin (Thymer'deki gibi).
