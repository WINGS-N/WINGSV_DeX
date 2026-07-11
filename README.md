# WINGSV_Dex

WINGSV_Dex - это настольный клиент Wails v3 для WINGS V. Текущий MVP сосредоточен на VK TURN и WireGuard: чтении ссылок `wingsv://`, управлении профилями, подготовке настроек WireGuard и отображении настроек VK TURN в интерфейсе.

Приложение использует backend на Go, frontend на Vue 3/Vite, системный WebKitGTK в Linux и вспомогательный `vk-turn-proxy`, собираемый из git-submodule.

## Структура репозитория

| Путь | Назначение |
| --- | --- |
| `main.go` | Точка входа Wails-приложения и специальные режимы помощников. |
| `internal/services` | Сервисы, доступные frontend Wails. |
| `internal/config` | Runtime-настройки и хранилище профилей. |
| `internal/vktp` | Дочерний процесс `vkturn` и AppControl gRPC-клиент. |
| `internal/wg`, `internal/wgwin` | Интеграция WireGuard для Linux и Windows. |
| `internal/wingsv` | Кодек `wingsv://` и модель конфигурации. |
| `internal/gen` | Сгенерированные protobuf- и gRPC-привязки. |
| `internal/nethelper`, `internal/dataplane`, `internal/shell`, `internal/vklogin` | Код помощников, dataplane, shell и входа VK. |
| `frontend` | UI на Vue 3, Vite, Tailwind и Wails runtime. |
| `external/wingsv-proto` | Локальный proto-subtree с определениями конфигурации WINGS V. |
| `external/vk-turn-proxy` | Git-submodule, используемый для сборки помощника `vkturn`. |
| `scripts` | Скрипты сопровождения, например генерация protobuf. |
| `build` | Общие и платформенные Taskfile для сборки и упаковки Wails. |

## Требования

Подтверждено файлами проекта:

- Go 1.25.0 или более новый совместимый toolchain.
- Node.js и `pnpm` для frontend. `pnpm` - целевой менеджер пакетов.
- Task (`task`) для команд корневого Taskfile.
- Wails v3 CLI (`wails3`).
- Инструменты Protobuf для `task generate:proto`: `protoc`, `protoc-gen-go` и `protoc-gen-go-grpc`.
- Поддержка git-submodule для `external/vk-turn-proxy`.
- Linux desktop/runtime-сборки используют GTK4, WebKitGTK 6.0, nftables и pkexec/polkit согласно metadata Linux-пакета.

## Настройка

Клонирование с submodule:

```bash
git clone --recurse-submodules https://github.com/WINGS-N/WINGSV_Dex.git
cd WINGSV_Dex
```

Если репозиторий уже склонирован, инициализируйте submodule перед сборкой `vkturn` или повторной генерацией protobuf-привязок appcontrol:

```bash
git submodule update --init --recursive
```

Установите зависимости frontend:

```bash
pnpm --dir frontend install
```

Перед запуском функционального локального приложения или созданием пакетов соберите помощник `vkturn`:

```bash
task build:vkturn
```

## Основные команды

Эти команды определены в `Taskfile.yml`, если не указано иначе.

| Команда | Что делает |
| --- | --- |
| `task build` | Собирает приложение для текущего `GOOS` через соответствующий платформенный Taskfile. |
| `task run` | Запускает уже собранный платформенный бинарник/приложение. При необходимости сначала выполните `task build`. |
| `task dev` | Запускает Wails в режиме разработки на настроенном Vite-порту. |
| `task package` | Упаковывает production-сборку через соответствующий платформенный Taskfile. |
| `task build:vkturn` | Собирает клиент `vk-turn-proxy` из `external/vk-turn-proxy` в `bin/vkturn` или `bin/vkturn.exe`. Требует submodule. |
| `task generate:proto` | Запускает `scripts/generate_proto.sh` для повторной генерации Go protobuf-привязок. Требует submodule и protobuf-инструменты. |
| `pnpm --dir frontend build` | Собирает Vue frontend через Vite. Ожидает сгенерированные Wails-привязки. |
| `pnpm --dir frontend dev` | Запускает отдельный dev-сервер Vite. Ожидает сгенерированные Wails-привязки. |
| `pnpm --dir frontend format` | Форматирует исходники frontend через Prettier. |
| `pnpm --dir frontend format:check` | Проверяет форматирование frontend. |

Отдельная корневая test-задача не определена. Package-тесты находятся в `internal/...`; при необходимости используйте Go tooling напрямую после подготовки сгенерированных prerequisites.

Отдельные frontend-команды build/dev ожидают Wails-привязки в `frontend/bindings`. Перед запуском raw Vite-команд в свежем checkout используйте `task build`, `task dev` или `task common:generate:bindings`.

## Заметки по frontend

Frontend - это приложение Vue 3 + Vite с `@wailsio/runtime`, Tailwind и Prettier. Vite по умолчанию настроен на порт `9245`. `frontend/README.md` содержит шаблонные заметки Vue/Vite и не является полной документацией проекта.

## Заметки по backend

Backend на Go запускает Wails-приложение, подключает сервисы для frontend и включает специальные режимы процесса для helper flows. Connection service управляет дочерним процессом `vkturn` и dataplane WireGuard. Значения runtime-настроек по умолчанию находятся в `internal/config`.

## Protobuf и submodules

`external/vk-turn-proxy` - git-submodule. Он нужен для `task build:vkturn` и `task generate:proto`, потому что `scripts/generate_proto.sh` читает `appcontrol.proto` из этого submodule.

Тот же скрипт синхронизирует `wingsv.proto` из репозитория WINGS V в `external/wingsv-proto` перед повторной генерацией Go-привязок в `internal/gen`.

## Лицензия

Проект распространяется под лицензией GPL v3. Metadata Linux-пакета указывает `GPL-3.0-only`.
