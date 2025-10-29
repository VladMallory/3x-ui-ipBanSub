package common

import (
	"fmt"
	"os/exec"
	"strings"
)

// IPTablesManager управляет блокировкой IP через iptables
type IPTablesManager struct {
	BlockedIPs map[string]bool // Карта заблокированных IP
}

// NewIPTablesManager создает новый менеджер iptables
func NewIPTablesManager() *IPTablesManager {
	return &IPTablesManager{
		BlockedIPs: make(map[string]bool),
	}
}

// BlockIP блокирует IP адрес через iptables
func (i *IPTablesManager) BlockIP(ipAddress string) error {
	// Проверяем, не заблокирован ли уже IP
	if i.BlockedIPs[ipAddress] {
		fmt.Printf("ℹ️  IP %s уже заблокирован\n", ipAddress)
		// Логируем в bot.log: IP уже заблокирован
		LogIPBanInfo("IP %s уже заблокирован через iptables", ipAddress)
		return nil
	}

	// Логируем в bot.log: начало блокировки IP
	LogIPBanInfo("Блокировка IP %s через iptables", ipAddress)

	// Блокируем IP через iptables
	cmd := fmt.Sprintf("iptables -I INPUT -s %s -j DROP", ipAddress)
	if err := i.executeCommand(cmd); err != nil {
		// Логируем в bot.log: ошибка блокировки IP
		LogIPBanError("Ошибка блокировки IP %s через iptables: %v", ipAddress, err)
		return fmt.Errorf("ошибка блокировки IP %s: %v", ipAddress, err)
	}

	// Добавляем IP в список заблокированных
	i.BlockedIPs[ipAddress] = true
	fmt.Printf("✅ IP %s успешно заблокирован через iptables\n", ipAddress)

	// Логируем в bot.log: успешная блокировка IP
	LogIPBanAction("IP_ЗАБЛОКИРОВАН", ipAddress, 0, []string{})

	return nil
}

// UnblockIP разблокирует IP адрес через iptables
func (i *IPTablesManager) UnblockIP(ipAddress string) error {
	// Проверяем, заблокирован ли IP
	if !i.BlockedIPs[ipAddress] {
		fmt.Printf("ℹ️  IP %s не был заблокирован\n", ipAddress)
		// Логируем в bot.log: IP не был заблокирован
		LogIPBanInfo("IP %s не был заблокирован через iptables", ipAddress)
		return nil
	}

	// Логируем в bot.log: начало разблокировки IP
	LogIPBanInfo("Разблокировка IP %s через iptables", ipAddress)

	// Разблокируем IP через iptables
	cmd := fmt.Sprintf("iptables -D INPUT -s %s -j DROP", ipAddress)
	if err := i.executeCommand(cmd); err != nil {
		// Логируем в bot.log: ошибка разблокировки IP
		LogIPBanError("Ошибка разблокировки IP %s через iptables: %v", ipAddress, err)
		return fmt.Errorf("ошибка разблокировки IP %s: %v", ipAddress, err)
	}

	// Удаляем IP из списка заблокированных
	delete(i.BlockedIPs, ipAddress)
	fmt.Printf("✅ IP %s успешно разблокирован через iptables\n", ipAddress)

	// Логируем в bot.log: успешная разблокировка IP
	LogIPBanAction("IP_РАЗБЛОКИРОВАН", ipAddress, 0, []string{})

	return nil
}

// executeCommand выполняет команду в системе
func (i *IPTablesManager) executeCommand(cmd string) error {
	// Используем os/exec для выполнения команды
	parts := strings.Fields(cmd)
	if len(parts) < 2 {
		return fmt.Errorf("неверная команда: %s", cmd)
	}

	// Выполняем команду
	execCmd := exec.Command(parts[0], parts[1:]...)
	output, err := execCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ошибка выполнения команды '%s': %v, output: %s", cmd, err, string(output))
	}

	return nil
}

// GetBlockedIPs возвращает список заблокированных IP
func (i *IPTablesManager) GetBlockedIPs() []string {
	var ips []string
	for ip := range i.BlockedIPs {
		ips = append(ips, ip)
	}
	return ips
}

// IsIPBlocked проверяет, заблокирован ли IP
func (i *IPTablesManager) IsIPBlocked(ipAddress string) bool {
	return i.BlockedIPs[ipAddress]
}
