package ipban

import (
	"fmt"
	"ipBanSystem/ipBan/logger/initLogs"
	"net"
	"os/exec"
	"sync"
)

// IPTablesManager управляет блокировкой IP через iptables
// mutex добавлен для предотвращения гонок при одновременном доступе к карте BlockedIPs
type IPTablesManager struct {
	BlockedIPs map[string]bool // Карта заблокированных IP
	mutex      sync.RWMutex    // Мьютекс для синхронизации доступа к карте BlockedIPs
}

// NewIPTablesManager создает новый менеджер iptables
func NewIPTablesManager() *IPTablesManager {
	return &IPTablesManager{
		BlockedIPs: make(map[string]bool),
	}
}

// validateIP проверяет, является ли строка действительным IP-адресом
// Добавлено для предотвращения командной инъекции через iptables
func validateIP(ipAddress string) bool {
	parsed := net.ParseIP(ipAddress)
	return parsed != nil
}

// BlockIP блокирует IP адрес через iptables
// - Проверяет действительность IP-адреса для предотвращения командной инъекции
// - Использует RWMutex для безопасного доступа к карте BlockedIPs
func (i *IPTablesManager) BlockIP(ipAddress string) error {
	// Проверяем, является ли IP действительным
	if !validateIP(ipAddress) {
		return fmt.Errorf("недействительный IP-адрес: %s", ipAddress)
	}

	// Проверяем, не заблокирован ли уже IP
	// Используем RLock для безопасного чтения из общей карты
	i.mutex.RLock()
	alreadyBlocked := i.BlockedIPs[ipAddress]
	i.mutex.RUnlock()

	if alreadyBlocked {
		fmt.Printf("ℹ️  IP %s уже заблокирован\n", ipAddress)
		// Логируем в bot.log: IP уже заблокирован
		initLogs.LogIPBanInfo("IP %s уже заблокирован через iptables", ipAddress)
		return nil
	}

	// Логируем в bot.log: начало блокировки IP
	initLogs.LogIPBanInfo("Блокировка IP %s через iptables", ipAddress)

	// Блокируем IP через iptables (используем безопасное выполнение команды)
	// Вместо fmt.Sprintf команды используем exec.Command с отдельными аргументами
	// для предотвращения командной инъекции
	cmd := exec.Command("iptables", "-I", "INPUT", "-s", ipAddress, "-j", "DROP")
	if err := cmd.Run(); err != nil {
		// Логируем в bot.log: ошибка блокировки IP
		initLogs.LogIPBanError("Ошибка блокировки IP %s через iptables: %v", ipAddress, err)
		return fmt.Errorf("ошибка блокировки IP %s: %v", ipAddress, err)
	}

	// Добавляем IP в список заблокированных
	// Используем Lock для безопасной модификации общей карты
	i.mutex.Lock()
	i.BlockedIPs[ipAddress] = true
	i.mutex.Unlock()

	fmt.Printf("✅ IP %s успешно заблокирован через iptables\n", ipAddress)

	// Логируем в bot.log: успешная блокировка IP
	initLogs.LogIPBanAction("IP_ЗАБЛОКИРОВАН", ipAddress, 0, []string{})

	return nil
}

// UnblockIP разблокирует IP адрес через iptables
// - Проверяет действительность IP-адреса для предотвращения командной инъекции
// - Использует RWMutex для безопасного доступа к карте BlockedIPs
func (i *IPTablesManager) UnblockIP(ipAddress string) error {
	// Проверяем, является ли IP действительным
	if !validateIP(ipAddress) {
		return fmt.Errorf("недействительный IP-адрес: %s", ipAddress)
	}

	// Проверяем, заблокирован ли IP
	// Используем RLock для безопасного чтения из общей карты
	i.mutex.RLock()
	isBlocked := i.BlockedIPs[ipAddress]
	i.mutex.RUnlock()

	if !isBlocked {
		fmt.Printf("ℹ️  IP %s не был заблокирован\n", ipAddress)
		// Логируем в bot.log: IP не был заблокирован
		initLogs.LogIPBanInfo("IP %s не был заблокирован через iptables", ipAddress)
		return nil
	}

	// Логируем в bot.log: начало разблокировки IP
	initLogs.LogIPBanInfo("Разблокировка IP %s через iptables", ipAddress)

	// Разблокируем IP через iptables (используем безопасное выполнение команды)
	// Вместо fmt.Sprintf команды используем exec.Command с отдельными аргументами
	// для предотвращения командной инъекции
	cmd := exec.Command("iptables", "-D", "INPUT", "-s", ipAddress, "-j", "DROP")
	if err := cmd.Run(); err != nil {
		// Логируем в bot.log: ошибка разблокировки IP
		initLogs.LogIPBanError("Ошибка разблокировки IP %s через iptables: %v", ipAddress, err)
		return fmt.Errorf("ошибка разблокировки IP %s: %v", ipAddress, err)
	}

	// Удаляем IP из списка заблокированных
	// Используем Lock для безопасной модификации общей карты
	i.mutex.Lock()
	delete(i.BlockedIPs, ipAddress)
	i.mutex.Unlock()

	fmt.Printf("✅ IP %s успешно разблокирован через iptables\n", ipAddress)

	// Логируем в bot.log: успешная разблокировка IP
	initLogs.LogIPBanAction("IP_РАЗБЛОКИРОВАН", ipAddress, 0, []string{})

	return nil
}

// GetBlockedIPs возвращает список заблокированных IP
// Использует RWMutex для безопасного чтения из общей карты BlockedIPs
func (i *IPTablesManager) GetBlockedIPs() []string {
	i.mutex.RLock()
	defer i.mutex.RUnlock()

	var ips []string
	for ip := range i.BlockedIPs {
		ips = append(ips, ip)
	}
	return ips
}

// IsIPBlocked проверяет, заблокирован ли IP
// Использует RWMutex для безопасного чтения из общей карты BlockedIPs
func (i *IPTablesManager) IsIPBlocked(ipAddress string) bool {
	i.mutex.RLock()
	defer i.mutex.RUnlock()

	return i.BlockedIPs[ipAddress]
}
