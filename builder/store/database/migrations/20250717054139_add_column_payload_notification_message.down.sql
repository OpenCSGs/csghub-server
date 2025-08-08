SET statement_timeout = 0;

--bun:split

DROP VIEW IF EXISTS notification_user_message_views;

CREATE VIEW notification_user_message_views AS
SELECT nmu.id, nmu.msg_uuid, nm.group_id, nm.notification_type, nm.sender_uuid,
    nm.summary, nm.title, nm.content, nm.action_url, nm.priority,
    nmu.user_uuid, nmu.read_at, nmu.is_notified, nmu.expire_at, nm.created_at, nm.updated_at
FROM notification_user_messages nmu
LEFT JOIN notification_messages nm ON nmu.msg_uuid = nm.msg_uuid;

--bun:split

ALTER TABLE notification_messages DROP COLUMN IF EXISTS payload;

--bun:split

ALTER TABLE notification_messages DROP COLUMN IF EXISTS template;
