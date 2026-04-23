#!/usr/bin/env bats

SCRIPT="$BATS_TEST_DIRNAME/../state-init.sh"

setup() {
  export BUGS_DIR="$(mktemp -d)"
}

teardown() {
  rm -rf "$BUGS_DIR"
}

@test "state-init: создаёт state.json с version и bugs={}, если его нет" {
  run "$SCRIPT"
  [ "$status" -eq 0 ]
  [ -f "$BUGS_DIR/state.json" ]
  grep -q '"version": 1' "$BUGS_DIR/state.json"
  grep -q '"bugs": {}' "$BUGS_DIR/state.json"
  grep -q '"max_parallel": 3' "$BUGS_DIR/state.json"
}

@test "state-init: не перезаписывает существующий валидный state.json" {
  echo '{"version":1,"custom":"xxx","bugs":{}}' > "$BUGS_DIR/state.json"
  run "$SCRIPT"
  [ "$status" -eq 0 ]
  grep -q '"custom":"xxx"' "$BUGS_DIR/state.json"
}

@test "state-init: при corrupt JSON делает .bak и создаёт новый" {
  echo 'garbage{not json' > "$BUGS_DIR/state.json"
  run "$SCRIPT"
  [ "$status" -eq 0 ]
  [ -f "$BUGS_DIR/state.json.bak" ]
  grep -q '"bugs": {}' "$BUGS_DIR/state.json"
}
