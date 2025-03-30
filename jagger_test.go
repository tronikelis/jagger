package jagger_test

import (
	"errors"
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

	oldSqlFile, err := os.OpenFile(file, os.O_CREATE|os.O_RDONLY, 0o644)
	if err != nil {
		panic(err)
	}
	defer oldSqlFile.Close()

	oldSqlBytes, err := io.ReadAll(oldSqlFile)
	if err != nil {
		panic(err)
	}

	assert.Equal(t, string(oldSqlBytes), newSql)
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
		if err != nil && !errors.Is(err, os.ErrClosed) {
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
	t.Parallel()

	file := TEST_SQL_BASE + "/test_simple_query"
	wg := sync.WaitGroup{}
	defer wg.Wait()

	snapshotQbAsync(t, &wg, qb().Select(User{}, "", ""), file+"1.sql")
	snapshotQbAsync(t, &wg, qb().Select(User{}, "", "select * from users"), file+"2.sql")
}

func TestOneToMany(t *testing.T) {
	t.Parallel()

	file := TEST_SQL_BASE + "/test_one_to_many"
	wg := sync.WaitGroup{}
	defer wg.Wait()

	snapshotQbAsync(t, &wg, qb().Select(User{}, "", "").LeftJoin("Songs", "", ""), file+"1.sql")
	snapshotQbAsync(t, &wg, qb().Select(User{}, "", "").LeftJoin("Songs", "", "select * from user_song"), file+"2.sql")
}

func TestManyToOne(t *testing.T) {
	t.Parallel()

	file := TEST_SQL_BASE + "/test_many_to_one"
	wg := sync.WaitGroup{}
	defer wg.Wait()

	snapshotQbAsync(t, &wg, qb().Select(UserSong{}, "", "").LeftJoin("User", "", ""), file+"1.sql")
}

func TestManyToOneSubQuery(t *testing.T) {
	t.Parallel()

	file := TEST_SQL_BASE + "/test_many_to_one_subquery"
	wg := sync.WaitGroup{}
	defer wg.Wait()

	snapshotQbAsync(t, &wg, qb().Select(UserSong{}, "", "").LeftJoin("User", "", "select * from users"), file+"1.sql")
}

func TestMultipleRelations(t *testing.T) {
	t.Parallel()

	file := TEST_SQL_BASE + "/test_multiple_relations"
	wg := sync.WaitGroup{}
	defer wg.Wait()

	snapshotQbAsync(t, &wg, qb().Select(User{}, "", "").LeftJoin("Songs.User", "", "").LeftJoin("Songs.Tracks", "", ""), file+"1.sql")
}

func TestBoth(t *testing.T) {
	t.Parallel()

	file := TEST_SQL_BASE + "/test_both"
	wg := sync.WaitGroup{}
	defer wg.Wait()

	snapshotQbAsync(t, &wg, qb().
		Select(User{}, "", "").
		LeftJoin("Songs", "", "").
		LeftJoin("Songs.Tracks", "", ""), file+"1.sql")
}

func TestJoinsMustBeValid(t *testing.T) {
	t.Parallel()

	_, _, err := qb().Select(User{}, "", "").LeftJoin("foo", "", "").ToSql()
	assert.Error(t, err)
}

func TestCorrectArgOrder(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

	file := TEST_SQL_BASE + "/test_quotes"
	wg := sync.WaitGroup{}
	defer wg.Wait()

	snapshotQbAsync(t, &wg, qb().Select(UserWithSpace{}, "", "").LeftJoin("Song", "", ""), file+"1.sql")
}

func TestClone(t *testing.T) {
	t.Parallel()

	q := qb()
	qClone := q.Clone()

	qClone.Select(User{}, "", "").LeftJoin("Songs", "", "")

	// q does not have a select statement
	_, _, err := q.ToSql()
	assert.Error(t, err)
}

func TestIncrementsArguments(t *testing.T) {
	t.Parallel()

	file := TEST_SQL_BASE + "/test_increments_arguments"
	wg := sync.WaitGroup{}
	defer wg.Wait()

	snapshotQbAsync(t, &wg, qb().
		Select(User{}, "", "$1", 11).
		LeftJoin("Songs", "", "$1 \"$3\" $2 ' '' $2'", 22, 33).
		LeftJoin("Songs.Tracks", "", "$1 $2 ' $3 ' ($3)", 1, 2), file+"1.sql")
}

func TestMustSql(t *testing.T) {
	t.Parallel()

	assert.Panics(t, func() {
		qb().MustSql()
	})
}

func TestJsonAggParams(t *testing.T) {
	t.Parallel()

	file := TEST_SQL_BASE + "/test_json_agg_params"
	wg := sync.WaitGroup{}
	defer wg.Wait()

	snapshotQbAsync(t, &wg, qb().Select(User{}, "order by id", ""), file+"1.sql")
}

func TestPassPointerTable(t *testing.T) {
	t.Parallel()

	file := TEST_SQL_BASE + "/test_pass_pointer_table"
	wg := sync.WaitGroup{}
	defer wg.Wait()

	snapshotQbAsync(t, &wg, qb().
		Select(&User{}, "", "").
		LeftJoin("Songs", "", ""), file+"1.sql")
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
	t.Parallel()

	file := TEST_SQL_BASE + "/test_embedded"
	wg := sync.WaitGroup{}
	defer wg.Wait()

	snapshotQbAsync(t, &wg, qb().
		Select(EmbeddedUser{}, "", "select *, foo, bar from user").
		LeftJoin("Songs", "", ""), file+"1.sql")
}

type EmptyPk struct {
	jagger.BaseTable `jagger:"ttt"`

	Id int `jagger:"id" json:"id"`
}

func TestEmptyPkSkipsCaseWhen(t *testing.T) {
	t.Parallel()

	file := TEST_SQL_BASE + "/test_empty_pk_skips_case_when"
	wg := sync.WaitGroup{}
	defer wg.Wait()

	snapshotQbAsync(t, &wg, qb().
		Select(EmptyPk{}, "", ""), file+"1.sql")
}
