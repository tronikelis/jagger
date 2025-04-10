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
    order by
      "user_song."."jagger_rn"
  ) "user_song._json"
from
  lateral (
    select
      *,
      row_number() over () as jagger_rn
    from
      "user_song" as "user_song."
  ) "user_song."
  left join lateral (
    select
      *
    from
      user
    where
      "user_song.user"."id" = "user_song."."user_id"
  ) "user_song.user" on "user_song.user"."id" = "user_song."."user_id"
