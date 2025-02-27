package jagger_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tronikelis/jagger"
)

type SongTack struct {
	jagger.BaseTable `jagger:"song_track"`

	ID     int       `jagger:"id, pk:" json:"id"`
	SongId int       `jagger:"song_id" json:"song_id"`
	Song   *UserSong `jagger:", fk:song_id" json:"song"`
}

type UserSong struct {
	jagger.BaseTable `jagger:"user_song"`

	ID     int        `jagger:"id, pk:" json:"id"`
	UserId int        `jagger:"user_id" json:"user_id"`
	User   *User      `jagger:", fk:user_id" json:"user"`
	Tracks []SongTack `jagger:", fk:song_id" json:"tracks"`
}

type User struct {
	jagger.BaseTable `jagger:"user"`

	ID    int        `jagger:"id, pk:" json:"id"`
	Songs []UserSong `jagger:", fk:user_id" json:"songs"`
}

func qb() *jagger.QueryBuilder {
	return jagger.NewQueryBuilder()
}

func trim(in string) string {
	return strings.TrimSpace(in)
}

func TestSimpleQuery(t *testing.T) {
	sql, args, err := qb().
		Select(User{}, "", "").
		ToSql()

	assert.Equal(t, trim(sql), `select json_agg(json_build_object('id', "user"."id")) "_json" from "user"`)
	assert.Equal(t, args, []any{})
	assert.Equal(t, err, nil)

	sql, args, err = qb().
		Select(User{}, "", "user subquery").
		ToSql()

	assert.Equal(t, trim(sql), `select json_agg(json_build_object('id', "user"."id")) "_json" from (user subquery) "user"`)
	assert.Equal(t, args, []any{})
	assert.Equal(t, err, nil)
}

func TestOneToMany(t *testing.T) {
	sql, args, err := qb().
		Select(User{}, "", "").
		LeftJoin("Songs", "", "").
		ToSql()

	assert.Equal(t, trim(sql), `select json_agg(json_build_object('id', "user"."id",'songs', "songs_json")) "_json" from "user" left join (select "user_song"."user_id", json_agg(json_build_object('id', "user_song"."id",'user_id', "user_song"."user_id")) "songs_json" from "user_song"  group by "user_song"."user_id") "user_song" on "user_song"."user_id" = "user"."id"`)
	assert.Equal(t, args, []any{})
	assert.Equal(t, err, nil)

	sql, args, err = qb().
		Select(User{}, "", "").
		LeftJoin("Songs", "", "song sub").
		ToSql()

	assert.Equal(t, trim(sql), `select json_agg(json_build_object('id', "user"."id",'songs', "songs_json")) "_json" from "user" left join (select "user_song"."user_id", json_agg(json_build_object('id', "user_song"."id",'user_id', "user_song"."user_id")) "songs_json" from (song sub) "user_song"  group by "user_song"."user_id") "user_song" on "user_song"."user_id" = "user"."id"`)
	assert.Equal(t, args, []any{})
	assert.Equal(t, err, nil)
}

func TestManyToOne(t *testing.T) {
	sql, args, err := qb().
		Select(UserSong{}, "", "").
		LeftJoin("User", "", "").
		ToSql()

	assert.Equal(t, trim(sql), `select json_agg(json_build_object('id', "user_song"."id",'user_id', "user_song"."user_id",'user', json_build_object('id', "user"."id"))) "_json" from "user_song" left join "user" on "user"."id" = "user_song"."user_id"`)
	assert.Equal(t, args, []any{})
	assert.Equal(t, err, nil)
}

func TestManyToOneSubQuery(t *testing.T) {
	sql, args, err := qb().
		Select(UserSong{}, "", "").
		LeftJoin("User", "", "select * from users").
		ToSql()

	assert.Equal(t, trim(sql), `select json_agg(json_build_object('id', "user_song"."id",'user_id', "user_song"."user_id",'user', json_build_object('id', "user"."id"))) "_json" from "user_song" left join (select * from users) "user" on "user"."id" = "user_song"."user_id"`)
	assert.Equal(t, args, []any{})
	assert.Equal(t, err, nil)
}

func TestMultipleRelations(t *testing.T) {
	sql, args, err := qb().
		Select(User{}, "", "").
		LeftJoin("Songs.User", "", "").
		LeftJoin("Songs.Tracks", "", "").
		ToSql()

	assert.NoError(t, err)
	assert.Equal(t, []any{}, args)
	assert.Equal(t, "", sql)
}

func TestBoth(t *testing.T) {
	sql, args, err := qb().
		Select(User{}, "", "").
		LeftJoin("Songs", "", "").
		LeftJoin("Songs.Tracks", "", "").
		ToSql()

	assert.Equal(t, trim(sql), `select json_agg(json_build_object('id', "user"."id",'songs', "songs_json")) "_json" from "user" left join (select "user_song"."user_id", json_agg(json_build_object('id', "user_song"."id",'user_id', "user_song"."user_id",'tracks', "tracks_json")) "songs_json" from "user_song" left join (select "song_track"."song_id", json_agg(json_build_object('id', "song_track"."id",'song_id', "song_track"."song_id")) "tracks_json" from "song_track"  group by "song_track"."song_id") "song_track" on "song_track"."song_id" = "user_song"."id" group by "user_song"."user_id") "user_song" on "user_song"."user_id" = "user"."id"`)
	assert.Equal(t, args, []any{})
	assert.Equal(t, err, nil)
}

func TestPanicsOnInvalidSelect(t *testing.T) {
	assert.Panics(t, func() {
		_ = qb().Select(struct {
			ID int
		}{}, "", "")
	})

	assert.Panics(t, func() {
		_ = qb().Select("hello", "", "")
	})
}

func TestJoinsMustBeValid(t *testing.T) {
	_, _, err := qb().Select(User{}, "", "").LeftJoin("foo", "", "").ToSql()
	assert.Error(t, err)
}

func TestCorrectArgOrder(t *testing.T) {
	sql, args, err := qb().Select(User{}, "", "", 1, 2).LeftJoin("Songs.Tracks", "", "select * from tracks", 3, 4).LeftJoin("Songs", "", "select * from songs", 5, 6).ToSql()
	assert.Equal(t, "", sql)
	assert.NoError(t, err)
	// user -> user song -> song track
	assert.Equal(t, []any{1, 2, 5, 6, 3, 4}, args)
}

type UserWithSpace struct {
	jagger.BaseTable `jagger:"user with space"`

	ID   int       `jagger:"id with space" json:"id with space"`
	Song *UserSong `jagger:", fk:song id" json:"song with space"`
}

func TestQuotes(t *testing.T) {
	sql, _, _ := qb().Select(UserWithSpace{}, "", "").LeftJoin("Song", "", "").ToSql()

	assert.Equal(t, trim(sql), `select json_agg(json_build_object('id with space', "user with space"."id with space",'song with space', json_build_object('id', "user_song"."id",'user_id', "user_song"."user_id"))) "_json" from "user with space" left join "user_song" on "user_song"."id" = "user with space"."song id"`)
}

func TestClone(t *testing.T) {
	q := qb()
	qClone := q.Clone()

	qClone.Select(User{}, "", "").LeftJoin("Songs", "", "")

	// q does not have a select statement
	_, _, err := q.ToSql()
	assert.Error(t, err)
}

func TestIncrementsArguments(t *testing.T) {
	sql, _, _ := qb().
		Select(User{}, "", "$1", 11).
		LeftJoin("Songs", "", "$1 \"$3\" $2 ' '' $2'", 22, 33).
		LeftJoin("Songs.Track", "", "$1 $2 ' $3 ' ($3)", 1, 2).
		ToSql()

	assert.Equal(t, sql, `select json_agg(json_build_object('id', "user"."id",'songs', "songs_json")) "_json" from ($1) "user" left join (select "user_song"."user_id", json_agg(json_build_object('id', "user_song"."id",'user_id', "user_song"."user_id",'tracks', "tracks_json")) "songs_json" from ($2 "$3" $3 ' '' $2') "user_song" left join (select "song_track"."song_id", json_agg(json_build_object('id', "song_track"."id",'song_id', "song_track"."song_id")) "tracks_json" from ($4 $5 ' $3 ' ($6)) "song_track"  group by "song_track"."song_id") "song_track" on "song_track"."song_id" = "user_song"."id" group by "user_song"."user_id") "user_song" on "user_song"."user_id" = "user"."id"`)
}

func TestMustSql(t *testing.T) {
	assert.Panics(t, func() {
		qb().MustSql()
	})
}

func TestJsonAggParams(t *testing.T) {
	sql, _, _ := qb().
		Select(User{}, "order by id", "").
		ToSql()

	assert.Equal(t, `select json_agg(json_build_object('id', "user"."id") order by id) "_json" from "user"`, trim(sql))
}

func TestPassPointerTable(t *testing.T) {
	sql, _ := qb().
		Select(&User{}, "", "").
		LeftJoin("Songs", "", "").
		MustSql()

	assert.NotEmpty(t, sql)
}
