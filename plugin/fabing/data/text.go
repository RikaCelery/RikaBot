// Package data 加载位于 datapath 的小作文
package data

import (
	"crypto/md5"
	"encoding/binary"
	"time"

	binutils "github.com/FloatTech/floatbox/binary"
	sql "github.com/FloatTech/sqlite"
	"github.com/sirupsen/logrus"
)

var db = sql.Sqlite{}

type text struct {
	ID   int64  `db:"id"`
	Data string `db:"data"`
}

// LoadText 加载小作文
func LoadText(dbfile string) error {
	db.DBPath = dbfile
	err := db.Open(time.Hour)
	if err != nil {
		return err
	}
	err = db.Create("text", &text{})
	if err != nil {
		return err
	}
	c, err := db.Count("text")
	if err != nil {
		return err
	}
	logrus.Printf("[fabing]读取%d条小作文", c)
	return nil
}

// CountText 发病文案数目
func CountText() int {
	l, err := db.Count("text")
	if err != nil {
		logrus.Warnln(err)
		return 0
	}
	return l
}

// AddText 添加小作文
func AddText(txt string) error {
	s := md5.Sum(binutils.StringToBytes(txt))
	i := binary.LittleEndian.Uint64(s[:8])
	return db.Insert("text", &text{ID: int64(i), Data: txt})
}

// RandText 随机小作文
func RandText() string {
	var t text
	err := db.Pick("text", &t)
	if err != nil {
		return err.Error()
	}
	return t.Data
}

// HentaiText 发大病
func HentaiText() string {
	var t text
	err := db.Find("text", &t, "where id = -3802576048116006195")
	if err != nil {
		return err.Error()
	}
	return t.Data
}
