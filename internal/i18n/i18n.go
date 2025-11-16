package i18n

import (
	"embed"
	"fmt"
	"io/fs"
	"os"

	"github.com/Xuanwo/go-locale"
	"github.com/vorlif/spreak"
	"golang.org/x/text/language"
)

var supportedLanguages = []language.Tag{
	language.English,
	language.German,
}

//go:embed locale/*
var locales embed.FS

func New(loc string) (*spreak.Localizer, error) {
	var tag language.Tag
	var err error
	if loc == "" {
		tag, err = locale.Detect()
		if err != nil {
			tag = language.English // Unable to detect locale, fallback to English
		}
	}

	localeFS, err := fs.Sub(locales, "locale")
	if err != nil {
		return nil, fmt.Errorf("failed to load locales: %w", err)
	}

	bundle, err := spreak.NewBundle(
		spreak.WithSourceLanguage(language.English),
		spreak.WithDomainFs("", localeFS),
		spreak.WithLanguage(tag),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create i18n bundle: %w", err)
	}
	return spreak.NewLocalizer(bundle, tag), nil
}

// languageFromEnv returns the language tag for the current locale.
//
// According to https://www.ibm.com/docs/en/aix/7.1.0?topic=locales-understanding-locale-environment-variables
// LC_ALL, LC_MESSAGES and LANG are highest to lowest priority.
func languageFromEnv() language.Tag {
	lang := os.Getenv("LANG")
	if val := os.Getenv("LC_MESSAGES"); val != "" {
		lang = val
	}
	if val := os.Getenv("LC_ALL"); val != "" {
		lang = val
	}
	matcher := language.NewMatcher(supportedLanguages)
	tag, _, _ := matcher.Match(language.Make(lang))
	return tag
}
