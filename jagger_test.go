package jagger_test

import (
	"io"
	"os"
	"os/exec"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tronikelis/jagger"
)

func snapshotQbAsync(t *testing.T, wg *sync.WaitGroup, qb *jagger.QueryBuilder, file string) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		snapshotQb(t, qb, file)
	}()
}

func snapshotQb(t *testing.T, qb *jagger.QueryBuilder, file string) {
	sql, _, err := qb.ToSql()
	assert.NoError(t, err)

	newSql, err := cmd(sql, "npx", "sql-formatter", "-l", "postgresql")
	if err != nil {
		panic(err)
	}

	if os.Getenv("WRITE_SQL_SNAPSHOTS") == "true" {
		if err := os.WriteFile(file, []byte(newSql), 0o644); err != nil {
			panic(err)
		}
		return
	}

	oldSqlBytes, err := os.ReadFile(file)
	if err != nil {
		panic(err)
	}

	oldSql := string(oldSqlBytes)
	assert.Equal(t, oldSql, newSql)
}

func cmd(stdin string, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}

	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		return "", nil
	}

	if err := cmd.Start(); err != nil {
		return "", err
	}

	stdinErrChan := make(chan error)
	stdoutErrChan := make(chan error)
	stdoutChan := make(chan []byte)

	go func() {
		_, err := stdinPipe.Write([]byte(stdin))
		if err != nil {
			stdinErrChan <- err
			return
		}

		if err := stdinPipe.Close(); err != nil {
			stdinErrChan <- err
			return
		}

		stdinErrChan <- nil
	}()

	go func() {
		read, err := io.ReadAll(stdoutPipe)
		if err != nil {
			stdoutErrChan <- err
			return
		}
		stdoutErrChan <- nil
		stdoutChan <- read
	}()

	if err := cmd.Wait(); err != nil {
		return "", err
	}

	if err := <-stdinErrChan; err != nil {
		return "", err
	}
	if err := <-stdoutErrChan; err != nil {
		return "", err
	}

	return string(<-stdoutChan), nil
}

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

const TEST_SQL_BASE = "tests/sql"

func TestSimpleQuery(t *testing.T) {
	file := TEST_SQL_BASE + "/test_simple_query"
	wg := sync.WaitGroup{}
	defer wg.Wait()

	snapshotQbAsync(t, &wg, qb().Select(User{}, "", ""), file+"1.sql")
	snapshotQbAsync(t, &wg, qb().Select(User{}, "", "select * from users"), file+"2.sql")
}

func TestOneToMany(t *testing.T) {
	file := TEST_SQL_BASE + "/test_one_to_many"
	wg := sync.WaitGroup{}
	defer wg.Wait()

	snapshotQbAsync(t, &wg, qb().Select(User{}, "", "").LeftJoin("Songs", "", ""), file+"1.sql")
	snapshotQbAsync(t, &wg, qb().Select(User{}, "", "").LeftJoin("Songs", "", "select * from user_song"), file+"2.sql")
}

func TestManyToOne(t *testing.T) {
	sql, args, err := qb().
		Select(UserSong{}, "", "").
		LeftJoin("User", "", "").
		ToSql()

	assert.Equal(t, `select json_agg(case when "user_song."."id" is null then null else json_strip_nulls(json_build_object('id', "user_song."."id",'user_id', "user_song."."user_id",'user', case when "user_song.user"."id" is null then null else json_strip_nulls(json_build_object('id', "user_song.user"."id")) end)) end) "user_song._json" from "user_song" as "user_song." left join "user" as "user_song.user" on "user_song.user"."id" = "user_song."."user_id"  `, sql)
	assert.Equal(t, []any{}, args)
	assert.NoError(t, err)
}

func TestManyToOneSubQuery(t *testing.T) {
	sql, args, err := qb().
		Select(UserSong{}, "", "").
		LeftJoin("User", "", "select * from users").
		ToSql()

	assert.Equal(t, `select json_agg(case when "user_song."."id" is null then null else json_strip_nulls(json_build_object('id', "user_song."."id",'user_id', "user_song."."user_id",'user', case when "user_song.user"."id" is null then null else json_strip_nulls(json_build_object('id', "user_song.user"."id")) end)) end) "user_song._json" from "user_song" as "user_song." left join (select * from users) "user_song.user" on "user_song.user"."id" = "user_song."."user_id"  `, sql)
	assert.Equal(t, []any{}, args)
	assert.NoError(t, err)
}

func TestMultipleRelations(t *testing.T) {
	sql, args, err := qb().
		Select(User{}, "", "").
		LeftJoin("Songs.User", "", "").
		LeftJoin("Songs.Tracks", "", "").
		ToSql()

	assert.NoError(t, err)
	assert.Equal(t, []any{}, args)
	assert.Equal(t, `select json_agg(case when "user."."id" is null then null else json_strip_nulls(json_build_object('id', "user."."id",'songs', "user.songs_json")) end) "user._json" from "user" as "user." left join (select "user.songs"."user_id", json_agg(case when "user.songs"."id" is null then null else json_strip_nulls(json_build_object('id', "user.songs"."id",'user_id', "user.songs"."user_id",'user', case when "user_song.user"."id" is null then null else json_strip_nulls(json_build_object('id', "user_song.user"."id")) end,'tracks', "user_song.tracks_json")) end) "user.songs_json" from "user_song" as "user.songs" left join "user" as "user_song.user" on "user_song.user"."id" = "user.songs"."user_id"  left join (select "user_song.tracks"."song_id", json_agg(case when "user_song.tracks"."id" is null then null else json_strip_nulls(json_build_object('id', "user_song.tracks"."id",'song_id', "user_song.tracks"."song_id")) end) "user_song.tracks_json" from "song_track" as "user_song.tracks"  group by "user_song.tracks"."song_id") "user_song.tracks" on "user_song.tracks"."song_id" = "user.songs"."id" group by "user.songs"."user_id") "user.songs" on "user.songs"."user_id" = "user."."id"`, sql)
}

func TestBoth(t *testing.T) {
	sql, args, err := qb().
		Select(User{}, "", "").
		LeftJoin("Songs", "", "").
		LeftJoin("Songs.Tracks", "", "").
		ToSql()

	assert.Equal(t, `select json_agg(case when "user."."id" is null then null else json_strip_nulls(json_build_object('id', "user."."id",'songs', "user.songs_json")) end) "user._json" from "user" as "user." left join (select "user.songs"."user_id", json_agg(case when "user.songs"."id" is null then null else json_strip_nulls(json_build_object('id', "user.songs"."id",'user_id', "user.songs"."user_id",'tracks', "user_song.tracks_json")) end) "user.songs_json" from "user_song" as "user.songs" left join (select "user_song.tracks"."song_id", json_agg(case when "user_song.tracks"."id" is null then null else json_strip_nulls(json_build_object('id', "user_song.tracks"."id",'song_id', "user_song.tracks"."song_id")) end) "user_song.tracks_json" from "song_track" as "user_song.tracks"  group by "user_song.tracks"."song_id") "user_song.tracks" on "user_song.tracks"."song_id" = "user.songs"."id" group by "user.songs"."user_id") "user.songs" on "user.songs"."user_id" = "user."."id"`, sql)
	assert.Equal(t, []any{}, args)
	assert.NoError(t, err)
}

func TestJoinsMustBeValid(t *testing.T) {
	_, _, err := qb().Select(User{}, "", "").LeftJoin("foo", "", "").ToSql()
	assert.Error(t, err)
}

func TestCorrectArgOrder(t *testing.T) {
	_, args, err := qb().Select(User{}, "", "", 1, 2).LeftJoin("Songs.Tracks", "", "select * from tracks", 3, 4).LeftJoin("Songs", "", "select * from songs", 5, 6).ToSql()
	assert.NoError(t, err)
	// user -> user song -> song track
	assert.Equal(t, []any{1, 2, 5, 6, 3, 4}, args)
}

type UserWithSpace struct {
	jagger.BaseTable `jagger:"user with space"`

	ID   int       `jagger:"id with space,pk:" json:"id with space"`
	Song *UserSong `jagger:", fk:song id" json:"song with space"`
}

func TestQuotes(t *testing.T) {
	sql, _, _ := qb().Select(UserWithSpace{}, "", "").LeftJoin("Song", "", "").ToSql()

	assert.Equal(t, `select json_agg(case when "user with space."."id with space" is null then null else json_strip_nulls(json_build_object('id with space', "user with space."."id with space",'song with space', case when "user with space.song with space"."id" is null then null else json_strip_nulls(json_build_object('id', "user with space.song with space"."id",'user_id', "user with space.song with space"."user_id")) end)) end) "user with space._json" from "user with space" as "user with space." left join "user_song" as "user with space.song with space" on "user with space.song with space"."id" = "user with space."."song id"  `, sql)
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
	sql, _, err := qb().
		Select(User{}, "", "$1", 11).
		LeftJoin("Songs", "", "$1 \"$3\" $2 ' '' $2'", 22, 33).
		LeftJoin("Songs.Tracks", "", "$1 $2 ' $3 ' ($3)", 1, 2).
		ToSql()

	assert.NoError(t, err)
	assert.Equal(t, `select json_agg(case when "user."."id" is null then null else json_strip_nulls(json_build_object('id', "user."."id",'songs', "user.songs_json")) end) "user._json" from ($1) "user." left join (select "user.songs"."user_id", json_agg(case when "user.songs"."id" is null then null else json_strip_nulls(json_build_object('id', "user.songs"."id",'user_id', "user.songs"."user_id",'tracks', "user_song.tracks_json")) end) "user.songs_json" from ($2 "$3" $3 ' '' $2') "user.songs" left join (select "user_song.tracks"."song_id", json_agg(case when "user_song.tracks"."id" is null then null else json_strip_nulls(json_build_object('id', "user_song.tracks"."id",'song_id', "user_song.tracks"."song_id")) end) "user_song.tracks_json" from ($4 $5 ' $3 ' ($6)) "user_song.tracks"  group by "user_song.tracks"."song_id") "user_song.tracks" on "user_song.tracks"."song_id" = "user.songs"."id" group by "user.songs"."user_id") "user.songs" on "user.songs"."user_id" = "user."."id"`, sql)
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

	assert.Equal(t, `select json_agg(case when "user."."id" is null then null else json_strip_nulls(json_build_object('id', "user."."id")) end order by id) "user._json" from "user" as "user." `, sql)
}

func TestPassPointerTable(t *testing.T) {
	sql, _, err := qb().
		Select(&User{}, "", "").
		LeftJoin("Songs", "", "").
		ToSql()

	assert.NoError(t, err)
	assert.NotEmpty(t, sql)
}

type SomeFieldBar struct {
	Bar string `json:"bar" jagger:"bar"`
}

type EmbeddedUser struct {
	User         `jagger:",embed:"`
	SomeFieldBar `jagger:",embed:"`
	Foo          string `json:"foo" jagger:"foo"`
}

func TestEmbedded(t *testing.T) {
	sql, _ := qb().
		Select(EmbeddedUser{}, "", "select *, foo, bar from user").
		LeftJoin("Songs", "", "").
		MustSql()

	assert.Equal(t, `select json_agg(case when "user."."id" is null then null else json_strip_nulls(json_build_object('id', "user."."id",'bar', "user."."bar",'foo', "user."."foo",'songs', "user.songs_json")) end) "user._json" from (select *, foo, bar from user) "user." left join (select "user.songs"."user_id", json_agg(case when "user.songs"."id" is null then null else json_strip_nulls(json_build_object('id', "user.songs"."id",'user_id', "user.songs"."user_id")) end) "user.songs_json" from "user_song" as "user.songs"  group by "user.songs"."user_id") "user.songs" on "user.songs"."user_id" = "user."."id"`, sql)
}

type EmptyPk struct {
	jagger.BaseTable `jagger:"ttt"`

	Id int `jagger:"id" json:"id"`
}

func TestEmptyPkSkipsCaseWhen(t *testing.T) {
	sql, _ := qb().
		Select(EmptyPk{}, "", "").
		MustSql()

	assert.Equal(t, `select json_agg(json_strip_nulls(json_build_object('id', "ttt."."id"))) "ttt._json" from "ttt" as "ttt." `, sql)
}
