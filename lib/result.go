package lib

import (
	"net"
	"time"
)

// Состояние проверки TCP-порта
type State string

const (
	StateOpen        State = "open"        // Успешное подключение
	StateClosed      State = "closed"      // Явный отказ в подключении
	StateTimeout     State = "timeout"     // Превышение времени ожидания подключения
	StateUnreachable State = "unreachable" // Недоступность узла или сети
	StateCanceled    State = "canceled"    // Отмена через контекст
	StateError       State = "error"       // Прочая ошибка проверки
)

// Результат проверки TCP-порта
type Result struct {
	Host     string
	IP       net.IP
	Port     uint16
	State    State
	Duration time.Duration
	Err      error
}
