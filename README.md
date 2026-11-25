# portscan

Библиотека на Go для конкурентного TCP Connect-сканирования портов по одному или нескольким хостам. Пакет спроектирован для встраивания в другие Go-приложения и не зависит от CLI, HTTP-фреймворков, баз данных или брокеров сообщений.

## Структура репозитория

- `lib/` — основной пакет (`package lib`, путь импорта `github.com/mtkmkv/portscan/lib`), содержащий логику работы с целями, спецификациями портов, TCP-зондирования, классификации ошибок, пула воркеров, сканера и модульные тесты.
- `README.md` — документация по использованию и устройству пакета.

## Как использовать пакет

Импортируйте пакет `github.com/mtkmkv/portscan/lib`, создайте `Scanner` с нужными опциями и вызовите `Scan`:

```go
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/mtkmkv/portscan/lib"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	scanner, err := lib.New(
		lib.WithConcurrency(100),
		lib.WithConnectTimeout(500*time.Millisecond),
	)
	if err != nil {
		log.Fatal(err)
	}

	results, err := scanner.Scan(
		ctx,
		[]string{"127.0.0.1", "localhost"},
		lib.Combine(
			lib.Ports(80, 443),
			lib.Range(8000, 8010),
		),
	)
	if err != nil {
		log.Fatal(err)
	}

	for res := range results {
		fmt.Printf(
			"%s (%v):%d -> %s (duration=%s, err=%v)\n",
			res.Host,
			res.IP,
			res.Port,
			res.State,
			res.Duration,
			res.Err,
		)
	}
}
```

### Поведение при ошибках и отмене контекста

- Ошибки валидации параметров конструктора (`New`) и входных аргументов `Scan` (пустой список хостов, некорректный адрес, пустая спецификация портов, порт `0`, обратный диапазон) возвращаются сразу в виде ошибок.

- Если DNS-имя не удалось разрешить, сканирование остальных целей не прерывается: для такого хоста возвращаются результаты со состоянием `StateError` и завёрнутой ошибкой разрешения (`ErrResolveFailure`).

- При отмене `context.Context` прекращается постановка новых задач, дожидаются уже запущенные проверки, и канал результатов гарантированно закрывается, в том числе при медленном чтении результатов вызывающим кодом.

- Экземпляр `Scanner` не хранит изменяемого состояния между вызовами и безопасен для повторного и конкурентного использования.

## Как устроен публичный API

- **Конструктор и опции:**
  - `New(opts ...Option) (*Scanner, error)` — создаёт экземпляр сканера без глобального изменяемого состояния.
  - `WithConcurrency(n int) Option` — задаёт максимальное число одновременно выполняемых проверок (`n > 0`).
  - `WithConnectTimeout(d time.Duration) Option` — задаёт таймаут TCP-подключения (`d > 0`).
- **Спецификации портов (`uint16`, диапазон `1..65535`):**
  - `Ports(ports ...uint16) PortSpec` — явный список портов (с валидацией, сортировкой, дедупликацией и изоляцией срезов).
  - `Range(start, end uint16) PortSpec` — инклюзивный диапазон портов (`start > 0`, `start <= end`).
  - `Combine(specs ...PortSpec) PortSpec` — объединяет несколько спецификаций и устраняет дубликаты.
- **Запуск и результаты:**
  - `(s *Scanner) Scan(ctx context.Context, hosts []string, specs ...PortSpec) (<-chan Result, error)` — разрешает цели, запускает конкурентные проверки и возвращает канал с результатами.
  - `Result` — содержит `Host string`, `IP net.IP`, `Port uint16`, `State State`, `Duration time.Duration`, `Err error`.
  - `State` — типизированное состояние проверки: `StateOpen` (`open`), `StateClosed` (`closed`), `StateTimeout` (`timeout`), `StateUnreachable` (`unreachable`), `StateCanceled` (`canceled`), `StateError` (`error`).

Состояния определяются через типизированные ошибки и платформенные коды ошибок, без сравнения строк текста ошибок. Открытые TCP-соединения корректно закрываются после проверки.

## Сторонние зависимости

Основная логика реализована на стандартной библиотеке Go (`context`, `net`, `sync`, `time`, `errors`).

Единственная внешняя зависимость — **`golang.org/x/sys/windows`**. Она используется в `lib/dialer_windows.go` (с build-тегом `windows`) для проверки кодов ошибок Winsock (`windows.WSAECONNREFUSED`, `windows.WSAENETUNREACH`, `windows.WSAEHOSTUNREACH` и др.) через `errors.Is`. В связке с `lib/dialer_unix.go` (для `!windows`) это обеспечивает надёжную кроссплатформенную классификацию сетевых ошибок без парсинга текста ошибок.

## Запуск тестов и проверок

```bash
go test -race -v -cover ./...
go vet ./...
GOOS=linux CGO_ENABLED=0 go build ./...
```
