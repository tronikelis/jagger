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
  ($1) "user."
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
              'tracks',
              "user_song.tracks_json"
            )
          )
        end
      ) "user.songs_json"
    from
      ($2 "$3" $3 ' '' $2') "user.songs"
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
          ) "user_song.tracks_json"
        from
          ($4 $5 ' $3 ' ($6)) "user_song.tracks"
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
