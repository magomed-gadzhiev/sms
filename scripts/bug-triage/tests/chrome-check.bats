#!/usr/bin/env bats

SCRIPT="$BATS_TEST_DIRNAME/../chrome-check.sh"

@test "chrome-check: exit 1, если порт 9222 не слушает" {
  # Убедимся, что порт свободен (если нет — пропускаем)
  if curl -s -m 1 http://localhost:9222/json/version >/dev/null 2>&1; then
    skip "Порт 9222 уже занят — тест не может проверить negative-case"
  fi
  run "$SCRIPT"
  [ "$status" -eq 1 ]
  [[ "$output" == *"9222"* ]]
}

@test "chrome-check: exit 0 и OK, если 9222 отвечает на /json/version" {
  # Поднимаем python-заглушку в фоне
  python -c "
import http.server, threading
class H(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        if self.path == '/json/version':
            self.send_response(200); self.end_headers()
            self.wfile.write(b'{\"Browser\": \"Chrome/120\"}')
        else:
            self.send_response(404); self.end_headers()
    def log_message(self,*a,**k): pass
srv = http.server.HTTPServer(('127.0.0.1', 9223), H)
print(srv.server_address, flush=True)
srv.serve_forever()
" &
  STUB_PID=$!
  sleep 0.5

  # Передаём другой порт через env var — скрипт должен это поддерживать
  run bash -c "CHROME_DEBUG_PORT=9223 $SCRIPT"
  local exit_status=$status
  local output_captured=$output

  kill $STUB_PID 2>/dev/null || true
  wait $STUB_PID 2>/dev/null || true

  [ "$exit_status" -eq 0 ]
  [[ "$output_captured" == *"OK"* ]]
}
