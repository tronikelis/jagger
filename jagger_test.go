package jagger_test

import (
	"errors"
	"fmt"
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

	snapshotQbAsync(t, &wg, qb().Select(User{}, nil), file+"1.sql")
	snapshotQbAsync(t, &wg, qb().Select(User{}, func(cond string) (string, []any, error) { return "select * from users", nil, nil }), file+"2.sql")
}

func TestOneToMany(t *testing.T) {
	t.Parallel()

	file := TEST_SQL_BASE + "/test_one_to_many"
	wg := sync.WaitGroup{}
	defer wg.Wait()

	snapshotQbAsync(t, &wg, qb().Select(User{}, nil).LeftJoin("Songs", nil), file+"1.sql")
	snapshotQbAsync(
		t,
		&wg,
		qb().
			Select(User{}, nil).
			LeftJoin("Songs", func(cond string) (string, []any, error) {
				return fmt.Sprintf("select *, row_number() over () as jagger_rn from songs where %s", cond), nil, nil
			}),
		file+"2.sql",
	)
}

func TestManyToOne(t *testing.T) {
	t.Parallel()

	file := TEST_SQL_BASE + "/test_many_to_one"
	wg := sync.WaitGroup{}
	defer wg.Wait()

	snapshotQbAsync(t, &wg, qb().Select(UserSong{}, nil).LeftJoin("User", nil), file+"1.sql")
}

func TestManyToOneSubQuery(t *testing.T) {
	t.Parallel()

	file := TEST_SQL_BASE + "/test_many_to_one_subquery"
	wg := sync.WaitGroup{}
	defer wg.Wait()

	snapshotQbAsync(t, &wg, qb().
		Select(UserSong{}, nil).
		LeftJoin(
			"User",
			func(cond string) (string, []any, error) {
				return fmt.Sprintf("select * from user where %s", cond), nil, nil
			}),
		file+"1.sql",
	)
}

func TestMultipleRelations(t *testing.T) {
	t.Parallel()

	file := TEST_SQL_BASE + "/test_multiple_relations"
	wg := sync.WaitGroup{}
	defer wg.Wait()

	snapshotQbAsync(t, &wg, qb().Select(User{}, nil).LeftJoin("Songs.User", nil).LeftJoin("Songs.Tracks", nil), file+"1.sql")
}

func TestBoth(t *testing.T) {
	t.Parallel()

	file := TEST_SQL_BASE + "/test_both"
	wg := sync.WaitGroup{}
	defer wg.Wait()

	snapshotQbAsync(t, &wg, qb().
		Select(User{}, nil).
		LeftJoin("Songs", nil).
		LeftJoin("Songs.Tracks", nil), file+"1.sql")
}

func TestJoinsMustBeValid(t *testing.T) {
	t.Parallel()

	_, _, err := qb().Select(User{}, nil).LeftJoin("foo", nil).ToSql()
	assert.Error(t, err)
}

func TestCorrectArgOrder(t *testing.T) {
	t.Parallel()

	_, args, err := qb().
		Select(User{}, func(cond string) (string, []any, error) { return "", []any{1, 2}, nil }).
		LeftJoin("Songs.Tracks", func(cond string) (string, []any, error) {
			return fmt.Sprintf("select * from tracks where %s", cond), []any{3, 4}, nil
		}).
		LeftJoin("Songs", func(cond string) (string, []any, error) {
			return fmt.Sprintf("select * from songs where %s", cond), []any{5, 6}, nil
		}).
		ToSql()
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

	snapshotQbAsync(t, &wg, qb().Select(UserWithSpace{}, nil).LeftJoin("Song", nil), file+"1.sql")
}

func TestClone(t *testing.T) {
	t.Parallel()

	q := qb()
	qClone := q.Clone()

	qClone.Select(User{}, nil).LeftJoin("Songs", nil)

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
		Select(User{}, func(cond string) (string, []any, error) { return "$1", []any{11}, nil }).
		LeftJoin("Songs", func(cond string) (string, []any, error) { return "$1 \"$3\" $2 ' '' $2'", []any{22, 33}, nil }).
		LeftJoin("Songs.Tracks", func(cond string) (string, []any, error) { return "$1 $2 ' $3 ' ($3)", []any{1, 2}, nil }), file+"1.sql")
}

func TestMustSql(t *testing.T) {
	t.Parallel()

	assert.Panics(t, func() {
		qb().MustSql()
	})
}

func TestPassPointerTable(t *testing.T) {
	t.Parallel()

	file := TEST_SQL_BASE + "/test_pass_pointer_table"
	wg := sync.WaitGroup{}
	defer wg.Wait()

	snapshotQbAsync(t, &wg, qb().
		Select(&User{}, nil).
		LeftJoin("Songs", nil), file+"1.sql")
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
		Select(EmbeddedUser{}, func(cond string) (string, []any, error) {
			return "select *, foo, bar from user", nil, nil
		}).
		LeftJoin("Songs", nil), file+"1.sql")
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
		Select(EmptyPk{}, nil), file+"1.sql")
}
