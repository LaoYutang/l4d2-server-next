package translation

import "golang.org/x/text/language"

var (
	FromSteamLanguage = map[string]language.Tag{
		"arabic":     language.Arabic,
		"brazilian":  language.BrazilianPortuguese,
		"bulgarian":  language.Bulgarian,
		"czech":      language.Czech,
		"danish":     language.Danish,
		"dutch":      language.Dutch,
		"english":    language.English,
		"finnish":    language.Finnish,
		"french":     language.French,
		"german":     language.German,
		"greek":      language.Greek,
		"hungarian":  language.Hungarian,
		"indonesian": language.Indonesian,
		"italian":    language.Italian,
		"japanese":   language.Japanese,
		"koreana":    language.Korean,
		"latam":      language.LatinAmericanSpanish,
		"norwegian":  language.Norwegian,
		"polish":     language.Polish,
		"portuguese": language.EuropeanPortuguese,
		"romanian":   language.Romanian,
		"russian":    language.Russian,
		"schinese":   language.SimplifiedChinese,
		"spanish":    language.EuropeanSpanish,
		"swedish":    language.Swedish,
		"tchinese":   language.TraditionalChinese,
		"thai":       language.Thai,
		"turkish":    language.Turkish,
		"ukrainian":  language.Ukrainian,
		"vietnamese": language.Vietnamese,
	}

	ToSteamLanguage = func() map[language.Tag]string {
		m := map[language.Tag]string{
			language.Chinese:    "schinese",
			language.Portuguese: "portuguese",
			language.Spanish:    "spanish",
		}

		for k, v := range FromSteamLanguage {
			m[v] = k
		}

		return m
	}()

	SteamLanguageMatcher = language.NewMatcher([]language.Tag{
		language.English,
		language.Arabic,
		language.Bulgarian,
		language.Chinese,
		language.SimplifiedChinese,
		language.TraditionalChinese,
		language.Czech,
		language.Danish,
		language.Dutch,
		language.Finnish,
		language.French,
		language.German,
		language.Greek,
		language.Hungarian,
		language.Indonesian,
		language.Italian,
		language.Japanese,
		language.Korean,
		language.Norwegian,
		language.Polish,
		language.Portuguese,
		language.EuropeanPortuguese,
		language.BrazilianPortuguese,
		language.Romanian,
		language.Russian,
		language.Spanish,
		language.EuropeanSpanish,
		language.LatinAmericanSpanish,
		language.Swedish,
		language.Thai,
		language.Turkish,
		language.Ukrainian,
		language.Vietnamese,
	})
)
