#!/usr/bin/env bash
# Draw SAM's masks and the keypoints derived from them on chosen frames of the
# production server's labels. See overlay.py for the frame syntax.
#
#   scripts/keypoints/overlay.sh GX012079:2:1-23 GX012079:5:14 clip=276 frame=338
#   scripts/keypoints/overlay.sh --track --model facebook/sam2.1-hiera-small \
#       --model facebook/sam2.1-hiera-base-plus GX012079:5:14-49
#
# LABEL_TYPE (name or id, default bin_blood) picks the label type and OUT
# (default /tmp/keypoint-overlays) where images go. Models load from the
# krill-hf volume; set HF_TOKEN to download ones it lacks, such as SAM 3.
set -euo pipefail

here=$(cd "$(dirname "$0")" && pwd)
repo=$(cd "$here/../.." && pwd)
deploy=${KRILL_DEPLOY:-/srv/krill/app/deploy}
project=${KRILL_PROJECT:-krill-server}
label_type=${LABEL_TYPE:-bin_blood}
out=${OUT:-/tmp/keypoint-overlays}
image=${KRILL_WORKER_IMAGE:-$(docker inspect --format '{{.Config.Image}}' "$project-worker-1")}

# deploy/.env has unquoted values with spaces, so it can't be sourced.
env_value() { grep -m1 "^$1=" "$deploy/.env" | cut -d= -f2-; }
export KRILL_S3_ACCESS_KEY KRILL_S3_SECRET_KEY KRILL_S3_BUCKET
KRILL_S3_ACCESS_KEY=$(env_value KRILL_S3_ACCESS_KEY)
KRILL_S3_SECRET_KEY=$(env_value KRILL_S3_SECRET_KEY)
KRILL_S3_BUCKET=$(env_value KRILL_S3_BUCKET || true)

mkdir -p "$out"
docker exec -i "$project-postgres-1" sh -c \
    'psql -U "$POSTGRES_USER" -d "${POSTGRES_DB:-$POSTGRES_USER}" -At -v ON_ERROR_STOP=1 -v type="$0"' \
    "$label_type" <"$here/export.sql" >"$out/labels.json"

offline=1
[ -n "${HF_TOKEN:-}" ] && offline=0

docker run --rm --network "${project}_default" \
    --device /dev/kfd --device /dev/dri -e HSA_OVERRIDE_GFX_VERSION=10.3.0 \
    -e PYTHONPATH=/src -e HF_HUB_OFFLINE=$offline -e HF_TOKEN \
    -e KRILL_S3_ACCESS_KEY -e KRILL_S3_SECRET_KEY -e KRILL_S3_BUCKET \
    -v "$repo/worker/src:/src:ro" -v "$here:/kp:ro" -v "$out:/out" \
    -v krill-hf:/root/.cache/huggingface \
    --entrypoint sh "$image" -c \
    'python /kp/overlay.py --labels /out/labels.json --out /out "$@"; s=$?; chown -R '"$(id -u):$(id -g)"' /out; exit $s' \
    sh "$@"
