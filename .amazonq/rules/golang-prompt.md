# 1. Настройка окружения

1. Установить:

   - Go (актуальная минорная версия, без зависания на устаревших)
   - golangci-lint
   - staticcheck
   - govulncheck
   - gci (если нужно более детальное управление импортами)

2. В VS Code включить (settings.json):

   ````markdown
   "go.useLanguageServer": true,
   "go.formatTool": "gofmt",
   "go.lintTool": "golangci-lint",
   "go.vetOnSave": "package",
   "go.lintOnSave": "file",
   "editor.formatOnSave": true,
   "go.testFlags": ["-race", "-count=1"]
   ````

3. Строго использовать go mod. Никаких GOPATH-сборок.

4. Все коммиты проходят: go fmt ./..., go vet ./..., golangci-lint run, go test -race ./...

5. Запрещено вручную править то, что исправляет автоформаттер.

# 2. Конфигурация golangci-lint (жёсткая база)

````markdown
# .golangci.yml
run:
  timeout: 5m
  tests: true
  allow-parallel-runners: true

output:
  sort-results: true
  formats:
    - format: colored-line-number

linters-settings:
  gci:
    sections:
      - Standard
      - Default
      - Prefix(your_org/your_repo)
    skip-generated: true
  govet:
    enable-all: true
    disable:
      - fieldalignment
  revive:
    severity: warning
  cyclop:
    max-complexity: 12
    package-average: 10
  goconst:
    min-len: 3
    min-occurrences: 3
  gocyclo:
    min-complexity: 12
  nakedret:
    max-func-lines: 0
  funlen:
    lines: 80
    statements: 50
  lll:
    line-length: 120
    tab-width: 1
  dupl:
    threshold: 80
  errcheck:
    check-type-assertions: true
    check-blank: true
  wrapcheck:
    ignoreSigs:
      - errors.New
      - fmt.Errorf
  misspell:
    locale: US
  nolintlint:
    require-explanation: true
    require-specific: true

linters:
  enable:
    - govet
    - staticcheck
    - revive
    - errcheck
    - ineffassign
    - gosimple
    - typecheck
    - unused
    - gocyclo
    - cyclop
    - funlen
    - nakedret
    - unparam
    - gci
    - misspell
    - lll
    - prealloc
    - dupl
    - gosec
    - exportloopref
    - wrapcheck
    - goconst
    - whitespace
    - nolintlint
  disable:
    - depguard
    - exhaustivestruct
    - varnamelen

issues:
  max-same-issues: 0
  exclude-rules:
    - path: "_test\\.go"
      linters:
        - funlen
        - gocyclo
        - cyclop
        - dupl
````

# 3. Структура проекта

1. Монорепозитории — модульная сегментация через internal/, pkg/, cmd/.
2. Корень:
   - cmd/<service>/main.go — только wiring (DI, init logging, start/stop)
   - internal/<bounded_context>/... — бизнес-логика (закрытая)
   - pkg/ — публичные стабильные пакеты (минимум)
   - api/ — openapi/grpc схемы
   - build/ — Dockerfile, CI скрипты
   - configs/ — базовые конфиги
   - scripts/ — одноразовые утилиты (Go или shell)
3. Запрещено растаскивать доменные типы по случайным пакетам.

# 4. Именование

1. Пакеты: коротко, семантично, без глаголов: http, auth, ledger.
2. Интерфейс: имя по потребителю, не по реализации: Writer, TokenIssuer.
3. Реализация: суффикс по источнику: PostgresLedger, MemoryCache.
4. Избегать сокращений, кроме устоявшихся: ctx, cfg, req, resp, id, ok.
5. Экспорт только того, что реально требуется внешним пакетам.

# 5. Интерфейсы

1. Объявляются рядом с кодом, который их потребляет (правило "интерфейс принадлежит потребителю").
2. Минимальная поверхность: 1–3 метода.
3. Запрещены "god" интерфейсы (Storage, Service с десятками методов).
4. Не создавай интерфейс заранее — только если есть >1 реализация или тестовая подмена.

# 6. Ошибки

1. Ошибка — часть протокола поведения, не контроль потока.
2. Использовать обёртки: fmt.Errorf("context: %w", err).
3. Свой тип ошибки — когда нужно различать поведение: sentinel + errors.Is / errors.As.
4. Не теряй стек — не обнуляй err.
5. Проверка сразу после вызова. Никаких "отложенных" проверок.
6. Запрещены игнорируемые ошибки (_). Исключение — io.Copy(io.Discard, r) с комментом //nolint и объяснением.
7. errors.Join — осмысленно, не превращать в свалку.

# 7. Контекст (context.Context)

1. Первый аргумент у всех внешних/IO/публичных функций: ctx context.Context.
2. Не храни context.Context в структурах.
3. Не передавай nil ctx. Используй context.Background()/context.TODO() строго осознанно (TODO — временно).
4. Вложенны�� таймауты — минимально. Центральное место управления временем — уровень orchestration.

# 8. Конкурентность

1. Не плодить go routines без контроля: каждая либо:
   - управляется errgroup
   - имеет канал завершения
   - обслуживается worker pool
2. Каналы:
   - Буфер только при доказанной необходимости
   - Закрывает тот, кто пишет
3. select с default не для busy loop — только для try-send/send-or-drop.
4. sync.Mutex / RWMutex: предпочтительнее атомиков, кроме горячих путей.
5. Data race => тест с -race обязателен в CI.
6. Не передавать pointer на мутабельную структуру в несколько сутей без синхронизации.

# 9. Генерики

1. Использовать только при реальном устранении дублирования семантики, не ради синтаксиса.
2. Не внедрять generics в публичный API до стабилизации.
3. Запрещены чрезмерные constraint-композиции без пользы.

# 10. Неизменяемость (где возможно)

1. Возвращать копии срезов и карт, если владелец не должен мутировать.
2. Не экспортировать поля структур (используй опции-конструкторы или функции).
3. Для конфигурации использовать отдельный immutable Config.
4. Внутренние кэши — защищены: sync.Map либо обёртки + RWMutex.

# 11. Логирование

1. Structured логгер (zap, zerolog).
2. Никаких fmt.Println в проде.
3. Логгирование ошибок — один раз на границе ответственности. Не дублировать шум.
4. Поля: trace_id, span_id, operation, elapsed_ms.
5. Уровни: debug/ info / warn / error. Fatal — только в main при неустранимом.

# 12. Тесты

1. Имена файлов: <unit>_test.go.
2. Табличные тесты для вариативных сценариев.
3. t.Helper() в вспомогательных функциях.
4. race, -count=1, покрытие целевого пакета не менее установленной нормы (определи порог, напр. 70% разумно для логики; цифра ради цифры не нужна).
5. Моки через интерфейсы, не через global patch.
6. Benchmark: go test -bench=. -benchtime=2s -count=5 | benchstat.
7. Fuzz для парсеров и трансформаций.

# 13. Производительность

1. Профили: pprof — cpu, heap до оптимизации, не гадать.
2. Избегать ненужных аллокаций: strings.Builder, bytes.Buffer.
3. Предвыделение: make([]T, 0, n) если известен верхний предел.
4. Лишние конверсии string <-> []byte — удалить.
5. map — не использовать как set без обоснования (можно: map[T]struct{}).
6. Критичный код — тест с go test -run=NONE -bench=... -benchmem.

# 14. Безопасность

1. govulncheck в CI.
2. Не логировать секреты. Маскировать токены / пароли.
3. Внешний ввод — валидировать: длины, форматы, диапазоны.
4. SQL — использовать prepared statements / builder, никакой конкатенации.
5. crypto/rand вместо math/rand для секретов.
6. net/http — выставлять разумные таймауты (Transport и Server).

# 15. Работа с внешними ресурсами

1. HTTP клиент — настроенный *http.Client с Timeout, Transport с ограничениями.
2. Retriable операции — использовать backoff (например, hashicorp/go-retryablehttp или своя обёртка).
3. Максимальное число попыток фиксировано. Джиттер обязателен.
4. gRPC — unary + stream interceptors (retry, tracing, metrics).

# 16. Метрики и наблюдаемость

1. Прометеевские метрики: latency histogram, error counter, inflight gauge.
2. Tracing — OpenTelemetry, пропагация через ctx.
3. Health endpoints: /live, /ready — минимальная логика.

# 17. Сборка и версии

1. Версионирование через git tag (semver).

2. Встраивание build-time данных:

   ````markdown
   go build -ldflags "-s -w -X 'internal/version.CommitHash=$(GIT_SHA)' -X 'internal/version.BuildTime=$(BUILD_TIME)'"
   ````

3. reproducible builds: фиксированный go.mod + go.sum.

# 18. Управление зависимостями

1. go get -u=patch периодически; минор / мажор — только через change review.
2. Запрещено тащить тяжёлые фреймворки ради одной функции.
3. Вендорить только если среда изолирована.

# 19. Документация

1. Godoc — только для экспортируемых символов, которые составляют публичный API.
2. Комментарий = объяснение "зачем", не "что".
3. Никаких романов. Если требуется долгий текст — DESIGN.md в пакете.

# 20. main.go политика

1. Только:
   - Парсинг флагов / конфигов
   - Инициализация логгера, DI граф
   - Старт сервера
   - Грациозное завершение (context + сигнал)
2. Никакой доменной логики в main.

# 21. Кодогенерация

1. go:generate директивы рядом с целевым файлом.
2. Генерированный код помечен // Code generated by ... DO NOT EDIT.
3. Генераторы детерминированы.

# 22. Стиль

1. Все проходит через gofmt + goimports.
2. Импорты: стандартная библиотека, затем внешние, затем внутренние (разделяются пустой строкой).
3. Не использовать глобальные var кроме:
   - Err... sentinel
   - Таблиц/констант (immutable)
4. Константы группировать; iota — осторожно, только когда улучшает читабельность.
5. Избегать magic numbers — именованные константы.

# 23. Паттерны отказоустойчивости

1. Circuit breaker над нестабильными внешними точками.
2. Timeout + context строгий. Retry без анализа класса ошибки — запрещён.
3. Dead-letter / fallback для критичных очередей.

# 24. Антипаттерны (запрет)

1. Глубокие пакетные зависимости (диаметр > 3 по цепочке) — пересмотреть архитектуру.
2. Пакет util / common / helpers — удалить, распределить по предметным областям.
3. Паники в продовой логике (panic допустим только при невосстановимой внутренней инвариантной порче).
4. Связывание через init() — минимально. init() только для регистраций (метрик, схем, embed).
5. Логика в тестах, которая отличается от реальной (дублирование альтернативных веток) — пересобрать публичный API для удобства теста.

# 25. Пример минимального каркаса

````markdown
module github.com/your_org/ledger

go 1.22

require (
    github.com/cenkalti/backoff/v4 v4.3.0
    go.uber.org/zap v1.27.0
)
````

````markdown
// cmd/ledger/main.go
package main

import (
	"context"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	"github.com/your_org/ledger/internal/app"
)

func main() {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer cancel()

	application := app.BuildApplication(logger)

	if err := application.Start(ctx); err != nil {
		logger.Fatal("startup failed", zap.Error(err))
	}

	<-ctx.Done()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := application.Stop(shutdownCtx); err != nil {
		logger.Error("graceful stop failed", zap.Error(err))
	}
}
````

````markdown
// internal/app/app.go
package app

import (
	"context"

	"go.uber.org/zap"
)

type Application struct {
	logger *zap.Logger
}

func BuildApplication(logger *zap.Logger) *Application {
	return &Application{
		logger: logger,
	}
}

func (a *Application) Start(ctx context.Context) error {
	a.logger.Info("application started")
	return nil
}

func (a *Application) Stop(ctx context.Context) error {
	a.logger.Info("application stopped")
	return nil
}
````

# 26. Метрики и трейсинг пример (упрощённо)

````markdown
// internal/observability/metrics.go
package observability

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Registry struct {
	requestLatency *prometheus.HistogramVec
}

func NewRegistry() *Registry {
	r := &Registry{
		requestLatency: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "ledger",
				Name:      "http_request_latency_seconds",
				Buckets:   prometheus.DefBuckets,
			},
			[]string{"route", "code"},
		),
	}
	prometheus.MustRegister(r.requestLatency)
	return r
}

func (r *Registry) Handler() http.Handler {
	return promhttp.Handler()
}
````

# 27. Эволюция и ревью

1. Любое ослабление правил — через письменное обоснование.
2. Новые зависимости — justification (трёхстрочный контекст: зачем, альтернатива, риски).
3. Миграции схем / протоколов — через версионирование модельных типов (v1, v2 пакеты).

# 28. Минимизация скрытой сложности

1. Сокращать вложенность через ранние возвраты (инверсия условий).
2. Сведение ветвлений (guard clauses).
3. Удалять неиспользуемый код немедленно (не хранить «вдруг пригодится»).

# 29. Диагностика

1. Сбои в проде анализировать по цепочке: симптом → лог → метрика → профиль.
2. Инструменты: pprof HTTP endpoint на закрытом порту с auth / firewall.
3. Запрещено включать непродуманный профайлинг в публичный интерфейс.

# 30. Код-ревью чеклист (жёсткий)

1. Нарушение интерфейсной сегрегации?
2. Избыточные экспорты?
3. Непрозрачные ошибки? Потерян контекст?
4. Расходование памяти и аллокации на горячем пути?
5. Неразделённые обязанности (SRP)?
6. Конкурентные участки без синхронизации?
7. Необоснованные inline-оптимизации (преждевременные)?
8. Логи лишние / дублирующиеся?
9. Документированы ли публичные типы?
10. Утечки goroutine (нет завершения)?
11. context нарушен?
12. Ошибки не обёрнуты контекстом?
13. Проверка на nil pointer / slice bounds?
14. Контроль версий контрактов (API/DTO) есть?

# 31. Принцип эскалации

1. Любой спор по стилю решается ссылкой: стандарт → линтер → документ. Если нет — добавить правило.
2. Отсутствие правила не оправдание циклической зависимости или неоправданной абстракции.

# 32. NFR требования (минимум для сервисов)

1. Latency p95 — целевое значение фиксируется в SLO (не писать код без чисел).
2. Error budget учитывается — переоптимизация после исчерпания.
3. Startup time — контролируем (нет тяжёлой инициализации до необходимости).

# 33. Сторонние пакеты (критерии допуска)

1. Maintained (commits < 6 месяцев давности).
2. Нет критичных CVE.
3. Лицензия совместима (MIT, BSD, Apache 2.0).
4. API устойчивый, без хаотичных breaking changes.

# 34. Пример ретрая (осознанный)

````markdown
var (
	ErrTemporary = errors.New("temporary")
)

func executeWithRetry(ctx context.Context, attempt func(context.Context) error) error {
	var last error
	backoff := time.Millisecond * 100
	for i := 0; i < 5; i++ {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		err := attempt(ctx)
		if err == nil {
			return nil
		}
		if !errors.Is(err, ErrTemporary) {
		 return err
		}
		last = err
		time.Sleep(backoff + time.Duration(rand.Intn(50))*time.Millisecond)
		backoff *= 2
	}
	return fmt.Errorf("exhausted retries: %w", last)
}
````

# 35. Что не обсуждается

1. gofmt — аксиома.
2. Локальные форки стандартных инструментов — запрещено.
3. Обфускация логики ради “умности” — удаляется при ревью.

# 36. Каталог tests

1. Дополнительно к _test.go в пакетах создаётся корневой каталог ./tests/.
2. Структура:
   - tests/unit — высокоуровневые unit сценарии (иногда дублируют сложные композиции без лишних зависимостей).
   - tests/integration — тесты с реальными адаптерами (docker-compose / testcontainers).
   - tests/performance — нагрузочные сценарии и benchmark-обвязка (go test -bench=..., отдельные профили).
   - tests/e2e — энд-ту-энд (HTTP/gRPC поверх запущенного сервиса).
   - tests/fuzz — входные корпуса и seed-файлы для fuzz (если отделено).
3. Общие хелперы: tests/internal/ или tests/support/ (не экспортируются).
4. Никакой бизнес-логики в tests/, только сценарии и вспомогательные фикстуры.
5. Конфиги тестов изолированы: configs/test/*.yaml (при необходимости).
6. Интеграционные и e2e тесты помечаются build tag `//go:build integration` / `//go:build e2e` для выборочного запуска.
7. CI матрица: unit (быстро), integration (дольше), performance (по расписанию или вручную).

# 37. Makefile

1. В корне Makefile — единая точка входа. Команды короткие, предсказуемые.

2. Обязательный минимальный набор таргетов:

   - make deps (go mod tidy; go mod download)
   - make lint (golangci-lint run ./...)
   - make test (go test -race -count=1 ./...)
   - make test-unit (фильтр без build tags)
   - make test-integration (go test -race -count=1 -tags=integration ./tests/integration/...)
   - make coverage (go test -coverprofile=coverage.out ./... && go tool cover -func=coverage.out)
   - make build (go build ./cmd/...)
   - make run (go run ./cmd/<service>)
   - make vet (go vet ./...)
   - make vuln (govulncheck ./...)
   - make swagger (генерация OpenAPI)
   - make tidy (go mod tidy)
   - make fmt (go fmt ./... && goimports -w .)

3. Пример:

   ````markdown
   .PHONY: deps lint test build run fmt coverage swagger tidy vet vuln
   
   deps:
   	go mod tidy
   	go mod download
   
   lint:
   	golangci-lint run ./...
   
   test:
   	go test -race -count=1 ./...
   
   test-unit:
   	go test -race -count=1 ./... -tags='!integration,!e2e'
   
   test-integration:
   	go test -race -count=1 -tags=integration ./tests/integration/...
   
   coverage:
   	go test -coverprofile=coverage.out ./...
   	go tool cover -func=coverage.out
   
   build:
   	go build -ldflags "-s -w" ./cmd/...
   
   run:
   	go run ./cmd/service
   
   fmt:
   	go fmt ./...
   	goimports -w .
   
   vet:
   	go vet ./...
   
   vuln:
   	govulncheck ./...
   
   swagger:
   	make -C api/openapi generate
   
   tidy:
   	go mod tidy
   ````

4. Никаких скрытых магических шагов. Явная декларация зависимостей.

# 38. Swagger / OpenAPI

1. Каждый HTTP сервис обязан иметь актуальную спецификацию OpenAPI (swagger).

2. Расположение: api/openapi/<service>/openapi.yaml (или .json). Генерация кода — в internal/gen/<service>/ (не смешивать с ручным кодом).

3. Инструменты (один из):

   - oapi-codegen
   - go-swagger
   - kin-openapi + собственный генератор
   - swag (только если есть строгая дисциплина по комментариям; предпочтительнее декларативный YAML, а не комментарии)

4. Генерация через make swagger (см. Makefile). Пример под oapi-codegen:

   ````markdown
   # api/openapi/service/Makefile
   .PHONY: generate
   generate:
   	oapi-codegen -generate types,chi-server -o ../../internal/gen/service/server.gen.go -package service openapi.yaml
   ````

5. Спецификация — источник правды; серверные хендлеры соответствуют контракту.

6. Любое изменение публичного API = Pull Request с обновлением openapi.yaml + README.md (секция API).

7. В CI — проверка чистоты git tree после make swagger (без “грязных” незафиксированных артефактов).

### 38.1 swag-аннотации (если вместо или дополнительно к YAML используется github.com/swaggo/swag)

1. Использование swag допустимо только если:

   - Требуется быстрое прототипирование и частые мелкие правки без ручного редактирования YAML.
   - Поддерживается строгая дисциплина обновления комментариев в том же PR, что меняет хендлер.
   - Генерируемый openapi.json идентичен по контракту фактическому поведению. Drift = дефект.

2. Структура вывода:

   - Сгенерированные файлы кладутся в internal/gen/swagger/ (или internal/gen/<service>/swagger/) и не редактируются вручную.
   - Документ корневых метаданных (doc.go или main.go) содержит только верхнеуровневые аннотации (title, version, security).
   - Make target `make swagger` выполняет: `swag fmt && swag init -g cmd/<service>/main.go -o internal/gen/swagger`.

3. Запрещено смешивать два источника правды. Если используется swag, YAML-спека не редактируется вручную (или удаляется). Если основной источник — YAML, swag запрещён.

4. Обязательные глобальные аннотации (пример в doc.go):

   ```go
   // Package app ...
   //
   // @title           Ledger Service API
   // @version         1.3.0
   // @description     Идемпотентные операции учёта. Консистентные балансы.
   // @contact.name    Core Platform
   // @contact.email   core-platform@example.com
   // @BasePath        /
   // @schemes         http https
   // @securityDefinitions.apikey ApiKeyAuth
   // @in              header
   // @name            Authorization
   package app
   ```

5. Обязательные аннотации для каждого публичного HTTP хендлера (порядок фиксирован):

   - @Summary (кратко, ≤ 12 слов, глагол в активном залоге)
   - @Description (не дублирует Summary; бизнес-условия, идемпотентность, ограничение rate)
   - @Tags (группы = доменные подсекции; 1–2 тега)
   - @Accept / @Produce (явно: json)
   - @Param (каждый вход: path, query, header, body; body ровно один)
   - @Success (все успешные коды; не использовать “default”)
   - @Failure (все значимые ошибки ≥ 400)
   - @Security (если эндпойнт аутентифицирован)
   - @Router (HTTP метод + путь)
   - Запрещены пустые @Description, дублирующие Summary.

6. Пример над функцией:

   ```go
   // @Summary      Create ledger entry
   // @Description  Идемпотентное создание проводки. Повтор по тем же (external_id, account_id) возвращает 200 с прежним объектом.
   // @Tags         ledger
   // @Accept       json
   // @Produce      json
   // @Param        input  body      CreateEntryRequest  true  "Payload"
   // @Success      201    {object}  EntryResponse
   // @Failure      400    {object}  ErrorResponse   "Валидация"
   // @Failure      409    {object}  ErrorResponse   "Конфликт идемпотентности"
   // @Failure      500    {object}  ErrorResponse
   // @Security     ApiKeyAuth
   // @Router       /v1/entries [post]
   func (h *Handler) createEntry(w http.ResponseWriter, r *http.Request) { ... }
   ```

7. Структуры запросов/ответов:

   - Поля документируются через @Description в теге json при необходимости уточнения — иначе не плодить лишнее.

   - Nullable поля — pointer типы или sql.NullX / *time.Time. Не использовать “пустое значение” как “отсутствие”.

   - Пример:

     ```go
     type CreateEntryRequest struct {
     	AccountID        string          `json:"account_id" example:"acc_123"`
     	ExternalID       string          `json:"external_id" example:"ext_789"`
     	AmountMinorUnits int64           `json:"amount_minor_units" example:"1500"`
     	Currency         string          `json:"currency" example:"RUB"`
     	Metadata         json.RawMessage `json:"metadata" example:"{\"key\":\"value\"}"` // Опционально
     }
     ```

8. Error модель унифицирована. Все ошибки возвращают:

   ```go
   type ErrorResponse struct {
     Code    string `json:"code" example:"VALIDATION_FAILED"`
     Message string `json:"message" example:"amount_minor_units must be > 0"`
     TraceID string `json:"trace_id" example:"4f9d3c2a..."` // Корреляция
   }
   ```

   - Никаких разнородных форматов ошибок.

9. Аннотации обновляются в том же PR, что меняет поведение эндпойнта. Запрещены “догоняющие” PR.

10. В CI:

    - После `make swagger` git status должен быть чистым.
    - Валидировать итоговую спецификацию (swagger validate или spectral lint) — target `make swagger-verify`.

11. Теги @Deprecated обязательны для устаревших эндпойнтов + README.md (секция API Changes) фиксирует срок удаления.

12. Запрещено:

    - Автоматически генерировать 200 и 500 без явной декларации.
    - Использовать generic обёртки {object} interface{} — только конкретные типы.
    - Избыточные “default” ответные секции.

13. Если эндпойнт идемпотентен — описание содержит:

    - Ключ идемпотентности
    - Семантику повторного вызова
    - Граничные состояния (timeout после записи, частичная фиксация)

14. Стабильность версий:

    - /v1/ путь фиксирует контракт; ввод /v2/ только при несовместимых изменениях.
    - Мелкие расширения (добавление необязательного поля) — без изменения версии.
    - Удаление или изменение типов полей → новая версия.

15. Принудительная унификация типов дат/времени: RFC3339 (time.RFC3339). В аннотациях пример: 2025-08-20T10:15:30Z.

16. Политика enum:

    - Перечисления документируются: @Description у поля + список допустимых значений.

    - Пример:

      ```go
      type EntryStatus string
      const (
      	EntryStatusPending  EntryStatus = "PENDING"
      	EntryStatusPosted   EntryStatus = "POSTED"
      	EntryStatusRejected EntryStatus = "REJECTED"
      )
      ```

      В спецификации эти значения отражены (swag подтянет из example или через отдельную ручную правку при необходимости).

17. Security:

    - Все защищённые эндпойнты содержат @Security ApiKeyAuth (или другой механизм).
    - Несекьюрные (health) аннотацию @Security не содержат.

18. Генерация клиента (если включена) — только из итоговой openapi.json после swag init, не из промежуточных черновиков.

19. Любое несоответствие runtime → спецификация классифицируется как контрактный дефект Severity=High.

# 39. README.md

1. README.md в корне обязателен и обновляется при любом изменении архитектуры, конфигурации запуска или интерфейсов.

2. Структура (минимум):

   - Назначение (1–2 абзаца)
   - Архитектурная схема (кратко + ссылка на подробный DESIGN.md при наличии)
   - Быстрый старт (зависимости, make deps, make build, make run)
   - Структура директорий (обновлять при изменениях)
   - Конфигурация (переменные окружения / файлы)
   - Команды Makefile (ключевые)
   - API (ссылка на api/openapi/.../openapi.yaml)
   - Тестирование (unit / integration / performance запуск)
   - Observability (метрики, health endpoints)
   - Версионирование и релизный процесс

3. Никаких устаревших инструкций. Удалять нерелевантное немедленно.

4. Пример минимального каркаса:

   ````markdown
   # Ledger Service
   
   Purpose: финализированный учёт транзакций с идемпотентностью и консистентными балансами.
   
   ## Quick Start
   make deps
   make build
   make run
   
   ## Directory Layout
   cmd/...
   internal/...
   api/openapi/...
   tests/...
   
   ## Configuration
   ENV:
     LEDGER_DB_DSN
     LEDGER_HTTP_ADDR
     LEDGER_LOG_LEVEL
   
   ## API
   OpenAPI: api/openapi/ledger/openapi.yaml
   
   ## Testing
   Unit: make test-unit
   Integration: make test-integration
   Performance: tests/performance scenario scripts
   
   ## Observability
   Metrics: /metrics
   Liveness: /live
   Readiness: /ready
   
   ## Release
   Tag (semver) -> CI build -> image publish
   ````

5. README.md — точка входа для новых разработчиков и аудиторов; отсутствие актуальности = дефект.
