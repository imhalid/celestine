package main

import (
	"fmt"
	"strings"
)

// The language of the window. The strings of the app are written in Turkish; T gives the English for
// those that have one, and the Turkish for the rest, so that a string that is not translated yet shows
// as it was written, not as a hole. The settings' names and labels, the menus, the palette, the commands
// and the main windows are translated; the long help lines of the settings are still Turkish only. A
// string is added in enText; TestEnglishLoses checks that nothing in it is lost or made twice.

// lang is "tr" or "en".
var lang = "tr"

// setLang sets the language from the setting: English, unless Turkish is chosen.
func setLang(setting string) {
	if setting == "tr" {
		lang = "tr"
		return
	}
	lang = "en" // English unless Turkish is chosen
}

// T is s in the language of the window.
func T(s string) string {
	if lang != "en" {
		if tr, ok := trText[s]; ok { // a string written in English (i18n_tr.go)
			return tr
		}
		return s
	}
	if e, ok := enText[s]; ok {
		return e
	}
	// a title with a name after it: "Çöpe taşı: Sayfa" → the part before is translated
	if i := strings.Index(s, ": "); i > 0 {
		if e, ok := enText[s[:i]]; ok {
			return e + s[i:]
		}
	}
	return s
}

// Tf is T for a sentence with values in it: the format is translated, then filled.
func Tf(format string, a ...any) string { return fmt.Sprintf(T(format), a...) }

// langName is the language's name, in itself.
func langName(l string) string {
	switch l {
	case "tr":
		return "Türkçe"
	}
	return "English"
}

var enText = map[string]string{
	// the settings' sections
	"Görünüm": "Appearance", "Tema ve sayfanın genişliği": "Theme and the page's width",
	"Editör": "Editor", "Yazı, satır aralığı ve caret": "Text, line spacing and caret",
	"Yazı tipi": "Typeface", "Uygulamanın, metnin ve kodun yazı tipleri": "The typefaces of the app, the text and the code",
	"Durum çubuğu": "Status bar", "Altta neyin görüneceği": "What is shown at the bottom",
	"MCP": "MCP", "Ajanların kitaplığa bağlanması": "Agents connecting to the library",
	"Kitaplık": "Library", "Dosyalar ve nerede durdukları": "The files and where they are",
	"Senkron": "Sync", "Başka bilgisayarlarla eşitleme, şifreleme": "Matching other computers, encryption",
	"Güncelleme": "Updates", "Yeni sürümler ve sürüm notları": "New versions and release notes",
	"Ayarlar": "Settings", "Tüm ayarlar…": "All settings…",
	// the settings' rows
	"Tema": "Theme", "Renk teması": "Color theme", "Caret": "Caret", "Köşeler": "Corners", "Sayfa genişliği": "Page width",
	"Sistem": "System", "Açık": "Light", "Koyu": "Dark", "Köşe yuvarlaklığı": "Corner roundness",
	"Jöle efekti": "Jelly effect", "Jöle sarsıntısı": "Jelly shake", "Otomatik kapak": "Automatic cover",
	"Yazı boyutu": "Font size", "Satır aralığı": "Line spacing", "Paragraf aralığı": "Paragraph spacing",
	"İlk satır girintisi": "First-line indent", "Otomatik kayıt": "Autosave", "Kayıt gecikmesi": "Save delay",
	"Boşlukları göster": "Show spaces", "Sekmeleri göster": "Show tabs", "Paragraf sonlarını göster": "Show paragraph ends",
	"Kod bloklarında boşlukları göster": "Show whitespace in code blocks", "Kod bloklarında da": "In code blocks, too",
	"Vim hareketleri": "Vim motions", "Önizleme": "Preview",
	"Uygulama yazı tipi": "App typeface", "Editör yazı tipi": "Editor typeface", "Kod yazı tipi": "Code typeface",
	"Ligatürler": "Ligatures", "Aynı genişlikte rakamlar": "Tabular numbers", "Yazı kalınlığı": "Font weight",
	"Hazır yazı tipleri": "Typeface presets", "Okuma": "Reading", "Yazı": "Writing", "Kod": "Code",
	"Kaydetme durumu": "Save status", "Harf sayısı": "Letter count", "Satır ve sütun": "Line and column",
	"Durum çubuğunda kaydetme durumu": "Save status in the status bar", "Durum çubuğunda harf sayısı": "Letter count in the status bar",
	"Durum çubuğunda satır ve sütun": "Line and column in the status bar",
	"Kenar çubuğu":                   "Sidebar", "Yan panel": "Side panel", "MCP sunucusu": "MCP server",
	"Günlük yedek": "Daily backup", "İstatistik toplama": "Collect statistics",
	"Yeni sürümü denetle": "Check for updates", "Güncelleme sessiz mod": "Quiet update mode", "Kendiliğinden güncelle": "Update automatically",
	"Güncelleme kanalı": "Update channel", "Güncelleme denetim sıklığı": "Update check interval",
	"Senkron aralığı": "Sync interval", "Kararlı": "Stable", "Deneme (beta)": "Trial (beta)", "kapalı": "off", "klasör": "folder", "sunucu": "server",
	"Dil":           "Language",
	"Yedek klasörü": "Backup folder", "Şimdi yedekle": "Back up now", "Yedekler": "Backups", "Geri yükle": "Restore",
	"Uçtan uca şifreleme": "End-to-end encryption", "Nereye": "Where to", "Durum": "State", "Şimdi eşitle": "Sync now",
	"Şimdi denetle": "Check now", "Sessiz mod": "Quiet mode", "Deneme sürümleri": "Trial versions", "Güncelleme adresi": "Update address",
	"Kendiliğinden indir ve kur": "Download and install by itself", "Yeni sürümü denetle ": "Check for updates",
	// the statistics page
	"İstatistikler": "Statistics", "Yazım": "Writing", "Kullanım": "Use", "Veri": "Data", "Yıl": "Year",
	"Hangi saatlerde yazıyorum": "When do I write", "Yazma oturumları": "Writing sessions", "Nerede yazıyorum": "Where I write",
	"Kitaplık sağlığı": "Library health", "bugün": "today", "bu hafta": "this week", "bu ay": "this month",
	"gün üst üste": "days in a row", "en iyi gün": "best day", "toplam": "total", "oturum": "sessions",
	"ortalama hız": "average speed", "ortalama süre": "average length", "koleksiyon": "collections", "sayfa": "pages",
	"kelime": "words", "etiket": "tags", "bağlantı": "links", "görev": "tasks", "Topla": "Collect",
	"CSV olarak dışa aktar": "Export as CSV", "JSON olarak dışa aktar": "Export as JSON",
	"Haftalık özeti günlüğe yaz": "Write the weekly summary to the journal", "Sıfırla…": "Reset…",
	// commands
	"Yeni sayfa": "New page", "Yeni koleksiyon": "New collection", "Görevler": "Tasks", "Çöp": "Trash",
	"Bugünün sayfası": "Today's page", "Rastgele gün": "Random day", "Geri": "Back", "İleri": "Forward",
	"Kenar çubuğunu aç/kapat": "Toggle the sidebar", "Caret'i değiştir (⇧⌘M)": "Change the caret (⇧⌘M)",
	"Sayfada bul": "Find in page", "Geri al": "Undo", "Yinele": "Redo", "Kitaplıkta ara": "Search the library",
	"Hızlı not": "Quick note", "Çöpe taşı": "Move to trash", "Alt sayfa ekle": "Add a sub-page",
	"Koleksiyon ayarları": "Collection settings", "Görünüm: ": "View: ", "Markdown klasörü içe aktar…": "Import a Markdown folder…",
	"Panodan tablo ekle (CSV)": "Paste a table (CSV)", "Çökme günlüğünü göster": "Show the crash log",
	"Markdown olarak dışa aktar (klasör)": "Export as Markdown (folder)", "Markdown olarak dışa aktar (zip)": "Export as Markdown (zip)",
	"Bu sürümde neler yeni": "What's new in this version", "Kısayollar ve yardım": "Shortcuts and help",
	"Klavye kipine geç (Esc)": "Enter the keyboard mode (Esc)", "Yazı tipi hazırı": "Typeface preset",
	"Yedekten geri yükle": "Restore from a backup", "Yan bölmede aç": "Open in a side pane", "Bölmeler arası geç": "Switch pane",
	"Bölmeyi kapat": "Close the pane", "Ayar": "Setting", "aç": "on", "kapat": "off", "artır": "increase", "azalt": "decrease",
	"varsayılan": "default", "Komut": "Command",
	// the palette
	"Sayfalar": "Pages", "Komutlar": "Commands", "Etiketler": "Tags", "Satıra git": "Go to line", "Sayfada ara": "Find in page",
	"Son komutlar": "Recent commands", "Yardım": "Help", "Son kullanılanlar": "Recently used", "Son açılanlar": "Recently opened",
	"Metinde": "In the text", "Daha fazlası": "More", "Eşleşen bir şey yok.": "Nothing matches.",
	"Komut ya da ayar… (yazı boyutu 18) · ? yardım":                "A command or setting… (font size 18) · ? help",
	"Sayfaya git… > komut · # etiket · : satır · / bul · ? yardım": "Go to a page… > command · # tag · : line · / find · ? help",
	"sayfa, koleksiyon ve metinde ara (öntanımlı)":                 "search pages, collections and text (default)",
	"uygulamanın yapabildiği her şey ve bütün ayarlar":             "everything the app can do, and all its settings",
	"bir etiketin sayfalarına git":                                 "go to the pages of a tag", "açık sayfada paragraf numarasına git": "go to a paragraph number in the open page",
	"açık sayfada bul": "find in the open page", "en son çalıştırdıkların": "what you ran last", "bu liste": "this list",
	// menus
	"Dosya": "File", "Yeni sayfa ": "New page", "Kaydet": "Save", "Blok": "Block", "Git": "Go", "Yukarı taşı": "Move up", "Aşağı taşı": "Move down",
	"Çoğalt": "Duplicate", "Sayfa ara…": "Find a page…", "Komutlar…": "Commands…", "Sayfada bul…": "Find in page…",
	"Kitaplıkta ara…": "Search the library…", "Hızlı not…": "Quick note…", "Yan panel ": "Side panel", "Yeni koleksiyon ": "New collection",
	"Yazıyı büyüt": "Bigger text", "Yazıyı küçült": "Smaller text", "Yazı boyutu: varsayılan": "Font size: default",
	"Ayarlar…": "Settings…", "Caret'i değiştir": "Change the caret",
	// the later additions
	"Anahtar": "Key", "Ayar dosyası oku": "Read a settings file", "Ayarları dosyaya yaz": "Write the settings to a file",
	"Caret uykusu": "Caret sleep", "Daktilo kaydırması": "Typewriter scrolling", "Hafif mod": "Light mode",
	"Komuta kısayol ver": "Give a command a shortcut", "Kurtarma anahtarı: bir yere yaz": "Recovery key: write it down",
	"Makro yaz": "Write a macro", "Markdown klasörü içe aktar": "Import a Markdown folder", "Markdown olarak dışa aktar": "Export as Markdown",
	"Markdown olarak zip": "Markdown as a zip", "Odak halkasını her zaman göster": "Always show the focus ring",
	"Odak modu": "Focus mode", "Tab uzunluğu": "Tab width", "Şimdikini profil olarak kaydet": "Save the current look as a profile",
	"Satır": "Line", "Paragraf": "Paragraph", "Kapalı": "Off", "Dışa aktar": "Export", "İçe aktar…": "Import…", "Dosya ": "File",
	"Ayarlarda ara…": "Search the settings…", "Değiştirilenler": "Changed", "Profiller": "Profiles", "Kısayollar ": "Shortcuts",
	"Eşleşen ayar yok.": "No setting matches.", "Hiçbir ayar değişmemiş: hepsi varsayılan.": "Nothing has changed: all are defaults.",
	"Hepsini varsayılana döndür": "Reset all to defaults", "Varsayılana dön": "Reset to default",
	"Hazır profil": "Built-in profile", "Kayıtlı profil": "Saved profile", "Uygula": "Apply", "Profil adı": "Profile name",
	"Kendi kısayolların": "Your shortcuts", "Makrolar": "Macros", "Kaldır": "Remove", "Ata": "Assign", "Komut adı": "Command name",
	"Makro adı": "Macro name", "Çalıştır": "Run", "Etiketler ": "Tags", "En çok vakit geçirilen sayfalar": "Pages you spent most time on",
	"Durum çubuğunu gizle": "Hide the status bar", "Okuma süresi": "Reading time", "Geri bağlantı": "Backlinks",
	"Seçimi kalın yap": "Make the selection bold", "Seçimi italik yap": "Make the selection italic", "Seçimin altını çiz": "Underline the selection",
	"Seçimin üstünü çiz": "Strike the selection through", "Seçimi kod yap": "Make the selection code",
	"Son kayıttan önceki hâle dön": "Go back to before the last save", "Ayarları dosyaya yaz ": "Write the settings to a file",
	"Odak mod": "Focus mode", "⌘P komutlar · ⌘K sayfa ara · ? kısayollar (Esc ile normal kip)": "⌘P commands · ⌘K find a page · ? shortcuts (Esc for normal mode)",
	"Bir sayfa seç": "Pick a page", "Çöp boş": "The trash is empty",
	// long lines of the settings, the ones written last
	"Satır ya da paragraf: caret'in bulunduğu kalır, sayfanın geri kalanı silikleşir.":                                           "Line or paragraph: the one the caret is in stays, the rest of the page fades.",
	"Yazdığın satır pencerenin ortasında kalır, sayfa altından kayar.":                                                           "The line you write stays in the middle of the window; the page slides under it.",
	"Tab'ın yazdığı boşluk sayısı; 0 gerçek bir sekme karakteridir.":                                                             "How many spaces Tab writes; 0 is a real tab character.",
	"Yazmadan bu kadar durunca caret uyur.":                                                                                      "The caret dozes off after this long without writing.",
	"Seçili caret ne olursa olsun efektsiz, düz bir çizgi çizilir.":                                                              "A plain line is drawn, whatever caret is chosen, with no effects.",
	"Klavyeyle seçilen satır, normal kipte olmasan da çerçevelenir.":                                                             "The row chosen with the keyboard is framed even outside the normal mode.",
	"İki paragraf arasındaki boşluk, yazı boyutunun katı olarak.":                                                                "The room between two paragraphs, as a multiple of the font size.",
	"Kapalıyken sayfa ⌘S ile kaydedilir; sayfa değişirken ve uygulama kapanırken yazılan yine kaydedilir, hiçbir şey kaybolmaz.": "When off, the page is saved with ⌘S; what is typed is still saved when the page changes or the app quits, so nothing is lost.",
	"Son yazıdan bu kadar sonra kaydedilir.":                                                                                     "It is saved this long after the last keystroke.",
	"Boşluk bir nokta, kırılmaz boşluk bir halka; satır sonundaki artık boşluklar uyarı renginde.":                               "A space is a dot, a non-breaking space a ring; spaces at the end of a line are in the warning color.",
	"Sekme bir ok.": "A tab is an arrow.", "Her paragrafın sonunda ¶.": "A ¶ at the end of each paragraph.", "Kod bloklarının içi de işaretlenir.": "The inside of code blocks is marked, too.",
	"Menüler, kenar çubuğu, başlıklar ve düğmeler.": "Menus, the sidebar, headings and buttons.", "Sayfanın metni. Boşsa uygulamanınki kullanılır.": "The page's text. When empty, the app's is used.",
	"Kod blokları ve satır içi kod.": "Code blocks and inline code.",
	"Her gün ilk açılışta kitaplığın sıkıştırılmış bir kopyası alınır; son 7 gün ve 4 hafta kalır. İçe aktarma, geri yükleme ve güncellemeden önce de bir tane alınır.":                                                      "A compressed copy of the library is taken at the first start of each day; the last 7 days and 4 weeks are kept. One is taken before an import, a restore and an update, too.",
	"Yalnız bu sürümün numarası sorulur: kitaplığından hiçbir şey gönderilmez. Kapalıyken uygulama ağa hiç çıkmaz (çevrimdışı mod).":                                                                                         "Only this version's number is asked: nothing of your library is sent. When off, the app never goes to the network (offline mode).",
	"Yeni sürüm iletişim kutusuyla çıkmaz, yalnız burada görünür.":                                                                                                                                                           "A new version does not come up in a dialog; it shows here only.",
	"Kararlı sürümlere ek olarak beta sürümleri de bildirilir.":                                                                                                                                                              "Beta versions are told of, as well as stable ones.",
	"Dosyalar bu bilgisayarda şifrelenir, adları da gizlenir: sunucu ya da klasör yalnız anlamsız bloklar görür. Parolayı ve kurtarma anahtarını kaybedersen veri kurtarılamaz.":                                             "Files are encrypted on this computer and their names hidden: the server or folder sees only meaningless blocks. If you lose the passphrase and the recovery key, the data cannot be recovered.",
	"Bir klasör (iCloud, Dropbox, Syncthing bırakınca onu taşır) ya da kendi sunucun (cloudflare/sync/worker.js).":                                                                                                           "A folder (iCloud, Dropbox or Syncthing then carry it) or a server of your own (cloudflare/sync/worker.js).",
	"Arayüzün dili. Menüler yeniden başlatınca değişir. Metin düzeltmeniz bundan ayrıdır.":                                                                                                                                   "The language of the interface. Menus change at the next start.",
	"Tema, renk, yazı tipleri, boyutlar, caret ve boşluk gösterimi bir ad altında saklanır; ⌘P'de “Profil: ad” ile geçilir.":                                                                                                 "Theme, color, typefaces, sizes, caret and white space display are kept under a name; switch with “Profile: name” in ⌘P.",
	"Görünüm ve yazı ayarları JSON olarak yazılır; parolalar, jetonlar ve bu bilgisayara ait konumlar yazılmaz. Başka bir bilgisayara taşımak için.":                                                                         "The look and writing settings are written as JSON; passphrases, tokens and places of this computer are not. For moving to another computer.",
	"Dosyadakiler ayarlanır; yazılmayanlar olduğu gibi kalır.":                                                                                                                                                               "What is in the file is set; what is not stays as it is.",
	"Komutun adını ve tuşu yaz: örn. “Şimdi yedekle” ve cmd+shift+y. Uygulamanın ve sistemin tuşları verilmez.":                                                                                                              "Type the command's name and the key: e.g. “Back up now” and cmd+shift+y. The app's and the system's keys are not given.",
	"Bir ad ve + ile ayrılmış komutlar: örn. “Günlük görev” için “Bugünün sayfası + Görevler”. ⌘P'de “Makro: ad” olarak çıkar.":                                                                                              "A name and commands separated by +: e.g. “Today's page + Tasks”. It shows in ⌘P as “Macro: name”.",
	"Altta hiçbir şey çizilmez: odak modu. Yazı tipi ve tema düğmeleri ⌘P'den yine ulaşılır.":                                                                                                                                "Nothing is drawn at the bottom: focus mode. The theme and typeface buttons are still reached from ⌘P.",
	"Esc ile normal kipe geçince metinde h j k l, w b e, 0 $, gg G, x, dd, yy, p, o O, A I çalışır; i ile yazmaya dönülür. Kapalıyken normal kipte yalnız listeler, harf etiketleri (f), leader (Space) ve komutlar vardır.": "After Esc, h j k l, w b e, 0 $, gg G, x, dd, yy, p, o O, A I work in the text; i goes back to writing. When off, the normal mode has only lists, hint letters (f), the leader (Space) and commands.",
	"Sayfayı yukarı taşı (sırada)": "Move the page up (in its row)", "Sayfayı aşağı taşı (sırada)": "Move the page down (in its row)",
	"Sayfayı üst sayfadan ayır": "Take the page out of its parent", "Sayfayı taşı": "Move the page", "Koleksiyonu yukarı taşı": "Move the collection up",
	"Koleksiyonu aşağı taşı": "Move the collection down", "Makro": "Macro", "Profil": "Profile",
	"Aynısı tek bir .zip dosyası: başka bir bilgisayara taşımak ya da arşivlemek için.":                                                                                                         "The same as a single .zip file: for moving to another computer or archiving.",
	"Açıkken ajanlar (Claude Code ya da başka bir MCP istemcisi) bu kitaplığa bağlanır: koleksiyon ve sayfa oluşturur, okur, düzenler, taşır. Yaptıkları pencerede anında görünür.":             "When on, agents (Claude Code or another MCP client) connect to this library: they create, read, edit and move collections and pages. What they do shows in the window at once.",
	"Basılı yazıdaki gibi paragrafın ilk satırı içeriden başlar; başlık, liste ve alıntıdan sonrakine uygulanmaz.":                                                                              "As in print, the first line of a paragraph starts indented; it does not apply to the one after a heading, a list or a quote.",
	"Bir ajan (MCP) sayfaya içerik eklerken pencere, eklenen kadar, jöle gibi sağa sola sallanır ve zıplar. Pencerenin yeri değiştirilemiyorsa (tam ekran, Wayland) içindeki görünüm sallanır.": "While an agent (MCP) adds content to a page, the window shakes from side to side and hops like jelly, as much as is added. When the window cannot be moved (full screen, Wayland), the view inside shakes.",
	"Bulunan sürüm sorulmadan indirilir, imzası doğrulanır ve yerine konur; açmak için yeniden başlatman istenir.":                                                                              "A version that is found is downloaded without asking, its signature is checked and it is put in place; you are asked to restart to use it.",
	"Bütün köşeler tek bir ölçüden hesaplanır: 0 köşeli, 100 tasarımın kendisi, 200 iki kat yuvarlak.":                                                                                          "All corners are worked out from one measure: 0 is square, 100 is the design itself, 200 is twice as round.",
	"Editör metninin kalınlığı; yazı tipinde o kalınlık yoksa en yakını çizilir.":                                                                                                               "The weight of the editor's text; if the typeface lacks it, the nearest is drawn.",
	"Her caret yolda uzar, yaylanarak yerine oturur. Slime zaten jöledir.":                                                                                                                      "Every caret stretches on its way and settles with a spring. The slime is jelly already.",
	"Kapağı olmayan her sayfa, kendi tohumundan üretilen bir manzara alır: dağlar, göl, sahil, orman, şehir, Japon bahçesi, mercan resifi, balonlar, kuzey ışıkları, uzay… Açık temada gündüzün bir saati, koyuda gece ve bugünün ayı; mevsim de bugünün. Bulutlar, kuşlar, trenler, balıklar yavaşça kıpırdar. Sayfada Kapağı kaldır dersen o sayfa kapaksız kalır.": "Every page with no cover gets a landscape made from its own seed: mountains, a lake, a beach, a forest, a city, a Japanese garden, a coral reef, balloons, the northern lights, space… An hour of the day in the light theme, the night and today's moon in the dark, and today's season. Clouds, birds, trains and fish move slowly. If you choose Remove cover on a page, it stays without one.",
	"Metin sütununun genişliği.": "The width of the text column.",
	"Okuma serif, yazı sans, kod tek aralıklı; bilgisayarda olanlardan ilki seçilir.":     "Reading serif, writing sans, code monospaced; the first of those on the computer is chosen.",
	"Rakamların hepsi aynı genişliktedir: sütunlar ve tablolar hizalı durur.":             "All digits have the same width: columns and tables stay aligned.",
	"Yalnız bu bilgisayardan, anahtarla bağlanılır.":                                      "Only from this computer, with a key.",
	"fi, fl gibi harf çiftleri tek harf olarak çizilir. Kod yazarken kapatmak işe yarar.": "Pairs like fi and fl are drawn as one letter. Turning it off helps when writing code.",
	"Satır uzunluğu": "Line length",
	"Metin sütunu en çok bu kadar karakter genişliğinde olur (okunaklı satır 60–80 karakterdir); 0 sayfa genişliği kadar.": "The text column is at most this many characters wide (a readable line is 60–80 characters); 0 is as wide as the page.",
	// help, tour, notices
	"Kısayollar": "Shortcuts", "Her yerde": "Everywhere", "Metin": "Text", "Palet kipleri": "Palette modes",
	"Normal kip (Esc ile girilir, i ile çıkılır)": "Normal mode (Esc enters, i leaves)", "Komutların kısayolları": "Shortcuts of commands",
	"Yaz": "Write", "Her şey ⌘P'de": "Everything is in ⌘P", "Fare olmadan": "Without the mouse", "Senin kitaplığın": "Your library",
	"İleri ": "Next", "Bitti": "Done", "Bir daha gösterme": "Don't show again", "Tamam": "OK", "Kopyala": "Copy", "Kapat": "Close",
	"kaydedildi": "saved", "kaydediliyor…": "saving…", "kaydedilmedi": "not saved",
	"Bir hata oldu": "Something went wrong", "Devam et": "Continue", "Beklenmedik bir hata oldu": "An unexpected error occurred",
	"Son oturum beklenmedik kapandı": "The last session ended unexpectedly",
	"Yeni sürüm hazır":               "A new version is ready", "Şimdi güncelle": "Update now", "3 gün sonra hatırlat": "Remind me in 3 days",
	"Bu sürümü atla": "Skip this version", "Yeniden başlat": "Restart", "Sonra": "Later", "Sürüm notu yok.": "No release notes.",
	"Geri yükle ": "Restore", "Senkron kapalı": "Sync is off", "Bunu kullan": "Use this", "Sil": "Delete",
	"Çakışmalar": "Conflicts", "Eşit": "In sync", "çevrimdışı": "offline", "eşitleniyor…": "syncing…",
	"Klavye kipi": "Keyboard mode",
}
