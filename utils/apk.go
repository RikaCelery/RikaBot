package utils

import (
	"image"

	"github.com/shogo82148/androidbinary"
	"github.com/shogo82148/androidbinary/apk"
)

// ParseApk 解析apk
func ParseApk(file string) (icon *image.Image, pkgName, labelCN string, manifest apk.Manifest, err error) {
	pkg, err := apk.OpenFile(file)
	if err != nil {
		return
	}
	i, e := pkg.Icon(nil)
	if e != nil {
		icon = nil
	} else {
		icon = &i
	}
	pkgName = FilterSensitive(pkg.PackageName())
	manifest = pkg.Manifest()
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
