// Package i18n provides display text only. Canonical matching values stay unchanged.
package i18n

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/BAITC-Hacks/hack-1ad72463-apex-ai/backend/internal/domain"
)

type Locale string

const (
	LocaleRU Locale = "ru"
	LocaleKK Locale = "kk"
	LocaleEN Locale = "en"
)

// ParseAcceptLanguage selects the first supported range in header order.
// Regional variants use their base language; unacceptable (q=0) ranges are skipped.
func ParseAcceptLanguage(header string) Locale {
	for _, part := range strings.Split(header, ",") {
		fields := strings.Split(part, ";")
		tag := strings.ToLower(strings.TrimSpace(fields[0]))
		quality := 1.0
		for _, parameter := range fields[1:] {
			key, value, ok := strings.Cut(strings.TrimSpace(parameter), "=")
			if !ok || !strings.EqualFold(strings.TrimSpace(key), "q") {
				continue
			}
			q, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
			if err != nil || math.IsNaN(q) || math.IsInf(q, 0) || q < 0 || q > 1 {
				quality = 0
				break
			}
			quality = q
		}
		if quality == 0 {
			continue
		}
		base, _, _ := strings.Cut(tag, "-")
		switch Locale(base) {
		case LocaleRU, LocaleKK, LocaleEN:
			return Locale(base)
		}
	}
	return LocaleRU
}

func index(locale Locale) int {
	switch locale {
	case LocaleKK:
		return 1
	case LocaleEN:
		return 2
	default:
		return 0
	}
}
func Message(locale Locale, key string, args ...any) string {
	texts, ok := messages[key]
	if !ok {
		return key
	}
	text := texts[index(locale)]
	if len(args) == 0 {
		return text
	}
	return fmt.Sprintf(text, args...)
}

var messages = map[string][3]string{
	"required":         {"Укажите обязательное поле.", "Міндетті өрісті толтырыңыз.", "This field is required."},
	"invalid_date":     {"Используйте дату YYYY-MM-DD.", "Күнді YYYY-MM-DD форматында енгізіңіз.", "Use the YYYY-MM-DD date format."},
	"date_range":       {"Выберите дату с %s по %s включительно.", "%s–%s аралығындағы күнді таңдаңыз.", "Choose a date from %s through %s."},
	"invalid_format":   {"Неизвестный формат мероприятия.", "Іс-шара форматы белгісіз.", "Unknown event format."},
	"budget":           {"Бюджет должен быть целым числом больше нуля.", "Бюджет нөлден үлкен бүтін сан болуы керек.", "Budget must be a positive integer."},
	"duration":         {"Длительность должна быть конечным числом больше нуля.", "Ұзақтық нөлден үлкен сан болуы керек.", "Duration must be a positive finite number."},
	"language":         {"Допустимы русский, казахский, английский.", "Орыс, қазақ және ағылшын тілдері қолдау көрсетіледі.", "Supported languages are Russian, Kazakh, and English."},
	"duplicate_field":  {"Поле не должно повторяться.", "Өріс қайталанбауы керек.", "A field must not be repeated."},
	"unknown_field":    {"Неизвестное поле.", "Белгісіз өріс.", "Unknown field."},
	"invalid_type":     {"Неверный тип или недопустимый размер числа.", "Дерек түрі қате немесе сан рұқсат етілген ауқымнан тыс.", "Invalid type or number out of range."},
	"no_catalog":       {"В каталоге нет категории «%s» в городе «%s». Измените город или категорию.", "«%[2]s» қаласында «%[1]s» санатындағы мердігерлер каталогта жоқ. Қаланы немесе санатты өзгертіңіз.", "The catalog has no contractors in category “%s” for “%s”. Change the city or category."},
	"no_match":         {"Ни один подрядчик не соответствует условиям. ", "Шарттарға сәйкес келетін мердігер табылмады. ", "No contractors meet the requirements. "},
	"change_request":   {" Измените условия запроса.", " Іздеу шарттарын өзгертіңіз.", " Change the search requirements."},
	"matches":          {"Подходят %d подрядчика; показаны %d по стартовой цене, затем ID.", "Сәйкес мердігерлер саны: %d; бастапқы бағасы, содан кейін ID бойынша %d мердігер көрсетілді.", "Matching contractors: %d; %d shown by starting price, then ID."},
	"small_catalog":    {" В каталоге города и категории всего %d профиля.", " Осы қала мен санат бойынша каталогта барлығы %d профиль бар.", " Profiles in the catalog for this city and category: %d."},
	"excluded":         {"Исключены по первой неподходящей проверке: %s.", "Алғашқы сәйкес келмеген шарт бойынша алынып тасталды: %s.", "Excluded by the first failed check: %s."},
	"price":            {"стартовая цена %d ₸ укладывается в бюджет %d ₸", "бастапқы баға %d ₸, бұл %d ₸ бюджеттен аспайды", "the starting price of KZT %d is within the KZT %d budget"},
	"price_equal":      {"стартовая цена %d ₸ равна бюджету", "бастапқы баға %d ₸ бюджетке тең", "the starting price of KZT %d equals the budget"},
	"price_imputed":    {" (оценка из датасета)", " (деректер жиынындағы болжамды баға)", " (estimate from the dataset)"},
	"supports":         {"Профиль поддерживает формат «%s» в городе %s", "Профильде «%s» форматы %s қаласында ұсынылған", "Supports the %s format in %s"},
	"profile_language": {"язык «%s» указан в профиле", "профильде %s көрсетілген", "%s is listed in the profile"},
	"hours_na":         {"ограничение часов для этой услуги неприменимо", "бұл қызметке сағат шектеуі қолданылмайды", "an hourly limit does not apply to this service"},
	"hours":            {"запрошено %g ч при лимите %g ч", "сұралған ұзақтық — %g сағат, шектеу — %g сағат", "%g hours were requested against a limit of %g hours"},
	"available":        {"%s не отмечено занятым в календаре датасета", "деректер жиынының күнтізбесінде %s күні бос емес деп белгіленбеген", "%s is not marked busy in the dataset calendar"},
	"profile_fallback": {"В профиле перечислены услуги: %s; языки: %s.", " Профильде көрсетілген қызметтер: %s; тілдер: %s.", " Services listed in the profile: %s; languages: %s."},
}
var reasons = map[string][3]string{
	"EVENT_FORMAT_UNSUPPORTED": {"не поддерживают формат", "форматқа сәйкес келмейді", "unsupported event format"},
	"BUDGET_TOO_LOW":           {"стартовая цена выше бюджета", "бастапқы баға бюджеттен жоғары", "starting price exceeds the budget"},
	"LANGUAGE_UNSUPPORTED":     {"не поддерживают язык", "қажетті тілде қызмет көрсетпейді", "unsupported language"},
	"DURATION_EXCEEDED":        {"превышена длительность", "ұзақтық шектеуден асады", "duration exceeds the limit"},
	"BUSY_ON_DATE":             {"заняты на выбранную дату", "таңдалған күні бос емес", "busy on the selected date"},
}
var languages = map[string][3]string{
	"русский":    {"русский", "орыс тілі", "Russian"},
	"казахский":  {"казахский", "қазақ тілі", "Kazakh"},
	"английский": {"английский", "ағылшын тілі", "English"},
}
var formats = map[string][3]string{
	"день рождения": {"день рождения", "туған күн", "birthday"},
	"конференция":   {"конференция", "конференция", "conference"},
	"корпоратив":    {"корпоратив", "корпоратив", "corporate event"},
	"свадьба":       {"свадьба", "үйлену тойы", "wedding"},
	"той":           {"той", "той", "toi celebration"},
	"юбилей":        {"юбилей", "мерейтой", "anniversary"},
}

func label(locale Locale, values map[string][3]string, key string) string {
	if v, ok := values[key]; ok {
		return v[index(locale)]
	}
	return key
}
func ReasonLabel(locale Locale, code string) string { return label(locale, reasons, code) }
func FormatLabel(locale Locale, canonical string) string {
	return label(locale, formats, domain.Normalize(canonical))
}
func LanguageLabel(locale Locale, canonical string) string {
	return label(locale, languages, domain.Normalize(canonical))
}

// Source-derived notes are never translated. RU retains its existing punctuation.
func ProfileNote(locale Locale, text string) string {
	switch locale {
	case LocaleKK:
		return "Профильдегі бастапқы мәтін: " + text
	case LocaleEN:
		return "Original profile note: " + text
	default:
		return "В описании отмечено: " + strings.TrimRight(text, ". ") + "."
	}
}
