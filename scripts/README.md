# Migration Scripts

## Миграция из логов в SQLite

Скрипт `migrate_from_logs.go` восстанавливает данные из логов старой версии бота в новую SQLite базу.

### Использование

```bash
# Локальная миграция
go run scripts/migrate_from_logs.go -logs=old_bot.log -db=./notifier.db

# Миграция в Docker контейнер
docker cp old_bot.log moto-gorod-notifier:/tmp/
docker exec moto-gorod-notifier /home/notifier/notifier -migrate -logs=/tmp/old_bot.log
```

### Что извлекается из логов

**Подписчики:**
- Ищет записи: `"message": "User subscribed"`
- Извлекает: `"chat_id": 123456789`

**Отправленные слоты:**
- Ищет записи: `"message": "New slot found"`
- Извлекает: `"service_id": 15728488, "staff_id": 2311362, "time": "2025-08-30T11:00:00+03:00"`

### Форматы логов

Поддерживает:
- **JSON логи** (структурированные)
- **Текстовые логи** (regex парсинг)

### Пример команды

```bash
# Получить логи из Docker
make docker-logs > old_bot.log

# Запустить миграцию
go run scripts/migrate_from_logs.go -logs=old_bot.log -db=/tmp/notifier.db

# Скопировать базу в контейнер
docker cp /tmp/notifier.db moto-gorod-notifier:/data/
```