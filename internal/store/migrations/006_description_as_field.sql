-- An event's description is no longer its own thing (2026-11-22, the user: "let description just be a field. Remove that
-- from header."): a plan whose events have descriptions gets a details row «Description» at the top (or «Event
-- description» when the plan already has a row of that name) holding them, on one line each, and the column goes.
UPDATE plan_field SET position = position + 1 WHERE plan_id IN (SELECT plan_id FROM event WHERE description != '');
INSERT INTO plan_field (org_id, plan_id, name, position, created_at)
SELECT DISTINCT e.org_id, e.plan_id,
       CASE WHEN EXISTS (SELECT 1 FROM plan_field f WHERE f.plan_id = e.plan_id AND f.name = 'Description')
            THEN 'Event description' ELSE 'Description' END,
       0, strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
FROM event e WHERE e.description != '';
INSERT INTO event_field (org_id, plan_id, event_id, field_id, value)
SELECT e.org_id, e.plan_id, e.id, f.id, trim(replace(replace(replace(e.description, char(13), ''), char(10), ' '), char(9), ' '))
FROM event e JOIN plan_field f ON f.plan_id = e.plan_id AND f.position = 0
WHERE trim(replace(replace(replace(e.description, char(13), ''), char(10), ' '), char(9), ' ')) != '';
ALTER TABLE event DROP COLUMN description;
