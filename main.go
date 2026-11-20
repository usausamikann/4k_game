package main

import (
	"errors"
	"flag"
	"fmt"
	"image/color"
	"log"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"
	metronome "github.com/usausamikann/4k_game/internal/audio"
	"github.com/usausamikann/4k_game/internal/chart"
	"github.com/usausamikann/4k_game/internal/input"
	"github.com/usausamikann/4k_game/internal/inputdebug"
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

// Use the same two-layer silhouette for every lane; only the palette changes.
var noteVisuals = func() [chart.LaneCount]noteVisual {
	var styles [chart.LaneCount]noteVisual
	for lane, c := range noteColors {
		styles[lane] = newNoteVisual(c)
	}
	return styles
}()

// Keep the pastel hue but reduce the full-lane press highlight to ~8%.
var highlightColors = func() [chart.LaneCount]color.NRGBA {
	var colors [chart.LaneCount]color.NRGBA
	for lane, c := range noteColors {
		colors[lane] = pressedLaneColor(c)
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
	session     *play.Session
	scroll      play.Scroll
	keys        [chart.LaneCount]ebiten.Key
	sudden      play.Sudden
	pause       pauseState
	hud         string
	hudState    hudState
	down        [chart.LaneCount]bool
	inputDebug  *inputdebug.HUD
	metronome   *metronome.Transport
	keyPlayback *metronome.KeyPlayback
	debugAudio  bool
	audioHUD    string
	rawInput    *input.RawReader
	rawBuf      []input.RawEvent
	lastRawReadErrors uint64
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
	wall := time.Now()
	focused := ebiten.IsFocused()
	escapeDown := ebiten.IsKeyPressed(ebiten.KeyEscape)
	if g.rawInput != nil {
		batch, lost, snapshot := g.rawInput.DrainUpTo(wall, g.rawBuf[:0])
		g.rawBuf = batch
		if failed := g.rawInput.FailedReads(); failed > g.lastRawReadErrors {
			log.Printf("raw keyboard input read errors: %d total", failed)
			g.lastRawReadErrors = failed
		}
		if lost != 0 {
			log.Printf("raw keyboard input dropped %d events; resynchronizing keys", lost)
			return g.updateWithRawResyncAt(wall, focused, snapshot, escapeDown)
		}
		return g.updateWithRawAt(wall, focused, g.rawBuf, escapeDown)
	}
	var samples [chart.LaneCount]input.KeySample
	for lane, key := range g.keys {
		samples[lane] = input.KeySample{
			Down: ebiten.IsKeyPressed(key),
			JustPressed: inpututil.IsKeyJustPressed(key),
			JustReleased: inpututil.IsKeyJustReleased(key),
			StillDown: inpututil.KeyPressDuration(key) > 0,
		}
	}
	return g.updateWithEdgesAt(wall, focused, samples, escapeDown)
}

// updateAt preserves the existing deterministic test entry point without
// simulating Escape; updateWithPauseAt also accepts the sampled Escape state.
func (g *game) updateAt(wall time.Time, focused bool, down [chart.LaneCount]bool) error {
	return g.updateWithPauseAt(wall, focused, down, false)
}

func (g *game) updateWithPauseAt(wall time.Time, focused bool, down [chart.LaneCount]bool, escapeDown bool) error {
	return g.updateFrame(wall, focused, escapeDown, nil, func(playing bool) error {
		g.down = down
		if !playing {
			g.down = [chart.LaneCount]bool{}
		}
		return g.session.UpdateAt(wall, playing, down)
	})
}

func (g *game) updateWithEdgesAt(wall time.Time, focused bool, samples [chart.LaneCount]input.KeySample, escapeDown bool) error {
	return g.updateFrame(wall, focused, escapeDown, nil, func(playing bool) error {
		if err := g.session.UpdateWithEdgesAt(wall, playing, samples); err != nil {
			return err
		}
		g.down = g.session.Pressed()
		return nil
	})
}

func (g *game) updateWithRawAt(wall time.Time, focused bool, raw []input.RawEvent, escapeDown bool) error {
	return g.updateFrame(wall, focused, escapeDown, func() { g.session.SyncRaw(raw) }, func(playing bool) error {
		if err := g.session.UpdateWithRawAt(wall, playing, raw); err != nil {
			return err
		}
		g.down = g.session.Pressed()
		return nil
	})
}

func (g *game) updateWithRawResyncAt(wall time.Time, focused bool, down [chart.LaneCount]bool, escapeDown bool) error {
	return g.updateFrame(wall, focused, escapeDown, func() { g.session.SyncRawState(down) }, func(playing bool) error {
		if err := g.session.UpdateWithRawResyncAt(wall, playing, down); err != nil {
			return err
		}
		g.down = g.session.Pressed()
		return nil
	})
}

func (g *game) updateFrame(wall time.Time, focused, escapeDown bool, onWait func(), step func(playing bool) error) error {
	g.pause.Update(focused, escapeDown)
	playing := focused && !g.pause.active
	if g.debugAudio && g.metronome != nil {
		g.metronome.EnableDiagnostics()
	}
	if g.metronome != nil && !g.metronome.UpdateAt(wall, playing) {
		if onWait != nil {
			onWait()
		}
		if g.keyPlayback != nil {
			if err := g.keyPlayback.SetActive(false); err != nil {
				return err
			}
		}
		g.hud = "4K Game\n\nWaiting for audio\nand window focus."
		return nil
	}

	if g.keyPlayback != nil {
		if err := g.keyPlayback.SetActive(playing); err != nil {
			return err
		}
	}
	if err := step(playing); err != nil {
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
	if g.inputDebug != nil && g.inputDebug.Due(wall) {
		g.inputDebug.Refresh(wall, ebiten.ActualTPS(), ebiten.ActualFPS(), inputdebug.State{
			LastJudge:     g.session.Last.Rank,
			LastHitOffset: g.session.LastHit.Offset,
			HasHit:        g.session.HasLastHit,
			EmptyPresses:  state.stats.EmptyPresses,
		})
	}

	if g.debugAudio && g.metronome != nil {
		g.audioHUD = g.metronome.DiagnosticText(wall, g.session.Now)
	}

	return nil
}

func (g *game) Draw(screen *ebiten.Image) {
	screen.Fill(color.RGBA{R: 24, G: 28, B: 36, A: 255})
	for lane := 0; lane < chart.LaneCount; lane++ {
		x := float32(laneLeft + lane*laneWidth)
		vector.FillRect(screen, x, 0, laneWidth, screenHeight, color.RGBA{R: 17, G: 23, B: 32, A: 255}, false)
		vector.FillRect(screen, x, 0, 1, screenHeight, color.RGBA{R: 80, G: 90, B: 105, A: 255}, false)
		if g.down[lane] {
			vector.FillRect(screen, x, 0, laneWidth, screenHeight, highlightColors[lane], false)
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
			x := float32(laneLeft + n.Lane*laneWidth + 6)
			drawNote(screen, x, y, n.Lane, g.session.States[index])
		}
	}
	// Cover notes only inside the lane rectangle; judgement is unaffected.
	if g.sudden > 0 {
		vector.FillRect(screen, laneLeft, 0, laneWidth*chart.LaneCount, float32(g.sudden.Height(judgeY)), color.RGBA{R: 12, G: 15, B: 20, A: 255}, false)
	}
	// Keep the line in front of the wall even at full coverage.
	vector.FillRect(screen, laneLeft, judgeY-1, laneWidth*chart.LaneCount, 2, color.White, false)
	ebitenutil.DebugPrint(screen, g.hud)
	if g.debugAudio {
		ebitenutil.DebugPrintAt(screen, g.audioHUD, laneLeft+laneWidth*chart.LaneCount+8, 140)
	}
	if g.inputDebug != nil {
		ebitenutil.DebugPrintAt(screen, g.inputDebug.Text(), laneLeft+laneWidth*chart.LaneCount+8, 0)
	}
	if g.pause.active {
		g.drawPause(screen)
	}
}

// Draw a dark base with a centered, bright band in the original lane color.
// Keep MISS as a single dark-red rectangle to distinguish resolved misses.
func drawNote(screen *ebiten.Image, x, y float32, lane int, state judge.State) {
	const width = float32(laneWidth - 12)
	const height = float32(noteHeight)
	if state == judge.Miss {
		vector.FillRect(screen, x, y, width, height, color.RGBA{R: 110, G: 55, B: 55, A: 255}, false)
		return
	}
	visual := noteVisuals[lane]
	vector.FillRect(screen, x, y, width, height, visual.body, false)
	coreX, coreY := centeredNoteCore(x, y, width, height)
	vector.FillRect(screen, coreX, coreY, noteCoreWidth, noteCoreHeight, visual.core, false)
}

const (
	pausePanelWidth  = 420
	pausePanelHeight = 280
	pausePanelX      = (screenWidth - pausePanelWidth) / 2
	pausePanelY      = (screenHeight - pausePanelHeight) / 2
)

// This first Pause screen is deliberately read-only. Exit/Retry and interactive
// settings are reserved for follow-up Step 6 changes.
func (g *game) drawPause(screen *ebiten.Image) {
	vector.FillRect(screen, 0, 0, screenWidth, screenHeight, color.NRGBA{R: 8, G: 10, B: 14, A: 170}, false)
	vector.FillRect(screen, pausePanelX, pausePanelY, pausePanelWidth, pausePanelHeight, color.RGBA{R: 43, G: 48, B: 62, A: 255}, false)
	vector.StrokeRect(screen, pausePanelX, pausePanelY, pausePanelWidth, pausePanelHeight, 1, color.RGBA{R: 130, G: 138, B: 154, A: 255}, false)
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf(
		"PAUSE\n\nSpeed: %g\nSUDDEN+: %.1f\n\nExit (not available)\nRetry (not available)\n\nEsc: Resume",
		g.scroll.Speed, float64(g.sudden)/10,
	), pausePanelX+32, pausePanelY+24)
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
	tps := flag.Int("tps", inputdebug.DefaultTPS, "update target: 60, 120, or 240")
	debugInput := flag.Bool("debug-input", false, "show TPS/FPS and input judgement diagnostics")
	sound := flag.String("metronome-sound", metronome.DefaultSound, "metronome: switch8, switch19, switch29, glass_002, pluck_001")
	debugAudio := flag.Bool("debug-audio", false, "show reference audio position and clock delta (not physical latency)")
	keyMode := flag.String("keysound-mode", "off", "keysounds: off, unified, per-lane")
	keySound := flag.String("keysound-sound", "drop_002", "unified keysound ID")
	keyLanes := flag.String("keysound-lanes", "drop_002,drop_002,drop_002,drop_002", "four keysound IDs in lane order")
	keyVolume := flag.Int("keysound-volume", 50, "keysound volume: 0..100 percent")
	metVolume := flag.Int("metronome-volume", 50, "metronome volume: 0..100 percent")
	rawInput := flag.Bool("raw-input", false, "experimental Windows raw input; use Ebitengine edge fallback by default")
	flag.Parse()
	audioSettings, err := metronome.ParseSettings(*keyMode, *keySound, *keyLanes, *keyVolume, *metVolume)
	if err != nil {
		log.Fatal(err)
	}
	if err := configureTPS(*tps); err != nil {
		log.Fatal(err)
	}
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
	if *rawInput {
		if codes, ok := scanCodesForBindings(g.keys); ok {
			reader, err := input.NewRawReader(codes)
			if err != nil {
				if !errors.Is(err, input.ErrRawInputUnsupported) {
					log.Printf("raw input unavailable; using Ebitengine edges: %v", err)
				}
			} else {
				g.rawInput = reader
				defer g.rawInput.Close()
			}
		} else {
			log.Printf("raw input disabled for non-letter bindings; using Ebitengine edges")
		}
	}
	if err := g.enableAudio(*sound, *bpm, audioSettings); err != nil {
		log.Fatal(err)
	}
	g.debugAudio = *debugAudio
	if *debugInput {
		debug, err := inputdebug.NewHUD(*tps)
		if err != nil {
			log.Fatal(err)
		}
		g.inputDebug = debug
	}
	ebiten.SetWindowSize(screenWidth, screenHeight)
	ebiten.SetWindowTitle("4K Game")
	// Receive focus changes even while unfocused; the game clock freezes explicitly.
	ebiten.SetRunnableOnUnfocused(true)
	err = ebiten.RunGame(g)
	g.metronome.Close()
	if g.keyPlayback != nil {
		if closeErr := g.keyPlayback.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}
	if err != nil {
		log.Fatal(err)
	}
}

// Set only the update target; leave FPS, clock, chart and scroll settings alone.
func configureTPS(tps int) error {
	if err := inputdebug.ValidateTPS(tps); err != nil {
		return err
	}
	ebiten.SetTPS(tps)
	return nil
}
