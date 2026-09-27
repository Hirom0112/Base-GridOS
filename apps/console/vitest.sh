#!/usr/bin/env bash
set -euo pipefail

source_root="$TEST_SRCDIR/$TEST_WORKSPACE"
buf=$(realpath "$1")
temporary=$(mktemp -d)
trap 'rm -rf "$temporary"' EXIT
app="$temporary/apps/console"
mkdir -p "$app" "$temporary/testdata"
cp -RL "$source_root/apps/console/src" "$source_root/apps/console/tests" "$app/"
cp -L "$source_root"/apps/console/*.{json,yaml,ts,js} "$app/"
cp -RL "$source_root/testdata/fixtures" "$temporary/testdata/"
rm -rf "$app/src/api/gen"
cat > "$temporary/ts.gen.json" <<'EOF'
{"version":"v2","plugins":[{"local":["pnpm","--package=@bufbuild/protoc-gen-es@2.11.0","dlx","protoc-gen-es"],"out":".","opt":"target=ts"}]}
EOF
cd "$app"
pnpm install --frozen-lockfile
"$buf" generate "$source_root/contracts" --template "$temporary/ts.gen.json" --output "$app/src/api/gen"
pnpm test
