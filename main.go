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
	"github.com/usausamikann/4k_game/internal/judge"
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

// Pastel lane colors at 25% opacity, drawn behind notes and the judge line.
var highlightColors = func() [chart.LaneCount]color.NRGBA {
	var colors [chart.LaneCount]color.NRGBA
	for lane, c := range noteColors {
		colors[lane] = color.NRGBA{
			R: uint8((uint16(c.R) + 255) / 2),
			G: uint8((uint16(c.G) + 255) / 2),
			B: uint8((uint16(c.B) + 255) / 2),
			A: 64,
		}
	}
	return colors
}()

type hudState struct {
	stats  judge.Stats
	rank   judge.Rank
	paused bool
	phase  string
	count  int
}
type game struct {
	session  *play.Session
	scroll   play.Scroll
	keys     [chart.LaneCount]ebiten.Key
	sudden   play.Sudden
	hud      string
	hudState hudState
	down     [chart.LaneCount]bool
}

func newGame(path string, bpm, speed float64, offsets play.Offsets, sudden play.Sudden) (*game, error) {
	if sudden < 0 || sudden > 400 {
		return nil, fmt.Errorf("sudden must be 0.0..40.0")
	}
	pattern, err := chart.Load(path)
	if err != nil {
		return nil, err
	}
	notes, err := chart.Schedule(pattern, bpm)
	if err != nil {
		return nil, err
	}
	scroll := play.Scroll{Height: screenHeight, JudgeY: judgeY, Speed: speed, NoteHeight: noteHeight}
	session, err := play.NewSession(notes, bpm, scroll, offsets)
	if err != nil {
		return nil, err
	}
	return &game{session: session, scroll: scroll, sudden: sudden, keys: defaultBindings()}, nil
}
func (g *game) Update() error {
	var down [chart.LaneCount]bool
	for lane, key := range g.keys {
		down[lane] = ebiten.IsKeyPressed(key)
	}
	return g.updateAt(time.Now(), ebiten.IsFocused(), down)
}
func (g *game) updateAt(wall time.Time, focused bool, down [chart.LaneCount]bool) error {
	g.down = down
	if !focused {
		g.down = [chart.LaneCount]bool{}
	}
	if err := g.session.UpdateAt(wall, focused, down); err != nil {
		return err
	}
	phase, count := g.session.Timeline.Phase(g.session.Now)
	state := hudState{stats: g.session.Statistics(), paused: g.session.Clock.Paused(), phase: phase, count: count}
	if g.session.HasLast {
		state.rank = g.session.Last.Rank
	}
	if g.hud == "" || state != g.hudState {
		rank := string(state.rank)
		if rank == "" {
			rank = "-"
		}
		status := state.phase
		if state.phase == "COUNT" {
			status = fmt.Sprintf("COUNT %d", state.count)
		}
		if state.paused {
			status = "PAUSED"
		}
		g.hud = fmt.Sprintf("4K Game\n\n%s\n\n%s\nCOMBO %d\nEMPTY %d", status, rank, state.stats.Combo, state.stats.EmptyPresses)
		g.hudState = state
	}
	return nil
}

func (g *game) Draw(screen *ebiten.Image) {
	screen.Fill(color.RGBA{R: 24, G: 28, B: 36, A: 255})
	for lane := 0; lane < chart.LaneCount; lane++ {
		x := float32(laneLeft + lane*laneWidth)
		vector.FillRect(screen, x, 0, laneWidth, screenHeight, color.RGBA{R: 36, G: 42, B: 52, A: 255}, false)
		vector.FillRect(screen, x, 0, 1, screenHeight, color.RGBA{R: 80, G: 90, B: 105, A: 255}, false)
		if g.down[lane] {
			vector.FillRect(screen, x, judgeY, laneWidth, screenHeight-judgeY, highlightColors[lane], false)
		}
	}
	first, end := g.session.View.Range()
	for _, group := range g.session.Chart.Groups[first:end] {
		y := float32(g.scroll.Y(group.Time, g.session.VisualNow)) - noteHeight/2
		for _, index := range group.NoteIndices {
			n := g.session.Chart.Notes[index]
			if g.session.States[index] == judge.Hit {
				continue
			}
			noteColor := noteColors[n.Lane]
			if g.session.States[index] == judge.Miss {
				noteColor = color.RGBA{R: 110, G: 55, B: 55, A: 255}
			}
			x := float32(laneLeft + n.Lane*laneWidth + 6)
			vector.FillRect(screen, x, y, laneWidth-12, noteHeight, noteColor, false)
		}
	}
	// Cover notes only inside the lane rectangle; judgement is unaffected.
	if g.sudden > 0 {
		vector.FillRect(screen, laneLeft, 0, laneWidth*chart.LaneCount, float32(g.sudden.Height(judgeY)), color.RGBA{R: 12, G: 15, B: 20, A: 255}, false)
	}
	// Keep the line in front of the wall even at full coverage.
	vector.FillRect(screen, laneLeft, judgeY-1, laneWidth*chart.LaneCount, 2, color.White, false)
	ebitenutil.DebugPrint(screen, g.hud)

}
func (*game) Layout(_, _ int) (int, int) { return screenWidth, screenHeight }

func main() {
	pattern := flag.String("pattern", "patterns/sample.json", "pattern JSON path")
	bpm := flag.Float64("bpm", 180, "positive playback BPM")
	speed := flag.Float64("speed", 240, "scroll speed in pixels/second")
	offsetA := flag.Float64("offset-a", 0, "visual and judgement offset in milliseconds")
	offsetB := flag.Float64("offset-b", 0, "judgement-only offset in milliseconds")
	suddenText := flag.String("sudden", "0.0", "upper lane cover: 0.0..40.0, step 0.1")
	keysText := flag.String("keys", "D,F,J,K", "four distinct letter keys in lane order")
	flag.Parse()
	a, err := play.OffsetMilliseconds(*offsetA)
	if err != nil {
		log.Fatal(err)
	}
	b, err := play.OffsetMilliseconds(*offsetB)
	if err != nil {
		log.Fatal(err)
	}
	sudden, err := play.ParseSudden(*suddenText)
	if err != nil {
		log.Fatal(err)
	}
	keys, err := parseBindings(*keysText)
	if err != nil {
		log.Fatal(err)
	}
	g, err := newGame(*pattern, *bpm, *speed, play.Offsets{A: a, B: b}, sudden)
	if err != nil {
		log.Fatal(err)
	}
	g.keys = keys
	ebiten.SetWindowSize(screenWidth, screenHeight)
	ebiten.SetWindowTitle("4K Game")
	// Receive focus changes even while unfocused; the game clock freezes explicitly.
	ebiten.SetRunnableOnUnfocused(true)
	if err := ebiten.RunGame(g); err != nil {
		log.Fatal(err)
	}
}
