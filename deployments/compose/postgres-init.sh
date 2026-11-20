#!/bin/sh
# Runs once on first container start, before migrations. Creates the runtime
# login roles; migration 00001 grants them privileges (and only creates
# NOLOGIN placeholders when these do not exist).
set -eu
psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" \
  -v app_pw="$HUGI_APP_PASSWORD" -v sys_pw="$HUGI_SYSTEM_PASSWORD" <<'SQL'
CREATE ROLE hugi_app LOGIN PASSWORD :'app_pw';
CREATE ROLE hugi_system LOGIN PASSWORD :'sys_pw';
SQL
