package relation

import (
	"fmt"
	"strings"
)

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
	// Thought to have *Relation, but I will only use parent table
	// So this is simpler
	ParentTable string
	Table       string

	// this can be empty, for example pivot tables
	PK string

	FK       string
	JsonName string

	JoinType JoinType
	SubQuery string

	Fields []Field
	One    []Relation
	Many   []Relation
}

func (r Relation) name() string {
	table := r.Table
	if r.ParentTable != "" {
		table = r.ParentTable
	}

	return fmt.Sprintf("%s.%s", table, r.JsonName)
}

func (r Relation) nameJson() string {
	return r.name() + "_json"
}

func (r Relation) jsonAgg() string {
	builder := strings.Builder{}

	builder.WriteString(fmt.Sprintf("json_agg(%s", r.jsonBuildObject()))
	builder.WriteString(fmt.Sprintf(" order by %s) %s", col(r.name(), "jagger_rn"), col(r.nameJson())))

	return builder.String()
}

func (self Relation) stripNulls(input string) string {
	builder := strings.Builder{}

	if self.PK != "" {
		builder.WriteString(fmt.Sprintf("case when %s is null then null else ", col(self.name(), self.PK)))
	}

	builder.WriteString("json_strip_nulls(json_build_object(")
	builder.WriteString(input)

	builder.WriteString("))")
	if self.PK != "" {
		builder.WriteString(" end")
	}

	return builder.String()
}

func (r Relation) jsonBuildObject() string {
	builder := strings.Builder{}

	for _, f := range r.Fields {
		builder.WriteString(fmt.Sprintf(`'%s', %s,`, f.Json, col(r.name(), f.Column)))
	}

	for _, o := range r.One {
		builder.WriteString(fmt.Sprintf(`'%s', %s,`, o.JsonName, o.jsonBuildObject()))
	}

	for _, m := range r.Many {
		builder.WriteString(fmt.Sprintf(`'%s', %s,`, m.JsonName, col(m.nameJson())))
	}

	result := builder.String()
	result = result[:len(result)-1]

	return r.stripNulls(result)
}

func (r Relation) oneJoin() string {
	builder := strings.Builder{}

	for _, o := range r.One {
		// this is reverse from many, theirs FK is ours
		builder.WriteString(fmt.Sprintf("%s %s on %s",
			o.JoinType, o.from(), o.onOneJoin(r)))

		builder.WriteString(fmt.Sprintf(" %s %s", o.oneJoin(), o.manyJoin()))
	}

	return builder.String()
}

func (r Relation) onManyJoin(parent Relation) string {
	return fmt.Sprintf("%s = %s", col(r.name(), r.FK), col(parent.name(), parent.PK))
}

func (r Relation) onOneJoin(parent Relation) string {
	return fmt.Sprintf("%s = %s", col(r.name(), r.PK), col(parent.name(), r.FK))
}

func (r Relation) manyJoin() string {
	builder := strings.Builder{}

	for _, m := range r.Many {
		builder.WriteString(fmt.Sprintf("%s lateral (%s) %s on %s",
			m.JoinType, m.Render(&r), col(m.name()), m.onManyJoin(r)))
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
	if r.SubQuery == "" {
		r.SubQuery = fmt.Sprintf("select *, row_number() over () as jagger_rn from %s", col(r.Table))
	}

	return fmt.Sprintf("(%s) %s", r.SubQuery, col(r.name()))
}

func (r Relation) Render(parent *Relation) string {
	builder := strings.Builder{}
	builder.WriteString("select ")

	if r.FK != "" {
		builder.WriteString(fmt.Sprintf("%s, ", col(r.name(), r.FK)))
	}

	builder.WriteString(fmt.Sprintf("%s from %s ", r.jsonAgg(), r.from()))
	builder.WriteString(r.join())

	if parent != nil {
		builder.WriteString(fmt.Sprintf("where %s", r.onManyJoin(*parent)))
	}

	if r.FK != "" {
		builder.WriteString(fmt.Sprintf(" group by %s", col(r.name(), r.FK)))
	}

	return builder.String()
}
