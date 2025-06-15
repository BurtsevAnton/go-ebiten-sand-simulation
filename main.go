package main

import (
	"fmt"
	"image/color"
	"log"
	"math/rand"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
)

const (
	WIDTH  = 400
	HEIGHT = 250
	CELL   = 4
)

type CellType int
type Status int

const (
	Empty     CellType = iota
	SandType0          // Текучесть 0 - очень грубый песок
	SandType1          // Текучесть 1 - грубый песок
	SandType2          // Текучесть 2 - обычный песок
	SandType3          // Текучесть 3 - мелкий песок
	SandType4          // Текучесть 4 - очень мелкий песок
	SandType5          // Текучесть 5 - пудровый песок
	Stone
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
	active    []Pos
	iteration = 0
)

// Конфигурация типов песка
var sandConfig = map[CellType]struct {
	fluidity   int
	rollChance float32
	fallDelay  int // Задержка перед следующим падением (в кадрах)
	color      color.RGBA
}{
	SandType0: {1, 0.85, 0, color.RGBA{159, 145, 105, 255}}, // Темные падают быстрее (без задержки)
	SandType1: {1, 0.90, 1, color.RGBA{180, 160, 118, 255}},
	SandType2: {2, 0.95, 0, color.RGBA{214, 174, 128, 255}},
	SandType3: {2, 0.98, 1, color.RGBA{228, 185, 92, 255}},
	SandType4: {3, 1.0, 0, color.RGBA{248, 213, 183, 255}},
	SandType5: {4, 1.0, 1, color.RGBA{255, 228, 196, 255}}, // Самые светлые падают медленнее
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
			case SandType0, SandType1, SandType2, SandType3, SandType4, SandType5:
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
			case Stone:
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

	for x := 0; x < WIDTH; x++ {
		grid[x][100] = Cell{Type: Stone, Status: Fixed}
		grid[x][HEIGHT-1] = Cell{Type: Stone, Status: Fixed}
	}

	numSlopes := rand.Intn(3) + 6
	slopeSpacing := WIDTH / (numSlopes + 1)

	for i := 0; i < numSlopes; i++ {
		startX := i*slopeSpacing + rand.Intn(slopeSpacing/3)
		length := slopeSpacing - 10 + rand.Intn(20)

		baseY := 160 + rand.Intn(60)
		if baseY >= HEIGHT {
			baseY = HEIGHT - 1
		}

		angle := 0.0 + rand.Float64()*1
		if rand.Intn(2) == 0 {
			angle = -angle
		}

		for dx := 0; dx < length; dx++ {
			x := startX + dx
			if x >= WIDTH {
				continue
			}

			y := baseY + int(float64(dx)*angle)
			if y >= HEIGHT {
				continue
			}

			grid[x][y] = Cell{Type: Stone, Status: Fixed}

			if rand.Float32() < 0.15 && dx > 10 && dx < length-10 {
				holeY := 100
				grid[x][holeY] = Cell{Type: Empty, Status: Idle}
				grid[x+1][holeY] = Cell{Type: Empty, Status: Idle}
			}
		}
	}

	for y := 0; y < 100; y++ {
		for x := 0; x < WIDTH; x++ {
			if grid[x][y].Type == Empty && rand.Float32() < 0.7 {
				sandTypes := []CellType{SandType0, SandType1, SandType2, SandType3, SandType4, SandType5}
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

	active = make([]Pos, 0, WIDTH*HEIGHT)
	for y := HEIGHT - 2; y >= 0; y-- {
		for x := 0; x < WIDTH; x++ {
			if isSandType(grid[x][y].Type) {
				updateCellStatus(x, y)
			}
		}
	}
}

func update() error {
	iteration++
	newActive := make([]Pos, 0, len(active)*2)

	for _, pos := range active {
		x, y := pos.X, pos.Y

		if !isSandType(grid[x][y].Type) {
			continue
		}

		switch grid[x][y].Status {
		case Falling:
			handleFalling(x, y, &newActive)
		case Rolling:
			handleRolling(x, y, &newActive)
		case PendingRoll:
			handlePendingRoll(x, y, &newActive)
		case Idle:
			handleIdle(x, y, &newActive)
		}
	}

	if iteration%60 == 0 {
		for y := HEIGHT - 2; y >= 0; y-- {
			for x := 0; x < WIDTH; x++ {
				if isSandType(grid[x][y].Type) && grid[x][y].Status == Settled {
					if hasEnvironmentChanged(x, y) {
						grid[x][y].Status = Idle
						grid[x][y].StableFrames = 0
						updateCellStatus(x, y)
						addActive(x, y, &newActive)
					}
				}
			}
		}
	}

	active = newActive

	return nil
}

func handleIdle(x, y int, activeSet *[]Pos) {
	grid[x][y].StableFrames++

	if grid[x][y].StableFrames > 60 {
		if rand.Float32() < 0.9 {
			grid[x][y].Status = Settled
		} else {
			grid[x][y].Status = Idle
		}

		return
	}

	if shouldActivate(x, y) {
		grid[x][y].StableFrames = 0
		updateCellStatus(x, y)
		addActive(x, y, activeSet)
	}
}

func shouldActivate(x, y int) bool {
	if y+1 < HEIGHT && grid[x][y+1].Type == Empty {
		return true
	}

	if canRoll(x, y) {
		return rand.Float32() < 0.3
	}

	return false
}

func hasEnvironmentChanged(x, y int) bool {
	if y+1 < HEIGHT && grid[x][y+1].Type == Empty {
		return true
	}

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

func handleFalling(x, y int, activeSet *[]Pos) {
	if y+1 >= HEIGHT {
		grid[x][y].Status = Idle
		grid[x][y].RollCount = 0
		grid[x][y].LastRollDir = 0
		grid[x][y].StableFrames = 0
		grid[x][y].FallCounter = 0

		return
	}

	config := sandConfig[grid[x][y].Type]

	grid[x][y].FallCounter++
	if grid[x][y].FallCounter <= config.fallDelay {
		addActive(x, y, activeSet)

		return
	}

	grid[x][y].FallCounter = 0

	if grid[x][y+1].Type == Empty {
		cellType := grid[x][y].Type
		fluidity := grid[x][y].Fluidity

		grid[x][y] = Cell{Type: Empty, Status: Idle}
		grid[x][y+1] = Cell{
			Type:        cellType,
			Status:      Falling,
			Fluidity:    fluidity,
			FallCounter: 0,
		}

		addActive(x, y+1, activeSet)
		reactivateNeighbors(x, y, activeSet)
		reactivateNeighbors(x, y+1, activeSet)
	} else {
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

func handlePendingRoll(x, y int, activeSet *[]Pos) {
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

func handleRolling(x, y int, activeSet *[]Pos) {
	fluidity := grid[x][y].Fluidity
	rollCount := grid[x][y].RollCount
	lastRollDir := grid[x][y].LastRollDir

	maxRolls := 1 + fluidity/2
	if rollCount >= maxRolls {
		grid[x][y].Status = Idle
		grid[x][y].RollCount = 0
		grid[x][y].LastRollDir = 0
		grid[x][y].StableFrames = 0
		addActive(x, y, activeSet)

		return
	}

	var directions []int

	if x > 0 && canRollToPosition(x, y, x-1) {
		directions = append(directions, -1)
	}

	if x < WIDTH-1 && canRollToPosition(x, y, x+1) {
		directions = append(directions, 1)
	}

	if len(directions) > 0 {
		var chosenDir int

		if len(directions) == 1 {
			chosenDir = directions[0]
		} else {
			if lastRollDir != 0 && contains(directions, lastRollDir) && rand.Float32() < 0.6 {
				chosenDir = lastRollDir
			} else {
				chosenDir = directions[rand.Intn(len(directions))]
			}
		}

		nx := x + chosenDir

		cellType := grid[x][y].Type
		cellFluidity := grid[x][y].Fluidity

		grid[x][y] = Cell{Type: Empty, Status: Idle}
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

	if y+1 >= HEIGHT || grid[x][y+1].Type == Empty {
		return false
	}

	config := sandConfig[grid[x][y].Type]

	rollChance := config.rollChance
	if grid[x][y].StableFrames > 10 {
		rollChance *= 0.5
	}

	if rand.Float32() > rollChance {
		return false
	}

	return (x > 0 && canRollToPosition(x, y, x-1)) ||
		(x < WIDTH-1 && canRollToPosition(x, y, x+1))
}

func canRollToPosition(fromX, fromY, toX int) bool {
	if grid[toX][fromY].Type != Empty {
		return false
	}

	fluidity := grid[fromX][fromY].Fluidity
	if fluidity > 2 {
		direction := sign(toX - fromX)
		foundSpace := false

		for checkDist := 1; checkDist <= fluidity && !foundSpace; checkDist++ {
			checkX := toX + direction*checkDist
			if checkX >= 0 && checkX < WIDTH {
				if grid[checkX][fromY].Type == Empty {
					if fromY+1 >= HEIGHT || grid[checkX][fromY+1].Type != Empty {
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

func updateCellStatus(x, y int) {
	if !isSandType(grid[x][y].Type) {
		return
	}

	if grid[x][y].Status == Settled {
		grid[x][y].Status = Idle
		grid[x][y].StableFrames = 0
	}

	if grid[x][y].Status == Falling || grid[x][y].Status == Rolling {
		return
	}

	if y+1 < HEIGHT && grid[x][y+1].Type == Empty {
		grid[x][y].Status = Falling
		grid[x][y].StableFrames = 0
		addActive(x, y, &active)

		return
	}

	if canRoll(x, y) {
		grid[x][y].Status = Rolling
		grid[x][y].StableFrames = 0
		addActive(x, y, &active)

		return
	}

	grid[x][y].Status = Idle
	addActive(x, y, &active)
}

func addActive(x, y int, activeSet *[]Pos) {
	if x >= 0 && x < WIDTH && y >= 0 && y < HEIGHT {
		*activeSet = append(*activeSet, Pos{x, y})
	}
}

func reactivateNeighbors(x, y int, activeSet *[]Pos) {
	directions := []Pos{
		{0, -1}, {-1, -1}, {1, -1},
		{-1, 0}, {1, 0},
	}

	for _, d := range directions {
		nx, ny := x+d.X, y+d.Y
		if nx >= 0 && nx < WIDTH && ny >= 0 && ny < HEIGHT {
			if isSandType(grid[nx][ny].Type) &&
				(grid[nx][ny].Status == Idle || grid[nx][ny].Status == Settled) {
				updateCellStatus(nx, ny)
				addActive(nx, ny, activeSet)
			}
		}
	}
}

func isSandType(cellType CellType) bool {
	return cellType >= SandType0 && cellType <= SandType5
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
