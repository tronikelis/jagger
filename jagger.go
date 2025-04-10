package jagger

import (
	"fmt"
	"maps"
	"reflect"
	"strings"

	"github.com/tronikelis/jagger/relation"
	"github.com/tronikelis/jagger/tags"
)

type BaseTable struct{}

type (
	JoinType = relation.JoinType
	SubQuery = relation.SubQuery
)

type joinParams struct {
	joinType JoinType
	subQuery SubQuery
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
	var inner func(typ reflect.Type) (table, error)
	inner = func(typ reflect.Type) (table, error) {
		if typ.Kind() == reflect.Pointer || typ.Kind() == reflect.Slice {
			typ = typ.Elem()
		}
		if typ.Kind() != reflect.Struct {
			return table{}, fmt.Errorf("Passed type not struct, got %v", typ)
		}

		t := table{fieldsByName: map[string]reflect.StructField{}}

		for i := range typ.NumField() {
			field := typ.Field(i)
			tag := tags.NewJaggerTag(field.Tag)

			if field.Type == reflect.TypeOf(BaseTable{}) {
				t.name = tag.Name
				continue
			}

			if tag.Embed {
				embedded, err := inner(field.Type)
				if err != nil {
					return table{}, err
				}

				maps.Copy(t.fieldsByName, embedded.fieldsByName)
				t.fields = append(t.fields, embedded.fields...)
				if embedded.name != "" {
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

	t, err := inner(typ)
	if err != nil {
		return table{}, err
	}

	if t.name == "" {
		return table{}, fmt.Errorf("Passed type does not have BaseTable embedded")
	}

	return t, nil
}

func toRelation(table table, joinTree *joinTree) (relation.Relation, error) {
	currentRel := relation.Relation{
		SubQuery: joinTree.params.subQuery,
		JoinType: joinTree.params.joinType,
		Table:    table.name,
	}

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

		rel, err := toRelation(t, child)
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

func (qb *QueryBuilder) ToSql() (string, []any, error) {
	if qb.target == nil {
		return "", nil, fmt.Errorf("ToSql called without target")
	}

	table, err := newTable(reflect.TypeOf(qb.target))
	if err != nil {
		return "", nil, err
	}

	rel, err := toRelation(table, newJoinTree(qb.params, qb.joins))
	if err != nil {
		return "", nil, err
	}

	var args []any
	rendered, err := rel.Render(nil, &args)
	if err != nil {
		return "", nil, err
	}

	return rendered, args, nil
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

func (qb *QueryBuilder) Select(table any, subQuery SubQuery) *QueryBuilder {
	qb.target = table
	qb.params = joinParams{
		subQuery: subQuery,
	}

	return qb
}

func (qb *QueryBuilder) Join(joinType JoinType, path string, subQuery SubQuery) *QueryBuilder {
	qb.joins[path] = joinParams{
		joinType: joinType,
		subQuery: subQuery,
	}

	return qb
}

func (qb *QueryBuilder) LeftJoin(path string, subQuery SubQuery) *QueryBuilder {
	return qb.Join(relation.LEFT_JOIN, path, subQuery)
}

func (qb *QueryBuilder) RightJoin(path string, subQuery SubQuery) *QueryBuilder {
	return qb.Join(relation.RIGHT_JOIN, path, subQuery)
}

func (qb *QueryBuilder) InnerJoin(path string, subQuery SubQuery) *QueryBuilder {
	return qb.Join(relation.INNER_JOIN, path, subQuery)
}

func (qb *QueryBuilder) FullOuterJoin(path string, subQuery SubQuery) *QueryBuilder {
	return qb.Join(relation.FULL_OUTER_JOIN, path, subQuery)
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
