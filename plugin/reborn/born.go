// Package reborn 重开插件
package reborn

import (
	wr "github.com/mroth/weightedrand"
)

//nolint:unused
type rate []struct {
	Name   string  `json:"name"`
	Weight float64 `json:"weight"`
}

var (
	//nolint:unused
	areac *wr.Chooser
	//nolint:unused
	gender, _ = wr.NewChooser(
		wr.Choice{Item: "男孩子", Weight: 50707},
		wr.Choice{Item: "女孩子", Weight: 48292},
		wr.Choice{Item: "雌雄同体", Weight: 1001},
	)
)

//nolint:unused
func randcoun() string {
	return areac.Pick().(string)
}

//nolint:unused
func randgen() string {
	return gender.Pick().(string)
}
