# 09 — Arama, görevler, günlük

## Arama sorgu dili [kod: MCP belgesi]
Aynı dil arama kutusunda, canlı sorgu bloklarında, koleksiyonun "Related Section"ında ve dinamik koleksiyonlarda kullanılır.

- **Metin**: yan yana kelimeler VE; tırnakla ifade: `"release notes"`.
- **Mantık**: `AND`, `OR`, `NOT` (`&&`, `||`, `!`), parantez.
- **Tarih**: `@today`, `@tomorrow`, `@monday`, `@"next monday"`, `@thisweek`, `@lastweek`, `@thismonth`, `@"week 10"`,
  `@2025-01-01`, karşılaştırma: `@due <= @today`.
- **Görevler**: `@task`, `@todo`, `@done`, `@due`, `@overdue`, `@assigned`, `@unassigned`, `@scheduled`, bayraklar
  (`@important`, `@inprogress`, `@waiting`…).
- **Kişiler**: `@me`, `@alice`, `@mention`, `@scheduled=@me`.
- **Etiketler**: `#tag`, önek: `#projects/`.
- **Tipler**: `@document`, `@heading`, `@text`, `@quote`, `@list`, `@image`, `@file`.
- **Zaman damgaları**: `@created_at`, `@modified_at`. **Bağlantılar**: `@link`, `@link="github"`.
- **Koleksiyon alanları**: `@collection="Projects"`, `@Project.status="In progress"`, operatörler `= === != < <= > >=`.
- `@due` (görevin kendi tarihi) ile `@scheduled` (birinin günlüğünde olması) farklıdır.

## Görevler [görüldü + kod]
- Kenar çubuğunda **Tasks**: hazır filtreler — All tasks, Due today, Overdue, Important, In progress, Assigned to me,
  Unassigned, Due this week/month… ("Tasks due today, overdue or important").
- Görev durumları: done, started (in progress), blocked (waiting), billable, important, discuss, alert, starred, canceled.
- Görev satırına doğal dilde tarih yazılabilir ("tomorrow 3pm"); "Schedule for today/tomorrow" görevi günlüğe koyar.

## Günlük (Journal) [görüldü]
- Kişi başına her gün bir sayfa ("Wed Nov 4"); başlığın altında **‹ Today ›** ve takvim düğmesi.
- Kenar çubuğunda Today / Yesterday / Previous 7 days / Previous 30 days grupları.
- **Quick Add** bugünün günlüğüne saat damgalı bir satır ekler ([05](05-not-ekleme-akislari.md)).
- Komutlar: Go to Today's Journal, Go to Journal date…, next/previous day, Switch Journal User….

## Bizim editör için [öneri]

**Adım adım:**
1. **İndeks**: her satırın metni, tipi, etiketleri, tarihleri ve bağlantıları için bellekte bir ters indeks; değişen satır
   yalnız kendini günceller. Kalıcı depoda SQLite FTS5 kullanılabilir (saf Go sürücü: `modernc.org/sqlite`, cgo yok —
   mizan-go'nun cgo'suz kapısıyla uyumlu).
2. **Sorgu dili**: önce metin + `#etiket` + `@todo/@done` + `@today`; sonra koleksiyon alanları ve karşılaştırmalar.
   Tek bir ayrıştırıcı, her yerde aynı.
3. **Canlı sorgu bloğu** ve **dinamik koleksiyon** bu motorun üstüne gelir.
4. Günlük ilk sürüm için isteğe bağlı; koleksiyonlar ve alt sayfalar önce.
