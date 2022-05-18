create or replace view gps
            (state_id, old_state_id, time, entity_id, latitude, longitude, gps_accuracy, altitude, course, speed,
             vertical_accuracy)
as
select state_id,
       old_state_id,
       created                                                             AS "time",
       entity_id,
       jsonb_extract_path(attributes::jsonb, 'latitude')::numeric          AS latitude,
       jsonb_extract_path(attributes::jsonb, 'longitude')::numeric         AS longitude,
       jsonb_extract_path(attributes::jsonb, 'gps_accuracy')::numeric      AS gps_accuracy,
       jsonb_extract_path(attributes::jsonb, 'altitude')::numeric          AS altitude,
       jsonb_extract_path(attributes::jsonb, 'course')::numeric            AS course,
       jsonb_extract_path(attributes::jsonb, 'speed')::numeric             AS speed,
       jsonb_extract_path(attributes::jsonb, 'vertical_accuracy')::numeric AS vertical_accuracy
from states
where jsonb_extract_path_text(attributes::jsonb, 'source_type') = 'gps'
order by state_id