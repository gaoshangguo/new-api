#!/usr/bin/env bash
# 三数据库通用备份脚本（P0-30）。
# 用法：
#   SQLITE_DSN=/data/new-api.db ./backup.sh sqlite /backup
#   MYSQL_DSN='root:pass@tcp(127.0.0.1:3306)/new_api?charset=utf8mb4' ./backup.sh mysql /backup
#   PG_DSN='postgres://root:pass@127.0.0.1:5432/new_api' ./backup.sh postgres /backup
#
# 产物：<target>/new-api-<dbtype>-<timestamp>.{sqlite.gz|sql.gz|dump.gz}
# 账务与审计表（business_ledgers、business_audit_events、logs 等）随全库备份
# 一并包含；如需单独保护账务，可用 --only 系列脚本（见 README）。
set -euo pipefail

DBTYPE="${1:?usage: backup.sh <sqlite|mysql|postgres> <target_dir>}"
TARGET="${2:?usage: backup.sh <sqlite|mysql|postgres> <target_dir>}"
STAMP="$(date +%Y%m%d%H%M%S)"
mkdir -p "$TARGET"

case "$DBTYPE" in
  sqlite)
    SRC="${SQLITE_DSN:-/data/new-api.db}"
    if command -v sqlite3 >/dev/null 2>&1; then
      sqlite3 "$SRC" ".backup '$TARGET/new-api-sqlite-$STAMP.db'"
    else
      cp "$SRC" "$TARGET/new-api-sqlite-$STAMP.db"
    fi
    gzip -f "$TARGET/new-api-sqlite-$STAMP.db"
    echo "SQLite backup: $TARGET/new-api-sqlite-$STAMP.db.gz"
    ;;
  mysql)
    # MYSQL_DSN 形如 user:pass@tcp(host:port)/db?charset=utf8mb4
    HOST="$(echo "$MYSQL_DSN" | sed -n 's/.*@tcp(\([^)]*\)).*/\1/p')"
    DB="$(echo "$MYSQL_DSN" | sed -n 's/.*\///p' | cut -d'?' -f1)"
    USER="$(echo "$MYSQL_DSN" | cut -d: -f1)"
    PASS="$(echo "$MYSQL_DSN" | cut -d: -f2 | sed 's/@tcp.*//')"
    MYSQL_PWD="$PASS" mysqldump -h "${HOST%%:*}" -P "${HOST##*:}" -u "$USER" --single-transaction "$DB" \
      | gzip > "$TARGET/new-api-mysql-$STAMP.sql.gz"
    echo "MySQL backup: $TARGET/new-api-mysql-$STAMP.sql.gz"
    ;;
  postgres)
    pg_dump "${PG_DSN:?PG_DSN required}" | gzip > "$TARGET/new-api-postgres-$STAMP.dump.gz"
    echo "PostgreSQL backup: $TARGET/new-api-postgres-$STAMP.dump.gz"
    ;;
  *)
    echo "unknown db type: $DBTYPE" >&2
    exit 1
    ;;
esac

# 异地/二次拷贝：设置 OFFSITE_DIR 后把备份再复制一份。
if [[ -n "${OFFSITE_DIR:-}" ]]; then
  mkdir -p "$OFFSITE_DIR"
  cp "$TARGET"/new-api-"$DBTYPE"-$STAMP.* "$OFFSITE_DIR/"
  echo "Offsite copy: $OFFSITE_DIR"
fi
echo "RPO target: 24h（建议 crontab 每日执行）；RTO 验证见 restore.sh 与备份恢复演练.md"
