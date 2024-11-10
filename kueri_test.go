package kueri_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tronikelis/kueri"
)

type SongTack struct {
	kueri.BaseTable `kueri:"song_track"`

	ID     int       `kueri:"id, pk:" json:"id"`
	SongId int       `kueri:"song_id" json:"song_id"`
	Song   *UserSong `kueri:", fk:song_id" json:"song"`
}

type UserSong struct {
	kueri.BaseTable `kueri:"user_song"`

	ID     int        `kueri:"id, pk:" json:"id"`
	UserId int        `kueri:"user_id" json:"user_id"`
	User   *User      `kueri:", fk:user_id" json:"user"`
	Tracks []SongTack `kueri:", fk:song_id" json:"tracks"`
}

type User struct {
	kueri.BaseTable `kueri:"user"`

	ID    int        `kueri:"id, pk:" json:"id"`
	Songs []UserSong `kueri:", fk:user_id" json:"songs"`
}

func qb() *kueri.QueryBuilder {
	return kueri.NewQueryBuilder()
}

func trim(in string) string {
	return strings.TrimSpace(in)
}

func TestSimpleQuery(t *testing.T) {
	sql, args, err := qb().
		Select(User{}, "").
		ToSql()

	assert.Equal(t, trim(sql), `select json_agg(json_build_object('id', "user"."id")) "_json"  from "user"`)
	assert.Equal(t, args, []any{})
	assert.Equal(t, err, nil)

	sql, args, err = qb().
		Select(User{}, "user subquery").
		ToSql()

	assert.Equal(t, trim(sql), `select json_agg(json_build_object('id', "user"."id")) "_json"  from (user subquery) "user"`)
	assert.Equal(t, args, []any{})
	assert.Equal(t, err, nil)
}

func TestOneToMany(t *testing.T) {
	sql, args, err := qb().
		Select(User{}, "").
		LeftJoin(UserSong{}, "").
		ToSql()

	assert.Equal(t, trim(sql), `select json_agg(json_build_object('id', "user"."id",'songs', "songs_json")) "_json"  from "user" left join (select "user_song"."user_id", json_agg(json_build_object('id', "user_song"."id",'user_id', "user_song"."user_id")) "songs_json"  from "user_song"  group by "user_song"."user_id") "user_song" on "user_song"."user_id" = "user"."id"`)
	assert.Equal(t, args, []any{})
	assert.Equal(t, err, nil)

	sql, args, err = qb().
		Select(User{}, "").
		LeftJoin(UserSong{}, "song sub").
		ToSql()

	assert.Equal(t, trim(sql), `select json_agg(json_build_object('id', "user"."id",'songs', "songs_json")) "_json"  from "user" left join (select "user_song"."user_id", json_agg(json_build_object('id', "user_song"."id",'user_id', "user_song"."user_id")) "songs_json"  from (song sub) "user_song"  group by "user_song"."user_id") "user_song" on "user_song"."user_id" = "user"."id"`)
	assert.Equal(t, args, []any{})
	assert.Equal(t, err, nil)
}

func TestManyToOne(t *testing.T) {
	sql, args, err := qb().
		Select(UserSong{}, "").
		LeftJoin(User{}, "").
		ToSql()

	assert.Equal(t, trim(sql), `select json_agg(json_build_object('id', "user_song"."id",'user_id', "user_song"."user_id",'user', json_build_object('id', "user"."id"))) "_json"  from "user_song" left join "user" on "user"."id" = "user_song"."user_id"`)
	assert.Equal(t, args, []any{})
	assert.Equal(t, err, nil)
}

func TestSkipCyclic(t *testing.T) {
	sql, args, err := qb().
		Select(User{}, "").
		LeftJoin(UserSong{}, "").
		LeftJoin(User{}, "").
		ToSql()

	assert.Equal(t, trim(sql), `select json_agg(json_build_object('id', "user"."id",'songs', "songs_json")) "_json"  from "user" left join (select "user_song"."user_id", json_agg(json_build_object('id', "user_song"."id",'user_id', "user_song"."user_id")) "songs_json"  from "user_song"  group by "user_song"."user_id") "user_song" on "user_song"."user_id" = "user"."id"`)
	assert.Equal(t, args, []any{})
	assert.Equal(t, err, nil)
}

func TestOrderDoesNotMatter(t *testing.T) {
	sql, args, err := qb().
		LeftJoin(SongTack{}, "").
		Select(User{}, "").
		LeftJoin(UserSong{}, "").
		ToSql()

	sql2, args2, err2 := qb().
		Select(User{}, "").
		LeftJoin(SongTack{}, "").
		LeftJoin(UserSong{}, "").
		ToSql()

	assert.Equal(t, sql, sql2)
	assert.Equal(t, args, args2)
	assert.Equal(t, err, err2)
}

func TestBoth(t *testing.T) {
	sql, args, err := qb().
		Select(User{}, "").
		LeftJoin(UserSong{}, "").
		LeftJoin(SongTack{}, "").
		ToSql()

	assert.Equal(t, trim(sql), `select json_agg(json_build_object('id', "user"."id",'songs', "songs_json")) "_json"  from "user" left join (select "user_song"."user_id", json_agg(json_build_object('id', "user_song"."id",'user_id', "user_song"."user_id",'tracks', "tracks_json")) "songs_json"  from "user_song" left join (select "song_track"."song_id", json_agg(json_build_object('id', "song_track"."id",'song_id', "song_track"."song_id")) "tracks_json"  from "song_track"  group by "song_track"."song_id") "song_track" on "song_track"."song_id" = "user_song"."id" group by "user_song"."user_id") "user_song" on "user_song"."user_id" = "user"."id"`)
	assert.Equal(t, args, []any{})
	assert.Equal(t, err, nil)
}

func TestPanicsOnInvalidSelect(t *testing.T) {
	assert.Panics(t, func() {
		_ = qb().Select(struct {
			ID int
		}{}, "")
	})

	assert.Panics(t, func() {
		_ = qb().Select("hello", "")
	})
}

func TestJoinsMustBeValid(t *testing.T) {
	_, _, err := qb().Select(User{}, "").LeftJoin(SongTack{}, "").ToSql()
	assert.Error(t, err)
}

func TestCorrectArgOrder(t *testing.T) {
	_, args, _ := qb().Select(User{}, "", 1, 2).LeftJoin(SongTack{}, "", 3, 4).LeftJoin(UserSong{}, "", 5, 6).ToSql()
	// user -> user song -> song track
	assert.Equal(t, args, []any{1, 2, 5, 6, 3, 4})
}

type UserWithSpace struct {
	kueri.BaseTable `kueri:"user with space"`

	ID   int       `kueri:"id with space" json:"id with space"`
	Song *UserSong `kueri:", fk:song id" json:"song with space"`
}

func TestQuotes(t *testing.T) {
	sql, _, _ := qb().Select(UserWithSpace{}, "").LeftJoin(UserSong{}, "").ToSql()

	assert.Equal(t, trim(sql), `select json_agg(json_build_object('id with space', "user with space"."id with space",'song with space', json_build_object('id', "user_song"."id",'user_id', "user_song"."user_id"))) "_json"  from "user with space" left join "user_song" on "user_song"."id" = "user with space"."song id"`)
}

func TestClone(t *testing.T) {
	q := qb()
	qClone := q.Clone()

	qClone.Select(User{}, "").LeftJoin(UserSong{}, "")

	// q does not have a select statement
	_, _, err := q.ToSql()
	assert.Error(t, err)
}

func TestIncrementsArguments(t *testing.T) {
	sql, _, _ := qb().
		Select(User{}, "$1", 11).
		LeftJoin(UserSong{}, "$1 \"$3\" $2 ' '' $2'", 22, 33).
		LeftJoin(SongTack{}, "$1 $2 ' $3 ' $$2", 1, 2).
		ToSql()

	assert.Equal(t, sql, `select json_agg(json_build_object('id', "user"."id",'songs', "songs_json")) "_json"  from ($1) "user" left join (select "user_song"."user_id", json_agg(json_build_object('id', "user_song"."id",'user_id', "user_song"."user_id",'tracks', "tracks_json")) "songs_json"  from ($2 "$3" $3 ' '' $2') "user_song" left join (select "song_track"."song_id", json_agg(json_build_object('id', "song_track"."id",'song_id', "song_track"."song_id")) "tracks_json"  from ($4 $5 ' $3 ' $$2) "song_track"  group by "song_track"."song_id") "song_track" on "song_track"."song_id" = "user_song"."id" group by "user_song"."user_id") "user_song" on "user_song"."user_id" = "user"."id"`)
}
