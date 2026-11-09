# 22 — Acil sorunlar (kullanıcı raporu, 7 Kasım 2026): kalanlar

Bildirilen 20 maddenin hepsi düzeltildi ve çoğu testli (istatistik donması, palet kayması, yazı tipi önizlemesi, kendi kaydırma çubuğu, köşe 0,
Ryugo bileşenleri, dört bölgeli pencere, bölmeler ve ayrı pencere, bul çubuğu, `@` arama indeksi, açılışta font, çıkışta asılı kalma…).
Açık kalan uçlar:

- **İstatistik ekranı (1):** gerçek büyük bir kitaplıkta açılış süresi ölçülmedi.
- **Kaydırma çubuğu (4, 5):** koleksiyonun tablo görünümü (Ryugo'nun `Table`'ı) hâlâ MyGo'nun çubuğunu kullanıyor: kaydırma durumu Ryugo'nun
  içinde ve dışarı açık değil; değiştirmek Ryugo'ya bir seçenek eklemeyi gerektirir.
- **Mizan, köşe 0 (6):** düzeltme gözle (ekran görüntüsüyle) doğrulanmadı.
- **Ayrı pencere (11):** Wayland pencerelerin yerini söylemediği için orada sekmeyi fareyle koparmak çalışmaz (komutu var: "Bölmeyi ayrı pencereye
  al"); web'de yok.
