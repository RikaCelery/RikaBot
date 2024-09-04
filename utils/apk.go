package utils

import (
	"github.com/shogo82148/androidbinary"
	"github.com/shogo82148/androidbinary/apk"
	"image"
)

func ParseApk(file string) (icon *image.Image, pkgName, labelCN string, err error) {
	pkg, err := apk.OpenFile(file)
	if err != nil {
		return
	}
	i, err := pkg.Icon(nil)
	if err != nil {
		return
	}
	icon = &i
	pkgName = FilterSensitive(pkg.PackageName())
	s, err := pkg.Label(&androidbinary.ResTableConfig{
		Language: [2]uint8{uint8('z'), uint8('h')},
		Country:  [2]uint8{uint8('C'), uint8('N')},
	})
	if err != nil {
		s2, err := pkg.Label(&androidbinary.ResTableConfig{
			Language: [2]uint8{uint8('e'), uint8('n')},
		})
		if err != nil {
			s = s2
		} else {
			s = "解析错误"
		}
	}
	labelCN = FilterSensitive(s)
	return
}
