#!/usr/bin/env bash
set -eufo pipefail
cd $(dirname $(readlink -f $0))
TMP=$(mktemp /tmp/.mm.XXXXXX)
clean() { rm -f ${TMP}; }
trap clean EXIT
set -x

curl -f -H "Authorization: $(pass show github/chmouel-token)" https://api.github.com/repos/chmouel/kss/releases  > ${TMP}

version=$(jq -r '.[0].tag_name' ${TMP})
[[ -z ${version} ]] && { echo "no version found??"; exit 1;}
tarball=https://codeload.github.com/chmouel/kss/legacy.tar.gz/refs/tags/${version}
curl -H"Authorization: $(pass show github/chmouel-token)"  -# -o ${TMP} ${tarball}

shasum=$(sha256sum ${TMP}|cut -d" " -f1)
sed -i "s/sha256 \".*\"/sha256 \"${shasum}\"/;s/version \".*\"/version \"${version}\"/" ../Formula/kss.rb

git commit ../Formula/kss.rb -m "Update for ${version}"
git push origin master --no-verify
