# Pre-commit hooks

Pre-commit hooks живут в `.githooks/` (не в дефолтном `.git/hooks/`). Чтобы git их использовал, нужно один раз настроить `core.hooksPath`.

## Установка (первый запуск)

```bash
git config core.hooksPath .githooks
```

## Проверка

```bash
git config core.hooksPath
# Expected: .githooks
```

Если вывод пустой или `.git/hooks` — hook'и НЕ запускаются автоматически. Каждый твой коммит проходит без quality-gate (`go vet`, `go build`, `tsc`, `eslint`).

## Что это даёт

`.githooks/pre-commit` запускает `scripts/check.sh` перед каждым коммитом. Падение проверок блокирует коммит. Это первая линия защиты — CI на сервере (.github/workflows/ci.yml) всё равно проверит, но pre-commit ловит ошибки за секунды, до push'а.

## Когда обходить

Только если падает что-то внешнее (не твой код). См. CLAUDE.md секцию "Обход в экстренной ситуации". Каждый `--no-verify` обоснован в теле коммита.

## Симптомы пропуска hook'а

- `git commit` срабатывает мгновенно (<1s) на изменении кода — hook не сработал.
- CI падает на проверке, которая локально не падала — потому что локально не запускалась.
- Memory `feedback_pre_commit_hooks_path` напоминает первым делом проверить `core.hooksPath`.
