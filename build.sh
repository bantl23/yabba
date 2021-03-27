#!/bin/bash

export HASH=`git log --pretty=format:'%h' -n 1`

export VERSION_MAJOR=$( git describe --tags --abbrev=0 | cut -f1 -d. )
if [ "x" == "x$VERSION_MAJOR" ]; then export VERSION_MAJOR="v0"; fi
export VERSION_MINOR=$( git describe --tags --abbrev=0 | cut -f2 -d. )
if [ "x" == "x$VERSION_MINOR" ]; then export VERSION_MINOR="0"; fi
export VERSION_PATCH=$( git describe --tags --abbrev=0 | cut -f3 -d. )
if [ "x" == "x$VERSION_PATCH" ]; then export VERSION_PATCH="0"; fi
export VERSION=$VERSION_MAJOR.$VERSION_MINOR.$VERSION_PATCH

export VERSION_BRANCH=$( git symbolic-ref --short HEAD | cut -f1 -d- )
if [ "$VERSION_BRANCH" == "main" ]; then export VERSION_BRANCH="0"; fi
if [ "x" == "x$VERSION_BRANCH" ]; then export VERSION_BRANCH="0"; fi

export VERSION_BUILD=$( git describe --tags | cut -f2 -d- )
if [ "$VERSION_BUILD" == "$VERSION" ]; then export VERSION_BUILD="0"; fi
if [ "x" == "x$VERSION_BUILD" ]; then export VERSION_BUILD="0"; fi
if [ "$VERSION_BRANCH" != "0" ]; then export VERSION=$VERSION-issue.$VERSION_BRANCH; fi
if [ "$VERSION_BUILD" != "0" ]; then export VERSION=$VERSION-build.$VERSION_BUILD; fi
echo $VERSION $VERSION_MAJOR $VERSION_MINOR $VERSION_PATCH $VERSION_ISSUE $VERSION_BUILD $HASH

set -x
go build -v -ldflags "-X github.com/bantl23/yabba/cmd.Version=$VERSION -X github.com/bantl23/yabba/cmd.Hash=$HASH"
