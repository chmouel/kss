#!/usr/bin/env bash
set -euf
VERSION=${1-""}

bumpversion() {
  current=$(git describe --tags $(git rev-list --tags --max-count=1) || true)
  if [[ -z ${current} ]]; then
    current=0.0.0
  fi
  current=${current#v}
  echo "Current version is ${current}"

  major=$(uv run --with semver python3 -c "import semver,sys;print(str(semver.VersionInfo.parse(sys.argv[1]).bump_major()))" ${current})
  minor=$(uv run --with semver python3 -c "import semver,sys;print(str(semver.VersionInfo.parse(sys.argv[1]).bump_minor()))" ${current})
  patch=$(uv run --with semver python3 -c "import semver,sys;print(str(semver.VersionInfo.parse(sys.argv[1]).bump_patch()))" ${current})

  echo "If we bump we get, Major: ${major} Minor: ${minor} Patch: ${patch}"
  read -p "To which version you would like to bump [M]ajor, Mi[n]or, [P]atch or Manua[l]: " ANSWER
  if [[ ${ANSWER,,} == "m" ]]; then
    mode="major"
  elif [[ ${ANSWER,,} == "n" ]]; then
    mode="minor"
  elif [[ ${ANSWER,,} == "p" ]]; then
    mode="patch"
  elif [[ ${ANSWER,,} == "l" ]]; then
    read -p "Enter version: " -e VERSION
    return
  else
    print "no or bad reply??"
    exit
  fi
  VERSION=$(uv run --with semver python3 -c "import semver,sys;print(str(semver.VersionInfo.parse(sys.argv[1]).bump_${mode}()))" ${current})
}

[[ $(git rev-parse --abbrev-ref HEAD) != main ]] && {
  echo "you need to be on the main branch"
  exit 1
}
[[ -z ${VERSION} ]] && bumpversion
[[ -z ${VERSION} ]] && {
  echo "no version specified"
  exit 1
}
if [[ ${VERSION} != v* ]]; then
  VERSION="v${VERSION}"
fi
echo "Releasing version ${VERSION}"

git tag -s ${VERSION} -m "Releasing version ${VERSION}"
git push --tags origin ${VERSION}
git pull origin main
git push origin main
