package relation

import (
	"fmt"
	"reflect"
	"strings"
)

type BaseTable struct{}

func IsTable(typ reflect.Type) bool {
	return typ.Kind() == reflect.Struct && typ.Field(0).Type == reflect.TypeOf(BaseTable{})
}

func col(cols ...string) string {
	builder := strings.Builder{}

	for i, col := range cols {
		val := fmt.Sprintf(`"%s"`, col)
		if i < len(cols)-1 {
			val += "."
		}

		builder.WriteString(val)
	}

	return builder.String()
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

	FK            string
	PK            string
	JsonAggName   string
	JsonAggParams string

	JoinType JoinType
	SubQuery string

	Fields []Field
	One    []Relation
	Many   []Relation
}

func (r Relation) jsonAgg() string {
	builder := strings.Builder{}

	builder.WriteString(fmt.Sprintf("json_agg(%v", r.jsonBuildObject()))
	if r.JsonAggParams != "" {
		builder.WriteString(fmt.Sprintf(" %v", r.JsonAggParams))
	}

	builder.WriteString(fmt.Sprintf(") %v", col(r.JsonAggName+"_json")))

	return builder.String()
}

func (r Relation) jsonBuildObject() string {
	builder := strings.Builder{}
	builder.WriteString("json_build_object(")

	for _, f := range r.Fields {
		builder.WriteString(fmt.Sprintf(`'%v', %v,`, f.Json, col(r.Table, f.Column)))
	}

	for _, o := range r.One {
		builder.WriteString(fmt.Sprintf(`'%v', %v,`, o.JsonAggName, o.jsonBuildObject()))
	}

	for _, m := range r.Many {
		builder.WriteString(fmt.Sprintf(`'%v', %v,`, m.JsonAggName, col(m.JsonAggName+"_json")))
	}

	result := builder.String()

	result = result[:len(result)-1]
	result += ")"

	return result
}

func (r Relation) oneJoin() string {
	builder := strings.Builder{}

	for _, o := range r.One {
		// this is reverse from many, theirs FK is ours
		builder.WriteString(fmt.Sprintf("%v %v on %v = %v",
			o.JoinType, o.from(), col(o.Table, o.PK), col(r.Table, o.FK)))

		builder.WriteString(fmt.Sprintf(" %v %v", o.oneJoin(), o.manyJoin()))
	}

	return builder.String()
}

func (r Relation) manyJoin() string {
	builder := strings.Builder{}

	for _, m := range r.Many {
		builder.WriteString(fmt.Sprintf(`%v (%v) %v on %v = %v`,
			m.JoinType, m.Render(), col(m.Table), col(m.Table, m.FK), col(r.Table, r.PK)))
	}

	return builder.String()
}

func (r Relation) join() string {
	builder := strings.Builder{}

	builder.WriteString(r.oneJoin())
	builder.WriteString(r.manyJoin())

	return builder.String()
}

func (r Relation) from() string {
	from := ""
	if r.SubQuery == "" {
		from = col(r.Table)
	} else {
		from = fmt.Sprintf("(%v) %v", r.SubQuery, col(r.Table))
	}

	return from
}

func (r Relation) Render() string {
	builder := strings.Builder{}
	builder.WriteString("select ")

	if r.FK != "" {
		builder.WriteString(fmt.Sprintf("%v, ", col(r.Table, r.FK)))
	}

	builder.WriteString(fmt.Sprintf("%v from %v %v", r.jsonAgg(), r.from(), r.join()))

	if r.FK != "" {
		builder.WriteString(fmt.Sprintf(" group by %v", col(r.Table, r.FK)))
	}

	return builder.String()
}
