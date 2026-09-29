#!/bin/sh
# Put the write-ahead log on its own volume so a checkpoint flush of table
# pages does not sit in front of the commit fsync.
set -eu

wal=/var/lib/postgresql/wal
data=${PGDATA:-/var/lib/postgresql/data}
link="$data/pg_wal"

mkdir -p "$wal"
if [ "$(id -u)" = 0 ]; then
	chown postgres:postgres "$wal"
fi

if [ -s "$data/PG_VERSION" ] && [ -d "$link" ] && [ ! -L "$link" ]; then
	if [ -n "$(ls -A "$wal" 2>/dev/null || true)" ]; then
		echo "WAL volume is not empty and pg_wal is still inside the data directory" >&2
		exit 1
	fi
	cp -a "$link"/. "$wal"/
	rm -rf "$link"
	ln -s "$wal" "$link"
	if [ "$(id -u)" = 0 ]; then
		chown -h postgres:postgres "$link"
		chown -R postgres:postgres "$wal"
	fi
fi

exec docker-entrypoint.sh "$@"
