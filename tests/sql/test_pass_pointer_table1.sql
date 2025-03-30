select
  json_agg(
    case
      when "user."."id" is null then null
      else json_strip_nulls(
        json_build_object('id', "user."."id", 'songs', "user.songs_json")
      )
    end
  ) "user._json"
from
  "user" as "user."
  left join lateral (
    select
      "user.songs"."user_id",
      json_agg(
        case
          when "user.songs"."id" is null then null
          else json_strip_nulls(
            json_build_object(
              'id',
              "user.songs"."id",
              'user_id',
              "user.songs"."user_id"
            )
          )
        end
      ) "user.songs_json"
    from
      "user_song" as "user.songs"
    where
      "user.songs"."user_id" = "user."."id"
    group by
      "user.songs"."user_id"
  ) "user.songs" on "user.songs"."user_id" = "user."."id"
