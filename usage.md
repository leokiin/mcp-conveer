# Использование

## Запуск

```bash
# Запустить Temporal + PostgreSQL
docker compose up -d

# Собрать
go build ./cmd/mcp-conveer ./cmd/worker ./cmd/agent

# Запустить worker с конфигом
./worker --config examples/incident-review/incident-review.yaml

# Запустить пайплайн
./mcp-conveer run --config examples/incident-review/incident-review.yaml \
  --task "find why auth/token.go panics on nil user"
```

---

## Режимы оркестрации

Два режима — выбор в конфиге:

| Режим | Кто решает граф | Когда использовать |
|---|---|---|
| `sequential` | конфиг — явные шаги, зависимости, циклы | основной режим для всего |
| `routed` | тимлид-LLM в рантайме | задачи разнородные, набор агентов заранее неизвестен |

`parallel` — это сокращение для `sequential` без `depends_on`. Все шаги без зависимостей запускаются одновременно автоматически.

---

## Режим sequential

Граф описан в конфиге. Шаги без `depends_on` запускаются параллельно, с `depends_on` — после зависимости.

Каждый шаг с `depends_on` автоматически получает выходы указанных шагов как дополнительный контекст к задаче (см. [Context Chaining](#context-chaining)).

### Простой случай: все агенты параллельно

```yaml
# conveer.yaml
teamlead:
  model: deepseek/deepseek-v4-flash
  mode: sequential

workflow:
  - agent: scanner
  - agent: security
  - agent: performance

agents:
  - name: scanner
    model: ollama/llama3          # локальная — код не уходит в облако
    role_file: ./agents/scanner.md

  - name: security
    model: deepseek/deepseek-v4-flash # дёшево, OpenAI-совместимый
    role_file: ./agents/security.md

  - name: performance
    model: claude-haiku-4-5       # Anthropic
    role_file: ./agents/performance.md
```

```
задача
  ├── scanner    ─┐
  ├── security   ─┤ → параллельно (нет depends_on) → результаты
  └── performance─┘
```

### Сложный случай: зависимости и цикл

```yaml
# incident-review.yaml
teamlead:
  model: deepseek/deepseek-v4-flash
  mode: sequential

workflow:
  - step: read-task
    agent: task-reader

  - step: build-callgraph
    agent: callgraph-builder
    depends_on: [read-task]        # получает анализ задачи как контекст

  - step: resolve
    depends_on: [build-callgraph]
    loop:
      max_iterations: 5
      until: incident-finder.found == true
      steps:
        - agent: incident-finder
          depends_on: [build-callgraph, read-task]   # получает оба выхода
        - agent: problem-solver
          depends_on: [incident-finder]              # получает выход incident-finder

agents:
  - name: task-reader
    model: deepseek/deepseek-v4-flash
    role: "Reads the incident description. Extracts key terms and expected behavior."

  - name: callgraph-builder
    model: deepseek/deepseek-v4-flash
    role_file: ./agents/callgraph-builder.md

  - name: incident-finder
    model: deepseek/deepseek-v4-flash
    role_file: ./agents/incident-finder.md

  - name: problem-solver
    model: deepseek/deepseek-v4-flash
    role_file: ./agents/problem-solver.md
```

```
задача
  └── task-reader
        └── callgraph-builder (получает выход task-reader)
              └── ЦИКЛ:
                    ├── incident-finder (получает callgraph + read-task) → found: false
                    │     └── problem-solver (получает выход incident-finder)
                    └── incident-finder → found: true → выход
```

---

## Режим routed

Тимлид-LLM сам решает каких агентов вызвать исходя из задачи.

```
задача
  └── тимлид (решает: ["security", "scanner"])
        ├── security ─┐
        └── scanner  ─┘ → тимлид (синтез) → отчёт
```

```yaml
# conveer.yaml
teamlead:
  model: deepseek/deepseek-v4-flash
  mode: routed

agents:
  - name: scanner
    model: ollama/llama3
    role_file: ./agents/scanner.md

  - name: security
    model: deepseek/deepseek-v4-flash
    role_file: ./agents/security.md

  - name: performance
    model: claude-haiku-4-5
    role_file: ./agents/performance.md
```

**Когда использовать:** задача формулируется свободно ("найди почему тормозит"), и заранее неизвестно какие агенты нужны. Тимлид тратит токены на роутинг — дороже чем `sequential`.

---

## Context Chaining

Каждый шаг с `depends_on` получает выходы указанных шагов как дополнительный контекст. Агент видит оригинальную задачу + сжатые результаты предыдущих шагов:

```
[оригинальная задача]

--- Results from previous steps ---

[read-task]
{"response": "...анализ задачи..."}

[build-callgraph]
{"callgraph": [...], "suspicious": ["GenerateToken"]}
```

Это позволяет каждому агенту работать с актуальными данными не засоряя собственное контекстное окно всей историей.

---

## Роли агентов

```yaml
agents:
  - name: scanner
    role: "Find structural and syntax issues"   # инлайн — для коротких

  - name: security
    role_file: ./agents/security.md             # файл — для детальных промптов
```

`role_file` разрешается относительно директории конфига.

---

## Модели

Каждый агент независимо выбирает провайдера и модель:

```yaml
agents:
  - name: scanner
    model: ollama/llama3              # локальная — код не уходит в облако

  - name: analyzer
    model: deepseek/deepseek-v4-flash     # дёшево, OpenAI-совместимый

  - name: security
    model: claude-sonnet-4-6          # Anthropic

  - name: reporter
    model: claude-opus-4-7            # Anthropic Opus
```

| Префикс | Провайдер | Env переменная |
|---|---|---|
| `deepseek/` | DeepSeek API | `DEEPSEEK_API_KEY` |
| `ollama/` | Local Ollama | — (ключ не нужен) |
| *(без префикса)* | Anthropic | `ANTHROPIC_API_KEY` |

---

## MCP-клиент агент

Любой stdio MCP сервер подключается как шаг пайплайна через `type: mcp-client`:

```yaml
agents:
  - name: context7-fetcher
    type: mcp-client
    command: "npx -y @upstash/context7-mcp@latest"  # запускается как subprocess
    tool: resolve-library-id                          # инструмент MCP сервера
    input_key: libraryName                            # ключ аргумента для задачи
    input_step: lib-extractor                         # брать только выход этого шага
    input_field: library                              # поле из JSON выхода: "gin"
    extra_args:                                       # дополнительные фиксированные аргументы
      query: "API documentation and usage examples"
```

| Поле | Назначение |
|---|---|
| `command` | Полная команда запуска MCP сервера, разбивается по пробелам |
| `tool` | Имя инструмента для вызова через `tools/call` |
| `input_key` | Ключ аргумента, которому передаётся задача (по умолчанию `query`) |
| `input_step` | Брать только выход указанного шага вместо всего обогащённого контекста |
| `input_field` | Извлечь одно строковое поле из JSON выхода `input_step` |
| `extra_args` | Дополнительные фиксированные аргументы (нужны когда инструмент требует несколько аргументов) |

**Любой MCP сервер через stdio подключается без изменений в коде** — Context7, filesystem, GitHub, собственный сервер. Достаточно изменить `command` и `tool` в YAML.

Смотри пример: [`examples/code-review/`](examples/code-review/)

---

## API ключи

```bash
# DeepSeek (рекомендуется — дёшево, OpenAI-совместимый)
export DEEPSEEK_API_KEY=sk-...

# Anthropic Claude
export ANTHROPIC_API_KEY=sk-ant-...

# Ollama (локально, ключ не нужен)
# OLLAMA_BASE_URL=http://localhost:11434  # по умолчанию
```
