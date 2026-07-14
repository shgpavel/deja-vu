#!/bin/sh
set -eu

db=${1:-$(dirname "$0")/opencode.db}
rm -f "$db"
sqlite3 "$db" <<'SQL'
create table session (id text primary key, directory text, time_created text, time_updated text);
create table message (id text primary key, session_id text, data text, time_created text);
create table part (id text primary key, message_id text, data text);
insert into session values ('opencode-abc', '/tmp/project', '2026-01-02T03:04:05Z', '2026-01-02T03:05:05Z');
insert into message values ('msg-1', 'opencode-abc', '{"role":"user","time":{"created":"2026-01-02T03:04:05Z"}}', '2026-01-02T03:04:05Z');
insert into part values ('part-1', 'msg-1', '{"type":"text","text":"opencode synthetic memory"}');
insert into message values ('msg-2', 'opencode-abc', '{"role":"assistant","time":{"created":"2026-01-02T03:05:05Z"}}', '2026-01-02T03:05:05Z');
insert into part values ('part-2', 'msg-2', '{"type":"text","text":"second opencode row"}');
SQL
