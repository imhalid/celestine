# 14 — Ürün önerileri ve uyarılar (4 Kasım listesinden kalanlar)

Benzer uygulamalardan (Thymer, Tana, Capacities, Obsidian, Notion, Bear, Craft) önerilerin ve uyarıların yapılanları silindi;
numaralar eski listedeki gibi kaldı. Çaba: **K** küçük, **O** orta, **B** büyük. İsteklerin kalanı [13](13-istekler.md)'te.

## Uyarılar

U1–U3, U7, U8 yapıldı; U4'ün (blok editör richtext'in sınırına çarpar) cevabı [15](15-bloklar-denemesi.md), U5 aşağıda "Yapmayalım"da.

### U6 — Ajanın yaptığını geri almak
Sürüm geçmişi var (yalnız tek dosya kitaplığında; MCP `list_versions` / `restore_version`), ama bir ajan yazmasının ardından
"Ajan 3 sayfayı değiştirdi — geri al" gibi bir yol ve koleksiyon bazında izin (aşağıda 29, 30) yok. *(O)*

### U10 — Bağımlılıklar
`ryu-go` ve MyGo (`imhalid/mygo` çatalı, `replace` ile) `main` dalından sözde sürümlere bağlı; MyGo 1.0 öncesi, API değişebilir.
**Öneri:** ikisine etiket at ve ona bağlan; MyGo güncellemelerini tek tek, testlerle al. *(K)*

## Öneriler

4. **Bağlantı yapıştırınca kart.** URL yapıştırınca başlığı ve sitesi çekilir (isteğe bağlı, ağ gerekir; ayarla kapatılabilir). *(O)*
8. **Canlı sorgu bloğu.** Dinamik koleksiyon var; aynı sorgu sayfa içinde blok olarak yok. *(B)*
12. **Takvimde sürükleyerek tarih değiştirme.** Takvim görünümü var, sürükleme yok. *(O)*
13. **Galeri görünümü.** Kartlar kapak ya da sayfanın ilk görseliyle (kitaplar, tarifler, projeler). *(O)*
14. **Filtreler ve özet satırı.** Her görünümde kalıcı, koşullu filtre ("Durum ≠ Bitti"); tabloda alt satırda sayı/toplam/ortalama. *(O)*
15. **Gruplama her görünümde.** Listede ve tabloda bir seçim özelliğine göre, katlanabilir gruplar. *(K)*
19. **Kod blokları:** dil korunuyor, ama renkli gösterim ve kopyala düğmesi yok. *(O)*
26. **Sayfa sabitleme.** Koleksiyonlar sabitlenebiliyor, tek tek sayfalar değil. *(K)*

### Ajanlar (MCP)
29. **Ajan etkinliği görünür olsun.** Ajan yazınca pencere sallanıyor (`wobble.go`); değiştirilen satırların kısa süre parlaması ve
    "Ajan geçmişi" listesi (ne zaman, hangi sayfa, ne yaptı, geri al) yok. *(O)*
30. **Koleksiyon izinleri.** Her koleksiyon için: kapalı / yalnız okuma / yazma. *(K)*
31. **Daha akıllı araçlar.** Kalanlar: bir başlığın altına ekleme (`update_page` yalnız değiştirir ya da sona ekler), özelliklere göre
    sorgu, `get_daily_note`, MCP "resources" ve "prompts". *(O)*
32. **Uygulama kapalıyken de MCP.** `ryudit mcp` komutu (stdio): pencere açık değilken ajanlar yine yazabilsin; açılınca uygulama
    dışarıdan değişikliği görür. *(K)*

### Dışarıyla
34. **Dışa aktar:** sayfa Markdown olarak var; HTML, PDF (yazdır), koleksiyonu tek dosya olarak yok. *(O)*

## Yapmayalım (en azından şimdi)
- **Uygulamanın içine yapay zekâ.** MCP bunu dışarıdaki ajanlara bırakıyor; içeride bir AI özelliği hem bakım hem gizlilik yükü.
- **Gerçek zamanlı ortak çalışma.**
- **Grafik görünümü.** Güzel görünür, az kullanılır.
- **Blok bağlantısı ve transklüzyon:** her bloğa kimlik ister; sayfa bağlantısı (`[[ ]]`) yeter, gerekirse başlığa bağlantı (`[[Sayfa#Başlık]]`).
- **Sütunlar:** [15](15-bloklar-denemesi.md)'teki karar.

## Kaynaklar
- [Tana — Live search](https://help.tana.inc/live-search.html), [Tana Learn: live search and views](https://tana.inc/articles/tana-learn-live-live-search-and-views)
- [Capacities — günlük ve takip](https://capacities.io/use-cases/daily-tracking), [Capacities incelemesi 2026](https://blog.saner.ai/capacities-review)
- [Obsidian Bases özeti](https://skillselion.com/skills/agricidaniel/claude-obsidian/obsidian-bases) (ikincil kaynak; resmî: help.obsidian.md/bases)
- Thymer: kendi incelememiz (almasada.thymer.com).
