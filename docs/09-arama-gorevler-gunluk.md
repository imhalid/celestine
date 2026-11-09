# 09 — Arama, görevler, günlük

Kitaplıkta arama (kelimeler, `"tam ifade"`, `başlık:`, `koleksiyon:`, `#etiket`), arama koleksiyonu, görev görünümü (Açık /
Tamamlanan, Bugün / Bu hafta / Gecikmiş), yinelenen görevler, görev türleri, günlük ve yazma istatistikleri yapıldı. Kalanlar:

## Arama sorgu dili [kod: MCP belgesi]
Thymer'de aynı dil arama kutusunda, canlı sorgu bloklarında, koleksiyonun "Related Section"ında ve dinamik koleksiyonlarda
kullanılır. Bizde olmayanlar:

- **Mantık**: `AND`, `OR`, `NOT` (`&&`, `||`, `!`), parantez.
- **Tarih**: `@today`, `@tomorrow`, `@monday`, `@"next monday"`, `@thisweek`, `@lastweek`, `@thismonth`, `@"week 10"`,
  `@2025-01-01`, karşılaştırma: `@due <= @today`.
- **Görevler**: `@task`, `@todo`, `@done`, `@due`, `@overdue`, `@scheduled`, bayraklar (`@important`, `@inprogress`, `@waiting`…).
- **Tipler**: `@document`, `@heading`, `@text`, `@quote`, `@list`, `@image`, `@file`.
- **Zaman damgaları**: `@created_at`, `@modified_at`. **Bağlantılar**: `@link`, `@link="github"`.
- **Koleksiyon alanları**: `@collection="Projects"`, `@Project.status="In progress"`, operatörler `= === != < <= > >=`.
- `@due` (görevin kendi tarihi) ile `@scheduled` (birinin günlüğünde olması) farklıdır.

## Görevler [görüldü + kod]
- Thymer'in hazır filtrelerinden bizde olmayanlar: Important, In progress (görev türleri var, türe göre süzgeç yok).
- Görev satırına doğal dilde tarih yazılabilir ("tomorrow 3pm"); "Schedule for today/tomorrow" görevi günlüğe koyar.

## Günlük (Journal) [görüldü]
- Komutlar: Go to Journal date…, next/previous day (bizde yalnız Bugünün sayfası ve Rastgele gün).

## Bizim editör için [öneri]

1. **Sorgu dili**, sırada: `@todo/@done` + `@today`; sonra koleksiyon alanları, karşılaştırmalar ve mantık. Tek bir ayrıştırıcı,
   her yerde aynı (arama, arama koleksiyonu, MCP `search`).
2. **Canlı sorgu bloğu** bu motorun üstüne gelir.
