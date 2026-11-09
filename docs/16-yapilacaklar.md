# 16 — Yapılacaklar (6–7 Kasım listesinden kalanlar)

6 Kasım'daki listenin (A–O, N1–N6) yapılanları silindi; burada yalnız yapılmayanlar ve hâlâ geçerli kararlar var.
Web (J), güncelleme (K), senkron (L) ve Android (M) için kalanlar kendi belgelerinde: [18](18-web.md), [19](19-guncelleme.md),
[20](20-senkron.md), [21](21-android.md). Boyut (I) ve performans (N6): [17](17-boyut-ve-performans.md).

## Kalanlar

**D — Editör ayarları**
- Harf ve kelime aralığı: imleç konumu düzenleyicinin düzeniyle hizalı kalmaz, bu yüzden yapılmadı.
- Yazım denetimi (işletim sistemi sözlüğü, dil seçimi, kelime ekleme): MyGo'dan işletim sistemi sözlüğüne yol yok.
- Sağdan sola yazı (Arapça/İbranice).
- İmleç yanıp sönme hızı.
- Boşlukların yalnız seçimde gösterilmesi.

**F — Yazı tipi**
- Seçilen fontta Türkçe harf (ı İ ş ğ ç ö ü) eksikse uyarı; italik kesimi yoksa sahte italik uyarısı. İkisi de MyGo font
  dosyasını okutmadığı için yapılmadı.
- Optik boyut.

**G — Klavye**
- Ayarlar ekranındaki kontrollerin (kaydırıcı, seçenek) ←→ ile değiştirilmesi.
- VoiceOver / erişilebilirlik etiketlerinin denetimi.
- Web'de harf tuşlarıyla klavye komutlarının doğrulanması.

**K — Güncelleme**
- Release'ler imzasız çıkıyor: GitHub secrets (`RELEASE_KEY`, `UPDATE_PUBLIC_KEY`, `UPDATE_URL`, R2 anahtarları) tanımlı değil.
  Bu yüzden programda manifest adresi ve açık anahtar yok, R2'ye manifest gitmiyor: kendiliğinden güncelleme fiilen çalışmıyor.
- Üç işletim sisteminde gerçek bir güncelleme hiç denenmedi.

**N4 — Dil**
- Menüler dil değişince ancak yeniden başlatmada değişir.
- `T()` ile sarılmamış metinler Türkçe kalabilir; test bunları yakalamaz.

**N6 — Performans**
- Bir tuş vuruşu bütçeyi aşıyor (`TestPerfBudget`, tam test setiyle koşunca başarısız); gerçek GPU'da ölçüm yok. Ayrıntı [17](17-boyut-ve-performans.md).

## Kararlar (hâlâ geçerli)

- **Vim:** tam Vim kipi yapılmayacak; her aksiyonun kendi kısayolu var (Ayarlar ▸ Kısayollar), Vim hareketleri isteğe bağlı.
- **Güncelleme ve dağıtım:** uygulama siteden herkese indirilir (güncelleme sunucusunun `/download`'ı, depo gizli kalır); Apple
  notarization ve Windows imzası henüz kararlaştırılmadı ([19](19-guncelleme.md)).
- **Senkron:** klasör ve sunucu ikisi de var; şifreleme şart (parola olmadan eşitleme başlamaz). QR eşleştirme istenmedi.
- **Kısayollar:** ⌘\ kenar çubuğu, ⌘, ayarlar; çakışma görülürse değişecek.
