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

	builder.WriteString(fmt.Sprintf("json_agg(%s", r.jsonBuildObject()))
	if r.JsonAggParams != "" {
		builder.WriteString(fmt.Sprintf(" %s", r.JsonAggParams))
	}

	builder.WriteString(fmt.Sprintf(") %s", col(r.JsonAggName+"_json")))

	return builder.String()
}

func (r Relation) jsonBuildObject() string {
	builder := strings.Builder{}
	builder.WriteString("json_build_object(")

	for _, f := range r.Fields {
		builder.WriteString(fmt.Sprintf(`'%s', %s,`, f.Json, col(r.Table, f.Column)))
	}

	for _, o := range r.One {
		builder.WriteString(fmt.Sprintf(`'%s', %s,`, o.JsonAggName, o.jsonBuildObject()))
	}

	for _, m := range r.Many {
		builder.WriteString(fmt.Sprintf(`'%s', %s,`, m.JsonAggName, col(m.JsonAggName+"_json")))
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

		if o.SubQuery == "" {
			builder.WriteString(fmt.Sprintf("%s %s on %s = %s",
				o.JoinType, col(o.Table), col(o.Table, o.PK), col(r.Table, o.FK)))
		} else {
			builder.WriteString(fmt.Sprintf("%s (%s) %s on %s = %s",
				o.JoinType, o.SubQuery, col(o.Table), col(o.Table, o.PK), col(r.Table, o.FK)))
		}

		builder.WriteString(fmt.Sprintf(" %s %s", o.oneJoin(), o.manyJoin()))
	}

	return builder.String()
}

func (r Relation) manyJoin() string {
	builder := strings.Builder{}

	for _, m := range r.Many {
		builder.WriteString(fmt.Sprintf(`%s (%s) %s on %s = %s`,
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

func (r Relation) Render() string {
	builder := strings.Builder{}
	builder.WriteString("select ")

	if r.FK != "" {
		builder.WriteString(fmt.Sprintf("%s, ", col(r.Table, r.FK)))
	}

	if r.SubQuery == "" {
		builder.WriteString(fmt.Sprintf("%s from %s %s", r.jsonAgg(), col(r.Table), r.join()))
	} else {
		builder.WriteString(fmt.Sprintf("%s from (%s) %s %s", r.jsonAgg(), r.SubQuery, col(r.Table), r.join()))
	}

	if r.FK != "" {
		builder.WriteString(fmt.Sprintf(" group by %s", col(r.Table, r.FK)))
	}

	return builder.String()
}
