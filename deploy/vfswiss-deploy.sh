#!/usr/bin/env bash
set -euo pipefail

app_root=/opt/vfswiss
archive=/var/lib/vfswiss-deploy/incoming/release.tar.gz
PATH=/usr/sbin:/usr/bin:/sbin:/bin
export PATH

if [[ $# -ne 0 ]]; then
	printf 'vfswiss-deploy does not accept arguments\n' >&2
	exit 1
fi

if [[ ${EUID} -ne 0 ]]; then
	printf 'vfswiss-deploy must run as root\n' >&2
	exit 1
fi

exec 9>/run/lock/vfswiss-deploy.lock
if ! flock -n 9; then
	printf 'Another VF Swiss deployment is already running\n' >&2
	exit 1
fi

if [[ ! -f "$archive" || -L "$archive" ]]; then
	printf 'Deployment archive is missing or not a regular file\n' >&2
	exit 1
fi

stage=$(mktemp -d "$app_root/.vfswiss-stage.XXXXXXXX")
backup=$(mktemp -d "$app_root/.vfswiss-backup.XXXXXXXX")
trap 'rm -rf -- "$stage" "$backup"' EXIT

python3 - "$archive" "$stage" <<'PY'
import os
import pwd
import stat
import sys
import tarfile
from pathlib import PurePosixPath

archive_path, destination = sys.argv[1:]
archive_stat = os.stat(archive_path, follow_symlinks=False)
deploy_uid = pwd.getpwnam("vfswiss-deploy").pw_uid

if not stat.S_ISREG(archive_stat.st_mode) or archive_stat.st_uid != deploy_uid:
    raise SystemExit("Deployment archive must be a regular file owned by vfswiss-deploy")
if archive_stat.st_size > 100 * 1024 * 1024:
    raise SystemExit("Deployment archive exceeds the 100 MiB limit")

with tarfile.open(archive_path, "r:gz") as bundle:
    members = bundle.getmembers()
    total_size = 0
    names = set()

    for member in members:
        name = member.name.rstrip("/")
        path = PurePosixPath(name)
        if path.is_absolute() or ".." in path.parts:
            raise SystemExit(f"Unsafe archive path: {member.name}")

        allowed_directory = name in {
            "web",
            "web/templates",
            "web/static",
            "web/static/css",
            "web/static/js",
            "web/static/img",
        }
        allowed_file = (
            name == "vfswiss"
            or name.startswith("web/templates/")
            or name == "web/static/yandex_e27940cec41102c4.html"
            or name.startswith("web/static/css/")
            or name.startswith("web/static/js/")
            or name.startswith("web/static/img/")
        )
        if not (allowed_directory or allowed_file):
            raise SystemExit(f"Unexpected archive path: {member.name}")
        if not (member.isdir() or member.isfile()):
            raise SystemExit(f"Unsupported archive entry: {member.name}")
        if member.isfile():
            total_size += member.size
            member.mode = 0o755 if member.mode & 0o111 else 0o644
        else:
            member.mode = 0o755
        names.add(name)

    required = {
        "vfswiss",
        "web/templates/layout.html",
        "web/static/css/style.css",
        "web/static/js/form.js",
        "web/static/yandex_e27940cec41102c4.html",
    }
    if not required.issubset(names) or total_size > 500 * 1024 * 1024:
        raise SystemExit("Deployment archive is incomplete or exceeds the extracted-size limit")

    bundle.extractall(destination, members=members, filter="data")
PY

for path in "$app_root/vfswiss" "$app_root/web/templates" "$app_root/web/static/css" "$app_root/web/static/js" "$app_root/web/static/img"; do
	if [[ ! -e "$path" ]]; then
		printf 'Required application path is missing: %s\n' "$path" >&2
		exit 1
	fi
done

restore_previous_release() {
	if [[ -e "$backup/vfswiss" ]]; then
		if [[ -e "$app_root/vfswiss" ]]; then
			mv "$app_root/vfswiss" "$stage/vfswiss.failed" || return 1
		fi
		mv "$backup/vfswiss" "$app_root/vfswiss" || return 1
	fi
	rm -f -- "$app_root/.vfswiss-new" || return 1

	if [[ -e "$backup/templates" ]]; then
		if [[ -e "$app_root/web/templates" ]]; then
			mv "$app_root/web/templates" "$stage/web/templates.failed" || return 1
		fi
		mv "$backup/templates" "$app_root/web/templates" || return 1
	fi

	for path in css js img; do
		if [[ -e "$backup/static-$path" ]]; then
			if [[ -e "$app_root/web/static/$path" ]]; then
				mv "$app_root/web/static/$path" "$stage/web/static/$path.failed" || return 1
			fi
			mv "$backup/static-$path" "$app_root/web/static/$path" || return 1
		fi
	done
	if [[ -e "$backup/static-yandex_e27940cec41102c4.html" || ! -e "$stage/web/static/yandex_e27940cec41102c4.html" ]]; then
		rm -f -- "$app_root/web/static/yandex_e27940cec41102c4.html" || return 1
		if [[ -e "$backup/static-yandex_e27940cec41102c4.html" ]]; then
			mv "$backup/static-yandex_e27940cec41102c4.html" "$app_root/web/static/yandex_e27940cec41102c4.html" || return 1
		fi
	fi

	chown -R vfswiss:vfswiss "$app_root/web/templates" "$app_root/web/static" || return 1
	systemctl restart vfswiss || return 1
	systemctl is-active --quiet vfswiss || return 1
}

apply_release() {
	mv "$app_root/vfswiss" "$backup/vfswiss" || return 1
	mv "$app_root/web/templates" "$backup/templates" || return 1
	install -o vfswiss -g vfswiss -m 0755 "$stage/vfswiss" "$app_root/.vfswiss-new" || return 1
	mv "$app_root/.vfswiss-new" "$app_root/vfswiss" || return 1
	mv "$stage/web/templates" "$app_root/web/templates" || return 1
	for path in css js img; do
		if [[ -d "$stage/web/static/$path" ]]; then
			mv "$app_root/web/static/$path" "$backup/static-$path" || return 1
			mv "$stage/web/static/$path" "$app_root/web/static/$path" || return 1
		fi
	done
	if [[ -f "$app_root/web/static/yandex_e27940cec41102c4.html" ]]; then
		mv "$app_root/web/static/yandex_e27940cec41102c4.html" "$backup/static-yandex_e27940cec41102c4.html" || return 1
	fi
	mv "$stage/web/static/yandex_e27940cec41102c4.html" "$app_root/web/static/yandex_e27940cec41102c4.html" || return 1
	chown -R vfswiss:vfswiss "$app_root/web/templates" "$app_root/web/static" || return 1
}

if ! apply_release; then
	printf 'Deployment failed while installing files; restoring previous release\n' >&2
	if ! restore_previous_release; then
		printf 'Rollback failed; inspect vfswiss.service and restore the previous files manually\n' >&2
	fi
	exit 1
fi

if ! systemctl restart vfswiss || ! systemctl is-active --quiet vfswiss; then
	printf 'Service did not become active; restoring previous release\n' >&2
	if ! restore_previous_release; then
		printf 'Rollback failed; inspect vfswiss.service and restore the previous files manually\n' >&2
	fi
	exit 1
fi

if ! curl --fail --silent --show-error --retry 5 --retry-connrefused --retry-delay 1 --max-time 10 http://127.0.0.1:8080/health >/dev/null; then
	printf 'Local health check failed; restoring previous release\n' >&2
	if ! restore_previous_release; then
		printf 'Rollback failed; inspect vfswiss.service and restore the previous files manually\n' >&2
	fi
	exit 1
fi

rm -f -- "$archive"
printf 'VF Swiss deployment completed successfully\n'
