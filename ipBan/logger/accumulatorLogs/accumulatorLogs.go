package accumulatorLogs

import (
	"bufio"
	"fmt"
	ipban "ipBanSystem/ipBan/BanService"
	"log"
	"os"
	"sync"
	"time"
)

// LogAccumulator накапливает строки из access.log в отдельный файл
type LogAccumulator struct {
	SourcePath      string // Путь к исходному access.log
	AccumulatedPath string // Путь к файлу накопленных логов
	LastReadPos     int64  // Позиция последнего прочитанного байта
	Running         bool   // Запущен ли сервис
	StopChan        chan bool
	mutex           sync.Mutex // Мьютекс для синхронизации доступа к LastReadPos, предотвращающий гонки
}

// NewLogAccumulator создает новый накопитель логов
func NewLogAccumulator(sourcePath, accumulatedPath string) *LogAccumulator {
	return &LogAccumulator{
		SourcePath:      sourcePath,
		AccumulatedPath: accumulatedPath,
		LastReadPos:     0,
		Running:         false,
		StopChan:        make(chan bool, 1),
	}
}

// Start запускает сервис накопления логов
func (la *LogAccumulator) Start() error {
	if la.Running {
		return fmt.Errorf("сервис накопления логов уже запущен")
	}

	la.Running = true
	log.Printf("LOG_ACCUMULATOR: Запуск сервиса накопления логов")
	log.Printf("LOG_ACCUMULATOR: Исходный файл: %s", la.SourcePath)
	log.Printf("LOG_ACCUMULATOR: Файл накопления: %s", la.AccumulatedPath)
	log.Printf("LOG_ACCUMULATOR: Интервал сохранения: %d минут", ipban.IP_SAVE_INTERVAL)

	// Восстанавливаем позицию чтения из файла состояния
	la.restorePosition()

	go la.accumulationLoop()
	return nil
}

// Stop останавливает сервис накопления логов
func (la *LogAccumulator) Stop() {
	if !la.Running {
		return
	}

	log.Printf("LOG_ACCUMULATOR: Остановка сервиса накопления логов")
	la.Running = false
	la.StopChan <- true
}

// accumulationLoop основной цикл накопления логов
func (la *LogAccumulator) accumulationLoop() {
	ticker := time.NewTicker(time.Duration(ipban.IP_SAVE_INTERVAL) * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			la.AccumulateNewLines()
		case <-la.StopChan:
			log.Printf("LOG_ACCUMULATOR: Сервис остановлен")
			return
		}
	}
}

// AccumulateNewLines читает новые строки из access.log и добавляет их в файл накопления
// Использует mutex для безопасного доступа к переменной LastReadPos, предотвращая гонки
func (la *LogAccumulator) AccumulateNewLines() {
	log.Printf("LOG_ACCUMULATOR: Начало накопления новых строк")

	// Открываем исходный файл
	sourceFile, err := os.Open(la.SourcePath)
	if err != nil {
		log.Printf("LOG_ACCUMULATOR: Ошибка открытия исходного файла %s: %v", la.SourcePath, err)
		return
	}
	defer sourceFile.Close()

	// Получаем размер файла
	fileInfo, err := sourceFile.Stat()
	if err != nil {
		log.Printf("LOG_ACCUMULATOR: Ошибка получения информации о файле: %v", err)
		return
	}

	// Если файл стал меньше (ротация лога), сбрасываем позицию
	la.mutex.Lock()
	currentLastReadPos := la.LastReadPos
	if fileInfo.Size() < currentLastReadPos {
		log.Printf("LOG_ACCUMULATOR: Обнаружена ротация лога, сбрасываем позицию чтения")
		la.LastReadPos = 0
		currentLastReadPos = 0 // update the local copy too
	}

	// Если нет новых данных, выходим
	if currentLastReadPos >= fileInfo.Size() {
		log.Printf("LOG_ACCUMULATOR: Нет новых данных для накопления")
		la.mutex.Unlock()
		return
	}

	// Переходим к позиции последнего прочитанного байта
	_, err = sourceFile.Seek(currentLastReadPos, 0)
	if err != nil {
		log.Printf("LOG_ACCUMULATOR: Ошибка позиционирования в файле: %v", err)
		la.mutex.Unlock()
		return
	}

	// Открываем файл накопления для записи
	accumulatedFile, err := os.OpenFile(la.AccumulatedPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Printf("LOG_ACCUMULATOR: Ошибка открытия файла накопления %s: %v", la.AccumulatedPath, err)
		la.mutex.Unlock()
		return
	}
	defer accumulatedFile.Close()

	// Читаем новые строки и записываем их
	scanner := bufio.NewScanner(sourceFile)
	linesCount := 0

	for scanner.Scan() {
		line := scanner.Text()

		// Пропускаем пустые строки
		if len(line) == 0 {
			continue
		}

		// Записываем строку в файл накопления
		_, err = accumulatedFile.WriteString(line + "\n")
		if err != nil {
			log.Printf("LOG_ACCUMULATOR: Ошибка записи строки в файл накопления: %v", err)
			continue
		}

		linesCount++
	}

	// Обновляем позицию чтения
	currentPos, err := sourceFile.Seek(0, 1) // Получаем текущую позицию
	if err == nil {
		la.LastReadPos = currentPos
	}
	la.mutex.Unlock()

	// Сохраняем позицию вне блокировки, чтобы избежать взаимной блокировки
	if err == nil {
		la.savePosition()
	}

	log.Printf("LOG_ACCUMULATOR: Накоплено %d новых строк, позиция: %d", linesCount, currentPos)

	if err := scanner.Err(); err != nil {
		log.Printf("LOG_ACCUMULATOR: Ошибка чтения исходного файла: %v", err)
	}
}

// cleanupOldLines очищает старые строки из файла накопления
func (la *LogAccumulator) cleanupOldLines() {
	if ipban.IP_COUNTER_RETENTION <= 0 {
		return // Если время хранения = 0, данные хранятся бесконечно
	}

	log.Printf("LOG_ACCUMULATOR: Начало очистки старых строк (старше %d минут)", ipban.IP_COUNTER_RETENTION)

	// Проверяем существование файла накопления
	if _, err := os.Stat(la.AccumulatedPath); os.IsNotExist(err) {
		log.Printf("LOG_ACCUMULATOR: Файл накопления не существует, пропускаем очистку")
		return
	}

	// Открываем файл накопления для чтения
	file, err := os.Open(la.AccumulatedPath)
	if err != nil {
		log.Printf("LOG_ACCUMULATOR: Ошибка открытия файла накопления для очистки: %v", err)
		return
	}
	defer file.Close()

	// Создаем временный файл для записи актуальных строк
	tempFile, err := os.Create(la.AccumulatedPath + ".tmp")
	if err != nil {
		log.Printf("LOG_ACCUMULATOR: Ошибка создания временного файла: %v", err)
		return
	}
	defer tempFile.Close()

	cutoffTime := time.Now().Add(-time.Duration(ipban.IP_COUNTER_RETENTION) * time.Minute)
	scanner := bufio.NewScanner(file)
	keptLines := 0
	removedLines := 0

	for scanner.Scan() {
		line := scanner.Text()

		// Пытаемся извлечь время из строки
		if timestamp, err := la.extractTimestamp(line); err == nil {
			if timestamp.After(cutoffTime) {
				// Строка актуальна, сохраняем её
				tempFile.WriteString(line + "\n")
				keptLines++
			} else {
				// Строка устарела, пропускаем её
				removedLines++
			}
		} else {
			// Если не удалось извлечь время, сохраняем строку (на всякий случай)
			tempFile.WriteString(line + "\n")
			keptLines++
		}
	}

	// Закрываем файлы
	file.Close()
	tempFile.Close()

	// Заменяем оригинальный файл временным
	if err := os.Rename(la.AccumulatedPath+".tmp", la.AccumulatedPath); err != nil {
		log.Printf("LOG_ACCUMULATOR: Ошибка замены файла: %v", err)
		return
	}

	log.Printf("LOG_ACCUMULATOR: Очистка завершена: сохранено %d строк, удалено %d строк", keptLines, removedLines)
}

// extractTimestamp извлекает время из строки лога
func (la *LogAccumulator) extractTimestamp(line string) (time.Time, error) {
	// Простое извлечение времени из начала строки
	// Формат: 2025/09/04 10:17:03.008517
	if len(line) < 26 {
		return time.Time{}, fmt.Errorf("строка слишком короткая")
	}

	timestampStr := line[:26] // Берем первые 26 символов
	return time.Parse("2006/01/02 15:04:05.000000", timestampStr)
}

// savePosition сохраняет текущую позицию чтения
// Использует mutex для безопасного доступа к переменной LastReadPos, предотвращая гонки
func (la *LogAccumulator) savePosition() {
	la.mutex.Lock()
	currentPos := la.LastReadPos
	la.mutex.Unlock()
	
	posFile := la.AccumulatedPath + ".pos"
	file, err := os.Create(posFile)
	if err != nil {
		log.Printf("LOG_ACCUMULATOR: Ошибка сохранения позиции: %v", err)
		return
	}
	defer file.Close()

	fmt.Fprintf(file, "%d", currentPos)
}

// restorePosition восстанавливает позицию чтения
// Использует mutex для безопасного доступа к переменной LastReadPos, предотвращая гонки
func (la *LogAccumulator) restorePosition() {
	posFile := la.AccumulatedPath + ".pos"
	file, err := os.Open(posFile)
	if err != nil {
		log.Printf("LOG_ACCUMULATOR: Позиция не найдена, начинаем с начала файла")
		la.mutex.Lock()
		la.LastReadPos = 0
		la.mutex.Unlock()
		return
	}
	defer file.Close()

	var pos int64
	_, err = fmt.Fscanf(file, "%d", &pos)
	if err != nil {
		log.Printf("LOG_ACCUMULATOR: Ошибка чтения позиции: %v", err)
		la.mutex.Lock()
		la.LastReadPos = 0
		la.mutex.Unlock()
		return
	}

	la.mutex.Lock()
	la.LastReadPos = pos
	la.mutex.Unlock()
	log.Printf("LOG_ACCUMULATOR: Восстановлена позиция чтения: %d", pos)
}

// StartCleanupService запускает сервис очистки старых строк
func (la *LogAccumulator) StartCleanupService() {
	go func() {
		// Ждем 1 час перед первой очисткой, чтобы файл успел накопиться
		time.Sleep(1 * time.Hour)

		// Очищаем старые строки
		la.cleanupOldLines()

		// Затем каждые IP_CLEANUP_INTERVAL часов
		ticker := time.NewTicker(time.Duration(ipban.IP_CLEANUP_INTERVAL) * time.Hour)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				la.cleanupOldLines()
			case <-la.StopChan:
				return
			}
		}
	}()
}
