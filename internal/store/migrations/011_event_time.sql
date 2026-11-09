-- A time of day and a description on an event. start_time and end_time are 'HH:MM' on a 24-hour clock (the local time of the
-- event, no time zone) or '' when not set; an end time needs a start time and comes after it on the same day. Both are
-- validated in the code, and the checks here are a second line of defence. description is free text (several lines allowed).
ALTER TABLE event ADD COLUMN start_time TEXT NOT NULL DEFAULT '' CHECK (start_time = '' OR start_time GLOB '[0-2][0-9]:[0-5][0-9]');
ALTER TABLE event ADD COLUMN end_time TEXT NOT NULL DEFAULT '' CHECK (end_time = '' OR (end_time GLOB '[0-2][0-9]:[0-5][0-9]' AND start_time <> '' AND end_time > start_time));
ALTER TABLE event ADD COLUMN description TEXT NOT NULL DEFAULT '';
