# 13 — İstekler ve düzeltmeler (4 Kasım 2026)

Uygulamayı kullanırken çıkanlar. Hepsi yapılacak; burada yalnız kayıt altında. Thymer ekran görüntüleri
örnek olarak verildi: aynısını değil, aynı rahatlığı istiyoruz. Her maddenin sonunda hangi aşamaya
([11](11-yol-haritasi.md)) girdiği yazıyor.

## Hatalar ve küçük düzeltmeler

1. **Caret satır başında kesiliyor.** Caret (Slime) satırın en başındayken sol yarısı editörün kenarında kırpılıyor.
   Caret editörün dışına taşabilmeli; kırpma sınırı editörün kutusu değil, sayfa sütununun biraz dışı olmalı. *(hemen)*
2. **Ana ekranın arka planı fazla siyah.** Koyu temada içerik alanı saf siyah (`#000`); kenar çubuğuyla arasındaki
   fark sert. Daha yumuşak, hafif gri bir koyu zemin olmalı (Ryugo'nun koyu teması bg'yi siyah veriyor; biz kendi
   değerimizi verelim, sonra temadan gelsin). *(hemen, sonra E)*
3. **Koleksiyon listesinde yol gösterilmesin.** Liste görünümünde alt sayfalar "Ajandan merhaba › test notum ›
   **test altı not**" diye yazılıyor; yalnız sayfanın kendi adı yazmalı. (Nerede olduğu istenirse görünüm ayarından
   bir sütun/özellik olarak açılır: "Sub-page of".) *(hemen)*
4. **Üstteki yol (breadcrumb) kalksın.** Başlık çubuğundaki "MCP Deneme › Ajandan merhaba › test notum › test altı
   not" yazısı gereksiz; kaldırılacak. *(hemen)*
5. **Kenar çubuğu düğmesi sabit dursun.** Kenar çubuğunu açıp kapatan düğme her zaman sol üstte, üç trafik
   lambasının hemen yanında olmalı; kenar çubuğu açıkken içeriğin başlık çubuğuna kaymamalı. *(hemen)*

## Özellikler

6. **Tarih özelliği takvimle seçilsin.** Tarih türünde değer bir takvimden (tarih seçici, Ryugo'da varsa onun
   bileşeni) seçilir; elle yazmak da mümkün kalır. *(A)*
7. **Sayı özelliği sayı alanı olsun.** Sayı türünde artı/eksi düğmeli bir sayı girişi (Ryugo NumberInput gibi);
   harf girilemez. *(A)*
8. **Özellikler koleksiyonlar arasında paylaşılsın.** Bir koleksiyona özellik eklerken daha önce başka
   koleksiyonlarda tanımlanmış özellikler otomatik tamamlamayla listelenir; biri seçilince adı, türü ve seçenekleri
   gelir (aynı özellik iki koleksiyonda aynı anlamı taşır, sorgular ikisini birlikte arayabilir). *(A)*
9. **Yerleşik özellikler görünsün.** Thymer'deki gibi: Başlık, Oluşturulma, Değiştirilme, Üst sayfa (Sub-page of),
   Koleksiyon, Banner. Bunlar silinmez ama sıralanır ve görünüm ayarlarında gösterilip gizlenir. *(A)*

## Koleksiyonlar

10. **Koleksiyon ayarları sayfası.** Thymer'in "Docs / Settings" paneli gibi, sekmeli:
    - **Genel:** ad, **ikon**, **renk**, açıklama, "bu koleksiyondaki bir sayfaya ne denir" (ör. Tarif), çöpe taşı.
    - **Özellikler:** özelliklerin listesi (tutamakla sürüklenip sıralanır, kalemle düzenlenir), **+ Özellik ekle**
      (otomatik tamamlamalı, madde 8), "Gelişmiş gösterim seçenekleri".
    - **Görünümler:** görünüm listesi, **+ Görünüm ekle** (liste, tablo, pano; sonra galeri, takvim).
    - **Sayfalar:** alt sayfalar açık mı, kenar çubuğunda görünsün mü, kenar çubuğunda sıralama.
    - **Kod olarak düzenle:** `collection.json`'ın kendisi.
    - Altta **Vazgeç / Önizle / Kaydet**: kaydedilene kadar uygulanmaz.

    Özellik ekleme sayfadaki "+ Özellik ekle"den buraya taşınır (sayfada kısayol olarak kalabilir). *(A)*
11. **Koleksiyon ikonu ve rengi.** Kenar çubuğunda, koleksiyon sayfasında ve kapsülde görünür. İkon bir listeden
    (ya da emoji) seçilir, renk Ryugo'nun renk paletinden. *(A)*
12. **Sadece sorgu olan koleksiyon (dinamik koleksiyon).** Bir koleksiyonun kendi sayfası olmak zorunda değil: bir
    sorguyla bütün koleksiyonlardaki sayfaları toplar (ör. "Durum = Sürüyor olan her şey", "etiketi okuma olanlar").
    Görünümleri (liste, tablo, pano) normal bir koleksiyon gibi çalışır; özellikleri kaynak koleksiyonlardan gelir.
    Thymer'de "dynamic collection" ([06](06-koleksiyonlar-ve-gorunumler.md)). Sorgu dili [09](09-arama-gorevler-gunluk.md). *(A'dan sonra, arama ile)*

## Görünümler

13. **Görünümler özelleştirilebilsin.** Her görünümde neyin, nasıl gösterileceği seçilir:
    - **Liste:** hangi özellikler satırın altında/yanında görünsün, önizleme metni olsun mu, gruplama (tarih,
      bir seçim özelliği, yok), sıralama.
    - **Tablo:** sütunlar (göster/gizle, sırala, genişlik sürüklenerek), sıralama, filtre.
    - **Pano:** hangi seçim özelliğine göre gruplanacağı, kartta hangi özelliklerin görüneceği, kart boyutu.
    *(A)*
14. **Pano sütunları daraltılabilsin.** Bir sütunun başlığına tıklayınca dar bir şeride iner (adı dikey, sayısı
    görünür); tekrar tıklayınca açılır. Daraltılmış hâli görünümde hatırlanır. *(A)*

## Sayfa

15. **Banner.** Sayfanın üstüne bir görsel (banner) eklenebilir: sayfa menüsünden "Banner değiştir…", sayfanın
    klasörüne kopyalanır, ön maddede yolu tutulur. *(A/B arası, medya ile)*
16. **Sayfa menüsü, Thymer'deki gibi.** Kenar çubuğunda sağ tık ve sayfadaki "…" aynı menüyü açar; menünün en
    üstünde **arama kutusu** ("'SSH Bağlantı Rehberi' seçeneklerinde ara…"), her öğenin ikonu ve kısayolu var:
    Aç, **Yanda aç**, Başlık ve ikon (F2), Sürüm geçmişi…, Banner değiştir…, Taşı…, Çöpe taşı, Alt sayfa oluştur,
    Sabitle. *(D ile)*
17. **Yan panel (Open Aside).** Bir sayfa ana sayfanın yanında ikinci bir panelde açılabilir (⌘↵ ya da "Yanda aç");
    iki sayfa yan yana düzenlenir. *(D'den sonra; ileride çoklu pencere)*

## Editör menüleri

18. **`/` ve `@` menüsü.** Boş ya da herhangi bir yerde `/` (ve `@`) yazınca caret'in altında bir öneri menüsü
    açılır, yazdıkça süzülür, klavyeyle seçilir. Thymer'deki düzen:
    - **Öneriler:** Görev (`*`), Başlık 1 (`#`), Başlık 2 (`##`), Madde listesi (`-`) — sağda Markdown kısayolu yazar.
    - **Satırı biçimlendir…** (başlıklar, listeler, alıntılar) →
    - **Ekle…** (bloklar, sorgular, görseller) →
    - **Kişi an** →, **Tarih ya da saat** →, **Görev durumu** →, **Bağlantı…** (sayfalar, koleksiyonlar, satırlar) →
    - `→` olanlar alt menü açar. Menünün başlığında "bu menü `@` ile de açılır" gibi bir ipucu.
    *(B)*
19. **`/` ve `@` ile özellikler.** Aynı menüden sayfanın özellikleri de listelenir ve değer verilir (ör. `@Durum`,
    `/Yazar`); bir satıra tarih ya da sayfa bağlantısı da buradan eklenir. *(B, A ile)*
20. **`*` ile görev menüsü.** Satır başında `*` yazınca (ya da menüden Görev) görev olur; görev durumları
    (yapıldı, başladı, beklemede, önemli…) bir açılır menüden seçilir ([02](02-veri-modeli.md) görev durumları). *(B)*

## Ayarlar ve durum çubuğu

21. **Ayarlar ekranı genişlesin.** Şu an yalnız MCP var. Eklenecek bölümler:
    - **Görünüm:** tema (açık / koyu / sistem; sonra tema dosyaları), vurgu rengi, yazı tipi ve boyutu.
    - **Editör:** caret seçimi (Slime, Mizan; önizlemeli), caret rengi, satır aralığı.
    - **Kısayollar:** liste ve değiştirme.
    - **MCP:** şimdiki bölüm.
    *(D/E)*
22. **Durum çubuğu MCP'yi göstersin.** MCP açıkken durum çubuğunda küçük bir gösterge (nokta + "MCP"); tıklayınca
    ayarlar açılır; bir ajan yazarken kısa bir etkinlik işareti. *(hemen)*
23. **Durum çubuğu özelleştirilebilsin.** Sağ tıkla hangi öğelerin görüneceği seçilir: yazma hızı (Mizan caret'inin
    ölçtüğü), kelime sayısı, harf sayısı, satır/sütun, kaydetme durumu, caret adı, kullanılabilecek kısayollar
    (bağlama göre: editördeyken biçim kısayolları, tabloda sıralama…), MCP. *(D)*

---

## Önerilen sıra

1. **Hemen (küçükler):** 1, 2, 3, 4, 5, 22.
2. **Aşama A'yı tamamla:** 10 (koleksiyon ayarları) ve içine 8, 9, 11, 13, 14; sonra 6, 7.
3. **Aşama B (bloklar)** ile birlikte 18, 19, 20.
4. **Ayarlar ve durum çubuğu:** 21, 23.
5. **Sonra:** 12 (dinamik koleksiyon, aramayla), 15 (banner, medyayla), 16–17 (sayfa menüsü, yan panel).
