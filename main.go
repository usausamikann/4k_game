package main

import (
	"flag"
	"fmt"
	"image/color"
	"log"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"github.com/usausamikann/4k_game/internal/chart"
	"github.com/usausamikann/4k_game/internal/play"
)

const (
	screenWidth  = 640
	screenHeight = 480
	laneLeft     = 152
	laneWidth    = 84
	judgeY       = 400
	noteHeight   = 12
)

var noteColors = [chart.LaneCount]color.RGBA{
	{R: 100, G: 190, B: 255, A: 255}, {R: 140, G: 225, B: 180, A: 255},
	{R: 255, G: 205, B: 110, A: 255}, {R: 235, G: 150, B: 235, A: 255},
}

type game struct {
	chart       *play.PlayChart
	view        *play.View
	scroll      play.Scroll
	clock       *play.Clock
	preparation time.Duration
	now         time.Duration
}

func newGame(path string, bpm, speed float64, preparation time.Duration) (*game, error) {
	if preparation < 0 {
		return nil, fmt.Errorf("preparation must be nonnegative")
	}
	pattern, err := chart.Load(path)
	if err != nil {
		return nil, err
	}
	notes, err := chart.Schedule(pattern, bpm)
	if err != nil {
		return nil, err
	}
	p, err := play.NewChart(notes)
	if err != nil {
		return nil, err
	}
	scroll := play.Scroll{Height: screenHeight, JudgeY: judgeY, Speed: speed, NoteHeight: noteHeight}
	view, err := play.NewView(p, scroll)
	if err != nil {
		return nil, err
	}
	g := &game{chart: p, view: view, scroll: scroll, preparation: preparation, now: -preparation}
	view.Update(g.now)
	return g, nil
}

func (g *game) Update() error { return g.updateAt(time.Now(), ebiten.IsFocused()) }
func (g *game) updateAt(wall time.Time, focused bool) error {
	if g.clock == nil {
		clock, err := play.NewClock(wall, g.preparation)
		if err != nil {
			return err
		}
		g.clock = clock // Start after loading and window initialization, at -preparation.
	}
	if err := g.clock.SetPaused(wall, !focused); err != nil {
		return err
	}
	g.now = g.clock.At(wall)
	g.view.Update(g.now)
	return nil
}

func (g *game) Draw(screen *ebiten.Image) {
	screen.Fill(color.RGBA{R: 24, G: 28, B: 36, A: 255})
	for lane := 0; lane < chart.LaneCount; lane++ {
		x := float32(laneLeft + lane*laneWidth)
		vector.FillRect(screen, x, 0, laneWidth, screenHeight, color.RGBA{R: 36, G: 42, B: 52, A: 255}, false)
		vector.FillRect(screen, x, 0, 1, screenHeight, color.RGBA{R: 80, G: 90, B: 105, A: 255}, false)
	}
	vector.FillRect(screen, laneLeft, judgeY-1, laneWidth*chart.LaneCount, 2, color.White, false)
	first, end := g.view.Range()
	for _, group := range g.chart.Groups[first:end] {
		y := float32(g.scroll.Y(group.Time, g.now)) - noteHeight/2
		for _, index := range group.NoteIndices {
			n := g.chart.Notes[index]
			x := float32(laneLeft + n.Lane*laneWidth + 6)
			vector.FillRect(screen, x, y, laneWidth-12, noteHeight, noteColors[n.Lane], false)
		}
	}
	message := "4K Game\n\nClose window to exit."
	if g.clock != nil && g.clock.Paused() {
		message += "\n\nPaused: focus window."
	}
	ebitenutil.DebugPrint(screen, message)
}
func (*game) Layout(_, _ int) (int, int) { return screenWidth, screenHeight }

func main() {
	pattern := flag.String("pattern", "patterns/sample.json", "pattern JSON path")
	bpm := flag.Float64("bpm", 180, "positive playback BPM")
	speed := flag.Float64("speed", 240, "scroll speed in pixels/second")
	preparation := flag.Duration("prepare", 2*time.Second, "preparation time before tick zero")
	flag.Parse()
	g, err := newGame(*pattern, *bpm, *speed, *preparation)
	if err != nil {
		log.Fatal(err)
	}
	ebiten.SetWindowSize(screenWidth, screenHeight)
	ebiten.SetWindowTitle("4K Game")
	// Receive focus changes even while unfocused; the game clock freezes explicitly.
	ebiten.SetRunnableOnUnfocused(true)
	if err := ebiten.RunGame(g); err != nil {
		log.Fatal(err)
	}
}
