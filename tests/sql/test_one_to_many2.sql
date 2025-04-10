select
  json_agg(
    case
      when "user."."id" is null then null
      else json_strip_nulls(
        json_build_object('id', "user."."id", 'songs', "user.songs_json")
      )
    end
    order by
      "user."."jagger_rn"
  ) "user._json"
from
  lateral (
    select
      *,
      row_number() over () as jagger_rn
    from
      "user" as "user."
  ) "user."
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
        order by
          "user.songs"."jagger_rn"
      ) "user.songs_json"
    from
      lateral (
        select
          *,
          row_number() over () as jagger_rn
        from
          songs
        where
          "user.songs"."user_id" = "user."."id"
      ) "user.songs"
    where
      "user.songs"."user_id" = "user."."id"
    group by
      "user.songs"."user_id"
  ) "user.songs" on "user.songs"."user_id" = "user."."id"
