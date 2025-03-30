select
  json_agg(
    case
      when "user_song."."id" is null then null
      else json_strip_nulls(
        json_build_object(
          'id',
          "user_song."."id",
          'user_id',
          "user_song."."user_id",
          'user',
          case
            when "user_song.user"."id" is null then null
            else json_strip_nulls(json_build_object('id', "user_song.user"."id"))
          end
        )
      )
    end
  ) "user_song._json"
from
  "user_song" as "user_song."
  left join (
    select
      *
    from
      users
  ) "user_song.user" on "user_song.user"."id" = "user_song."."user_id"
