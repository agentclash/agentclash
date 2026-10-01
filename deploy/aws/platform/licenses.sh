#!/bin/sh
set -eu
destination=$1
mkdir -p "$destination/source"
cp /src/go.mod /src/go.sum "$destination/source/"
find /src -maxdepth 1 -type f \( -iname 'LICENSE*' -o -iname 'NOTICE*' -o -iname 'COPYING*' \) -exec cp '{}' "$destination/source/" \;
# Preserve upstream notices from downloaded modules without copying source caches.
go list -mod=readonly -m -f '{{if .Dir}}{{.Dir}}{{end}}' all | while IFS= read -r directory; do
    case "$directory" in
        /go/pkg/mod/*)
            relative=${directory#/go/pkg/mod/}
            mkdir -p "$destination/modules/$relative"
            find "$directory" -maxdepth 1 -type f \( -iname 'LICENSE*' -o -iname 'NOTICE*' -o -iname 'COPYING*' \) -exec cp '{}' "$destination/modules/$relative/" \;
            ;;
    esac
done
cp /usr/local/go/LICENSE "$destination/GO-LICENSE"
