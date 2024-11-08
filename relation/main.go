package relation

import (
	"fmt"
	"reflect"
)

type BaseTable struct{}

func IsTable(typ reflect.Type) bool {
	return typ.Kind() == reflect.Struct && typ.Field(0).Type == reflect.TypeOf(BaseTable{})
}

type JoinType string

const (
	LEFT_JOIN       JoinType = "left join"
	RIGHT_JOIN      JoinType = "right join"
	FULL_OUTER_JOIN JoinType = "full outer join"
	INNER_JOIN      JoinType = "inner join"
)

type Field struct {
	Json   string
	Column string
}

type Relation struct {
	Table string

	FK          string
	PK          string
	JsonAggName string

	JoinType JoinType
	SubQuery string

	Fields []Field
	One    []Relation
	Many   []Relation
}

func (r Relation) jsonAgg() string {
	return fmt.Sprintf("json_agg(%v) %v_json ", r.jsonBuildObject(), r.JsonAggName)

}

func (r Relation) jsonBuildObject() string {
	result := "json_build_object("

	for _, f := range r.Fields {
		result += fmt.Sprintf(`'%v', %v.%v,`, f.Json, r.Table, f.Column)
	}

	for _, o := range r.One {
		result += fmt.Sprintf(`'%v', %v,`, o.JsonAggName, o.jsonBuildObject())
	}

	for _, m := range r.Many {
		result += fmt.Sprintf(`'%v', %v_json,`, m.JsonAggName, m.JsonAggName)
	}

	result = result[:len(result)-1]

	result += ")"

	return result
}

func (r Relation) oneJoin() string {
	result := ""

	for _, o := range r.One {
		// this is reverse from many, theirs FK is ours
		result += fmt.Sprintf("%v %v on %v.%v = %v.%v",
			o.JoinType, o.Table, o.from(), o.PK, r.Table, o.FK)

		result += fmt.Sprintf(" %v %v", o.oneJoin(), o.manyJoin())
	}

	return result
}

func (r Relation) manyJoin() string {
	result := ""

	for _, m := range r.Many {
		result += fmt.Sprintf(`%v (%v) %v on %v.%v = %v.%v`,
			m.JoinType, m.Render(), m.Table, m.Table, m.FK, r.Table, r.PK)
	}

	return result
}

func (r Relation) join() string {
	result := ""

	result += r.oneJoin()
	result += r.manyJoin()

	return result
}

func (r Relation) from() string {
	from := ""
	if r.SubQuery == "" {
		from = r.Table
	} else {
		from = fmt.Sprintf("(%v) %v", r.SubQuery, r.Table)
	}

	return from
}

func (r Relation) Render() string {
	result := "select "

	if r.FK != "" {
		result += fmt.Sprintf("%v.%v, ", r.Table, r.FK)
	}

	result += fmt.Sprintf("%v from %v %v", r.jsonAgg(), r.from(), r.join())

	if r.FK != "" {
		result += fmt.Sprintf(" group by %v.%v", r.Table, r.FK)
	}

	return result
}
