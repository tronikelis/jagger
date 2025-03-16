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

type BaseTable struct{}

type (
	JoinType = relation.JoinType
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
	children []*joinTree
}

func upsertJoinTree(current *joinTree, params joinParams, fields []string) {
	for i, field := range fields {
		found := false

		for _, child := range current.children {
			if child.field == field {
				current = child
				found = true
				break
			}
		}

		if !found {
			j := &joinTree{field: field}
			j.params.joinType = params.joinType
			current.children = append(current.children, j)
			current = j
		}

		if i == len(fields)-1 {
			current.params = params
		}
	}
}

func newJoinTree(rootParams joinParams, joins map[string]joinParams) *joinTree {
	joinTree := &joinTree{params: rootParams}

	for k, v := range joins {
		fields := strings.Split(k, ".")
		upsertJoinTree(joinTree, v, fields)
	}

	return joinTree
}

type QueryBuilder struct {
	// the target struct
	target any
	params joinParams

	joins map[string]joinParams
}

type table struct {
	name         string
	fieldsByName map[string]reflect.StructField
	fields       []reflect.StructField
}

func newTable(typ reflect.Type) (table, error) {
	if typ.Kind() == reflect.Pointer || typ.Kind() == reflect.Slice {
		typ = typ.Elem()
	}
	if typ.Kind() != reflect.Struct {
		return table{}, fmt.Errorf("Passed type not struct, got %v", typ)
	}

	var i int

	t := table{fieldsByName: map[string]reflect.StructField{}}
	if typ.NumField() > 1 && typ.Field(0).Type == reflect.TypeOf(BaseTable{}) {
		t.name = tags.NewJaggerTag(typ.Field(0).Tag).Name
		i++
	}

	for ; i < typ.NumField(); i++ {
		field := typ.Field(i)
		tag := tags.NewJaggerTag(field.Tag)

		if tag.Embed {
			embedded, err := newTable(field.Type)
			if err != nil {
				return table{}, err
			}

			maps.Copy(t.fieldsByName, embedded.fieldsByName)
			t.fields = append(t.fields, embedded.fields...)
			if t.name == "" {
				t.name = embedded.name
			}
			continue
		}

		if tag.Name == "-" || reflect.ValueOf(tag).IsZero() {
			continue
		}

		t.fieldsByName[field.Name] = field
		t.fields = append(t.fields, field)
	}

	return t, nil
}

func toRelation(table table, joinTree *joinTree, args *[]any) (relation.Relation, error) {
	subQuery, err := toIncrementedArgsQuery(joinTree.params.subQuery, len(*args))
	if err != nil {
		return relation.Relation{}, err
	}

	currentRel := relation.Relation{
		JoinType:      joinTree.params.joinType,
		JsonAggParams: joinTree.params.jsonAggParams,
		Table:         table.name,
		SubQuery:      subQuery,
	}

	*args = append(*args, joinTree.params.args...)

	for _, f := range table.fields {
		tag := tags.NewJaggerTag(f.Tag)
		jsonTag := tags.ParseSliceTag(f.Tag.Get("json"))

		if tag.PK {
			currentRel.PK = tag.Name
		}

		// this is a join field, will add these in the next loop
		if tag.FK != "" {
			continue
		}

		currentRel.Fields = append(currentRel.Fields, relation.Field{Json: jsonTag[0], Column: tag.Name})
	}

	for _, child := range joinTree.children {
		f, ok := table.fieldsByName[child.field]
		if !ok {
			return relation.Relation{}, fmt.Errorf("Field %s not found", child.field)
		}

		t, err := newTable(f.Type)
		if err != nil {
			return relation.Relation{}, err
		}

		rel, err := toRelation(t, child, args)
		if err != nil {
			return relation.Relation{}, err
		}
		rel.ParentTable = currentRel.Table

		tag := tags.NewJaggerTag(f.Tag)
		jsonTag := tags.ParseSliceTag(f.Tag.Get("json"))

		rel.FK = tag.FK
		rel.JsonName = jsonTag[0]

		fType := f.Type
		if fType.Kind() == reflect.Pointer {
			fType = fType.Elem()
		}

		switch fType.Kind() {
		case reflect.Slice:
			currentRel.Many = append(currentRel.Many, rel)
		case reflect.Struct:
			currentRel.One = append(currentRel.One, rel)
		default:
			return relation.Relation{}, fmt.Errorf("Cant join %s type", fType.String())
		}
	}

	return currentRel, nil
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

	table, err := newTable(reflect.TypeOf(qb.target))
	if err != nil {
		return "", nil, err
	}

	rel, err := toRelation(table, newJoinTree(qb.params, qb.joins), &args)
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
