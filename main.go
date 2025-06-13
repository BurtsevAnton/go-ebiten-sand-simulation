package main

import (
	"fmt"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"image/color"
	"log"
	"math/rand"
	"time"
)

const (
	WIDTH  = 400
	HEIGHT = 300
	CELL   = 3
)

type CellType int
type Status int

const (
	EMPTY       CellType = iota
	SAND_TYPE_0          // Текучесть 0 - очень грубый песок
	SAND_TYPE_1          // Текучесть 1 - грубый песок
	SAND_TYPE_2          // Текучесть 2 - обычный песок
	SAND_TYPE_3          // Текучесть 3 - мелкий песок
	SAND_TYPE_4          // Текучесть 4 - очень мелкий песок
	SAND_TYPE_5          // Текучесть 5 - пудровый песок
	STONE
)

const (
	Idle Status = iota
	Falling
	Rolling
	PendingRoll
	Fixed
	Settled
)

type Cell struct {
	Type         CellType
	Status       Status
	Fluidity     int
	RollCount    int
	LastRollDir  int
	StableFrames int // Счетчик кадров в стабильном состоянии
	LastChecked  int // Последняя итерация проверки
	FallCounter  int // Счетчик для задержки падения
}

type Pos struct {
	X, Y int
}

var (
	grid      [WIDTH][HEIGHT]Cell
	active    map[Pos]struct{}
	iteration = 0
)

// Конфигурация типов песка
var sandConfig = map[CellType]struct {
	fluidity   int
	rollChance float32
	fallDelay  int // Задержка перед следующим падением (в кадрах)
	color      color.RGBA
}{
	SAND_TYPE_0: {1, 0.8, 0, color.RGBA{139, 115, 85, 255}}, // Темные падают быстрее (без задержки)
	SAND_TYPE_1: {2, 0.9, 1, color.RGBA{160, 130, 98, 255}},
	SAND_TYPE_2: {2, 0.95, 1, color.RGBA{194, 154, 108, 255}},
	SAND_TYPE_3: {3, 0.98, 2, color.RGBA{218, 165, 32, 255}},
	SAND_TYPE_4: {4, 1.0, 3, color.RGBA{238, 203, 173, 255}},
	SAND_TYPE_5: {5, 1.0, 4, color.RGBA{255, 228, 196, 255}}, // Самые светлые падают медленнее
}

func main() {
	initGrid()
	ebiten.SetWindowSize(WIDTH*CELL, HEIGHT*CELL)
	ebiten.SetWindowTitle("Sand Simulation")
	if err := ebiten.RunGame(&Game{}); err != nil {
		log.Fatal(err)
	}
}

type Game struct{}

func (g *Game) Update() error {
	return update()
}

func (g *Game) Draw(screen *ebiten.Image) {
	screen.Fill(color.Black)

	for x := 0; x < WIDTH; x++ {
		for y := 0; y < HEIGHT; y++ {
			switch grid[x][y].Type {
			case SAND_TYPE_0, SAND_TYPE_1, SAND_TYPE_2, SAND_TYPE_3, SAND_TYPE_4, SAND_TYPE_5:
				if config, exists := sandConfig[grid[x][y].Type]; exists {
					clr := config.color
					// Немного затемняем settled частицы для визуализации
					if grid[x][y].Status == Settled {
						clr.R = uint8(float32(clr.R) * 0.9)
						clr.G = uint8(float32(clr.G) * 0.9)
						clr.B = uint8(float32(clr.B) * 0.9)
					}
					ebitenutil.DrawRect(screen, float64(x*CELL), float64(y*CELL), CELL, CELL, clr)
				}
			case STONE:
				ebitenutil.DrawRect(screen, float64(x*CELL), float64(y*CELL), CELL, CELL, color.RGBA{128, 128, 128, 255})
			}
		}
	}

	// Отладочная информация
	ebitenutil.DebugPrint(screen, fmt.Sprintf("TPS:%f", ebiten.ActualTPS()))
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("FPS:%f", ebiten.ActualFPS()), 0, 20)
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("Active: %d", len(active)), 0, 40)
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("Iteration: %d", iteration), 0, 60)
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return WIDTH * CELL, HEIGHT * CELL
}

func initGrid() {
	rand.Seed(time.Now().UnixNano())

	// Создаем каменные границы
	for x := 0; x < WIDTH; x++ {
		grid[x][100] = Cell{Type: STONE, Status: Fixed}      // Верхняя граница
		grid[x][HEIGHT-1] = Cell{Type: STONE, Status: Fixed} // Нижняя граница
	}

	// Создаем случайное количество наклонных границ (2-4)
	numSlopes := rand.Intn(3) + 6
	slopeSpacing := WIDTH / (numSlopes + 1)

	for i := 0; i < numSlopes; i++ {
		startX := i*slopeSpacing + rand.Intn(slopeSpacing/3)
		length := slopeSpacing - 10 + rand.Intn(20)

		// Определяем параметры наклона
		baseY := 160 + rand.Intn(60)    // Базовый уровень (60-90)
		angle := 0.0 + rand.Float64()*1 // Угол наклона (0.2-0.8)
		if rand.Intn(2) == 0 {
			angle = -angle // Случайное направление наклона
		}

		// Рисуем наклонную границу
		for dx := 0; dx < length; dx++ {
			x := startX + dx
			if x >= WIDTH {
				continue
			}

			y := baseY + int(float64(dx)*angle)

			grid[x][y] = Cell{Type: STONE, Status: Fixed}

			// Создаем двойные отверстия с вероятностью 15%
			if rand.Float32() < 0.15 && dx > 10 && dx < length-10 {
				holeY := 100
				grid[x][holeY] = Cell{Type: EMPTY, Status: Idle}
				grid[x+1][holeY] = Cell{Type: EMPTY, Status: Idle}
			}
		}
	}

	// Заполняем верхнюю часть песком
	for y := 0; y < 100; y++ {
		for x := 0; x < WIDTH; x++ {
			if grid[x][y].Type == EMPTY && rand.Float32() < 0.7 {
				sandTypes := []CellType{SAND_TYPE_0, SAND_TYPE_1, SAND_TYPE_2, SAND_TYPE_3, SAND_TYPE_4, SAND_TYPE_5}
				sandType := sandTypes[rand.Intn(len(sandTypes))]
				config := sandConfig[sandType]

				grid[x][y] = Cell{
					Type:     sandType,
					Status:   Idle,
					Fluidity: config.fluidity,
				}
			}
		}
	}

	// Инициализация активных клеток
	active = make(map[Pos]struct{})
	for y := HEIGHT - 2; y >= 0; y-- {
		for x := 0; x < WIDTH; x++ {
			if isSandType(grid[x][y].Type) {
				updateCellStatus(x, y, active)
			}
		}
	}
}

func update() error {
	iteration++
	newActive := make(map[Pos]struct{})

	for pos := range active {
		x, y := pos.X, pos.Y

		if !isSandType(grid[x][y].Type) {
			continue
		}

		switch grid[x][y].Status {
		case Falling:
			handleFalling(x, y, newActive)
		case Rolling:
			handleRolling(x, y, newActive)
		case PendingRoll:
			handlePendingRoll(x, y, newActive)
		case Idle:
			handleIdle(x, y, newActive)
		}
	}

	// Периодическая проверка settled частиц (гораздо реже)
	if iteration%60 == 0 {
		for y := HEIGHT - 2; y >= 0; y-- {
			for x := 0; x < WIDTH; x++ {
				if isSandType(grid[x][y].Type) && grid[x][y].Status == Settled {
					// Проверяем, изменилось ли окружение
					if hasEnvironmentChanged(x, y) {
						grid[x][y].Status = Idle
						grid[x][y].StableFrames = 0
						updateCellStatus(x, y, newActive)
					}
				}
			}
		}
	}

	active = newActive
	return nil
}

func handleIdle(x, y int, activeSet map[Pos]struct{}) {
	// Увеличиваем счетчик стабильных кадров
	grid[x][y].StableFrames++

	// Если частица стабильна достаточно долго, переводим в settled
	if grid[x][y].StableFrames > 60 {
		if rand.Float32() < 0.9 {
			grid[x][y].Status = Settled
		} else {
			grid[x][y].Status = Idle
		}
		return
	}

	// Проверяем, нужно ли активировать
	if shouldActivate(x, y) {
		grid[x][y].StableFrames = 0
		updateCellStatus(x, y, activeSet)
	}
}

func shouldActivate(x, y int) bool {
	// Проверяем падение
	if y+1 < HEIGHT && grid[x][y+1].Type == EMPTY {
		return true
	}

	// Проверяем возможность скатывания с уменьшенной вероятностью
	if canRoll(x, y) {
		// Добавляем случайность для предотвращения постоянной активации
		return rand.Float32() < 0.3
	}

	return false
}

func hasEnvironmentChanged(x, y int) bool {
	// Проверяем, изменилось ли что-то в окружении
	if y+1 < HEIGHT && grid[x][y+1].Type == EMPTY {
		return true
	}

	// Проверяем соседние ячейки на движение
	for dx := -1; dx <= 1; dx++ {
		for dy := -1; dy <= 1; dy++ {
			if dx == 0 && dy == 0 {
				continue
			}
			nx, ny := x+dx, y+dy
			if nx >= 0 && nx < WIDTH && ny >= 0 && ny < HEIGHT {
				if isSandType(grid[nx][ny].Type) &&
					(grid[nx][ny].Status == Falling || grid[nx][ny].Status == Rolling) {
					return true
				}
			}
		}
	}

	return false
}

func handleFalling(x, y int, activeSet map[Pos]struct{}) {
	if y+1 >= HEIGHT {
		grid[x][y].Status = Idle
		grid[x][y].RollCount = 0
		grid[x][y].LastRollDir = 0
		grid[x][y].StableFrames = 0
		return
	}

	if grid[x][y+1].Type == EMPTY {
		// Перемещаем частицу вниз
		cellType := grid[x][y].Type
		fluidity := grid[x][y].Fluidity

		grid[x][y] = Cell{Type: EMPTY, Status: Idle}
		grid[x][y+1] = Cell{
			Type:     cellType,
			Status:   Falling,
			Fluidity: fluidity,
		}

		addActive(x, y+1, activeSet)
		reactivateNeighbors(x, y, activeSet)
		reactivateNeighbors(x, y+1, activeSet)
	} else {
		// Не можем падать, проверяем скатывание
		if canRoll(x, y) {
			grid[x][y].Status = Rolling
			grid[x][y].StableFrames = 0
			addActive(x, y, activeSet)
		} else {
			grid[x][y].Status = Idle
			grid[x][y].RollCount = 0
			grid[x][y].LastRollDir = 0
			grid[x][y].StableFrames = 0
			addActive(x, y, activeSet)
		}
	}
}

func handlePendingRoll(x, y int, activeSet map[Pos]struct{}) {
	if canRoll(x, y) {
		grid[x][y].Status = Rolling
		grid[x][y].StableFrames = 0
		addActive(x, y, activeSet)
	} else {
		grid[x][y].Status = Idle
		grid[x][y].StableFrames = 0
		addActive(x, y, activeSet)
	}
}

func handleRolling(x, y int, activeSet map[Pos]struct{}) {
	fluidity := grid[x][y].Fluidity
	rollCount := grid[x][y].RollCount
	lastRollDir := grid[x][y].LastRollDir

	// Ограничиваем количество скатываний
	maxRolls := 1 + fluidity/2
	if rollCount >= maxRolls {
		grid[x][y].Status = Idle
		grid[x][y].RollCount = 0
		grid[x][y].LastRollDir = 0
		grid[x][y].StableFrames = 0
		addActive(x, y, activeSet)
		return
	}

	// Находим возможные направления
	var directions []int

	// Проверяем левое направление
	if x > 0 && canRollToPosition(x, y, x-1) {
		directions = append(directions, -1)
	}

	// Проверяем правое направление
	if x < WIDTH-1 && canRollToPosition(x, y, x+1) {
		directions = append(directions, 1)
	}

	if len(directions) > 0 {
		// Выбираем направление с учетом предыдущего направления
		var chosenDir int

		if len(directions) == 1 {
			chosenDir = directions[0]
		} else {
			// Если есть выбор, добавляем небольшое предпочтение для продолжения в том же направлении
			if lastRollDir != 0 && contains(directions, lastRollDir) && rand.Float32() < 0.6 {
				chosenDir = lastRollDir
			} else {
				chosenDir = directions[rand.Intn(len(directions))]
			}
		}

		nx := x + chosenDir

		// Перемещаем частицу
		cellType := grid[x][y].Type
		cellFluidity := grid[x][y].Fluidity

		grid[x][y] = Cell{Type: EMPTY, Status: Idle}
		grid[nx][y] = Cell{
			Type:        cellType,
			Status:      Falling,
			Fluidity:    cellFluidity,
			RollCount:   rollCount + 1,
			LastRollDir: chosenDir,
		}

		addActive(nx, y, activeSet)
		reactivateNeighbors(x, y, activeSet)
		reactivateNeighbors(nx, y, activeSet)
	} else {
		grid[x][y].Status = Idle
		grid[x][y].RollCount = 0
		grid[x][y].LastRollDir = 0
		grid[x][y].StableFrames = 0
		addActive(x, y, activeSet)
	}
}

func canRoll(x, y int) bool {
	if !isSandType(grid[x][y].Type) {
		return false
	}

	// Должно быть препятствие снизу
	if y+1 >= HEIGHT || grid[x][y+1].Type == EMPTY {
		return false
	}

	config := sandConfig[grid[x][y].Type]

	// Проверяем вероятность с учетом стабильности
	rollChance := config.rollChance
	if grid[x][y].StableFrames > 10 {
		rollChance *= 0.5 // Уменьшаем вероятность для стабильных частиц
	}

	if rand.Float32() > rollChance {
		return false
	}

	// Проверяем возможность скатывания в любую сторону
	return (x > 0 && canRollToPosition(x, y, x-1)) ||
		(x < WIDTH-1 && canRollToPosition(x, y, x+1))
}

func canRollToPosition(fromX, fromY, toX int) bool {
	// Целевая позиция должна быть свободна
	if grid[toX][fromY].Type != EMPTY {
		return false
	}

	// Упрощенная проверка - убираем требование препятствия под целевой позицией
	// Это поможет избежать "невидимых стенок"

	// Проверяем, что можем скатиться (есть место для движения)
	fluidity := grid[fromX][fromY].Fluidity

	// Для высокой текучести проверяем дальше
	if fluidity > 2 {
		// Проверяем, есть ли вообще возможность для растекания в этом направлении
		direction := sign(toX - fromX)
		foundSpace := false

		for checkDist := 1; checkDist <= fluidity && !foundSpace; checkDist++ {
			checkX := toX + direction*checkDist
			if checkX >= 0 && checkX < WIDTH {
				// Если есть пустое место или возможность упасть
				if grid[checkX][fromY].Type == EMPTY {
					if fromY+1 >= HEIGHT || grid[checkX][fromY+1].Type != EMPTY {
						foundSpace = true
					}
				}
			}
		}

		if !foundSpace {
			return false
		}
	}

	return true
}

func updateCellStatus(x, y int, activeSet map[Pos]struct{}) {
	if !isSandType(grid[x][y].Type) {
		return
	}

	// Сбрасываем статус settled при активации
	if grid[x][y].Status == Settled {
		grid[x][y].Status = Idle
		grid[x][y].StableFrames = 0
	}

	if grid[x][y].Status == Falling || grid[x][y].Status == Rolling {
		return
	}

	if y+1 < HEIGHT && grid[x][y+1].Type == EMPTY {
		grid[x][y].Status = Falling
		grid[x][y].StableFrames = 0
		addActive(x, y, activeSet)
		return
	}

	if canRoll(x, y) {
		grid[x][y].Status = Rolling
		grid[x][y].StableFrames = 0
		addActive(x, y, activeSet)
		return
	}

	grid[x][y].Status = Idle
	addActive(x, y, activeSet)
}

func addActive(x, y int, activeSet map[Pos]struct{}) {
	if x >= 0 && x < WIDTH && y >= 0 && y < HEIGHT {
		activeSet[Pos{x, y}] = struct{}{}
	}
}

func reactivateNeighbors(x, y int, activeSet map[Pos]struct{}) {
	directions := []Pos{
		{0, -1}, {-1, -1}, {1, -1}, // Сверху
		{-1, 0}, {1, 0}, // По бокам
	}

	for _, d := range directions {
		nx, ny := x+d.X, y+d.Y
		if nx >= 0 && nx < WIDTH && ny >= 0 && ny < HEIGHT {
			if isSandType(grid[nx][ny].Type) &&
				(grid[nx][ny].Status == Idle || grid[nx][ny].Status == Settled) {
				updateCellStatus(nx, ny, activeSet)
			}
		}
	}
}

// Вспомогательные функции
func isSandType(cellType CellType) bool {
	return cellType >= SAND_TYPE_0 && cellType <= SAND_TYPE_5
}

func sign(x int) int {
	if x < 0 {
		return -1
	}
	if x > 0 {
		return 1
	}
	return 0
}

func contains(slice []int, item int) bool {
	for _, v := range slice {
		if v == item {
			return true
		}
	}
	return false
}
