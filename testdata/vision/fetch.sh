#!/bin/sh
# Downloads part of VISION (Shullani et al., 2017), the University of
# Florence's set of photos from 35 phones with their WhatsApp and Facebook
# copies, into the folder given. It is CC BY-SA, so it is downloaded and
# not kept in this repository.
#
#   sh fetch.sh DIR FLAT NAT DEVICE...
#
# FLAT photos of the sky and walls per phone for the fingerprint, and NAT
# ordinary photos per phone with each of their shared copies.
set -eu
dir=$1 flat=$2 nat=$3
shift 3
mkdir -p "$dir"
list="$dir/VISION_files.txt"
[ -s "$list" ] || curl -sSf https://lesc.dinfo.unifi.it/VISION/VISION_files.txt -o "$list"
for device in "$@"; do
	grep "/dataset/${device}_[^/]*/images/flat/" "$list" | head -n "$flat"
	for kind in nat natWA natFBH natFBL; do
		grep "/dataset/${device}_[^/]*/images/$kind/" "$list" | head -n "$nat"
	done
done | sed 's/\r$//' > "$dir/wanted.txt"
cd "$dir"
# One folder per phone and kind, the way the dataset lays them out.
sed 's|.*/dataset/||' wanted.txt | while read -r path; do
	[ -s "$path" ] || printf '%s\n' "$path"
done | xargs -P 8 -I{} sh -c 'mkdir -p "$(dirname "{}")" && curl -sSf "https://lesc.dinfo.unifi.it/VISION/dataset/{}" -o "{}"'
