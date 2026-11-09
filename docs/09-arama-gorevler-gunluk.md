# 09 — Arama, görevler, günlük

Kitaplıkta arama (kelimeler, `"tam ifade"`, `başlık:`, `koleksiyon:`, `#etiket`), arama koleksiyonu, görev görünümü (Açık /
Tamamlanan, Bugün / Bu hafta / Gecikmiş), yinelenen görevler, görev türleri, günlük ve yazma istatistikleri yapıldı. Kalanlar:

## Arama sorgu dili [kod: MCP belgesi]
Yapıldı (#41): `OR`/`VEYA`, `-`/`NOT`/`DEĞİL`, parantez; `@todo`, `@done`, `@today`; `created:`/`modified:` tarih ve aralıkla;
`alan:değer`, `alan>değer`; sayfada canlı sorgu bloğu. Thymer'de olup bizde olmayanlar:

- **Tarih**: `@tomorrow`, `@monday`, `@thisweek`, `@lastweek`, `@thismonth`, `@"week 10"`; `@due <= @today` gibi karşılaştırma.
- **Görevler**: `@due`, `@overdue`, `@scheduled`; `@due` (görevin kendi tarihi) ile `@scheduled` (birinin günlüğünde olması) farklıdır.
- **Tipler**: `@document`, `@heading`, `@text`, `@quote`, `@list`, `@image`, `@file`. **Bağlantılar**: `@link`, `@link="github"`.

## Görevler [görüldü + kod]
Türe göre süzgeç (Önemli, Sürüyor…) ve doğal dilde tarih yapıldı (#41). Kalan: "Schedule for today/tomorrow" (görevi günlüğe koymak).

## Günlük (Journal) [görüldü]
Önceki gün, sonraki gün ve tarihe git yapıldı (#41).

## Bizim editör için [öneri]

Sorgu dili tek ayrıştırıcıyla her yerde aynı (arama, arama koleksiyonu, sorgu bloğu). Sırada: yukarıdaki tarih ve görev terimleri,
sonra satır tipleri.
