# Moto Gorod Slot Notifier

🚗 Telegram-бот для автоматического уведомления о появлении свободных слотов для записи в автошколе "Мото Город".

## Возможности

- 🔍 **Автоматический мониторинг** - отслеживает появление новых слотов каждую минуту
- 📱 **Telegram уведомления** - мгновенные сообщения о доступных окнах
- 🎹 **Reply клавиатура** - удобное управление через постоянные кнопки
- 📅 **Детальная информация** - дата, время, день недели, сотрудник
- 🔗 **Прямая ссылка** - кнопка для перехода к записи
- 💾 **SQLite база данных** - персистентное хранение данных
- 🔄 **Автообновление интерфейса** - бесшовные обновления для пользователей
- 🐳 **Docker поддержка** - простое развертывание

## Быстрый старт

### Локальный запуск

```bash
# Клонирование репозитория
git clone <repository-url>
cd moto_gorod-notifier

# Настройка окружения
cp .env.example .env
# Отредактируйте .env файл с вашими данными

# Сборка и запуск
make build
make run
```

### Docker запуск

```bash
# Сборка и запуск контейнера
make docker-run

# Запуск с DEBUG логированием
make docker-run LOG_LEVEL=DEBUG

# Просмотр логов
make docker-logs

# Остановка
make docker-stop
```

## Конфигурация

Создайте `.env` файл на основе `.env.example`:

```env
# Telegram Bot
TELEGRAM_TOKEN=your_telegram_bot_token

# YCLIENTS API
YCLIENTS_LOGIN=your_email@example.com
YCLIENTS_PASSWORD=your_password
YCLIENTS_PARTNER_TOKEN=your_partner_token
YCLIENTS_COMPANY_ID=780413
YCLIENTS_SERVICE_IDS=15728488
YCLIENTS_FORM_ID=n841217

# Настройки
TIMEZONE=Europe/Moscow
CHECK_INTERVAL_SECONDS=60
LOG_LEVEL=INFO
```

## Архитектура

```
cmd/notifier/          # Точка входа приложения
internal/
├── bot/              # Telegram бот с reply клавиатурой
├── config/           # Загрузка конфигурации
├── logger/           # Структурированное логирование
├── notifier/         # Основная логика мониторинга
│   └── templates/    # Шаблоны сообщений
├── storage/          # SQLite база данных
└── yclients/         # Клиент для YCLIENTS API
data/                 # Персистентные данные (SQLite)
scripts/              # Утилиты миграции
```

## База данных

Проект использует SQLite для персистентного хранения:

- **subscribers** - подписанные пользователи
- **seen_slots** - история отправленных слотов (дедупликация)

### Миграция из старых логов

```bash
# Миграция данных из логов старой версии
make migrate-logs LOGS=old_bot.log

# Указать путь к базе данных
make migrate-logs LOGS=old_bot.log DB=./custom.db
```

## Команды Makefile

### Разработка
- `make build` - сборка бинарного файла
- `make run` - локальный запуск
- `make test` - запуск тестов
- `make fmt` - форматирование кода
- `make vet` - статический анализ

### Docker
- `make docker-run` - запуск в контейнере
- `make docker-stop` - остановка контейнера
- `make docker-logs` - просмотр логов
- `make docker-clean` - очистка ресурсов

### Миграция
- `make migrate-logs LOGS=file.log` - миграция из логов

### Тестирование
- `make test-shutdown` - тест graceful shutdown
- `make help` - справка по командам

## Права доступа

При использовании Docker на Linux может потребоваться настройка прав:

```bash
# Автоматически устанавливается при make docker-run
chmod 777 ./data/
chmod 666 ./data/notifier.db  # если файл уже существует
```

## Технологии

- **Go 1.22** - основной язык
- **SQLite** - база данных
- **Telegram Bot API** - интеграция с Telegram
- **YCLIENTS API** - получение данных о слотах
- **Docker** - контейнеризация
- **Alpine Linux** - базовый образ

## Требования

- Go 1.22+
- Docker (для контейнерного запуска)
- Telegram Bot Token
- Учетные данные YCLIENTS

## Лицензия

MIT License

## Поддержка

Для вопросов и предложений создавайте Issues в репозитории.