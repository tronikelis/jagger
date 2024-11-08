# kueri

A different way to query data from RDBMS

```go
func main() {
    sql, args, err := kueri.NewQueryBuilder().
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
