#!/usr/bin/env bash
# 三数据库恢复脚本（P0-30）。用于演练与灾难恢复。
# 用法：restore.sh <sqlite|mysql|postgres> <backup_file>
# 恢复前请先停止新写入（停服或只读），并在测试库演练后再上生产。
set -euo pipefail

DBTYPE="${1:?usage: restore.sh <sqlite|mysql|postgres> <backup_file>}"
FILE="${2:?usage: restore.sh <sqlite|mysql|postgres> <backup_file>}"

case "$DBTYPE" in
  sqlite)
    DEST="${SQLITE_DSN:-/data/new-api.db}"
    if [[ "$FILE" == *.gz ]]; then
      gzip -dc "$FILE" > "${FILE%.gz}.db"
      FILE="${FILE%.gz}.db"
    fi
    cp "$FILE" "$DEST"
    echo "SQLite restored to $DEST（账务/审计表随库一并恢复）"
    ;;
  mysql)
    HOST="$(echo "$MYSQL_DSN" | sed -n 's/.*@tcp(\([^)]*\)).*/\1/p')"
    DB="$(echo "$MYSQL_DSN" | sed -n 's/.*\///p' | cut -d'?' -f1)"
    USER="$(echo "$MYSQL_DSN" | cut -d: -f1)"
    PASS="$(echo "$MYSQL_DSN" | cut -d: -f2 | sed 's/@tcp.*//')"
    gzip -dc "$FILE" | MYSQL_PWD="$PASS" mysql -h "${HOST%%:*}" -P "${HOST##*:}" -u "$USER" "$DB"
    echo "MySQL restored to $DB"
    ;;
  postgres)
    gzip -dc "$FILE" | psql "${PG_DSN:?PG_DSN required}"
    echo "PostgreSQL restored"
    ;;
  *)
    echo "unknown db type: $DBTYPE" >&2
    exit 1
    ;;
esac
