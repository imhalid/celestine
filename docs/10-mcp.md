# 10 — MCP

## Thymer'de nasıl [görüldü + kod]
- Ayarlar → **MCP (AI Agents)**: ayrı bir "Thymer CLI" yardımcı uygulaması indirilir, çalıştırılır, **Connect** ile bağlanır.
  Her çalışma alanı için erişim **Off / Read only / Read & write**.
- 71 araç. Gruplar:
  - **Çalışma alanı**: list_workspaces, get_workspace_info, list_users, get_current_user
  - **Koleksiyon**: list_collections, get_collection, get_collection_schema, get/update_collection_config_json,
    create_collection, add_property_to_collection, add_view_to_collection, list/get_collection_view, custom order araçları
  - **Kayıt**: list_records, get_record (Markdown ya da yapısal), get_record_properties, query_records, create_record,
    update_record, update_record_property, move_record_to_collection, trash_record, untrash_record
  - **Satır**: get_line_items, create_line_item, update_line_item, move_line_item, trash_line_item, set/get_task_status,
    insert_markdown
  - **Arama ve bağlar**: search, get_backlinks
  - **Günlük**: get_journal_page, add_to_journal, navigate_to_journal
  - **Arayüz**: get_open_panels, navigate_to_record, navigate_to_collection, create_panel, close_panel
  - **Tema**: list_themes, get_theme, set_theme, set_appearance, get/set_custom_css
  - **Dosyalar**: upload_file, download_file, set_record_banner, set_collection_banner, set_file_property
  - **Eklenti** (bizde olmayacak): list_plugins, get/update_plugin_code, preview_plugin…
  - **Belgeleme**: data_help (agent'a veri modelini anlatan uzun bir metin)

İyi fikirler:
- **`data_help`**: agent'a modeli, satır tiplerini, sorgu dilini anlatan tek bir araç. Agent ne yapacağını buradan öğreniyor.
- **`insert_markdown`**: agent Markdown verir, uygulama onu doğru tipte satırlara çevirir. Agent'ın satır satır uğraşmasına gerek kalmaz.
- **"Var olan düzene uy" talimatı**: belge, agent'a "kendi düzenini dayatma, çalışma alanının yapısına uy" diyor.

## Bizim editör için [öneri]

Karar: MCP bir eklenti değil; uygulamanın kendi işlemleri dışarıya açılır. Eklenti/kod araçları **yok**. Sunucu uygulamanın
içinde (Streamable HTTP, `127.0.0.1`, token, varsayılan kapalı) ve ilk araç seti yapıldı. Kalanlar:

1. **stdio köprüsü**: stdio isteyen istemciler için uygulamanın kendisi `ryudit mcp` diye bir stdio köprüsü olarak da çalışabilsin.
2. **Koleksiyon başına** erişim: Kapalı / Salt okunur / Okuma-yazma (bugün yalnız aç/kapat).
3. **Eksik araçlar**:

   | Araç | İş |
   |---|---|
   | `help` | Veri modeli, satır tipleri, sorgu dili (Thymer'in `data_help`'i gibi) |
   | `update_collection` | Koleksiyon ayarlama (görünümler, alt sayfa); bugün yalnız `add_property` var |
   | `insert_markdown` | Bir sayfanın istenen yerine Markdown ekleme (bugün `update_page` yalnız sona ekler) |
   | `restore_page` | Çöpten geri alma (kalıcı silme yok) |
   | `list_themes`, `set_theme`, `create_theme` | Tema (veri olduğu için güvenli) |
   | `open_page` | Sayfayı arayüzde açma |

4. **Güvenlik**: her yazma işlemi geçmişe "agent: …" diye yazılsın; çöpe atma onay istesin (ayardan kapatılabilir); kalıcı
   silme MCP'de yok.
5. **Görünürlük**: değişen satırda kısa bir vurgu (pencerenin sallanması ve durum çubuğundaki nokta var).
