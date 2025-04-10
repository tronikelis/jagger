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
      "user"
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
              "user.songs"."user_id",
              'user',
              case
                when "user_song.user"."id" is null then null
                else json_strip_nulls(json_build_object('id', "user_song.user"."id"))
              end,
              'tracks',
              "user_song.tracks_json"
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
          "user_song"
        where
          "user_song"."user_id" = "user."."id"
      ) "user.songs"
      left join lateral (
        select
          *,
          row_number() over () as jagger_rn
        from
          "user"
        where
          "user"."id" = "user.songs"."user_id"
      ) "user_song.user" on "user_song.user"."id" = "user.songs"."user_id"
      left join lateral (
        select
          "user_song.tracks"."song_id",
          json_agg(
            case
              when "user_song.tracks"."id" is null then null
              else json_strip_nulls(
                json_build_object(
                  'id',
                  "user_song.tracks"."id",
                  'song_id',
                  "user_song.tracks"."song_id"
                )
              )
            end
            order by
              "user_song.tracks"."jagger_rn"
          ) "user_song.tracks_json"
        from
          lateral (
            select
              *,
              row_number() over () as jagger_rn
            from
              "song_track"
            where
              "song_track"."song_id" = "user.songs"."id"
          ) "user_song.tracks"
        where
          "user_song.tracks"."song_id" = "user.songs"."id"
        group by
          "user_song.tracks"."song_id"
      ) "user_song.tracks" on "user_song.tracks"."song_id" = "user.songs"."id"
    where
      "user.songs"."user_id" = "user."."id"
    group by
      "user.songs"."user_id"
  ) "user.songs" on "user.songs"."user_id" = "user."."id"
