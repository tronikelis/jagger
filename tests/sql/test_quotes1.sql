select
  json_agg(
    case
      when "user with space."."id with space" is null then null
      else json_strip_nulls(
        json_build_object(
          'id with space',
          "user with space."."id with space",
          'song with space',
          case
            when "user with space.song with space"."id" is null then null
            else json_strip_nulls(
              json_build_object(
                'id',
                "user with space.song with space"."id",
                'user_id',
                "user with space.song with space"."user_id"
              )
            )
          end
        )
      )
    end
  ) "user with space._json"
from
  "user with space" as "user with space."
  left join "user_song" as "user with space.song with space" on "user with space.song with space"."id" = "user with space."."song id"
