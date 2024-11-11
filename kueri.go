package kueri

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"unicode"

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

func toIncrementedArgsQuery(query string, by int) (string, error) {
	QUOTE := '"'
	COMMA := '\''

	counts := map[rune]int{}

	bySpace := strings.Split(query, " ")

	for i, v := range bySpace {
		for iChunk, chunk := range v {
			if chunk == COMMA || chunk == QUOTE {
				counts[chunk]++
			}

			// we are only interested in chunks which start with $
			if iChunk != 0 || chunk != '$' {
				continue
			}
			// we are not inside a ' or a ""
			if counts[COMMA]%2 != 0 || counts[QUOTE]%2 != 0 {
				continue
			}
			// chunk is too small to be viable
			if len(v) < 2 {
				continue
			}
			// $<char should be number>
			if !unicode.IsDigit(rune(v[1])) {
				continue
			}

			arg, err := strconv.Atoi(v[1:])
			if err != nil {
				return "", err
			}

			arg += by
			bySpace[i] = "$" + strconv.Itoa(arg)

			break
		}
	}

	return strings.Join(bySpace, " "), nil
}

func (qb *QueryBuilder) toRelation(target any, seen map[string]bool, args *[]any) (*relation.Relation, error) {
	t, ok := target.(reflect.Type)
	if !ok {
		t = reflect.TypeOf(target)
	}

	if !relation.IsTable(t) {
		return nil, errors.New("target not a table")
	}

	name := tags.NewKueriTag(t.Field(0).Tag.Get("kueri")).Name
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

		tag := tags.NewKueriTag(f.Tag.Get("kueri"))
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
		SubQuery: subQuery,
		JoinType: join.joinType,
		PK:       pk,
		Table:    name,
		Fields:   fields,
		One:      one,
		Many:     many,
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

func tableName(structure any) (string, error) {
	t := reflect.TypeOf(structure)
	if !relation.IsTable(t) {
		return "", errors.New("non table passed to dbTable")
	}

	return tags.NewKueriTag(t.Field(0).Tag.Get("kueri")).Name, nil
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
