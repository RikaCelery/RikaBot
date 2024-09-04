package utils

import (
	"github.com/FloatTech/floatbox/file"
	"github.com/FloatTech/ttl"
	"github.com/mattn/go-runewidth"
	log "github.com/sirupsen/logrus"
	"strings"
	"time"
)

var cache = ttl.NewCache[string, []string](24 * time.Hour)

func FilterSensitive(s string) string {
	key := "零时-Tencent.txt"
	v := cache.Get(key)
	if v == nil || len(v) == 0 {
		open, err := file.GetCustomLazyData("https://raw.githubusercontent.com/konsheng/Sensitive-lexicon/main/Vocabulary/", "filter_words/"+key)
		if err != nil {
			panic(err)
		}
		cache.Set(key, strings.Split(string(open), "\n"))
	}
	for _, word := range cache.Get(key) {
		if word == "" {
			continue
		}
		if !strings.Contains(s, word) {
			continue
		}
		s = strings.ReplaceAll(s, word, strings.Repeat("▢", runewidth.StringWidth(word)))
		log.Infof("replaced %s", word)
	}
	return s
}
