package tags

import (
	"reflect"
	"strings"
)

type JaggerTag struct {
	Name  string
	PK    bool
	FK    string
	Embed bool
}

func NewJaggerTag(tag reflect.StructTag) JaggerTag {
	dt := JaggerTag{}

	comma := strings.Split(tag.Get("jagger"), ",")
	dt.Name = strings.TrimSpace(comma[0])

	mp := ParseMapTag(strings.Join(comma[1:], ","))

	for k, v := range mp {
		switch k {
		case "pk":
			dt.PK = true
		case "fk":
			dt.FK = v
		case "embed":
			dt.Embed = true
		}
	}

	return dt
}

func ParseMapTag(tag string) map[string]string {
	comma := strings.Split(tag, ",")
	result := map[string]string{}

	for _, v := range comma {
		v = strings.TrimSpace(v)
		colon := strings.Split(v, ":")

		key := colon[0]
		value := ""

		if len(colon) == 2 {
			value = colon[1]
		}

		result[key] = value
	}

	return result
}

func ParseSliceTag(tag string) []string {
	comma := strings.Split(tag, ",")

	for i := range comma {
		comma[i] = strings.TrimSpace(comma[i])
	}

	return comma
}
