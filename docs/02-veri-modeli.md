# 02 — Veri modeli

Thymer'in veri modelinden, bizde henüz olmayanlara yol gösteren parçalar.

### Satır tipleri [kod]
Bizde olmayanlar: `ref` (başka kayda bağlantı satırı), `transclusion` (başka yerden **satır** gömme; bizde yalnız sayfa
gömme var), `query` (canlı sorgu, bkz. [09](09-arama-gorevler-gunluk.md)), `file` (resim dışı dosya eki), `ascii-banner`.

### Metin segmentleri (zengin metin) [kod]
`mention` (kullanıcı), `datetime` ("tomorrow 3pm", "monday to friday", "Q1 2024" gibi doğal dilde tarih), `icon`. Bizde
tarih yalnız gün çipi (`@` menüsünden ya da `2026-11-10`); doğal dil ve saat yok.

### Özellik tipleri [görüldü + kod]
Arayüzdeki "Add property" menüsü: **Choice, Number, Text, File, Image, Date & Time, URL, Page Link, User, Tag**.
Kod tarafında bunlara ek olarak `banner` ve `dynamic` (formülle hesaplanan) var. Bizde yalnız seçim, metin, sayı, tarih ve
bağlantı (URL) var; Page Link, Tag, File/Image ve formül yok.

## Kitaplık dosyası

Dosya şifreli değil (senkronun uçtan uca şifrelemesi ayrı). Parola istenirse sonraki iş (SQLCipher ya da alan şifreleme);
şimdilik yapılmayacak.
