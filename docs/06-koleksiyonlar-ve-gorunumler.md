# 06 — Koleksiyonlar ve görünümler

Koleksiyon sayfası, ayarları (Genel, Özellikler, Görünümler, Sayfalar, Kod; taslak → önizleme → kaydet), liste, tablo, pano ve
takvim görünümleri, şablonlar, arama koleksiyonu (Thymer'in dinamik koleksiyonu) ve toplu işlem yapıldı. Kalanlar:

## Görünüm tipleri [kod]
Bizde olmayanlar:

| Tip | Ne | Gereken |
|---|---|---|
| `gallery` | Kart galerisi (kapak görselleri ya da sayfa önizlemesi) | — |
| `record` | "Page Shortcut": koleksiyonun içinde bir sayfayı sekme gibi açar | — |

Liste Thymer'de sayılabilir bir alana göre de gruplanabilir; bizimki yalnız tarihe göre.

Görünümlerde **Custom Order** var: kayıtlar elle sıralanabilir (tablo, pano sütunları, galeri). Bir kayıt pano görünümünde
yalnızca bir sütunda olabilir: "Allow multiple" açık bir alanla pano gruplanamaz. [kod] Bizde elle sıra yalnız panoda var;
liste ve tabloda yok.

Koleksiyon `…` menüsünde **Change Collection Banner…** (koleksiyona kapak) bizde yok.

## Bizim editör için [öneri]

1. **Görünümler**: sıradaki `gallery`.
2. **Alan tipleri**: sıradakiler Page Link, Tag; sonra File/Image, User ([02](02-veri-modeli.md)).
3. Her görünümde **elle sıralama** (Custom Order) kalıcı olsun; sürükle-bırak ile.
