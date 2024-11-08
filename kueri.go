package kueri

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/tronikelis/kueri/relation"
	"github.com/tronikelis/kueri/tags"
)

type BaseTable = relation.BaseTable
type JoinType = relation.JoinType

type QueryBuilderJoin struct {
	joinType JoinType
	subQuery string
	args     []any
}

type QueryBuilderJoins map[string]QueryBuilderJoin

type QueryBuilder struct {
	// the target struct
	target any
	joins  QueryBuilderJoins
}

func (qb *QueryBuilder) toRelation(target any, seen map[string]bool, args *[]any) *relation.Relation {
	t, ok := target.(reflect.Type)
	if !ok {
		t = reflect.TypeOf(target)
	}

	if !relation.IsTable(t) {
		return nil
	}

	name := tags.NewDbTag(t.Field(0).Tag.Get("db")).Name
	join, joinExists := qb.joins[name]

	if seen[name] || !joinExists {
		return nil
	}

	*args = append(*args, join.args...)

	seen[name] = true

	fields := []relation.Field{}
	one := []relation.Relation{}
	many := []relation.Relation{}
	pk := ""

	for i := 1; i < t.NumField(); i++ {
		f := t.Field(i)

		db := tags.NewDbTag(f.Tag.Get("db"))
		jsonTag := tags.ParseSliceTag(f.Tag.Get("json"))

		if db.PK {
			pk = db.Name
		}

		if db.FK == "" {
			fields = append(fields, relation.Field{
				Json:   jsonTag[0],
				Column: db.Name,
			})
			continue
		}

		rel := (*relation.Relation)(nil)

		switch f.Type.Kind() {
		case reflect.Struct:
			rel = qb.toRelation(f.Type, seen, args)
		case reflect.Pointer, reflect.Slice:
			rel = qb.toRelation(f.Type.Elem(), seen, args)
		}

		if rel == nil {
			continue
		}

		typ := f.Type
		if f.Type.Kind() == reflect.Pointer {
			typ = typ.Elem()
		}

		rel.FK = db.FK
		rel.JsonAggName = jsonTag[0]

		switch typ.Kind() {
		case reflect.Slice:
			many = append(many, *rel)
		case reflect.Struct:
			one = append(one, *rel)
		}
	}

	rel := relation.Relation{
		SubQuery: join.subQuery,
		JoinType: join.joinType,
		PK:       pk,
		Table:    name,
		Fields:   fields,
		One:      one,
		Many:     many,
	}

	return &rel
}

func (qb *QueryBuilder) ToSql() (string, []any, error) {
	seen := map[string]bool{}
	args := []any{}

	if qb.target == nil {
		return "", nil, errors.New("ToSql called without target")
	}

	rel := qb.toRelation(qb.target, seen, &args)
	if rel == nil {
		return "", nil, errors.New("unsupported target")
	}

	if len(seen) != len(qb.joins) {
		return "", nil, errors.New(fmt.Sprintf("didn't use all joins (%v/%v)", len(seen), len(qb.joins)))
	}

	return rel.Render(), args, nil
}

func tableName(structure any) (string, error) {
	t := reflect.TypeOf(structure)
	if !relation.IsTable(t) {
		return "", errors.New("non table passed to dbTable")
	}

	return tags.NewDbTag(t.Field(0).Tag.Get("db")).Name, nil
}

func NewQueryBuilder() *QueryBuilder {
	return &QueryBuilder{joins: QueryBuilderJoins{}}
}

// panics if table arg is not table-like
func (qb *QueryBuilder) Select(table any, subQuery string, args ...any) *QueryBuilder {
	qb.target = table
	qb.Join("", table, subQuery, args...)
	return qb
}

// panics if table is not table-like
func (qb *QueryBuilder) Join(joinType JoinType, table any, subQuery string, args ...any) *QueryBuilder {
	name, err := tableName(table)
	if err != nil {
		panic(err)
	}

	qb.joins[name] = QueryBuilderJoin{
		joinType: joinType,
		subQuery: subQuery,
		args:     args,
	}

	return qb
}

// panics if table arg is not table-like
func (qb *QueryBuilder) LeftJoin(table any, subQuery string, args ...any) *QueryBuilder {
	return qb.Join(relation.LEFT_JOIN, table, subQuery, args...)
}

// panics if table arg is not table-like
func (qb *QueryBuilder) RightJoin(table any, subQuery string, args ...any) *QueryBuilder {
	return qb.Join(relation.RIGHT_JOIN, table, subQuery, args...)
}

// panics if table arg is not table-like
func (qb *QueryBuilder) InnerJoin(table any, subQuery string, args ...any) *QueryBuilder {
	return qb.Join(relation.INNER_JOIN, table, subQuery, args...)
}

// panics if table arg is not table-like
func (qb *QueryBuilder) FullOuterJoin(table any, subQuery string, args ...any) *QueryBuilder {
	return qb.Join(relation.FULL_OUTER_JOIN, table, subQuery, args...)
}
