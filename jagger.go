package jagger

import (
	"errors"
	"fmt"
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

type QueryBuilderJoin struct {
	joinType      JoinType
	subQuery      string
	jsonAggParams string
	args          []any
}

type QueryBuilderJoins map[string]QueryBuilderJoin

type QueryBuilder struct {
	// the target struct
	target any
	joins  QueryBuilderJoins
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

func (qb *QueryBuilder) toRelation(target any, seen map[string]bool, args *[]any) (*relation.Relation, error) {
	t, ok := target.(reflect.Type)
	if !ok {
		t = reflect.TypeOf(target)
	}

	if !relation.IsTable(t) {
		return nil, errors.New("target not a table")
	}

	name := tags.NewJaggerTag(t.Field(0).Tag.Get("jagger")).Name
	join, joinExists := qb.joins[name]

	if seen[name] || !joinExists {
		return nil, nil
	}

	subQuery, err := toIncrementedArgsQuery(join.subQuery, len(*args))
	if err != nil {
		return nil, err
	}

	*args = append(*args, join.args...)

	seen[name] = true

	fields := []relation.Field{}
	one := []relation.Relation{}
	many := []relation.Relation{}
	pk := ""

	for i := 1; i < t.NumField(); i++ {
		f := t.Field(i)

		tag := tags.NewJaggerTag(f.Tag.Get("jagger"))
		jsonTag := tags.ParseSliceTag(f.Tag.Get("json"))

		if tag.PK {
			pk = tag.Name
		}

		if tag.FK == "" {
			fields = append(fields, relation.Field{
				Json:   jsonTag[0],
				Column: tag.Name,
			})
			continue
		}

		rel := (*relation.Relation)(nil)
		err := (error)(nil)

		switch f.Type.Kind() {
		case reflect.Struct:
			rel, err = qb.toRelation(f.Type, seen, args)
		case reflect.Pointer, reflect.Slice:
			rel, err = qb.toRelation(f.Type.Elem(), seen, args)
		}

		if err != nil {
			return nil, err
		}
		if rel == nil {
			continue
		}

		typ := f.Type
		if f.Type.Kind() == reflect.Pointer {
			typ = typ.Elem()
		}

		rel.FK = tag.FK
		rel.JsonAggName = jsonTag[0]

		switch typ.Kind() {
		case reflect.Slice:
			many = append(many, *rel)
		case reflect.Struct:
			one = append(one, *rel)
		}
	}

	rel := relation.Relation{
		SubQuery:      subQuery,
		JoinType:      join.joinType,
		PK:            pk,
		Table:         name,
		Fields:        fields,
		One:           one,
		Many:          many,
		JsonAggParams: join.jsonAggParams,
	}

	return &rel, nil
}

func (qb *QueryBuilder) ToSql() (string, []any, error) {
	seen := map[string]bool{}
	args := []any{}

	if qb.target == nil {
		return "", nil, errors.New("ToSql called without target")
	}

	rel, err := qb.toRelation(qb.target, seen, &args)
	if err != nil {
		return "", nil, err
	}

	if len(seen) != len(qb.joins) {
		return "", nil, errors.New(fmt.Sprintf("didn't use all joins (%v/%v)", len(seen), len(qb.joins)))
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

func tableName(structure any) (string, error) {
	t := reflect.TypeOf(structure)
	if !relation.IsTable(t) {
		return "", errors.New("non table passed to dbTable")
	}

	return tags.NewJaggerTag(t.Field(0).Tag.Get("jagger")).Name, nil
}

func NewQueryBuilder() *QueryBuilder {
	return &QueryBuilder{joins: QueryBuilderJoins{}}
}

func maybeDeref(v any) any {
	val := reflect.ValueOf(v)
	if val.Kind() == reflect.Pointer {
		return val.Elem().Interface()
	}

	return v
}

// panics if table arg is not table-like
func (qb *QueryBuilder) Select(table any, jsonAggParams string, subQuery string, args ...any) *QueryBuilder {
	table = maybeDeref(table)

	qb.target = table
	qb.Join("", table, jsonAggParams, subQuery, args...)
	return qb
}

// panics if table is not table-like
func (qb *QueryBuilder) Join(joinType JoinType, table any, jsonAggParams string, subQuery string, args ...any) *QueryBuilder {
	table = maybeDeref(table)

	name, err := tableName(table)
	if err != nil {
		panic(err)
	}

	qb.joins[name] = QueryBuilderJoin{
		joinType:      joinType,
		subQuery:      subQuery,
		args:          args,
		jsonAggParams: jsonAggParams,
	}

	return qb
}

// panics if table arg is not table-like
func (qb *QueryBuilder) LeftJoin(table any, jsonAggParams string, subQuery string, args ...any) *QueryBuilder {
	return qb.Join(relation.LEFT_JOIN, table, jsonAggParams, subQuery, args...)
}

// panics if table arg is not table-like
func (qb *QueryBuilder) RightJoin(table any, jsonAggParams string, subQuery string, args ...any) *QueryBuilder {
	return qb.Join(relation.RIGHT_JOIN, table, jsonAggParams, subQuery, args...)
}

// panics if table arg is not table-like
func (qb *QueryBuilder) InnerJoin(table any, jsonAggParams string, subQuery string, args ...any) *QueryBuilder {
	return qb.Join(relation.INNER_JOIN, table, jsonAggParams, subQuery, args...)
}

// panics if table arg is not table-like
func (qb *QueryBuilder) FullOuterJoin(table any, jsonAggParams string, subQuery string, args ...any) *QueryBuilder {
	return qb.Join(relation.FULL_OUTER_JOIN, table, jsonAggParams, subQuery, args...)
}

func (qb *QueryBuilder) Clone() *QueryBuilder {
	copied := NewQueryBuilder()

	// only a shallow copy
	// the caller could still change for example
	// the arguments, but this is fine
	copied.target = qb.target
	for k, v := range qb.joins {
		copied.joins[k] = v
	}

	return copied
}
