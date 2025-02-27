package jagger

import (
	"fmt"
	"maps"
	"reflect"
	"strconv"
	"strings"
	"unicode"

	"github.com/tronikelis/jagger/relation"
	"github.com/tronikelis/jagger/tags"
)

type (
	BaseTable = relation.BaseTable
	JoinType  = relation.JoinType
)

type joinParams struct {
	joinType      JoinType
	subQuery      string
	jsonAggParams string
	args          []any
}

type joinTree struct {
	field    string
	params   joinParams
	children []joinTree
}

func upsertJoinTree(current []joinTree, params joinParams, fields []string, index int) []joinTree {
	field := fields[index]

	for i, child := range current {
		if child.field == field {
			if index == len(fields)-1 {
				current[i].params = params
				return current
			}

			current[i].children = upsertJoinTree(child.children, params, fields, index+1)
			return current
		}
	}

	j := joinTree{field: field}
	if j.params.joinType == "" {
		j.params.joinType = params.joinType
	}

	current = append(current, upsertJoinTree([]joinTree{j}, params, fields, index)...)

	return current
}

func newJoinTree(rootParams joinParams, joins map[string]joinParams) joinTree {
	joinTree := joinTree{params: rootParams}

	for k, v := range joins {
		fields := strings.Split(k, ".")
		joinTree.children = upsertJoinTree(joinTree.children, v, fields, 0)
	}

	return joinTree
}

type QueryBuilder struct {
	// the target struct
	target any
	params joinParams

	joins map[string]joinParams
}

func toRelation(typ reflect.Type, joinTree joinTree, args *[]any) (relation.Relation, error) {
	if typ.Kind() == reflect.Pointer || typ.Kind() == reflect.Slice {
		typ = typ.Elem()
	}

	if !relation.IsTable(typ) {
		return relation.Relation{}, fmt.Errorf("Passed type not a table")
	}

	rel := relation.Relation{}
	rel.JoinType = joinTree.params.joinType
	rel.JsonAggParams = joinTree.params.jsonAggParams

	jaggerTag := tags.NewJaggerTag(typ.Field(0).Tag.Get("jagger"))
	rel.Table = jaggerTag.Name

	subQuery, err := toIncrementedArgsQuery(joinTree.params.subQuery, len(*args))
	if err != nil {
		return relation.Relation{}, err
	}
	rel.SubQuery = subQuery

	*args = append(*args, joinTree.params.args...)

	fields := []relation.Field{}
	for i := 1; i < typ.NumField(); i++ {
		f := typ.Field(i)

		tag := tags.NewJaggerTag(f.Tag.Get("jagger"))
		jsonTag := tags.ParseSliceTag(f.Tag.Get("json"))

		if tag.PK {
			rel.PK = tag.Name
		}

		// this is a join field, will add these in the next loop
		if tag.FK != "" {
			continue
		}

		fields = append(fields, relation.Field{Json: jsonTag[0], Column: tag.Name})
	}

	rel.Fields = fields

	var one, many []relation.Relation
	for _, child := range joinTree.children {
		f, ok := typ.FieldByName(child.field)
		if !ok {
			return relation.Relation{}, fmt.Errorf("Field %s not found", child.field)
		}

		rel, err := toRelation(f.Type, child, args)
		if err != nil {
			return relation.Relation{}, err
		}

		tag := tags.NewJaggerTag(f.Tag.Get("jagger"))
		jsonTag := tags.ParseSliceTag(f.Tag.Get("json"))

		rel.FK = tag.FK
		rel.JsonName = jsonTag[0]

		fType := f.Type
		if fType.Kind() == reflect.Pointer {
			fType = fType.Elem()
		}

		switch fType.Kind() {
		case reflect.Slice:
			many = append(many, rel)
		case reflect.Struct:
			one = append(one, rel)
		default:
			return relation.Relation{}, fmt.Errorf("Cant join %s type", fType.String())
		}
	}

	rel.One = one
	rel.Many = many

	return rel, nil
}

func toIncrementedArgsQuery(query string, by int) (string, error) {
	counts := struct {
		quote  int
		quotes int
	}{}
	runes := []rune(query)

	builder := strings.Builder{}
	acc := strings.Builder{}

	for i := 0; i < len(runes); i++ {
		builder.WriteRune(runes[i])

		if runes[i] == '$' && counts.quote%2 == 0 && counts.quotes%2 == 0 {
			acc.WriteString(builder.String())
			builder.Reset()

			for i+1 < len(runes) && unicode.IsDigit(runes[i+1]) {
				i++
				builder.WriteRune(runes[i])
			}
			if builder.Len() == 0 {
				continue
			}

			num, err := strconv.Atoi(builder.String())
			if err != nil {
				return "", err
			}
			builder.Reset()

			num += by

			acc.WriteString(strconv.Itoa(num))
			continue
		}

		switch runes[i] {
		case '"':
			counts.quotes++
		case '\'':
			counts.quote++
		}
	}

	acc.WriteString(builder.String())

	return acc.String(), nil
}

func (qb *QueryBuilder) ToSql() (string, []any, error) {
	args := []any{}

	if qb.target == nil {
		return "", nil, fmt.Errorf("ToSql called without target")
	}

	rel, err := toRelation(reflect.TypeOf(qb.target), newJoinTree(qb.params, qb.joins), &args)
	if err != nil {
		return "", nil, err
	}

	return rel.Render(), args, nil
}

// calls .ToSql and panics if error
func (qb *QueryBuilder) MustSql() (string, []any) {
	sql, args, err := qb.ToSql()
	if err != nil {
		panic(err)
	}

	return sql, args
}

func NewQueryBuilder() *QueryBuilder {
	return &QueryBuilder{joins: map[string]joinParams{}}
}

func (qb *QueryBuilder) Select(table any, jsonAggParams string, subQuery string, args ...any) *QueryBuilder {
	qb.target = table
	qb.params = joinParams{
		jsonAggParams: jsonAggParams,
		subQuery:      subQuery,
		args:          args,
	}

	return qb
}

func (qb *QueryBuilder) Join(joinType JoinType, path string, jsonAggParams string, subQuery string, args ...any) *QueryBuilder {
	qb.joins[path] = joinParams{
		joinType:      joinType,
		subQuery:      subQuery,
		args:          args,
		jsonAggParams: jsonAggParams,
	}

	return qb
}

func (qb *QueryBuilder) LeftJoin(path string, jsonAggParams string, subQuery string, args ...any) *QueryBuilder {
	return qb.Join(relation.LEFT_JOIN, path, jsonAggParams, subQuery, args...)
}

func (qb *QueryBuilder) RightJoin(path string, jsonAggParams string, subQuery string, args ...any) *QueryBuilder {
	return qb.Join(relation.RIGHT_JOIN, path, jsonAggParams, subQuery, args...)
}

func (qb *QueryBuilder) InnerJoin(path string, jsonAggParams string, subQuery string, args ...any) *QueryBuilder {
	return qb.Join(relation.INNER_JOIN, path, jsonAggParams, subQuery, args...)
}

func (qb *QueryBuilder) FullOuterJoin(path string, jsonAggParams string, subQuery string, args ...any) *QueryBuilder {
	return qb.Join(relation.FULL_OUTER_JOIN, path, jsonAggParams, subQuery, args...)
}

func (qb *QueryBuilder) Clone() *QueryBuilder {
	copied := NewQueryBuilder()

	// only a shallow copy
	// the caller could still change for example
	// the arguments, but this is fine
	copied.target = qb.target
	maps.Copy(copied.joins, qb.joins)

	return copied
}
