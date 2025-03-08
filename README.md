# jagger

What if you could `json.Unmarshal` your rdbms relations? (only pg supported for now)

```go
type User struct {
  jagger.BaseTable `jagger:"users"`
  Songs []Song `json:"songs" jagger:",fk:user_id"`
}

type Song struct {
  jagger.BaseTable `jagger:"songs"`
  UserId int `json:"user_id" jagger:"user_id"`
  User *User `json:"user" jagger:",fk:user_id"`
}

func main() {
  sql, args, err := jagger.NewQueryBuilder().
    // Select initial struct, add json_agg suffix if desired, subquery which to select from (optional)
    Select(User{}, "json_agg suffix", "select * from users").
    // left join direct field
    LeftJoin("Songs", "", "").
    // nested relations also supported
    LeftJoin("Songs.User", "", "").
    ToSql()
}
```

This turns your relation into expected json format when you call `.ToSql()` (just a random example, not the direct generation from the above query)

```sql
select
  json_agg (
    case
      when "user."."id" is null then null
      else json_strip_nulls (
        json_build_object ('id', "user."."id", 'songs', "user.songs_json")
      )
    end
  ) "user._json"
from
  "user" as "user."
  left join (
    select
      "user.songs"."user_id",
      json_agg (
        case
          when "user.songs"."id" is null then null
          else json_strip_nulls (
            json_build_object (
              'id',
              "user.songs"."id",
              'user_id',
              "user.songs"."user_id",
              'user',
              case
                when "user_song.user"."id" is null then null
                else json_strip_nulls (json_build_object ('id', "user_song.user"."id"))
              end
            )
          )
        end
      ) "user.songs_json"
    from
      "user_song" as "user.songs"
      left join (
        select
          *
        from
          user_songs
        where
          id = ?
      ) "user_song.user" on "user_song.user"."id" = "user.songs"."user_id"
    group by
      "user.songs"."user_id"
  ) "user.songs" on "user.songs"."user_id" = "user."."id"
```

When you send this sql to postgres, it will return this json

```jsonc
[
  {
    // user
    "id": 1,
    // user has many songs
    "songs": [
      {
        // song has one user
        "user": {
          "id": 1,
        },
        "user_id": 1
      }
    ]
  }
]
```

Now all thats left is to unmarshal it into `User` struct

```go
var b []byte
if err := pg.Query(sql, args).Scan(&b); err != nil {
  return err
}

var u []User
if err := json.Unmarshal(b, &u); err != nil {
  return err
}
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

`pk:` is to specify that this column is the primary key, should only be set on one column per struct

```go
type User struct {
  ID int `jagger:"id,pk:"`
}
```

### Querying

This package is responsible only for the json aggregation,
this is why you should probably use another query builder tool with this tool

The methods accept an optional sub query as the second parameter to get the table rows


```go
sql, args, err := jagger.NewQueryBuilder().
  Select(User{}, "", "select * from users order by id desc", arg1, arg2).
  // join Songs AND User, which in this case is unnecessary as we already have user from select
  LeftJoin("Songs.User", "", "").
  // Join User.Songs.Tracks
  LeftJoin("Songs.Tracks", "", "").
  ToSql()
```

The query builder is mutable, so select and join methods mutate, if you want to clone
the current state, use `.Clone()` method, but beware that this will be a shallow clone
