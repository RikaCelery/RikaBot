package utils

import "github.com/mattn/go-runewidth"

func Truncate(title string, maxlen int) string {
	return runewidth.Truncate(title, maxlen, "...")
}
