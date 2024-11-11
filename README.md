# jagger

A different way to query data from RDBMS

```go
func main() {
    sql, args, err := jagger.NewQueryBuilder().
        Select(User{}, "").
        LeftJoin(UserSong{}, "select * from user_songs where id = ?", 2).
        ToSql()
}
```

This generates something similar to this

```sql
SELECT
	JSON_AGG(
		JSON_BUILD_OBJECT('id', USER.ID, 'songs', SONGS_JSON)
	) _JSON
FROM
	USER
	LEFT JOIN (
		SELECT
			USER_SONG.USER_ID,
			JSON_AGG(
				JSON_BUILD_OBJECT('id', USER_SONG.ID, 'user_id', USER_SONG.USER_ID)
			) SONGS_JSON
		FROM
			(select * from user_song where id = ?) USER_SONG
		GROUP BY
			USER_SONG.USER_ID
	) USER_SONG ON USER_SONG.USER_ID = USER.ID
```


<!--toc:start-->
- [jagger](#jagger)
  - [Usage](#usage)
    - [Struct tags](#struct-tags)
    - [Querying](#querying)
<!--toc:end-->


## Usage

The package officially supports postgres, because that is what I personally use,
if you want to use this for other databases such as mysql a pr with extra config would be
appreciated


### Struct tags

The query builder supports a struct if it has `jagger.BaseTable` embedded like so

```go
type User struct {
	jagger.BaseTable `jagger:"user"`
}
```

jagger uses `jagger` as its struct tag, with the structure like this:
`jagger:"<name>, [k:v, k:v, k:v]`

`<name>` is an optional name for the table and columns, you don't need to
specify it on relation fields, e.g.

```go
type Song struct {
	User *User `jagger:", fk:user_id"`
}
```

`fk:<col>` is to specify how to connect this relation, this **always** has to be
the column on which the foreign key resides


```go
type User struct {
	Songs []Song `jagger:", fk:user_id"`
}
type Song struct {
	UserId int `jagger:"user_id"`
	User *User `jagger:", fk:user_id"`
}
```

notice how the `fk` is the same on both relations `User/Song`

### Querying

This package is responsible only for the json aggregation,
this is why you should probably use another query builder tool with this tool

The methods accept an optional sub query as the second parameter to get the table rows


```go
sql, args, err := jagger.NewQueryBuilder()
	// [panics if struct is not a table] mandatory, will start to aggregate from this struct
	Select(User{}, "select * from users order by id desc", arg1, arg2).
	// [panics if struct is not a table] optional, all joins are available LeftJoin / RightJoin ... etc
	LeftJoin(Song{}, "").
	ToSql()
```

The query builder is mutable, so select and join methods mutate, if you want to clone
the current state, use `.Clone()` method
