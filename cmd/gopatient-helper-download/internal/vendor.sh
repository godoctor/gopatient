#!/bin/bash

# Vendoring of third-party packages for the Go Doctor

if [ `dirname $0` != '.' ]; then
	echo "vendor.sh must be run with internal as the current directory"
	exit 1
fi

FILE=`pwd`/versions.txt

echo "Logging versions to $FILE..."
date >$FILE
for pkg in github.com/cheggaaa/pb github.com/mattn/go-sqlite3; do
	pushd . >/dev/null
	cd $pkg
	echo "" >>$FILE
	echo "$pkg" >>$FILE
	git remote -v | head -1 >>$FILE
	git log --pretty=format:'%H %d %s' -1 >>$FILE
	echo "" >>$FILE
	popd >/dev/null
done

echo "Removing _example from go-sqlite3..."
rm -rf github.com/mattn/go-sqlite3/_example

echo "Removing tests from third-party packages..."
find . -iname '*_test.go' -delete

echo "Rewriting import paths in Go Doctor and third-party sources..."
find .. -iname '*.go' -exec sed -e 's/"github.com\/cheggaaa\//"github.com\/godoctor\/gopatient\/cmd\/gopatient-helper-download\/internal\/github.com\/cheggaaa\//g' -i '' '{}' ';'
find .. -iname '*.go' -exec sed -e 's/"github.com\/mattn\//"github.com\/godoctor\/gopatient\/cmd\/gopatient-helper-download\/internal\/github.com\/mattn\//g' -i '' '{}' ';'

echo "DONE"
exit 0
