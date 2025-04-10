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

type SubQuery = func(cond string) (string, []any, error)

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
	SubQuery SubQuery

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

func (r Relation) oneJoin(args *[]any) (string, error) {
	builder := strings.Builder{}

	for _, o := range r.One {
		from, err := o.from(o.onOneJoin(r, o.Table), args)
		if err != nil {
			return "", err
		}

		// this is reverse from many, theirs FK is ours
		builder.WriteString(fmt.Sprintf("%s lateral %s on %s",
			o.JoinType, from, o.onOneJoin(r, o.name())))

		one, err := o.oneJoin(args)
		if err != nil {
			return "", err
		}

		many, err := o.manyJoin(args)
		if err != nil {
			return "", err
		}

		builder.WriteString(fmt.Sprintf(" %s %s", one, many))
	}

	return builder.String(), nil
}

func (r Relation) onManyJoin(parent Relation, name string) string {
	return fmt.Sprintf("%s = %s", col(name, r.FK), col(parent.name(), parent.PK))
}

func (r Relation) onOneJoin(parent Relation, name string) string {
	return fmt.Sprintf("%s = %s", col(name, r.PK), col(parent.name(), r.FK))
}

func (r Relation) manyJoin(args *[]any) (string, error) {
	builder := strings.Builder{}

	for _, m := range r.Many {
		from, err := m.Render(&r, args)
		if err != nil {
			return "", err
		}

		builder.WriteString(fmt.Sprintf("%s lateral (%s) %s on %s",
			m.JoinType, from, col(m.name()), m.onManyJoin(r, m.name())))
	}

	return builder.String(), nil
}

func (r Relation) join(args *[]any) (string, error) {
	builder := strings.Builder{}

	one, err := r.oneJoin(args)
	if err != nil {
		return "", err
	}

	many, err := r.manyJoin(args)
	if err != nil {
		return "", err
	}

	builder.WriteString(one)
	builder.WriteString(many)

	return builder.String(), nil
}

func (r Relation) from(cond string, args *[]any) (string, error) {
	if r.SubQuery == nil {
		subQuery := fmt.Sprintf("select *, row_number() over () as jagger_rn from %s", col(r.Table))
		if cond != "" {
			subQuery += fmt.Sprintf(" where %s", cond)
		}

		return fmt.Sprintf("(%s) %s", subQuery, col(r.name())), nil
	}

	subQuery, subQueryArgs, err := r.SubQuery(cond)
	if err != nil {
		return "", err
	}

	incrementSubQueryBy := len(*args)
	*args = append(*args, subQueryArgs...)

	subQuery, err = toIncrementedArgsQuery(subQuery, incrementSubQueryBy)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("(%s) %s", subQuery, col(r.name())), nil
}

func (r Relation) Render(parent *Relation, args *[]any) (string, error) {
	builder := strings.Builder{}
	builder.WriteString("select ")

	if r.FK != "" {
		builder.WriteString(fmt.Sprintf("%s, ", col(r.name(), r.FK)))
	}

	var joinCond string
	if parent != nil {
		joinCond = r.onManyJoin(*parent, r.Table)
	}
	from, err := r.from(joinCond, args)
	if err != nil {
		return "", err
	}

	builder.WriteString(fmt.Sprintf("%s from lateral %s ", r.jsonAgg(), from))

	join, err := r.join(args)
	if err != nil {
		return "", err
	}

	builder.WriteString(join)

	if parent != nil {
		builder.WriteString(fmt.Sprintf("where %s", r.onManyJoin(*parent, r.name())))
	}

	if r.FK != "" {
		builder.WriteString(fmt.Sprintf(" group by %s", col(r.name(), r.FK)))
	}

	return builder.String(), nil
}
