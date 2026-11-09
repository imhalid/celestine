# 12 — MyGo ekosistemi: işimize yarayanlar

Kaynak: [awesome-mygo](https://github.com/agnogad/awesome-mygo) (250 proje) ve MyGo v0.7.4'ün kendi paketleri.
İncelendi: 2026-11-04.

## Karar (2026-11-04)

Çekirdek MyGo'nun `ui/richtext`'i oldu. Caret'in yeri, paragrafı richtext'in yaptığı gibi yeniden dizerek bulunuyor (`caret.go`)
ve `caret_test.go` bunu her karakterde richtext'in kendi caret'iyle karşılaştırıyor. richtext v0.7.4'te bulunan bir hata
(paragraf sonunda kapanan kalın, alttaki paragrafa `[] ` dönüşümünden sonra sızıyor) uygulamada önleniyor (`main.go`, `track`).
MyGo'nun bir çatalı `go.mod`'da `replace` ile kullanılıyor (`richtext.EditorOptions.Menu` kancası için).

## Hâlâ alınacaklar

| Proje | Ne | Neden ilgili | Lisans |
|---|---|---|---|
| [sorrycc/Deckle](https://github.com/sorrycc/Deckle) | MyGo ile yerel Markdown editörü | Performans teknikleri: Fenwick ağacıyla yükseklik tahmini (100.000 satır akıcı), bölüm bölüm artımlı ayrıştırma; matematik/Mermaid'i bir resim olarak satıra yerleştirme; baskı/PDF. Ölçülmüş performans belgesi var: 300 KB'lık notta tuş başına 0,74 ms | MIT |
| `ui/editor` (MyGo içinde) | Kod editörü | `LanguageFor`, 32 dil için hazır sözdizimi renklendirme: kod bloklarımız için (bugün renklendirme yok) | MIT |
| [auron-labs/mygo-mcp](https://github.com/auron-labs/mygo-mcp) | MyGo görünümlerini bir MCP istemcisine açan geliştirme aracı | Agent uygulamanın arayüzünü pencere açmadan görebilir, tıklayabilir, yazabilir, ekran görüntüsü alabilir. Geliştirirken test için | **lisanssız** — kod alınamaz, yalnız fikir |
