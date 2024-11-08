package kueri_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tronikelis/kueri"
)

type SongTack struct {
	kueri.BaseTable `db:"song_track"`

	ID     int       `db:"id, pk:" json:"id"`
	SongId int       `db:"song_id" json:"song_id"`
	Song   *UserSong `db:", fk:song_id" json:"song"`
}

type UserSong struct {
	kueri.BaseTable `db:"user_song"`

	ID     int        `db:"id, pk:" json:"id"`
	UserId int        `db:"user_id" json:"user_id"`
	User   *User      `db:", fk:user_id" json:"user"`
	Tracks []SongTack `db:", fk:song_id" json:"tracks"`
}

type User struct {
	kueri.BaseTable `db:"user"`

	ID    int        `db:"id, pk:" json:"id"`
	Songs []UserSong `db:", fk:user_id" json:"songs"`
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

	assert.Equal(t, trim(sql), "select json_agg(json_build_object('id', user.id)) _json  from user")
	assert.Equal(t, args, []any{})
	assert.Equal(t, err, nil)

	sql, args, err = qb().
		Select(User{}, "user subquery").
		ToSql()

	assert.Equal(t, trim(sql), "select json_agg(json_build_object('id', user.id)) _json  from (user subquery) user")
	assert.Equal(t, args, []any{})
	assert.Equal(t, err, nil)
}

func TestOneToMany(t *testing.T) {
	sql, args, err := qb().
		Select(User{}, "").
		LeftJoin(UserSong{}, "").
		ToSql()

	assert.Equal(t, trim(sql), "select json_agg(json_build_object('id', user.id,'songs', songs_json)) _json  from user left join (select user_song.user_id, json_agg(json_build_object('id', user_song.id,'user_id', user_song.user_id)) songs_json  from user_song  group by user_song.user_id) user_song on user_song.user_id = user.id")
	assert.Equal(t, args, []any{})
	assert.Equal(t, err, nil)

	sql, args, err = qb().
		Select(User{}, "").
		LeftJoin(UserSong{}, "song sub").
		ToSql()

	assert.Equal(t, trim(sql), "select json_agg(json_build_object('id', user.id,'songs', songs_json)) _json  from user left join (select user_song.user_id, json_agg(json_build_object('id', user_song.id,'user_id', user_song.user_id)) songs_json  from (song sub) user_song  group by user_song.user_id) user_song on user_song.user_id = user.id")
	assert.Equal(t, args, []any{})
	assert.Equal(t, err, nil)
}

func TestManyToOne(t *testing.T) {
	sql, args, err := qb().
		Select(UserSong{}, "").
		LeftJoin(User{}, "").
		ToSql()

	assert.Equal(t, trim(sql), "select json_agg(json_build_object('id', user_song.id,'user_id', user_song.user_id,'user', json_build_object('id', user.id))) _json  from user_song left join user on user.id = user_song.user_id")
	assert.Equal(t, args, []any{})
	assert.Equal(t, err, nil)
}

func TestSkipCyclic(t *testing.T) {
	sql, args, err := qb().
		Select(User{}, "").
		LeftJoin(UserSong{}, "").
		LeftJoin(User{}, "").
		ToSql()

	assert.Equal(t, trim(sql), "select json_agg(json_build_object('id', user.id,'songs', songs_json)) _json  from user left join (select user_song.user_id, json_agg(json_build_object('id', user_song.id,'user_id', user_song.user_id)) songs_json  from user_song  group by user_song.user_id) user_song on user_song.user_id = user.id")
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

	assert.Equal(t, trim(sql), "select json_agg(json_build_object('id', user.id,'songs', songs_json)) _json  from user left join (select user_song.user_id, json_agg(json_build_object('id', user_song.id,'user_id', user_song.user_id,'tracks', tracks_json)) songs_json  from user_song left join (select song_track.song_id, json_agg(json_build_object('id', song_track.id,'song_id', song_track.song_id)) tracks_json  from song_track  group by song_track.song_id) song_track on song_track.song_id = user_song.id group by user_song.user_id) user_song on user_song.user_id = user.id")
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
