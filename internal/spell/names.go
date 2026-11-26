package spell

import (
	"regexp"
	"strings"

	"github.com/imhalid/celestine/internal/spell/tr"
)

// This file is tinyfix-tr's src/rules/proper-names.ts: the TDK rules of proper names, their capitals and their
// apostrophes, with the closed lists they use. Its regular expressions are the TypeScript's as RE2 says them; the
// one with a lookahead is code (fixNumberSuffix).

// Months are the month names, capitalized per TDK only when tied to a specific date ("17 Aralık'a").
var Months = NewWordSet(
	"ocak", "şubat", "mart", "nisan", "mayıs", "haziran", "temmuz", "ağustos", "eylül", "ekim", "kasım",
	"aralık",
)

// Weekdays are the weekday names, capitalized per TDK only when tied to a specific date ("12 Haziran 2027
// Cumartesi").
var Weekdays = NewWordSet(
	"pazartesi", "salı", "çarşamba", "perşembe", "cuma", "cumartesi", "pazar",
)

// InstitutionHeads are TDK institution, organization, board, assembly and workplace head words (singular or
// P3sg). Suffixes on them take no apostrophe per TDK ("Türk Dil Kurumundan", "Ankara Üniversitesine", "Bakanlar
// Kurulunun", "Mavi Köşe Bakkaliyesinden"); the exceptions are "Avrupa Birliği'ne" (TDK's explicit one) and "Rusya
// Federasyonu'na" (a state's name).
var InstitutionHeads = setOf(
	"kurumu", "kuruluşu", "kurulu", "bakanlığı", "müdürlüğü", "başkanlığı", "üniversitesi", "fakültesi",
	"enstitüsü", "yüksekokulu", "okulu", "ilkokulu", "ortaokulu", "lisesi", "koleji", "akademisi", "bölümü",
	"rektörlüğü", "dekanlığı", "meclisi", "belediyesi", "valiliği", "kaymakamlığı", "muhtarlığı", "mahkemesi",
	"savcılığı", "başsavcılığı", "barosu", "komisyonu", "komitesi", "heyeti", "konseyi", "senatosu", "oturumu",
	"birleşimi", "hastanesi", "polikliniği", "kliniği", "eczanesi", "kütüphanesi", "tiyatrosu", "tiyatroları",
	"operası", "balesi", "konservatuvarı", "müzesi", "bankası", "borsası", "ajansı", "vakfı", "derneği",
	"odası", "sendikası", "federasyonu", "konfederasyonu", "birliği", "partisi", "teşkilatı", "örgütü",
	"idaresi", "müsteşarlığı", "koordinatörlüğü", "temsilciliği", "konsolosluğu", "büyükelçiliği", "elçiliği",
	"komutanlığı", "emniyeti", "şirketi", "holdingi", "ortaklığı", "kooperatifi", "işletmesi", "fabrikası",
	"atölyesi", "matbaası", "yayınevi", "kitabevi", "basımevi", "öğretmenevi", "polisevi", "orduevi",
	"mağazası", "marketi", "bakkaliyesi", "pastanesi", "fırını", "lokantası", "restoranı", "kafesi", "oteli",
	"kulübü", "yolları", "havayolları", "demiryolları", "karayolları", "petrolleri", "enerjisi", "telekomu",
	"cemiyeti", "çiftliği",
)

// OfficerTitleHeads are officer and leader titles right after a proper institution ("Türk Dil Kurumu Başkanı'na",
// "Ankara Üniversitesi Rektörü'ne").
var OfficerTitleHeads = setOf(
	"başkanı", "müdürü", "rektörü", "dekanı", "bakanı", "komutanı", "başsavcısı", "savcısı", "sekreteri",
	"sözcüsü", "temsilcisi", "müsteşarı", "valisi", "kaymakamı", "yardımcısı",
)

// OfficerTitleMods are the first words of a two-word officer title after an institution ("Genel Müdürü", "Genel
// Başkanı", "Genel Sekreteri", "Başkan Yardımcısı").
var OfficerTitleMods = setOf(
	"genel", "başkan", "belediye", "okul", "bölüm", "cumhuriyet", "meclis", "yüksek",
)

// GenericAloneInstHeads are institution heads capitalized only with a proper-name anchor before them: alone they
// are generic ("tıp fakültesi").
var GenericAloneInstHeads = setOf(
	"fakültesi", "bölümü", "okulu", "ilkokulu", "ortaokulu", "lisesi", "enstitüsü", "yüksekokulu", "hastanesi",
	"kliniği", "polikliniği", "eczanesi", "kütüphanesi", "müdürlüğü", "başkanlığı", "komisyonu", "komitesi",
	"mahkemesi", "odası", "derneği", "vakfı", "sendikası", "şirketi", "kooperatifi", "işletmesi", "fabrikası",
	"atölyesi", "mağazası", "marketi", "fırını", "lokantası", "restoranı", "kafesi", "oteli", "kulübü",
	"partisi", "birliği", "meclisi", "kurulu", "kurumu", "bankası", "ajansı", "belediyesi", "valiliği",
	"kaymakamlığı", "tiyatrosu", "operası", "müzesi", "merkezi", "ortaklığı", "pastanesi", "bakkaliyesi",
	"yolları", "petrolleri", "üniversitesi", "bakanlığı", "koleji", "akademisi",
)

// GenericInstModifiers are modifier words that alone, without a proper-name anchor, make lowercase generic phrases
// like "tıp fakültesi", "devlet hastanesi", "ilçe belediyesi".
var GenericInstModifiers = setOf(
	"tıp", "hukuk", "mühendislik", "fen", "edebiyat", "iktisat", "işletme", "eğitim", "mimarlık", "eczacılık",
	"ilahiyat", "iletişim", "ziraat", "veteriner", "turizm", "devlet", "özel", "şehir", "ilçe", "il", "bölge",
	"araştırma", "semt", "mahalle", "köy", "merkez", "genel", "yeni", "eski", "büyük", "küçük", "ilk", "son",
	"sağlık", "kültür", "spor", "sanat", "bilim", "çocuk", "kadın", "erkek", "kız", "askeri", "askerî",
	"belediye", "halk", "kamu", "anonim", "limited", "ticaret", "siyasal", "bilgiler", "idari", "iktisadi",
	"bilimler", "dil", "dili", "edebiyatı", "tarih", "matematik", "fizik", "kimya", "biyoloji", "psikoloji",
	"sosyoloji", "felsefe", "coğrafya", "bilgisayar", "elektrik", "elektronik", "makine", "inşaat", "endüstri",
	"gıda", "çevre", "orman", "maden", "jeoloji", "harita", "tekstil", "diş", "hekimliği", "hemşirelik",
	"güzel", "sanatlar", "sahne", "müzik", "ticari", "meslek", "teknik", "anadolu", "imam", "hatip", "sosyal",
	"güvenlik", "üst", "düzenleme", "denetleme", "bankacılık", "radyo", "televizyon", "hava", "demir", "kara",
	"su", "işleri", "petrolleri", "cumhuriyet", "millet", "bakanlar", "yüksek", "öğretim", "yükseköğretim",
	"millî", "milli", "içişleri", "dışişleri", "adalet", "maliye", "hazine", "sanayi", "teknoloji", "tarım",
	"şehircilik", "ulaştırma", "altyapı", "çalışma", "gençlik", "aile", "savunma", "enerji", "tabii",
	"kaynaklar", "meteoroloji", "emniyet", "diyanet", "büyükşehir", "köşe", "mavi", "çam", "sakura", "dokuz",
	"eylül", "on", "sekiz", "mart", "ondokuz", "mayıs", "yirmi", "sütçü", "namık", "orta", "doğu", "batı",
	"kuzey", "güney", "vakıf", "iş", "garanti", "yapı", "kredi",
)

// NonInstPrefix are words that cannot be the unanchored first word of a compositional institution name
// (postpositions, verbs, determiners, adjectives).
var NonInstPrefix = setOf(
	"bir", "bu", "şu", "o", "her", "hiç", "bazı", "tüm", "bütün", "diğer", "başka", "aynı", "yeni", "eski",
	"iyi", "kötü", "güzel", "büyük", "küçük", "uzun", "kısa", "ilk", "son", "ikinci", "üçüncü", "geçen",
	"gelecek", "yarın", "bugün", "dün", "sabah", "akşam", "gece", "hafta", "ay", "yıl", "için", "göre", "kadar",
	"sonra", "önce", "karşı", "doğru", "boyunca", "hakkında", "üzerine", "ile", "ve", "veya", "ama", "fakat",
	"ancak", "çünkü", "de", "da", "ki", "mi", "mı", "mu", "mü", "çok", "az", "daha", "en", "pek", "hem", "ne",
	"ya", "ben", "sen", "biz", "siz", "onlar", "benim", "senin", "onun", "bizim", "sizin", "onların", "kendi",
	"köşedeki", "mahalledeki", "yakındaki", "karşıdaki", "yanındaki", "buradaki", "oradaki", "şuradaki",
	"bizimki", "yıllık", "devlet", "özel", "tıp", "hukuk", "mühendislik", "eğitim", "sağlık", "spor", "kültür",
	"semt", "il", "ilçe", "köy", "mahalle", "şehir", "bölge", "merkez", "genel",
)

// GeoHeads are TDK geographic, urban and landmark head words, capitalized and taking an apostrophe after a proper
// name.
var GeoHeads = setOf(
	"dağı", "dağları", "sıradağları", "gölü", "gölleri", "denizi", "okyanusu", "okyanus", "nehri", "ırmağı",
	"deresi", "boğazı", "körfezi", "kanalı", "şelalesi", "yarımadası", "takımadası", "burnu", "ovası",
	"yaylası", "platosu", "kanyonu", "çölü", "ormanı", "mağarası", "havzası", "mahallesi", "caddesi", "sokağı",
	"bulvarı", "meydanı", "parkı", "şehitliği", "sarayı", "köşkü", "hisarı", "köprüsü", "kulesi", "camisi",
	"camii", "türbesi", "anıtı", "katedrali", "manastırı", "tapınağı", "külliyesi", "kervansarayı",
	"havalimanı", "garı", "stadyumu",
)

// StateHeads are state and polity head words, capitalized and taking an apostrophe after a proper name.
var StateHeads = setOf(
	"cumhuriyeti", "devleti", "imparatorluğu", "krallığı", "prensliği", "beyliği", "hanlığı", "sultanlığı",
	"kağanlığı",
)

// EventHeads are head words of historical events, wars, treaties and eras, capitalized and taking an apostrophe
// after a proper place or nationality.
var EventHeads = setOf(
	"savaşı", "muharebesi", "antlaşması", "anlaşması", "ihtilali", "devrimi", "fermanı", "göçü", "devri",
)

// Directions are direction words capitalized before a proper place ("Batı Anadolu", "Doğu Karadeniz"), never
// after it ("Anadolu'nun batısı").
var Directions = setOf(
	"doğu", "batı", "kuzey", "güney", "orta", "iç", "kuzeydoğu", "kuzeybatı", "güneydoğu", "güneybatı",
)

// PreTitlesAbbr are abbreviated titles before a person's name ("Prof. Dr. Halil İnalcık", "Av. Melike Sönmez",
// "Op. Dr. Sinan Güler").
var PreTitlesAbbr = setOf(
	"prof", "doç", "doc", "yrd", "op", "uzm", "dr", "av", "öğr", "ogr", "gör", "gor", "müh", "muh", "mim",
	"ecz", "dt", "vet", "arb", "sn", "hz", "mr", "mrs", "ms",
)

// PreTitlesFull are titles and ranks written out, capitalized before a person's name ("Cumhurbaşkanı İsmet İnönü",
// "Albay Nurettin Şahin", "Bayan Elif Tan").
var PreTitlesFull = setOf(
	"bay", "bayan", "cumhurbaşkanı", "başbakan", "bakan", "vali", "kaymakam", "başkan", "müdür", "dekan",
	"rektör", "profesör", "doçent", "doktor", "avukat", "hâkim", "hakim", "savcı", "başsavcı", "komiser",
	"başkomiser", "müfettiş", "kaptan", "albay", "yarbay", "binbaşı", "yüzbaşı", "üsteğmen", "teğmen",
	"asteğmen", "astsubay", "başçavuş", "çavuş", "general", "orgeneral", "korgeneral", "tümgeneral",
	"tuğgeneral", "mareşal", "amiral", "oramiral", "koramiral", "tümamiral", "tuğamiral", "komutan",
	"büyükelçi", "başkonsolos", "konsolos", "başhekim", "başöğretmen", "sultan", "padişah", "şehzade", "kral",
	"kraliçe", "prens", "prenses", "imparator", "çar", "mimar", "gazi", "hazreti", "şeyh", "imam", "hacı",
)

// SayinTitles are official titles capitalized right after "Sayın" ("Sayın Başkan", "Sayın Müdür", "Sayın
// Büyükelçi"; not "Sayın hocam").
var SayinTitles = setOf(
	"başkan", "başkanım", "müdür", "müdürüm", "dekan", "dekanım", "rektör", "rektörüm", "vali", "valim",
	"kaymakam", "bakan", "bakanım", "büyükelçi", "komutan", "komutanım", "cumhurbaşkanı", "başbakan",
	"milletvekili", "savcı", "başsavcı", "hâkim", "hakim", "avukat", "profesör", "doçent", "doktor", "yargıç",
	"elçi", "konsolos", "başkonsolos", "general", "amiral", "albay", "müsteşar", "başhekim", "üyeler",
	"konuklar", "misafirler", "veliler", "öğrenciler", "katılımcılar", "dinleyiciler", "izleyiciler",
	"seyirciler", "milletvekilleri", "öğretmenler", "meslektaşlar", "meslektaşlarım", "hanımlar", "beyler",
)

// SayinCompoundFirst are the first words of a two-word official title after "Sayın" ("Sayın Genel Müdür", "Sayın
// Belediye Başkanı", "Sayın Cumhuriyet Başsavcısı").
var SayinCompoundFirst = setOf(
	"genel", "belediye", "okul", "bölüm", "cumhuriyet", "meclis", "kurul", "devlet", "millî", "milli",
)

// SayinCompoundTail are the second or third words that continue an official title after "Sayın" ("Müdür
// Yardımcısı", "Genel Sekreter", "Belediye Başkanı").
var SayinCompoundTail = setOf(
	"başkanı", "müdürü", "savcısı", "başsavcısı", "sekreteri", "sekreter", "yardımcısı", "kurulu", "üyesi",
	"müdür", "başkan", "bakanı", "komutanı", "vekil", "vekili",
)

// PostHonorifics are honorific titles after a person's name, capitalized and taking an apostrophe ("Ayşe Hanım'a",
// "Orhan Bey'de"), in the order they are tried.
var PostHonorifics = NewWordSet(
	"bey", "hanım", "efendi", "paşa", "ağa", "beyefendi", "hanımefendi",
)

// PostTitlesWithApos are occupational and historical titles after a person's name, capitalized and taking an
// apostrophe ("Ahmet Hoca'ya", "Mehmet Usta'nın", "Ali Çavuş'un", "Seyit Onbaşı'yı", "Temel Kaptan'a"), in the
// order they are tried.
var PostTitlesWithApos = NewWordSet(
	"hoca", "usta", "öğretmen", "çavuş", "onbaşı", "reis", "çelebi", "hatun", "kaptan",
)

// KinshipStems are kinship words after a person's name: they stay lowercase per TDK ("Tülay ablama", "Zehra
// halamın") and prove the word before is a person's name.
var KinshipStems = NewWordSet(
	"abla", "abi", "ağabey", "hala", "teyze", "amca", "dayı", "enişte", "yenge", "dede", "nine", "babaanne",
	"anneanne",
)

// PersonRelBefore are nominative relationship and occupation words right before a person's name ("arkadaşım
// Melek", "kardeşim Ateş", "komşumuz Bahar'ın", "yazar Defne Arslan").
var PersonRelBefore = setOf(
	"arkadaşım", "arkadaşımız", "komşum", "komşumuz", "kardeşim", "abim", "ablam", "ağabeyim", "kuzenim",
	"yeğenim", "oğlum", "kızım", "eşim", "sevgilim", "nişanlım", "dostum", "öğrencim", "öğrencimiz",
	"meslektaşım", "ortağım", "yazar", "şair", "ressam", "yönetmen", "oyuncu", "şarkıcı", "müzisyen",
	"futbolcu", "basketbolcu", "voleybolcu", "sporcu", "sanatçı", "gazeteci", "adı",
)

// MediaTypeStems are lowercase media and work type words after a capitalized title of a work or publication
// ("Varlık dergisine", "Muhsin Bey filmini").
var MediaTypeStems = NewWordSet(
	"dergi", "gazete", "roman", "kitap", "film", "dizi", "oyun", "şiir", "öykü", "hikâye", "hikaye", "eser",
	"albüm", "şarkı", "tablo", "opera",
)

// LanguageNames are language names: always capitalized, never with an apostrophe before suffixes ("Türkçeyi",
// "Almancaya", "Yunancanın").
var LanguageNames = setOf(
	"türkçe", "almanca", "ingilizce", "fransızca", "ispanyolca", "italyanca", "portekizce", "rusça", "arapça",
	"farsça", "yunanca", "japonca", "çince", "korece", "moğolca", "türkmence", "gürcüce", "lazca", "osmanlıca",
	"latince", "sanskritçe", "felemenkçe", "hollandaca", "kürtçe", "ermenice", "azerice", "özbekçe", "kazakça",
	"kırgızca", "uygurca", "tatarca", "bulgarca", "sırpça", "hırvatça", "boşnakça", "arnavutça", "macarca",
	"çekçe", "slovakça", "slovence", "rumence", "isveççe", "norveççe", "danca", "fince", "izlandaca",
	"ukraynaca", "belarusça", "litvanca", "letonca", "estonca", "katalanca", "baskça", "galce", "irlandaca",
	"iskoçça", "ibranice", "hintçe", "urduca", "bengalce", "tayca", "vietnamca", "endonezce", "malayca",
	"svahilice", "amharca", "süryanice", "aramice", "kıptice", "akatça", "hititçe", "sümerce", "göktürkçe",
	"çağatayca", "kıpçakça", "oğuzca", "esperanto", "abazaca", "acemce", "çerkezce", "abhazca", "osetçe",
	"zazaca", "rumca", "pomakça", "gagavuzca", "yakutça", "çuvaşça", "tuvaca", "hakasça", "altayca",
	"başkurtça", "kumukça", "karaçayca", "nogayca", "karakalpakça", "peştuca", "tacikçe", "beluçça", "pencapça",
	"tamilce", "teluguca", "kannadaca", "malayalamca", "maratice", "güceratça", "nepalce", "sinhalaca",
	"birmanca", "khmerce", "laoca", "tagalogca", "endonezyaca", "maltaca", "afrikanca", "zuluca", "somalice",
	"hausaca", "yorubaca", "igboca",
)

// NoAposReligionOrDerived are proper nouns of religions, sects and ideologies derived with Turkish suffixes
// (-lIk/-lI/-cI), which take no apostrophe per TDK ("Hristiyanlıkta", "Müslümanlık").
var NoAposReligionOrDerived = setOf(
	"hristiyanlık", "müslümanlık", "yahudilik", "musevilik", "türklük", "osmanlılık", "atatürkçülük",
	"zerdüştlük", "protestanlık", "katoliklik", "ortodoksluk", "alevilik", "sünnilik", "şiilik", "hanefilik",
	"şafiilik", "osmanlıcılık", "türkçülük", "avrupalı", "asyalı", "afrikalı", "amerikalı", "anadolulu",
	"istanbullu", "ankaralı", "izmirli", "bursalı", "adanalı", "antalyalı", "konyalı", "trabzonlu", "rizeli",
	"erzurumlu", "diyarbakırlı", "eskişehirli", "tokatlı", "galatasaraylı", "fenerbahçeli", "beşiktaşlı",
	"trabzonsporlu",
)

// OptionalAposIsmIyet are foreign and Arabic -izm/-iyet names of religions and movements: capitalized without
// forcing an apostrophe when joined ("hinduizmde" -> "Hinduizmde"), but an apostrophe typed is never taken away
// ("İslamiyet'e", "Budizm'in", "Hinduizm'de" stay).
var OptionalAposIsmIyet = setOf(
	"islamiyet", "hinduizm", "budizm", "taoizm", "şintoizm", "sihizm", "marksizm", "leninizm", "kemalizm",
)

// PluralPlaceKeepApos are proper names whose own root ends in -lar/-ler (places, ranges), which keep an
// apostrophe.
var PluralPlaceKeepApos = setOf(
	"bağcılar", "avcılar", "adalar", "toroslar", "alpler", "balkanlar", "andlar", "himalayalar", "pireneler",
	"karpatlar", "kırklar", "atalar", "kavaklar", "ışıklar", "çınarlar", "kayalar", "konaklar", "sarılar",
	"yaylalar", "pınarlar", "taşlar", "kızıllar", "bayırlar", "kuzular", "doğular", "çamlar", "güller",
)

// DerivLookingKeepApos are proper names whose own root ends in -lı/-li/-lık/-ca/-ce, which keep an apostrophe.
var DerivLookingKeepApos = setOf(
	"denizli", "kocaeli", "tunceli", "kırklareli", "rumeli", "şişli", "aydınlı", "osmanlı", "selçuklu",
	"karamanlı", "mali", "şili", "bali", "kandilli", "tuzla", "ayvalık", "gemlik", "susurluk", "bilecik",
	"zonguldak", "karabük", "çankırı", "karaca", "çamlıca", "sütlüce", "büyükçekmece", "küçükçekmece",
	"kızılca", "yenice", "ali", "veli", "aslı", "nazlı", "ece", "tülay", "hüsnü", "şükrü", "binali", "hayali",
	"karaköy", "kadıköy", "bakırköy", "kayaköy", "urla", "muğla", "amasra",
)

// TitleConjunctions are the lowercase conjunctions inside a multi-word proper name ("ve", "ile", "ya", "veya",
// "de", "da").
var TitleConjunctions = setOf(
	"ve", "ile", "ya", "veya", "yahut", "da", "de", "ki", "mi", "mı", "mu", "mü",
)

// inverseFrontStems are the stems with inverse front-vowel harmony ("Kemal'e", "Cemal'in", "Hilal'e", "İhtilali",
// "Resul'e").
var inverseFrontStems = setOf(
	"kemal", "cemal", "hilal", "bilal", "nihal", "zuhal", "hayal", "celal", "delal",
	"istiklal", "ihtilal", "resul", "kabil", "hal", "hâl", "saat", "kalp", "harf",
	"alkol", "rol", "gol", "sembol", "kontrol", "petrol", "protokol", "bandrol", "kabul",
	"usul", "meşgul", "makul", "mahsul", "kefil", "sefil", "nesil", "fasıl", "asıl",
)

// IsInstitutionHead reports whether a stem is an institution head in its context, minding the TDK exceptions
// "Avrupa Birliği", "Rusya Federasyonu", "Kutup Dairesi" and "Elçilerin İşleri" (prevLower is the word before, ""
// for none).
func IsInstitutionHead(lowerStem, prevLower string) bool {
	if !InstitutionHeads[lowerStem] {
		return false
	}
	if lowerStem == "birliği" && prevLower == "avrupa" {
		return false
	}
	if lowerStem == "federasyonu" && prevLower == "rusya" {
		return false
	}
	if lowerStem == "dairesi" && prevLower == "kutup" {
		return false
	}
	if lowerStem == "işleri" && prevLower == "elçilerin" {
		return false
	}
	return true
}

var (
	vowelInitialSuffix = regexp.MustCompile(`^[aeıioöuü]`)
	likFinal           = regexp.MustCompile(`l[ıiuü]k$`)
)

// softened are the final consonants that soften before a vowel, and what they become.
var softened = map[string]string{"k": "ğ", "p": "b", "ç": "c", "t": "d"}

// JoinWithoutApostrophe joins stem and suffix when a wrong apostrophe is taken away, softening the final consonant
// (k->ğ, p->b, ç->c, t->d) before a vowel-initial suffix where Turkish does ("Hristiyanlık'ın" -> "Hristiyanlığın",
// "Atatürkçülük'ün" -> "Atatürkçülüğün", "kitap'ı" -> "kitabı", "ağaç'a" -> "ağaca"). isMorphWord tells the words
// the morphology knows.
func JoinWithoutApostrophe(stem, suffix string, isMorphWord func(lower string) bool) string {
	if stem == "" || suffix == "" || !vowelInitialSuffix.MatchString(tr.Lower(suffix)) {
		return stem + suffix
	}
	last := tr.Lower(sliceFrom(stem, -1))
	soft, ok := softened[last]
	if !ok || runeLen(stem) < 4 {
		return stem + suffix
	}
	plainJoined := stem + suffix
	softJoined := sliceRunes(stem, 0, -1) + soft + suffix
	lowPlain := tr.Lower(plainJoined)
	lowSoft := tr.Lower(softJoined)
	// Turkish -lIk/-lik/-luk/-lük always softens before a vowel ("Hristiyanlığın", "Atatürkçülüğün", "Başbakanlığa").
	if last == "k" && likFinal.MatchString(tr.Lower(stem)) {
		return softJoined
	}
	if isMorphWord(lowSoft) && !isMorphWord(lowPlain) {
		return softJoined
	}
	return plainJoined
}

// numberUnitBlock are units and codes glued to a number that are not Turkish suffixes ("5cm", "20px").
var numberUnitBlock = setOf(
	"cm", "mm", "km", "kg", "mg", "gr", "lt", "ml", "gb", "mb", "kb", "tb", "pb",
	"hz", "khz", "mhz", "ghz", "kw", "mw", "gw", "kwh", "ms", "dk", "sn", "sa",
	"tl", "usd", "eur", "px", "pt", "em", "rem", "vh", "vw", "bit", "fps", "rpm",
	"mp", "db", "mah", "cc", "st", "nd", "rd", "th", "am", "pm", "bc", "ad", "id",
	"d", "k", "g", "x", "m", "s", "h", "v", "w", "a", "b", "c", "e", "f", "l", "n", "p", "r", "t",
)

// numberPhonology is how an integer ends when read aloud in Turkish: its last vowel, whether it ends in a vowel,
// and whether in a voiceless consonant.
type numberPhonology struct {
	lastV      string
	vowelFinal bool
	voiceless  bool
}

// turkishNumberPhonology is how the integer numStr ends when read aloud, or false when it is not one it knows.
func turkishNumberPhonology(numStr string) (numberPhonology, bool) {
	digits := strings.TrimLeft(numStr, "0")
	if digits == "" {
		digits = "0"
	}
	switch sliceFrom(digits, -1) {
	case "1":
		return numberPhonology{"i", false, false}, true // bir
	case "2":
		return numberPhonology{"i", true, false}, true // iki
	case "3":
		return numberPhonology{"ü", false, true}, true // üç
	case "4":
		return numberPhonology{"ö", false, true}, true // dört
	case "5":
		return numberPhonology{"e", false, true}, true // beş
	case "6":
		return numberPhonology{"ı", true, false}, true // altı
	case "7":
		return numberPhonology{"i", true, false}, true // yedi
	case "8":
		return numberPhonology{"i", false, false}, true // sekiz
	case "9":
		return numberPhonology{"u", false, false}, true // dokuz
	}
	// The number ends in 0.
	switch {
	case digits == "0":
		return numberPhonology{"ı", false, false}, true // sıfır
	case strings.HasSuffix(digits, "000000000"):
		return numberPhonology{"a", false, false}, true // milyar
	case strings.HasSuffix(digits, "000000"):
		return numberPhonology{"o", false, false}, true // milyon
	case strings.HasSuffix(digits, "000"):
		return numberPhonology{"i", false, false}, true // bin
	case strings.HasSuffix(digits, "00"):
		return numberPhonology{"ü", false, false}, true // yüz
	}
	switch sliceRunes(digits, -2, -1) {
	case "1":
		return numberPhonology{"o", false, false}, true // on
	case "2":
		return numberPhonology{"i", true, false}, true // yirmi
	case "3":
		return numberPhonology{"u", false, false}, true // otuz
	case "4":
		return numberPhonology{"ı", false, true}, true // kırk
	case "5":
		return numberPhonology{"i", true, false}, true // elli
	case "6":
		return numberPhonology{"ı", false, true}, true // altmış
	case "7":
		return numberPhonology{"i", false, true}, true // yetmiş
	case "8":
		return numberPhonology{"e", false, false}, true // seksen
	case "9":
		return numberPhonology{"a", false, false}, true // doksan
	}
	return numberPhonology{}, false
}

var (
	numberInflection  = regexp.MustCompile(`^(?:[dt][ae]n?|[dt][ae]ki(?:l[ae]r(?:[aeıiuü]|[dt][ae]n?|[ıiuü]n)?)?|l[ae]r(?:[dt][ae]n?|[dt][ae]ki|[aeıiuü]|[ıiuü]n)?|n?[ıiuü]n|y?[aeıiuü]|y?l[ae]|(?:[ıiuü]?nc[ıiuü])(?:s[ıiuü](?:n[aeıiuü]|nd[ae]n?)?|l[ae]r)?|[dt][ıiuü]r)$`)
	fourDigitYear     = regexp.MustCompile(`^(?:1[0-9]{3}|20[0-9]{2})$`)
	numberVowelSuffix = regexp.MustCompile(`^[aeıiuü]`)
	numberBareLa      = regexp.MustCompile(`^l[ae](?:$|[^r])`)
	numberOrdinal     = regexp.MustCompile(`^nc[ıiuü]`)
)

// FixNumberSuffix checks and fixes a Turkish inflectional or ordinal suffix on a number ("1985te" -> "1985'te",
// "2023de" -> "2023'te", "1985'de" -> "1985'te", "1990larda" -> "1990'larda", "2nci" -> "2'nci"): the suffix to
// put after the apostrophe, or false when it is left as it is or is not a suffix of the number.
func FixNumberSuffix(numStr, rawSuffix string, hadApostrophe bool) (string, bool) {
	if rawSuffix == "" || rawSuffix != tr.Lower(rawSuffix) {
		return "", false
	}
	if !hadApostrophe && numberUnitBlock[rawSuffix] {
		return "", false
	}
	// A one-letter suffix glued to anything but a four-digit year may be a section or a code ("3a", "2e", "5i").
	if !hadApostrophe && runeLen(rawSuffix) == 1 && !fourDigitYear.MatchString(numStr) {
		return "", false
	}
	if !numberInflection.MatchString(rawSuffix) {
		return "", false
	}
	ph, ok := turkishNumberPhonology(numStr)
	if !ok {
		return "", false
	}

	// The suffix's first vowel must follow the number's last.
	if firstV := firstOf(rawSuffix, "aeıioöuü"); firstV != "" && !FollowsHarmony(ph.lastV, firstV) {
		return "", false
	}

	// The buffer and initial consonants.
	if ph.vowelFinal {
		if numberVowelSuffix.MatchString(rawSuffix) {
			return "", false
		}
		if numberBareLa.MatchString(rawSuffix) {
			return "", false
		}
	} else {
		// /^(?:n(?!c)|y)/: a buffer "n" not starting "nc", or a buffer "y".
		if strings.HasPrefix(rawSuffix, "y") || strings.HasPrefix(rawSuffix, "n") && !strings.HasPrefix(rawSuffix, "nc") {
			return "", false
		}
		// Consonant-final numbers take -ıncı/-inci/-uncu/-üncü.
		if numberOrdinal.MatchString(rawSuffix) {
			return "", false
		}
	}

	// The voicing of d/t-initial case and copula suffixes ("1985de" -> "1985'te", "2023de" -> "2023'te").
	fixed := rawSuffix
	if strings.HasPrefix(fixed, "d") && ph.voiceless {
		fixed = "t" + fixed[1:]
	} else if strings.HasPrefix(fixed, "t") && !ph.voiceless {
		fixed = "d" + fixed[1:]
	}
	if hadApostrophe && fixed == rawSuffix {
		return "", false
	}
	return fixed, true
}

// NonApostropheSuffixRE are the suffixes starting with a TDK derivational or plural suffix, never after an
// apostrophe on a proper noun that is not an acronym.
var NonApostropheSuffixRE = regexp.MustCompile(`^(?:l[ae]r|l[ıiuü]k|l[ıiuü](?:k|l[ae]r|y?m[ıiuü]ş|y?d[ıiuü]|y?s[ae]|y?k[ae]n|y?[aeıiuü]|n[ıiuü]n|d[ae]n?|d[ae]ki|yl[ae]|d[ıiuü]r|$)|[cç][ae](?:l[ae]r|s[ıiuü]|d[ae]n?|d[ae]ki|n[ıiuü]n|y?[aeıiuü]|yl[ae]|d[ıiuü]r|$)|[cç][ıiuü](?:l[ıiuü]k|l[ae]r|s[ıiuü]|d[ae]n?|d[ae]ki|n[ıiuü]n|y?[aeıiuü]|yl[ae]|d[ıiuü]r|$)|s[ae]l|s[ıiuü]z)`)

// ProperInflectionalSuffixRE are the Turkish inflectional suffixes attached with an apostrophe to a bare proper
// noun: case, possessive with case (the buffer "n" required after the 3sg -sI/-I), -ki with plural and case, the
// instrumental, and copula and predicative endings.
var ProperInflectionalSuffixRE = regexp.MustCompile(`^(?:(?:[ıiuü]m(?:[ıiuü]z)?|m[ıiuü]z|[ıiuü]n(?:[ıiuü]z)?|n[ıiuü]z)(?:[dt][ae]n?|[dt][ae]ki(?:l[ae]r(?:[aeıiuü]|[dt][ae]n?|[ıiuü]n)?)?|[aeıiuü]|[ıiuü]n|l[ae])?|s[ıiuü](?:n[dt][ae]n?|n[dt][ae]ki(?:l[ae]r(?:[aeıiuü]|[dt][ae]n?|[ıiuü]n)?)?|n[aeıiuü]|n[ıiuü]n|yl[ae])?|[ıiuü](?:n[dt][ae]n?|n[dt][ae]ki(?:l[ae]r(?:[aeıiuü]|[dt][ae]n?|[ıiuü]n)?)?|n[aeıiuü]|n[ıiuü]n|yl[ae])|[dt][ae]ki(?:l[ae]r(?:[aeıiuü]|[dt][ae]n?|[ıiuü]n)?|n[aeıiuü]|n[dt][ae]n?|n[ıiuü]n)?|n?[ıiuü]nki(?:l[ae]r(?:[aeıiuü]|[dt][ae]n?|[ıiuü]n)?|n[aeıiuü]|n[dt][ae]n?|n[ıiuü]n)?|n?[ıiuü]n|[dt][ae]n?|y?l[ae](?:yd[ıiuü][mk]?|ym[ıiuü]ş[ıiuü]?[mn]?|ys[ae][mk]?)?|y?[aeıiuü]|[dt][ıiuü]r|yd[ıiuü][mnk]?|ym[ıiuü]ş|ys[ae]|yken|s[ıiuü]n[ıiuü]z|s[ıiuü]n|y[ıiuü]m|y[ıiuü]z)$`)

// HeadSuffixRE are the inflectional suffixes on a P3sg vowel-final head word ("Üniversitesi" + "nde", "Bakanlığı"
// + "na", "Parkı" + "nda", "Camisi" + "nin", "Gazete" + "de", "Savaşı" + "nı").
var HeadSuffixRE = regexp.MustCompile(`^(?:n[ıiuü]n|nd[ae]n?|nd[ae]ki(?:l[ae]r(?:[aeıiuü]|[dt][ae]n?|[ıiuü]n)?)?|n[aeıiuü]|yl[ae]|d[ae]n?|d[ae]ki|y[aeıiuü]|d[ıiuü]r|yd[ıiuü]|ym[ıiuü]ş|ys[ae])$`)

// NoAposProperSuffixRE are the suffixes after a proper noun that takes no apostrophe (languages, nationalities,
// religions, derivations, plurals).
var NoAposProperSuffixRE = regexp.MustCompile(`^(?:(?:l[ae]r)?(?:(?:[ıiuü]m(?:[ıiuü]z)?|[ıiuü]n(?:[ıiuü]z)?|s?[ıiuü])(?:n?[dt][ae]n?|n?[dt][ae]ki(?:l[ae]r(?:[aeıiuü]|[dt][ae]n?|[ıiuü]n)?)?|n?[aeıiuü]|n[ıiuü]n|y?l[ae])?|[dt][ae]ki(?:l[ae]r(?:[aeıiuü]|[dt][ae]n?|[ıiuü]n)?|n[aeıiuü]|n[dt][ae]n?|n[ıiuü]n)?|n?[ıiuü]nki(?:l[ae]r)?|n?[ıiuü]n|[dt][ae]n?|y?l[ae]|y?[aeıiuü]|[dt][ıiuü]r|y?d[ıiuü]|y?m[ıiuü]ş|y?s[ae]|y?ken)?)$`)

var (
	vowelRE      = regexp.MustCompile(`[aeıioöuüâîû]`)
	vowelFinalRE = regexp.MustCompile(`[aeıioöuüâîû]$`)
)

// IsInverseFrontStem reports whether a stem takes front vowels against its own ("Kemal'e", "alkol'ü").
func IsInverseFrontStem(stem string) bool {
	if inverseFrontStems[stem] {
		return true
	}
	return strings.HasSuffix(stem, "al") && inverseFrontStems[sliceFrom(stem, -5)]
}

// ProperSuffixOptions say how IsPhonologicallyValidProperSuffix reads a stem: as an acronym, said letter by
// letter, which relaxes harmony; or as a P3sg head word ("Parkı", "Camisi"), which takes the pronominal "n".
type ProperSuffixOptions struct {
	Acronym  bool
	P3sgHead bool
}

var (
	acronymPlural      = regexp.MustCompile(`^l[ae]r`)
	p3sgHeadSuffix     = regexp.MustCompile(`^(?:n[ıiuü]n|nd[ae]n?|nd[ae]ki|n[aeıiuü]|yl[ae]|d[ıiuü]r|yd[ıiuü]|ym[ıiuü]ş|ys[ae])`)
	bareVowelSuffix    = regexp.MustCompile(`^[aeıiuü]`)
	bareLaSuffix       = regexp.MustCompile(`^l[ae](?:$|y)`)
	highMSuffix        = regexp.MustCompile(`^[ıiuü]m`)
	consonantBufferSfx = regexp.MustCompile(`^(?:n|y|s[ıiuü]|m[ıiuü]z)`)
)

// IsPhonologicallyValidProperSuffix reports whether suffix attaches to stem by the sounds of Turkish: vowel
// harmony, buffer letters, voicing.
func IsPhonologicallyValidProperSuffix(stem, suffix string, options ProperSuffixOptions) bool {
	if suffix == "" {
		return false
	}
	if options.Acronym {
		return ProperInflectionalSuffixRE.MatchString(suffix) || acronymPlural.MatchString(suffix)
	}
	vowelFinal := vowelFinalRE.MatchString(stem)
	// The buffer letters.
	if options.P3sgHead && vowelFinal {
		// P3sg heads ("Parkı", "Camisi", "Üniversitesi", "Bakanlığı", "Savaşı") take the pronominal "n" before case
		// and the genitive, "y" before -la/-le.
		if !p3sgHeadSuffix.MatchString(suffix) {
			return false
		}
	} else if vowelFinal {
		// A vowel-final bare stem takes no bare vowel-initial case (-a/-e/-ı/-i/-u/-ü/-ın/-in), but for the rare
		// silent "e" of foreign names.
		if bareVowelSuffix.MatchString(suffix) && !strings.HasSuffix(stem, "e") {
			return false
		}
		// Nor a bare -la/-le: TDK writes "Ayşe'yle" and "Ankara'yla" (and "Bolu'dan", "Rize'nin", "Özge'yi",
		// "Türkiye'mizin").
		if bareLaSuffix.MatchString(suffix) {
			return false
		}
		// Nor the 1pl -ımız straight on the vowel ("Türkiye'miz", not "Türkiye'imiz").
		if highMSuffix.MatchString(suffix) {
			return false
		}
		// Nor a voicing-assimilated "t-" after a vowel.
		if strings.HasPrefix(suffix, "t") {
			return false
		}
	} else {
		// A consonant-final stem takes no buffer "n-" (-nın, -nda, -na), "y-" (-ya, -yı, -yla) or "-s-" (-sı,
		// -sini), nor a bare -mız/-miz.
		if consonantBufferSfx.MatchString(suffix) {
			return false
		}
		// The voicing of d-/t-initial suffixes.
		if strings.HasPrefix(suffix, "d") && IsVoicelessFinal(stem) {
			return false
		}
		if strings.HasPrefix(suffix, "t") && !IsVoicelessFinal(stem) {
			return false
		}
	}

	// The vowel harmony of the suffix's first vowel.
	if firstV := firstOf(suffix, "aeıioöuü"); firstV != "" {
		if IsInverseFrontStem(stem) {
			// Inverse front harmony allows e/i/ü.
			lv := LastVowel(stem)
			round := lv == "o" || lv == "u" || lv == "ö" || lv == "ü"
			if firstV == "a" || firstV == "e" {
				if firstV != "e" {
					return false
				}
			} else {
				want := "i"
				if round {
					want = "ü"
				}
				if firstV != want && !FollowsHarmony(stem, firstV) {
					return false
				}
			}
		} else if !FollowsHarmony(stem, firstV) {
			return false
		}
	}

	// The vowel harmony inside the suffix, the invariant -ki and -ken aside.
	check := suffix
	if ki := strings.Index(check, "ki"); ki != -1 {
		check = check[:ki]
	}
	check = strings.TrimSuffix(check, "ken")
	prev := "e"
	if !IsInverseFrontStem(stem) {
		if lv := LastVowel(stem); lv != "" {
			prev = lv
		}
	}
	for i, r := range []rune(check) {
		if strings.ContainsRune("aeıioöuü", r) {
			ch := string(r)
			if i > 0 && !FollowsHarmony(prev, ch) {
				return false
			}
			prev = ch
		}
	}
	return true
}

// HarmonizeASCIISuffix restores the Turkish vowels stripped from a suffix typed in ASCII so that it harmonizes
// with stem ("burak" + "in" -> "ın", "kibris" + "ta" -> "ta", "oya" + "nin" -> "nın", "turkiye" + "mizin" ->
// "mizin"): the harmonized suffix, or false when the suffix cannot harmonize with stem.
func HarmonizeASCIISuffix(stem, rawSuffix string, options ProperSuffixOptions) (string, bool) {
	if rawSuffix == "" {
		return "", false
	}
	suffixRE := ProperInflectionalSuffixRE
	if options.P3sgHead {
		suffixRE = HeadSuffixRE
	}
	if suffixRE.MatchString(rawSuffix) && IsPhonologicallyValidProperSuffix(stem, rawSuffix, options) {
		return rawSuffix, true
	}
	// Only ASCII i->ı, u->ü and s->ş are toggled, for a suffix typed without Turkish letters ("in" -> "ın",
	// "nin" -> "nın", "mis" -> "mış").
	head, tail := rawSuffix, ""
	if ki := strings.Index(rawSuffix, "ki"); ki != -1 {
		head, tail = rawSuffix[:ki], rawSuffix[ki:]
	}

	lv := "e"
	if !IsInverseFrontStem(stem) {
		lv = LastVowel(stem)
	}
	if lv == "" {
		return "", false
	}
	prevBack := strings.Contains("aıouâû", lv)
	prevRound := strings.Contains("ouöüû", lv)
	var out strings.Builder
	hr := []rune(head)
	for i, ch := range hr {
		switch {
		case ch == 'i' && prevBack:
			if prevRound {
				ch = 'u'
			} else {
				ch = 'ı'
			}
		case ch == 'u' && !prevBack:
			ch = 'ü'
		case ch == 's' && i == len(hr)-1 && i >= 2 && endsWithAny(out.String(), "mı", "mi", "mu", "mü"):
			ch = 'ş'
		}
		out.WriteRune(ch)
		if strings.ContainsRune("aeıioöuü", ch) {
			prevBack = strings.ContainsRune("aıou", ch)
			prevRound = strings.ContainsRune("ouöü", ch)
		}
	}
	cand := out.String() + tail
	if cand != rawSuffix && ProperInflectionalSuffixRE.MatchString(cand) && IsPhonologicallyValidProperSuffix(stem, cand, options) {
		return cand, true
	}
	return "", false
}

// endsWithAny reports whether text ends with one of the suffixes.
func endsWithAny(text string, suffixes ...string) bool {
	for _, s := range suffixes {
		if strings.HasSuffix(text, s) {
			return true
		}
	}
	return false
}

var kinshipRest = regexp.MustCompile(`^(?:m(?:[aeıiuü]|d[ae]n?|[ıiuü]n|l[ae]|l[ae]r(?:[aeıiuü]|[ıiuü]n|d[ae]n?)?)?|n(?:[aeıiuü]|d[ae]n?|[ıiuü]n)?|s?[ıiuü](?:n[aeıiuü]|nd[ae]n?|n[ıiuü]n|yl[ae])?|y?[aeıiuü]|d[ae]n?|y?l[ae]|l[ae]r)?$`)

// IsKinshipWord reports whether a word is an inflected kinship word ("ablama", "halamın", "enişteme", "ninemin",
// "teyzem", "amcam").
func IsKinshipWord(lower string) bool {
	n := runeLen(lower)
	for _, k := range KinshipStems.Words() {
		if lower == k {
			return true
		}
		if kn := runeLen(k); strings.HasPrefix(lower, k) && n > kn && n <= kn+7 {
			if kinshipRest.MatchString(lower[len(k):]) {
				return true
			}
		}
	}
	return false
}

var mediaRest = regexp.MustCompile(`^(?:s?[ıiuü](?:n[aeıiuü]|nd[ae]n?|n[ıiuü]n|yl[ae])?|l[ae]r(?:[ıiuü]n?[aeıiuü]?|d[ae]n?)?|[dt][ae]n?|y?[aeıiuü]|n?[ıiuü]n|y?l[ae])$`)

// IsMediaTypeWord reports whether a word is an inflected media or work type word ("dergisine", "filmini",
// "dizisini", "romanını", "kitabını", "gazetesinde").
func IsMediaTypeWord(lower string) bool {
	n := runeLen(lower)
	for _, m := range MediaTypeStems.Words() {
		if lower == m {
			return true
		}
		bases := []string{m}
		if strings.HasSuffix(m, "p") {
			bases = append(bases, m[:len(m)-1]+"b")
		}
		for _, base := range bases {
			if bn := runeLen(base); strings.HasPrefix(lower, base) && n > bn && n <= bn+7 {
				if mediaRest.MatchString(lower[len(base):]) {
					return true
				}
			}
		}
	}
	return false
}

// CapTr capitalizes the first letter of a Turkish word the Turkish way ("i" -> "İ", "ı" -> "I").
func CapTr(word string) string {
	if word == "" {
		return word
	}
	rs := []rune(word)
	return tr.Upper(string(rs[0])) + string(rs[1:])
}

// asciiFold are the letters FoldTurkish takes the marks off.
var asciiFold = map[rune]rune{'ç': 'c', 'ğ': 'g', 'ı': 'i', 'ö': 'o', 'ş': 's', 'ü': 'u', 'â': 'a', 'î': 'i', 'û': 'u'}

// FoldTurkish lowercases a word the Turkish way and takes the marks off its letters, for lookups of text typed in
// ASCII.
func FoldTurkish(word string) string {
	return strings.Map(func(r rune) rune {
		if f, ok := asciiFold[r]; ok {
			return f
		}
		return r
	}, tr.Lower(word))
}

// TitleSplit is a title word cut into its head and its suffix.
type TitleSplit struct {
	Head          string
	DisplayHead   string
	Suffix        string
	HadApostrophe bool
}

var possessiveOrPlural = regexp.MustCompile(`^(?:[ıiuü]m|m[ıiuü]z|l[ae]r|s[ıiuü])`)

// splitTitle cuts a title of titles off a word, with an optional inflectional suffix: the 1sg, 1pl and 3sg
// possessives and the plural ("efendimiz", "efendisi", "paşası", "beyim", "beyler") are not ones, while the
// accusative after a person's name is ("Ayşe Hanım'ı", "Kerem Bey'i", "Paşa'yı").
func splitTitle(lower string, titles WordSet) (TitleSplit, bool) {
	valid := func(stem, suffix string) bool {
		return ProperInflectionalSuffixRE.MatchString(suffix) && !possessiveOrPlural.MatchString(suffix) &&
			IsPhonologicallyValidProperSuffix(stem, suffix, ProperSuffixOptions{})
	}
	if apos := strings.IndexByte(lower, '\''); apos > 0 {
		stem, suffix := lower[:apos], lower[apos+1:]
		if titles.Has(stem) && valid(stem, suffix) {
			return TitleSplit{Head: stem, DisplayHead: CapTr(stem), Suffix: suffix, HadApostrophe: true}, true
		}
		return TitleSplit{}, false
	}
	for _, h := range titles.Words() {
		if lower == h {
			return TitleSplit{Head: h, DisplayHead: CapTr(h)}, true
		}
		if strings.HasPrefix(lower, h) && len(lower) > len(h) {
			if suffix := lower[len(h):]; valid(h, suffix) {
				return TitleSplit{Head: h, DisplayHead: CapTr(h), Suffix: suffix}, true
			}
		}
	}
	return TitleSplit{}, false
}

// SplitHonorific cuts a bare or suffixed honorific ("bey", "beye", "beyi", "beyin", "beyde", "hanım", "hanımı",
// "hanımın", "hanımda", "efendiye", "paşanın", "ağa'nın").
func SplitHonorific(lower string) (TitleSplit, bool) { return splitTitle(lower, PostHonorifics) }

// SplitPostTitle cuts a title after a name (PostTitlesWithApos: "hoca", "usta", "çavuş", "onbaşı", "kaptan",
// "öğretmen", ...) with an optional inflectional suffix.
func SplitPostTitle(lower string) (TitleSplit, bool) { return splitTitle(lower, PostTitlesWithApos) }

// HeadSplit is a head word cut from its suffix.
type HeadSplit struct {
	Head          string
	Suffix        string
	HadApostrophe bool
}

// SplitHeadWord cuts an institution, geographic, state or event head word and its optional inflectional suffix
// ("üniversitesinde" -> "üniversitesi" + "nde").
func SplitHeadWord(lower string, headSet map[string]bool) (HeadSplit, bool) {
	if apos := strings.IndexByte(lower, '\''); apos > 0 {
		stem, suffix := lower[:apos], lower[apos+1:]
		if headSet[stem] && (HeadSuffixRE.MatchString(suffix) || ProperInflectionalSuffixRE.MatchString(suffix)) {
			return HeadSplit{Head: stem, Suffix: suffix, HadApostrophe: true}, true
		}
		return HeadSplit{}, false
	}
	if headSet[lower] {
		return HeadSplit{Head: lower}, true
	}
	rs := []rune(lower)
	for cut := len(rs) - 1; cut >= 3; cut-- {
		stem := string(rs[:cut])
		if !headSet[stem] {
			continue
		}
		suffix := string(rs[cut:])
		var ok bool
		if vowelFinalRE.MatchString(stem) {
			ok = HeadSuffixRE.MatchString(suffix) && IsPhonologicallyValidProperSuffix(stem, suffix, ProperSuffixOptions{P3sgHead: true})
		} else {
			ok = ProperInflectionalSuffixRE.MatchString(suffix) && IsPhonologicallyValidProperSuffix(stem, suffix, ProperSuffixOptions{})
		}
		if ok {
			return HeadSplit{Head: stem, Suffix: suffix}, true
		}
	}
	return HeadSplit{}, false
}

// CalendarSplit is a month or weekday name cut from its suffix.
type CalendarSplit struct {
	Stem          string
	Suffix        string
	HadApostrophe bool
}

// SplitCalendarWord cuts a month or weekday name and its optional inflectional suffix ("aralıka" -> "aralık" +
// "a", "pazartesinin" -> "pazartesi" + "nin"). The set's first name that fits wins.
func SplitCalendarWord(lower string, set WordSet) (CalendarSplit, bool) {
	if apos := strings.IndexByte(lower, '\''); apos > 0 {
		stem, suffix := lower[:apos], lower[apos+1:]
		if set.Has(stem) && ProperInflectionalSuffixRE.MatchString(suffix) {
			return CalendarSplit{Stem: stem, Suffix: suffix, HadApostrophe: true}, true
		}
		return CalendarSplit{}, false
	}
	if set.Has(lower) {
		return CalendarSplit{Stem: lower}, true
	}
	for _, stem := range set.Words() {
		if !strings.HasPrefix(lower, stem) || len(lower) <= len(stem) {
			continue
		}
		suffix := lower[len(stem):]
		if ProperInflectionalSuffixRE.MatchString(suffix) && IsPhonologicallyValidProperSuffix(stem, suffix, ProperSuffixOptions{}) {
			return CalendarSplit{Stem: stem, Suffix: suffix}, true
		}
	}
	return CalendarSplit{}, false
}

// PhraseContext is the contextual gate of a multi-word canonical phrase that overlaps with ordinary Turkish
// phrases, or "" for none.
type PhraseContext string

// The phrase contexts.
const (
	PhraseEra  PhraseContext = "era"
	PhraseDay  PhraseContext = "day"
	PhraseWork PhraseContext = "work"
)

// CanonicalPhrase is a multi-word proper phrase as it is written, whether its suffixes take an apostrophe, and its
// context.
type CanonicalPhrase struct {
	Display string
	Apos    bool
	Context PhraseContext
}

// CanonicalPhrases are the canonical multi-word Turkish holidays, special days, historical events and eras,
// landmarks and works.
var CanonicalPhrases = []CanonicalPhrase{
	// Holidays and special days (WITH apostrophe on suffixes)
	{"Cumhuriyet Bayramı", true, ""},
	{"Zafer Bayramı", true, ""},
	{"Ramazan Bayramı", true, ""},
	{"Kurban Bayramı", true, ""},
	{"Şeker Bayramı", true, ""},
	{"Gençlik ve Spor Bayramı", true, ""},
	{"Ulusal Egemenlik ve Çocuk Bayramı", true, ""},
	{"Atatürk'ü Anma, Gençlik ve Spor Bayramı", true, ""},
	{"Atatürk'ü Anma Gençlik ve Spor Bayramı", true, ""},
	{"Demokrasi ve Millî Birlik Günü", true, ""},
	{"Emek ve Dayanışma Günü", true, ""},
	{"İşçi Bayramı", true, ""},
	{"Kabotaj Bayramı", true, ""},
	{"Denizcilik ve Kabotaj Bayramı", true, ""},
	{"Öğretmenler Günü", true, PhraseDay},
	{"Anneler Günü", true, PhraseDay},
	{"Babalar Günü", true, PhraseDay},
	{"Gaziler Günü", true, ""},
	{"Şehitler Günü", true, ""},
	{"Kadınlar Günü", true, PhraseDay},
	{"Dünya Kadınlar Günü", true, ""},
	{"Dünya Tiyatro Günü", true, ""},
	{"Dünya Çevre Günü", true, ""},
	{"Dünya Sağlık Günü", true, ""},
	{"Dünya Barış Günü", true, ""},
	{"Dünya Çocuk Günü", true, ""},
	{"Dünya Kitap Günü", true, ""},
	{"Dünya Su Günü", true, ""},
	{"İnsan Hakları Günü", true, ""},
	{"Sevgililer Günü", true, PhraseDay},
	{"Şükran Günü", true, ""},
	{"Cadılar Bayramı", true, ""},
	{"Aşure Günü", true, ""},
	{"Tıp Bayramı", true, ""},
	{"Nevruz Bayramı", true, ""},
	{"Kadir Gecesi", true, ""},
	{"Miraç Kandili", true, ""},
	{"Regaip Kandili", true, ""},
	{"Berat Kandili", true, ""},
	{"Mevlit Kandili", true, ""},
	// Historical events, wars, revolutions, eras, treaties (WITH apostrophe)
	{"Kurtuluş Savaşı", true, ""},
	{"Millî Mücadele", true, ""},
	{"Millî Mücadele Dönemi", true, ""},
	{"Cumhuriyet Dönemi", true, ""},
	{"Büyük Taarruz", true, ""},
	{"Birinci Dünya Savaşı", true, ""},
	{"İkinci Dünya Savaşı", true, ""},
	{"I. Dünya Savaşı", true, ""},
	{"II. Dünya Savaşı", true, ""},
	{"Birinci İnönü Savaşı", true, ""},
	{"İkinci İnönü Savaşı", true, ""},
	{"I. İnönü Savaşı", true, ""},
	{"II. İnönü Savaşı", true, ""},
	{"Başkomutanlık Meydan Muharebesi", true, ""},
	{"Dumlupınar Meydan Muharebesi", true, ""},
	{"Çanakkale Savaşı", true, ""},
	{"Çanakkale Zaferi", true, ""},
	{"Kore Savaşı", true, ""},
	{"Kırım Savaşı", true, ""},
	{"Mohaç Savaşı", true, ""},
	{"Malazgirt Savaşı", true, ""},
	{"Malazgirt Meydan Muharebesi", true, ""},
	{"Sakarya Meydan Muharebesi", true, ""},
	{"Preveze Deniz Savaşı", true, ""},
	{"İnebahtı Deniz Savaşı", true, ""},
	{"Balkan Savaşları", true, ""},
	{"Trablusgarp Savaşı", true, ""},
	{"Soğuk Savaş", true, ""},
	{"Yüz Yıl Savaşları", true, ""},
	{"Otuz Yıl Savaşları", true, ""},
	{"Yedi Yıl Savaşları", true, ""},
	{"Haçlı Seferleri", true, ""},
	{"Sanayi Devrimi", true, ""},
	{"Fransız İhtilali", true, ""},
	{"Fransız Devrimi", true, ""},
	{"Ekim Devrimi", true, ""},
	{"Harf İnkılabı", true, ""},
	{"Harf Devrimi", true, ""},
	{"Kavimler Göçü", true, ""},
	{"Fetret Devri", true, ""},
	{"Lale Devri", true, ""},
	{"Tanzimat Dönemi", true, ""},
	{"Tanzimat Fermanı", true, ""},
	{"Islahat Fermanı", true, ""},
	{"Meşrutiyet Dönemi", true, ""},
	{"Birinci Meşrutiyet", true, ""},
	{"İkinci Meşrutiyet", true, ""},
	{"Eski Çağ", true, PhraseEra},
	{"İlk Çağ", true, PhraseEra},
	{"Orta Çağ", true, ""},
	{"Yeni Çağ", true, PhraseEra},
	{"Yakın Çağ", true, PhraseEra},
	{"Aydınlanma Çağı", true, ""},
	{"Taş Devri", true, ""},
	{"Cilalı Taş Devri", true, ""},
	{"Yontma Taş Devri", true, ""},
	{"Bakır Çağı", true, ""},
	{"Tunç Çağı", true, ""},
	{"Demir Çağı", true, ""},
	{"Maden Çağı", true, ""},
	{"Lozan Antlaşması", true, ""},
	{"Lozan Barış Antlaşması", true, ""},
	{"Mondros Ateşkes Antlaşması", true, ""},
	{"Sevr Antlaşması", true, ""},
	{"Mudanya Ateşkes Antlaşması", true, ""},
	{"Karlofça Antlaşması", true, ""},
	{"Küçük Kaynarca Antlaşması", true, ""},
	{"Montrö Boğazlar Sözleşmesi", true, ""},
	{"Amasya Genelgesi", true, ""},
	{"Erzurum Kongresi", true, ""},
	{"Sivas Kongresi", true, ""},
	{"Büyük Buhran", true, ""},
	{"Coğrafi Keşifler", true, ""},
	// Established directional regions and territories (WITH apostrophe)
	{"Batı Anadolu", true, ""},
	{"Doğu Anadolu", true, ""},
	{"İç Anadolu", true, ""},
	{"Güneydoğu Anadolu", true, ""},
	{"Doğu Karadeniz", true, ""},
	{"Batı Karadeniz", true, ""},
	{"Batı Trakya", true, ""},
	{"Doğu Trakya", true, ""},
	{"Kuzey Kıbrıs", true, ""},
	{"Güney Kıbrıs", true, ""},
	{"Orta Doğu", true, ""},
	{"Kuzey İrlanda", true, ""},
	{"Batı Şeria", true, ""},
	{"Batı Sahra", true, ""},
	// Geographic, monument and landmark phrases (WITH apostrophe)
	{"Kaz Dağları", true, ""},
	{"Toros Dağları", true, ""},
	{"Kaçkar Dağları", true, ""},
	{"Ağrı Dağı", true, ""},
	{"Erciyes Dağı", true, ""},
	{"Nemrut Dağı", true, ""},
	{"Tuz Gölü", true, ""},
	{"Van Gölü", true, ""},
	{"Beyşehir Gölü", true, ""},
	{"Eğirdir Gölü", true, ""},
	{"İznik Gölü", true, ""},
	{"Sapanca Gölü", true, ""},
	{"Abant Gölü", true, ""},
	{"Salda Gölü", true, ""},
	{"Manyas Gölü", true, ""},
	{"Atlas Okyanusu", true, ""},
	{"Büyük Okyanus", true, ""},
	{"Hint Okyanusu", true, ""},
	{"Kızıl Deniz", true, ""},
	{"Ölü Deniz", true, ""},
	{"Marmara Bölgesi", true, ""},
	{"Ege Bölgesi", true, ""},
	{"Akdeniz Bölgesi", true, ""},
	{"Karadeniz Bölgesi", true, ""},
	{"İç Anadolu Bölgesi", true, ""},
	{"Doğu Anadolu Bölgesi", true, ""},
	{"Güneydoğu Anadolu Bölgesi", true, ""},
	{"Göller Bölgesi", true, ""},
	{"Topkapı Sarayı", true, ""},
	{"Dolmabahçe Sarayı", true, ""},
	{"Çırağan Sarayı", true, ""},
	{"Beylerbeyi Sarayı", true, ""},
	{"İshak Paşa Sarayı", true, ""},
	{"Çankaya Köşkü", true, ""},
	{"Rumeli Hisarı", true, ""},
	{"Anadolu Hisarı", true, ""},
	{"Galata Kulesi", true, ""},
	{"Kız Kulesi", true, ""},
	{"Boğaziçi Köprüsü", true, ""},
	{"Fatih Sultan Mehmet Köprüsü", true, ""},
	{"Yavuz Sultan Selim Köprüsü", true, ""},
	{"Osmangazi Köprüsü", true, ""},
	{"Mısır Çarşısı", true, ""},
	{"Sümela Manastırı", true, ""},
	{"Haydarpaşa Garı", true, ""},
	{"Sirkeci Garı", true, ""},
	{"Ankara Garı", true, ""},
	{"Sabiha Gökçen Havalimanı", true, ""},
	{"İstanbul Havalimanı", true, ""},
	{"Esenboğa Havalimanı", true, ""},
	{"Atatürk Havalimanı", true, ""},
	{"Kıbrıs Şehitleri Caddesi", true, ""},
	{"Gazi Mustafa Kemal Bulvarı", true, ""},
	{"İstiklal Caddesi", true, ""},
	{"Bağdat Caddesi", true, ""},
	{"Taksim Meydanı", true, ""},
	{"Kızılay Meydanı", true, ""},
	{"Maçka Parkı", true, ""},
	{"Gülhane Parkı", true, ""},
	{"Yıldız Parkı", true, ""},
	{"Gençlik Parkı", true, ""},
	{"Kocatepe Camisi", true, ""},
	{"Sultanahmet Camisi", true, ""},
	{"Süleymaniye Camisi", true, ""},
	{"Selimiye Camisi", true, ""},
	{"Dara Antik Kenti", true, ""},
	{"Efes Antik Kenti", true, ""},
	{"Rio de Janeiro", true, ""},
	{"Avrupa Birliği", true, ""},
	{"Resmî Gazete", true, ""},
	// Iconic literary works and films (WITH apostrophe; everyday-word titles require literary/media context!)
	{"Sinekli Bakkal", true, PhraseWork},
	{"İnce Memed", true, PhraseWork},
	{"Benim Adım Kırmızı", true, PhraseWork},
	{"Küçük Prens", true, PhraseWork},
	{"Selvi Boylum Al Yazmalım", true, PhraseWork},
	{"Beyaz Kale", true, PhraseWork},
	{"Kürk Mantolu Madonna", true, PhraseWork},
	{"Saatleri Ayarlama Enstitüsü", true, PhraseWork},
	{"Hanımın Çiftliği", true, PhraseWork},
	{"Kurtlar Vadisi", true, PhraseWork},
	{"Ölü Ozanlar Derneği", true, PhraseWork},
	{"Aşk-ı Memnu", true, ""},
	{"Suç ve Ceza", true, PhraseWork},
	{"Savaş ve Barış", true, PhraseWork},
	{"Karamazov Kardeşler", true, ""},
	{"Kuyucaklı Yusuf", true, ""},
	{"Mai ve Siyah", true, PhraseWork},
	{"Araba Sevdası", true, PhraseWork},
	{"Dokuzuncu Hariciye Koğuşu", true, ""},
	{"Kara Kitap", true, PhraseWork},
	{"Masumiyet Müzesi", true, PhraseWork},
	{"Aylak Adam", true, PhraseWork},
	{"Anayurt Oteli", true, PhraseWork},
	{"Puslu Kıtalar Atlası", true, PhraseWork},
	{"Kırmızı Saçlı Kadın", true, PhraseWork},
	{"Leyla ile Mecnun", true, PhraseWork},
	{"Hababam Sınıfı", true, ""},
	{"Muhsin Bey", true, ""},
	{"Devlet Ana", true, PhraseWork},
	{"Yılanların Öcü", true, PhraseWork},
	{"Üç İstanbul", true, ""},
	{"Ağrı Dağı Efsanesi", true, ""},
	{"Yaprak Dökümü", true, PhraseWork},
	{"Yeşil Gece", true, PhraseWork},
	{"Kiralık Konak", true, PhraseWork},
	{"Sodom ve Gomore", true, ""},
	{"Ateşten Gömlek", true, PhraseWork},
	{"Vurun Kahpeye", true, PhraseWork},
	{"Handan", true, PhraseWork},
	{"Acımak", true, PhraseWork},
	{"Dudaktan Kalbe", true, PhraseWork},
	{"Fatih-Harbiye", true, ""},
	{"Matmazel Noraliya'nın Koltuğu", true, ""},
	{"Ayaşlı ve Kiracıları", true, ""},
	{"Aganta Burina Burinata", true, ""},
	{"İçimizdeki Şeytan", true, PhraseWork},
	{"Esir Şehrin İnsanları", true, PhraseWork},
	{"Yorgun Savaşçı", true, PhraseWork},
	{"Hakkâri'de Bir Mevsim", true, ""},
	{"Fikrimin İnce Gülü", true, PhraseWork},
	{"Bir Düğün Gecesi", true, PhraseWork},
	{"Korkuyu Beklerken", true, PhraseWork},
	{"Lüzumsuz Adam", true, PhraseWork},
	{"Sırça Köşk", true, PhraseWork},
	{"Dede Korkut Hikâyeleri", true, ""},
	{"Kutadgu Bilig", true, ""},
	{"İstiklal Marşı", true, ""},
	{"Keşanlı Ali Destanı", true, ""},
	{"Susuz Yaz", true, PhraseWork},
	{"Kış Uykusu", true, PhraseWork},
	{"Bir Zamanlar Anadolu'da", true, ""},
	{"Ahlat Ağacı", true, PhraseWork},
	{"Babam ve Oğlum", true, PhraseWork},
	{"Tosun Paşa", true, ""},
	{"Süt Kardeşler", true, PhraseWork},
	{"Kapıcılar Kralı", true, PhraseWork},
	{"Çöpçüler Kralı", true, PhraseWork},
	{"Neşeli Günler", true, PhraseWork},
	{"Mona Lisa", true, ""},
	{"Kaplumbağa Terbiyecisi", true, PhraseWork},
	{"Yıldızlı Gece", true, PhraseWork},
	{"Son Akşam Yemeği", true, PhraseWork},
	{"Felatun Bey ile Rakım Efendi", true, ""},
	{"Taaşşuk-ı Talat ve Fitnat", true, ""},
	{"Kuyruklu Yıldız Altında Bir İzdivaç", true, ""},
	{"Vatan yahut Silistre", true, ""},
	{"Bereketli Topraklar Üzerinde", true, PhraseWork},
	{"Orta Direk", true, PhraseWork},
	{"Yer Demir Gök Bakır", true, PhraseWork},
	{"Ölmez Otu", true, PhraseWork},
	{"Yılanı Öldürseler", true, PhraseWork},
	{"Tehlikeli Oyunlar", true, PhraseWork},
	{"Sevgili Arsız Ölüm", true, PhraseWork},
	{"Cevdet Bey ve Oğulları", true, ""},
	{"Sessiz Ev", true, PhraseWork},
	{"Yeni Hayat", true, PhraseWork},
	{"Kafamda Bir Tuhaflık", true, PhraseWork},
	{"Veba Geceleri", true, PhraseWork},
	{"Memleket Hikâyeleri", true, PhraseWork},
	{"Gurbet Hikâyeleri", true, PhraseWork},
	{"Son Kuşlar", true, PhraseWork},
	{"Memleketimden İnsan Manzaraları", true, PhraseWork},
	{"Bülbülü Öldürmek", true, PhraseWork},
	{"Hayvan Çiftliği", true, PhraseWork},
	{"Yüzüklerin Efendisi", true, ""},
	{"Küçük Kadınlar", true, PhraseWork},
	{"Don Kişot", true, ""},
	{"İlahi Komedya", true, ""},
	{"Genç Werther'in Acıları", true, ""},
	{"Romeo ve Juliet", true, ""},
	{"Venedik Taciri", true, ""},
	{"Kral Lear", true, ""},
	{"Bir Yaz Gecesi Rüyası", true, PhraseWork},
	{"Yeraltından Notlar", true, PhraseWork},
	{"İnsan Ne ile Yaşar", true, PhraseWork},
	{"Vişne Bahçesi", true, PhraseWork},
	{"Üç Kız Kardeş", true, PhraseWork},
	{"Notre Dame'ın Kamburu", true, ""},
	{"Üç Silahşorlar", true, ""},
	{"Goriot Baba", true, ""},
	{"Parma Manastırı", true, ""},
	{"Madame Bovary", true, ""},
	{"Aşk ve Gurur", true, PhraseWork},
	{"Jane Eyre", true, ""},
	{"Oliver Twist", true, ""},
	{"David Copperfield", true, ""},
	{"Moby Dick", true, ""},
	{"Yaşlı Adam ve Deniz", true, PhraseWork},
	{"Çanlar Kimin İçin Çalıyor", true, PhraseWork},
	{"Muhteşem Gatsby", true, ""},
	{"Çavdar Tarlasında Çocuklar", true, PhraseWork},
	{"Vahşetin Çağrısı", true, PhraseWork},
	{"Kırmızı Pazartesi", true, PhraseWork},
	{"Bilinmeyen Bir Kadının Mektubu", true, PhraseWork},
	{"Gılgamış Destanı", true, ""},
	{"Gazap Üzümleri", true, PhraseWork},
	{"Fareler ve İnsanlar", true, PhraseWork},
	{"Şeker Portakalı", true, PhraseWork},
	{"Gurur ve Önyargı", true, PhraseWork},
	{"Uğultulu Tepeler", true, PhraseWork},
	{"Monte Kristo Kontu", true, ""},
	{"Vadideki Zambak", true, PhraseWork},
	{"Kırmızı ve Siyah", true, PhraseWork},
	{"İki Şehrin Hikâyesi", true, PhraseWork},
	{"Beyaz Diş", true, PhraseWork},
	{"Anna Karenina", true, ""},
	{"Babalar ve Oğullar", true, PhraseWork},
	{"Ölü Canlar", true, PhraseWork},
	{"Büyük Umutlar", true, PhraseWork},
	{"Cesur Yeni Dünya", true, PhraseWork},
	{"Yüzyıllık Yalnızlık", true, PhraseWork},
	// Iconic multi-word institutions (NO apostrophe!)
	{"Türk Dil Kurumu", false, ""},
	{"Türk Tarih Kurumu", false, ""},
	{"Türkiye Büyük Millet Meclisi", false, ""},
	{"Türkiye Cumhuriyet Merkez Bankası", false, ""},
	{"Türkiye Petrolleri Anonim Ortaklığı", false, ""},
	{"Türkiye İstatistik Kurumu", false, ""},
	{"Türkiye Bilimsel ve Teknolojik Araştırma Kurumu", false, ""},
	{"Türkiye Radyo Televizyon Kurumu", false, ""},
	{"Türk Standardları Enstitüsü", false, ""},
	{"Türk Hava Yolları", false, ""},
	{"Türk Silahlı Kuvvetleri", false, ""},
	{"Genelkurmay Başkanlığı", false, ""},
	{"Türk Kızılayı", false, ""},
	{"Yeşilay Cemiyeti", false, ""},
	{"Anadolu Ajansı", false, ""},
	{"Ziraat Bankası", false, ""},
	{"Halk Bankası", false, ""},
	{"Vakıf Bank", false, ""},
	{"Türkiye İş Bankası", false, ""},
	{"İş Bankası", false, ""},
	{"Garanti Bankası", false, ""},
	{"Yapı Kredi Bankası", false, ""},
	{"Bakanlar Kurulu", false, ""},
	{"Yüksek Seçim Kurulu", false, ""},
	{"Millî Güvenlik Kurulu", false, ""},
	{"Hakimler ve Savcılar Kurulu", false, ""},
	{"Anayasa Mahkemesi", false, ""},
	{"Atatürk Orman Çiftliği", false, ""},
	{"Atatürk Kültür Merkezi", false, ""},
	{"Devlet Tiyatroları", false, ""},
	{"Devlet Opera ve Balesi", false, ""},
	{"Devlet Malzeme Ofisi", false, ""},
	{"Devlet Hava Meydanları İşletmesi", false, ""},
	{"Kredi ve Yurtlar Kurumu", false, ""},
	{"Türkiye İş Kurumu", false, ""},
	{"Toplu Konut İdaresi", false, ""},
	{"Gelir İdaresi Başkanlığı", false, ""},
	{"Tapu ve Kadastro Genel Müdürlüğü", false, ""},
	{"Vakıflar Genel Müdürlüğü", false, ""},
	{"Türkiye Futbol Federasyonu", false, ""},
	{"Türkiye Basketbol Federasyonu", false, ""},
	{"Türkiye Voleybol Federasyonu", false, ""},
	{"Türk Tabipleri Birliği", false, ""},
	{"Türkiye Barolar Birliği", false, ""},
	{"İstanbul Ticaret Odası", false, ""},
	{"Ankara Ticaret Odası", false, ""},
	{"İstanbul Sanayi Odası", false, ""},
	{"Kişisel Verileri Koruma Kurumu", false, ""},
	{"Bilgi Teknolojileri ve İletişim Kurumu", false, ""},
	{"Enerji Piyasası Düzenleme Kurumu", false, ""},
	{"Cumhuriyet Halk Partisi", false, ""},
	{"Adalet ve Kalkınma Partisi", false, ""},
	{"Milliyetçi Hareket Partisi", false, ""},
	{"Millî Kütüphane", false, ""},
	{"Beyazıt Devlet Kütüphanesi", false, ""},
	{"Anadolu Medeniyetleri Müzesi", false, ""},
	{"İstanbul Arkeoloji Müzeleri", false, ""},
	{"Topkapı Sarayı Müzesi", false, ""},
	{"Pera Müzesi", false, ""},
	{"Rahmi Koç Müzesi", false, ""},
	{"Mevlana Müzesi", false, ""},
	{"Zeugma Mozaik Müzesi", false, ""},
	{"Mavi Köşe Bakkaliyesi", false, ""},
	{"Karadeniz Teknik Üniversitesi", false, ""},
	{"Çukurova Üniversitesi", false, ""},
	{"Akdeniz Üniversitesi", false, ""},
	{"Anadolu Üniversitesi", false, ""},
	{"Atatürk Üniversitesi", false, ""},
	{"Selçuk Üniversitesi", false, ""},
	{"Erciyes Üniversitesi", false, ""},
	{"Ondokuz Mayıs Üniversitesi", false, ""},
	{"Çanakkale Onsekiz Mart Üniversitesi", false, ""},
	{"Süleyman Demirel Üniversitesi", false, ""},
	{"Mersin Üniversitesi", false, ""},
	{"Kocaeli Üniversitesi", false, ""},
	{"Sakarya Üniversitesi", false, ""},
	{"Trakya Üniversitesi", false, ""},
	{"Pamukkale Üniversitesi", false, ""},
	{"Fırat Üniversitesi", false, ""},
	{"İnönü Üniversitesi", false, ""},
	{"Dicle Üniversitesi", false, ""},
	{"Gaziantep Üniversitesi", false, ""},
	{"Bilkent Üniversitesi", false, ""},
	{"Koç Üniversitesi", false, ""},
	{"Sabancı Üniversitesi", false, ""},
	{"Özyeğin Üniversitesi", false, ""},
	{"Bahçeşehir Üniversitesi", false, ""},
	{"Bilgi Üniversitesi", false, ""},
	{"Yeditepe Üniversitesi", false, ""},
	{"Başkent Üniversitesi", false, ""},
	{"Galatasaray Üniversitesi", false, ""},
	{"Kadir Has Üniversitesi", false, ""},
	{"Mimar Sinan Güzel Sanatlar Üniversitesi", false, ""},
	{"Gebze Teknik Üniversitesi", false, ""},
	{"İzmir Yüksek Teknoloji Enstitüsü", false, ""},
	{"İzmir Ekonomi Üniversitesi", false, ""},
	{"Robert Kolej", false, ""},
	{"Ankara Fen Lisesi", false, ""},
	{"İzmir Fen Lisesi", false, ""},
	{"İstanbul Atatürk Fen Lisesi", false, ""},
	{"Kadıköy Anadolu Lisesi", false, ""},
	{"Bornova Anadolu Lisesi", false, ""},
	{"Cağaloğlu Anadolu Lisesi", false, ""},
	{"Vefa Lisesi", false, ""},
	{"Pertevniyal Lisesi", false, ""},
	{"Haydarpaşa Lisesi", false, ""},
	{"Darüşşafaka Lisesi", false, ""},
	{"Darüşşafaka Cemiyeti", false, ""},
	{"Dünya Sağlık Örgütü", false, ""},
	{"Dünya Bankası", false, ""},
	{"Dünya Ticaret Örgütü", false, ""},
	{"Uluslararası Para Fonu", false, ""},
	{"Uluslararası Af Örgütü", false, ""},
	{"Avrupa Konseyi", false, ""},
	{"Avrupa Parlamentosu", false, ""},
	{"Avrupa İnsan Hakları Mahkemesi", false, ""},
	{"Birleşmiş Milletler Güvenlik Konseyi", false, ""},
	{"Sosyal Güvenlik Kurumu", false, ""},
	{"Yükseköğretim Kurulu", false, ""},
	{"Radyo ve Televizyon Üst Kurulu", false, ""},
	{"Bankacılık Düzenleme ve Denetleme Kurumu", false, ""},
	{"Sermaye Piyasası Kurulu", false, ""},
	{"Rekabet Kurumu", false, ""},
	{"Devlet Su İşleri", false, ""},
	{"Meteoroloji Genel Müdürlüğü", false, ""},
	{"Emniyet Genel Müdürlüğü", false, ""},
	{"Orman Genel Müdürlüğü", false, ""},
	{"Karayolları Genel Müdürlüğü", false, ""},
	{"Devlet Demir Yolları", false, ""},
	{"Diyanet İşleri Başkanlığı", false, ""},
	{"Millî Eğitim Bakanlığı", false, ""},
	{"İçişleri Bakanlığı", false, ""},
	{"Dışişleri Bakanlığı", false, ""},
	{"Sağlık Bakanlığı", false, ""},
	{"Adalet Bakanlığı", false, ""},
	{"Hazine ve Maliye Bakanlığı", false, ""},
	{"Maliye Bakanlığı", false, ""},
	{"Millî Savunma Bakanlığı", false, ""},
	{"Kültür ve Turizm Bakanlığı", false, ""},
	{"Ulaştırma ve Altyapı Bakanlığı", false, ""},
	{"Çevre, Şehircilik ve İklim Değişikliği Bakanlığı", false, ""},
	{"Tarım ve Orman Bakanlığı", false, ""},
	{"Sanayi ve Teknoloji Bakanlığı", false, ""},
	{"Ticaret Bakanlığı", false, ""},
	{"Enerji ve Tabii Kaynaklar Bakanlığı", false, ""},
	{"Gençlik ve Spor Bakanlığı", false, ""},
	{"Çalışma ve Sosyal Güvenlik Bakanlığı", false, ""},
	{"Aile ve Sosyal Hizmetler Bakanlığı", false, ""},
	{"Orta Doğu Teknik Üniversitesi", false, ""},
	{"İstanbul Teknik Üniversitesi", false, ""},
	{"Yıldız Teknik Üniversitesi", false, ""},
	{"Dokuz Eylül Üniversitesi", false, ""},
	{"Boğaziçi Üniversitesi", false, ""},
	{"Ankara Üniversitesi", false, ""},
	{"İstanbul Üniversitesi", false, ""},
	{"Marmara Üniversitesi", false, ""},
	{"Ege Üniversitesi", false, ""},
	{"Hacettepe Üniversitesi", false, ""},
	{"Gazi Üniversitesi", false, ""},
	{"Bursa Uludağ Üniversitesi", false, ""},
	{"Eskişehir Osmangazi Üniversitesi", false, ""},
	{"Galatasaray Lisesi", false, ""},
	{"Kabataş Erkek Lisesi", false, ""},
	{"İstanbul Erkek Lisesi", false, ""},
	{"Türk Dili ve Edebiyatı Bölümü", false, ""},
	{"Siyasal Bilgiler Fakültesi", false, ""},
	{"Başakşehir Çam ve Sakura Şehir Hastanesi", false, ""},
	{"İstanbul Büyükşehir Belediyesi", false, ""},
	{"Ankara Büyükşehir Belediyesi", false, ""},
	{"İzmir Büyükşehir Belediyesi", false, ""},
	{"Birleşmiş Milletler", false, ""},
}

// HasVowel reports whether a lowercase string has a vowel.
func HasVowel(s string) bool { return vowelRE.MatchString(s) }
