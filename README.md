# 4K Game

Go + Ebitengine で作る固定4レーンのリズムゲーム。
現在は実装計画の Step 5 と、Step 5.1 の音声同期診断まで実装済み。単一パターンの JSON 読み込み・検証と
BPM による時刻変換を `internal/chart`、判定エンジンを `internal/judge` に用意している。
アプリは 640 × 480 のウィンドウに4レーン・判定線と流下する TAP ノーツを表示する。
ゲーム時計と描画対象管理は GUI 非依存の `internal/play` に用意している。
D/F/J/K の入力判定、判定表示、タイミング調整 A/B、上部遮蔽 SUDDEN+ に対応。
5種類から選べるメトロノーム音声に対応。キー別の打鍵音・パターン連結・本格的な設定画面と結果画面は今後実装する。

## 必要なもの

- Go 1.25.0 以上（クラウド環境では Go 1.26.9 で検証）
- Ebitengine v2.10.5（`go.mod` で固定）
- ウィンドウを表示できるデスクトップ環境

Go の依存ライブラリは初回のビルド・起動時に取得される。

### Windows

Go をインストールし、このリポジトリのディレクトリで以下のコマンドを実行する。
Ebitengine の Windows ビルドには C コンパイラは不要。

### Linux / WSL

Linux では C コンパイラと X11・OpenGL の開発ライブラリが必要。
Debian / Ubuntu では以下で準備できる。

```sh
sudo apt-get update
sudo apt-get install -y build-essential pkg-config libx11-dev libxrandr-dev \
  libxcursor-dev libxi-dev libxinerama-dev libxxf86vm-dev libgl1-mesa-dev libasound2-dev
```

WSL でウィンドウを表示するには WSLg などが必要。
音声のビルドには ALSA の開発ライブラリ、実行には利用可能な音声出力デバイスが必要。

## 起動・ビルド・テスト

リポジトリのルートで実行する。

```sh
go run .
go build -o bin/4k_game .
go test ./...
```

起動すると「4K Game」というタイトルのウィンドウが表示される。
既定ではサンプル譜面を BPM180、準備時間8拍（約2.667秒）、速度240px/sで表示する。
A/B=0の場合、時刻0のノーツは準備時間を経て判定線に到達する。
左から D/F/J/K を押すと判定し、押下中は判定線から画面下端までレーン全幅を強調表示する。
ハイライトはノーツ色を淡くした約25%不透明の色で、ノーツと判定線の背面に描画する。
ウィンドウを閉じると終了する。Windows 用の実行ファイルを作る場合は
`go build -o bin/4k_game.exe .` を使う。

設定は個別に指定できる。BPMと速度は正の有限値を使う。
準備時間は固定8拍に変更したため、以前の `-prepare` は廃止（指定するとエラー）。

```sh
go run . -pattern patterns/sample.json -bpm 120 -speed 300 -offset-a 10 -offset-b -5 -sudden 20.0
go run . -keys A,S,L,P
go run . -tps 120 -debug-input
go run . -metronome-sound glass_002 -debug-audio
```

`go test ./...` は画面なしで譜面・判定・時計・描画対象計算の単体テストを実行する。
既存の描画テストは `go test -tags=graphics ./...` で明示的に実行できる。
描画テストは実際にウィンドウを開き、数フレーム後に自動で閉じる。

### 準備済み Codex Cloud 環境

クラウド環境では、各シェルで準備済みのツールとライブラリを有効にする。
画面のない環境では Xvfb による仮想画面を使う。

```sh
source /workspace/.cloud-4k/env.sh
cd /workspace/4k_game
go build -mod=readonly -o bin/4k_game .
go test -mod=readonly ./...
xvfb-run -a -s '-screen 0 640x480x24 -nolisten tcp' go test -mod=readonly -tags=graphics ./...
xvfb-run -a -s '-screen 0 640x480x24 -nolisten tcp' go run -mod=readonly .
```

最後のコマンドは実行を続けるので、終了時は Ctrl+C を使う。
仮想画面での検証後も、入力精度・音声同期・CPU負荷の最終確認は Windows 実機で行う。

## パターン JSON と時刻変換

[`patterns/sample.json`](patterns/sample.json) が実際に読み込めるサンプル。
4レーン（0～3）、4/4拍子、1拍 = 960tick、1小節 = 3840tick を使用する。

```json
{
  "name": "sample",
  "length_ticks": 3840,
  "notes": [{"lane": 0, "tick": 0}, {"lane": 1, "tick": 480}]
}
```

`name`（文字列）、`length_ticks`（整数）、`notes`（配列）と、各ノーツの
`lane`・`tick`（整数）は必須で、null は不可。未知のフィールドや末尾の別 JSON
もエラーになる。空のノーツ配列は休符小節として許可する。
長さは 3840tick 固定、ノーツは `0 <= tick < 3840` に限定する。
同一レーン・同一tick の重複は拒否し、別レーンの同時tick は許可する。
読み込み時に tick 昇順、同tick はレーン昇順で整列する。

リポジトリ内の Go コードからは次のように利用できる（`internal` パッケージ）。

```go
pattern, err := chart.Load("patterns/sample.json")
if err != nil {
    return err
}
notes, err := chart.Schedule(pattern, 180)
if err != nil {
    return err
}
// notes は []chart.ScheduledNote。ID、Lane、AbsoluteTick、Time を持つ。
```

`chart` のインポート先は `github.com/usausamikann/4k_game/internal/chart`。
相対ファイルパスは実行時の作業ディレクトリを基準にする。
`Pattern` の `Note.Tick` はパターン内の相対tick。
`Schedule` は原本を変更せず、単一パターンを絶対tick 0 から配置した独立の配列を返す。
ID は整列後の順序で 0 から割り当て、入力順に依存しない。
直接 Go で作った Pattern も `chart.Validate` または `Schedule` で検証できる。

時刻は `tick × 60 / (BPM × 960)` 秒を各tickから直接計算する。
`time.Duration` のナノ秒単位に四捨五入（ちょうど半分は大きい側）し、
丸めた1tickの時間を加算しない。BPM は正の有限な `float64` のみ許可する。
計算ではその `float64` 値を正確な有理数として扱い、中間演算のオーバーフローと
整数tickの精度損失を避ける。Duration に収まらない結果はエラーになる。
`chart.TickToTime` は将来の絶対tickも扱えるが、圧縮・連結の機能はまだ持たない。
BPM180 の 480／960／3840tick はそれぞれ
166,666,667／333,333,333／1,333,333,333ns。

## 判定エンジン

`internal/judge` は描画や入力デバイスに依存せず、確定済みの
`[]chart.ScheduledNote` と時刻付き Press/Release イベントを受け取る。
単一パターンに長さを限定せず、将来連結した譜面の全体時刻もそのまま扱える。
連結そのものの処理は持たない。ID は譜面全体で一意にする。

```go
engine, err := judge.New(notes, judge.DefaultWindows())
if err != nil {
    return err
}
err = engine.Process([]judge.Event{
    {Lane: 0, Time: 0, Kind: judge.Press},
    {Lane: 0, Time: time.Millisecond, Kind: judge.Release},
}, time.Millisecond)
if err != nil {
    return err
}
results := engine.Results()
stats := engine.Statistics()
emptyPresses := engine.EmptyPresses()
```

`judge` のインポート先は `github.com/usausamikann/4k_game/internal/judge`。
時間はすべて同じゲーム開始基準の `time.Duration` とする。
エンジンは譜面をコピーし、結果と統計のスナップショットを返す。

| 判定ランク | offset の絶対値（標準設定） | 状態 | コンボ |
| --- | --- | --- | --- |
| P-CLEAN | 30ms 以下 | Hit | 継続 |
| CLEAN | 30ms 超、60ms 以下 | Hit | 継続 |
| BAD | 60ms 超、90ms 以下 | Hit | 継続 |
| MISS | 受付期限まで未判定 | Miss | 0 に戻す |

Pending は未判定状態で、ランクを持たない。状態とランクは別フィールドに保持する。
各 Hit は入力時刻、予定時刻、符号付き `Offset = 入力時刻 - 予定時刻` を記録する。
負は早い入力、正は遅い入力。MISS の InputTime/Offset は使用しない。
ランク別件数、現在・最大コンボ、空打ち件数を統計に持ち、空打ちイベントも保存する。
空打ちはノーツを消費せずコンボを変えない。Press はキー状態の立ち上がりのみ判定し、
Release までの繰り返し Press は空打ちにも数えない。

同じレーンの元ノーツ間の中点で担当区間を固定し、中点ちょうどは後続ノーツに所属する。
中点が半ナノ秒の場合は切り上げ、整数ナノ秒の入力で数学上の中点との前後関係を保つ。
先頭・末尾の片側は無限遠として扱うが、成功には最大判定幅も必要。
判定済みノーツの区間で押しても、未判定ノーツへフォールバックしない。
受付終了は右中点に達した時点、または予定時刻 + BAD 幅を超えた最初のナノ秒。
90ms ちょうどは成功可能だが、そこが右中点なら後続ノーツが担当する。
`ResolvedAt` は Hit の入力時刻または自動 MISS の確定時刻を表す。

`Process` はバッチ全体を検証してから時刻順に安定整列し、各イベント直前に自動 MISS、
全イベント処理後に現在時刻まで自動 MISS を進める。同時刻のイベントは渡した順序を保つ。
自動 MISS は期限順、同期限は予定時刻・レーン・ID の順で確定する。
現在時刻の逆行、前回の現在時刻より古いイベント、未来のイベント、不正なレーン・種別は
状態を変更せずエラーにする。未来のイベントは呼び出し側で次のバッチまで保持する。
補正や時刻の推定は行わず、同時刻の Press/Release の順序も呼び出し側で決める。

`Windows` で判定幅を変更でき、`0 < P-CLEAN < CLEAN < BAD` を必須とする。
同一レーンの重複tick・重複予定時刻、重複ID、負の譜面座標、
MISS期限が `time.Duration` に収まらない譜面を拒否する。
エンジンの操作は単一の goroutine から行う。

## ゲーム時計と描画対象

`internal/play.Clock` は Go の単調時計を含む `time.Time` を基準として、
実経過時間から `time.Duration` のゲーム時刻を直接計算する。
開始基準は譜面読み込み・ウィンドウ初期化後の最初の Update。
初期時刻は8拍分の負の時刻、時刻0が譜面開始で、フレーム数や更新回数を加算しない。
Updateで取得した時刻を描画にも共有し、Drawでは時計や判定状態を変更しない。

ウィンドウのフォーカスを失ったことを Update で検知すると時計を停止し、
復帰を検知すると再開する。停止中の実時間はゲーム時刻から除外する。
フォーカスがなくても Update を受け取るため `SetRunnableOnUnfocused(true)` を設定している。
停止・再開の履歴を保持するため、`Clock.At(eventTimestamp)` は将来の入力イベントでも
同じ時間軸へ変換できる。`time.Now()` 由来の単調時計を含む値をそのまま渡す。
実時間からの停止・再開操作は逆行を拒否し、操作は単一goroutineで行う。

`play.NewChart` は全体譜面をコピーした `PlayChart.Notes` と、時刻昇順の `Groups` を作る。
同時刻のノーツは一つの `NoteGroup` にまとめ、`NoteIndices` は元の共通配列の添字を保持する。
IDやレーン内添字とは別物で、ノーツ順序を並べ替えたり原本を変更したりしない。
生成後の Notes/Groups は読み取り専用として扱う。
Hit/Miss 表示更新では `IndexForID` でノーツIDから描画用添字を得る。
`DrainChanges` で新しく確定したノーツだけを取り出し、`Results()` のフレームごとのコピーは行わない。

ノーツ中心の座標は `y = judgeY - speed × (noteTime - now).Seconds()`。
既定では判定線がy=400px、ノーツ高が12px。準備時間と速度は独立している。
ノーツ矩形が画面 `[0, height]` に接する期間を表示対象とし、
表示開始は `ceil(noteTime - (judgeY + noteHeight/2) / speed)`、
終了は `floor(noteTime + (height - judgeY + noteHeight/2) / speed)` のナノ秒。
両端を含み、境界だけは初期化時の有理数計算で1nsの丸め誤差を防ぐ。

`View` が `Groups[first:end]` を管理する。初期化時は二分探索、通常再生では
退出・進入するグループに応じて二つの添字を前進させ、表示中のグループを維持する。
描画は表示対象Kノーツだけを処理する。大きな時間ジャンプでは飛ばしたグループも
一度走査するが、通常再生全体で各カーソルが各グループを通過するのは一回だけ。
描画中の全件走査や描画対象リストのフレームごとの生成は行わない。
将来のリトライでは `Clock.Reset(start)` と `View.Reset()` を呼ぶ。
時刻が後退した場合も View は二分探索で範囲を再初期化する。リトライUIは今回追加していない。

単体テストは同時刻グループと共通添字、ID対応、サイズを含む表示境界、
カーソルの維持・時間ジャンプ・リセット、異なるサンプリング頻度での位置一致、
停止・再開と過去イベント時刻の変換を検証する。
graphicsテストは実際のピクセルを確認し、4レーン・判定線・流下と
準備時間後の最初のノーツ到達、Drawが状態を変更しないことを検証する。

Step 3 の検証結果（Go 1.26.9 / Linux）：通常テスト、Xvfb上のgraphicsテスト、
`internal/play` のレース検出付きテスト（文カバレッジ97.8%）、`go vet` が成功。
Linux・Windows amd64 向けのビルドと、Xvfb上の `go run .` のウィンドウ起動も確認済み。
Windows実機での描画・フォーカス操作の確認は未実施。

## 入力・表示接続と追加設定（Step 4）

`main.Update` は Ebitengine の `IsKeyPressed` でキー状態を取得し、検出した Update の
`time.Now()` を全レーンの共通時刻として扱う。設定は `bindings.go` に分離し、
`-keys` で4つの異なる A〜Z キーをレーン順に指定できる（既定 D,F,J,K）。
`internal/input.Tracker` はデバイス非依存の Press/Release 遷移を生成し、
`play.Session` が時計変換・判定・IDベースの描画更新を接続する。
入力がない Update でも自動 MISS を進め、各入力直前に MISS、最後に現在時刻まで
MISS を処理する既存ルールを維持する。P-CLEAN / CLEAN / BAD は全てコンボ継続。

OSの実イベント時刻は取得しておらず、補間・推定も行わない。
既定240 TPSの更新周期の目安は4.17ms（60 TPSは16.67ms、120 TPSは8.33ms）。
Ebitengine v2.10.5の `IsKeyPressed` は「tick中に押されたキー」を検出できる場合もあり、
単なる瞬間的なOSキー状態と同一ではない。一方、同じtick内の複数のPress/Release往復を
現在のTrackerで全件復元できるわけではない。Releaseを独立に認識できないと次のPressを
数えられない場合もある。同じUpdateの複数キーには同じ検出時刻を付ける。
TPSは更新の目標値であり、OSイベント取得周期や実入力時刻の精度の保証ではない。
複数UpdateとOSイベントの取り込みが同じ描画フレームで処理される場合もある。
Windowsネイティブ入力へ移行する場合、取得部分を差し替え、`Clock.At` と
同じオフセットでイベントを判定時刻に変換する。Windows実機で入力精度・高速連打・
描画・フォーカス操作の最終検証が必要で、今回それらの実測は行っていない。

押しっぱなしは再判定しない。フォーカス喪失時は判定側へ Release を送り、
復帰時や起動直後の保持中キーは一度離すまで無視する。停止中は時計が進まず、
停止の実経過時間による MISS は発生しない。
Hit は描画から除外し、MISS は暗い赤で退出まで表示する。画面には直近ランク、
コンボ、空打ち件数、COUNT/SILENT/PLAY/PAUSED を簡素に表示する。
`Session.States` は描画配列の添字で管理し、判定結果は必ずノーツIDで反映する。
差分バッファとイベントバッファを再利用し、通常の無入力 Update は追加確保しない。

### タイミング調整 A/B

`-offset-a` と `-offset-b` はミリ秒の数値（小数・負数も可）、既定はともに0。
設定はプレイ開始前に固定する。正の値は遅らせる方向。

| 対象 | 時刻補正 | 元の予定時刻 T に対する実際の時刻 |
| --- | --- | --- |
| 描画・可視範囲 | visualNow = now − A | 判定線到達 T + A |
| 入力・判定・自動 MISS | judgeNow = now − A − B | 判定中心 T + A + B |
| カウント・音声 | 元の now | 拍時刻 T |

元の `ScheduledNote.Time`、中点境界、判定幅は変更しない。
入力イベントにも現在時刻と同じ補正を適用する。`Result.InputTime` は補正後の
判定時間軸の時刻で、`Offset` は `元の入力時刻 − (T + A + B)` に相当する。
Bだけでは描画は変わらず、A=-Bでは判定中心は元に戻る。Duration範囲外はエラー。

### SUDDEN+

`-sudden` は0.0〜40.0、0.1刻み。整数または小数1桁で指定する。
内部では0〜400の整数単位で保持し、壁の高さは `judgeY × value / 40.0`。
judgeY=400pxの場合、20.0=200px、30.0=300px、40.0=400px。
仕様中の30.0で全遮蔽という記述は、確認済みの指定式に従い40.0へ統一した。

ノーツを描画した後、4レーン内だけに不透明な壁を重ねる。判定線は最後に描画し、
全遮蔽でも見える。ノーツが壁の下へ進むと通常どおり見える。判定時刻、ゲーム時計、
BPM、スクロール速度には影響しない。上・下キーでの変更やHIDDEN+/LIFTは未実装。

### 固定8拍の準備タイムライン

COUNTの4拍、SILENTの4拍、その後に譜面時刻0という構成。Step 5ではCOUNTとPLAYの拍に音声を配置する。
BPM180の基準時刻は以下のとおり（表示上のミリ秒は丸めた値）。

| 区間 | 元のゲーム時刻 |
| --- | --- |
| カウント1〜4 | −2666.667 / −2333.333 / −2000 / −1666.667ms |
| 無音開始 | −1333.333ms |
| 譜面開始・将来のメトロノーム再開 | 0ms |

`Timeline.Preparation`、`CountTimes`、`SilenceStart` と `BeatTime(beat)` が基準時刻を定義する。
8拍や各拍の絶対tickから個別に `chart.TickToTime` で計算し、丸めた1拍を加算しない。
停止・再開を含む元の時計を使うため、音声タイムラインにA/Bは適用しない。
カウントと無音中もノーツはスクロールする。最初のノーツの早側判定幅や負のオフセットも
既存の判定ルールどおり受付するため、時刻0は譜面・音声の基準であり、早側入力を
独立したゲートで無効にはしない。準備中の受付幅外の押下は空打ちとして記録される。

Step 4 の検証結果（Go 1.26.9 / Linux）：通常テスト、Xvfb上のgraphicsテスト、
input/judge/playのレース検出付きテスト、`go vet`、Linux/Windows amd64ビルドが成功。
入力・判定パッケージの文カバレッジは100%。GUIテストは制御されたキー状態を使い、
壁の遮蔽・レーン外の保持・判定線・Hit非表示・MISS色を実際のピクセルで確認する。
実機の打鍵とOS入力遅延を検証したものではない。Step 4の時点では音声は未実装。

## TPS切替と入力デバッグ表示（Step 4.5）

`-tps` は60 / 120 / 240のみ受け付け、暫定の既定値は240。
起動前に `ebiten.SetTPS` へ設定し、不正値は丸めずにエラーにする。
TPSを変えても判定幅・中点・譜面予定時刻・BPM・実時間ベースの時計・速度は変えない。
描画FPSの上限やVSync設定は変更しない。

`-debug-input` を指定した場合のみ、画面右側のレーン外に以下を表示する。

- Target TPS：設定した更新目標値。
- TPS / FPS：`ebiten.ActualTPS()` / `ebiten.ActualFPS()` の参考値。
- Last judge：直近の確定判定。
- Last HIT offset：直近HitのA/B補正後の `Result.Offset` を符号付きmsで表示。
- Empty presses：空打ち回数。

負の誤差はFAST、正はLATE、0はON TIME。例は `-12.3ms FAST` / `+8.7ms LATE`。
MISSには入力誤差がないため、MISSのoffsetは生成しない。Last judgeがMISSになっても
Last HIT offsetには最後のHitの誤差を残す。Hit未発生なら `- (no Hit yet)` と表示する。
Sessionは判定差分を受け取る際に直近Hitを一件保持し、全結果配列をコピーしない。

表示文字列と実測TPS/FPSの取得は実時間で最大4回/秒に制限し、描画では文字列を再利用する。
停止中も実時間の間隔で更新する。無効時は実測値取得・デバッグ文字列生成・追加描画をせず、
従来の簡素なHUDを維持する。ActualTPS/FPSは揺れや起動時の0を含む参考値で、
OS入力のタイムスタンプ精度やCPU使用率そのものを測定する値ではない。

Windowsのリポジトリルートから、同じexe・譜面・BPM・A/B・入力デバイスで比較する。
各コマンドの比較が終わったらウィンドウを閉じて次を実行する。

```powershell
go build -o bin/4k_game.exe .
.\bin\4k_game.exe -tps 60 -debug-input
.\bin\4k_game.exe -tps 120 -debug-input
.\bin\4k_game.exe -tps 240 -debug-input
```

CPU使用率、認識漏れ、FAST/LATEの偏り、TPSの安定性、描画とフォーカス復帰を実機で確認する。
デバッグ表示自体の負荷もあるため、必要に応じて無効時と比較する。
クラウド上のテストは実機の打鍵精度・TPS安定性・CPU負荷の検証を代替しない。
240 TPSが常に優れる／十分と決めず、Windows実機で問題を確認してから
高速イベント方式への移行や既定値を再評価する。今回はRaw Input等を導入しない。

テストでは同一の人工イベント時刻列を異なるTPS相当の更新バッチに分割し、
全判定・offset・空打ち・統計と譜面時刻の一致を確認する。
これは同じ物理打鍵が各TPSで同じように検出されることを証明するテストではない。

Step 4.5のクラウド検証（Go 1.26.9 / Linux）は通常・graphicsテスト、`go vet`、
inputdebug/playのレース検出、Linux/Windows amd64ビルドが成功。
新しいinputdebugパッケージの文カバレッジは100%。GUIテストでは実際のTPS設定値、
デバッグ表示の有無、直近MISSと過去のHit誤差の分離を確認している。
Windows実機の入力精度・TPS安定性・CPU使用率は未検証。


## メトロノーム（Step 5）

起動時の `-metronome-sound` で5候補のいずれかを選択する。暫定既定は `glass_002`。

```sh
go run . -bpm 180 -metronome-sound glass_002
go run . -bpm 220 -metronome-sound switch8 -debug-audio
```

| 選択値 | 元のファイル | 起動時に除く先行部分（48kHzでの測定） |
| --- | --- | --- |
| switch8 | switch8.ogg | 約113.167ms |
| switch19 | switch19.ogg | 約119.583ms |
| switch29 | switch29.ogg | 約113.250ms |
| glass_002 | glass_002.ogg | 0ms |
| pluck_001 | pluck_001.ogg | 0ms |

元のKenney Interface Sounds音源はCC0 1.0。出典・ライセンスは
[音源README](assets/audio/README.md)を参照。OGGは変更せず `go:embed` でexeへ同梱するため、
音源を探す作業ディレクトリには依存しない（譜面JSONは従来どおり相対パス）。
ファイル名／選択値を識別子に使い、不一致のあるTITLEタグで音を選ばない。

`internal/audio` が選択したOGGを起動時に一度だけデコードする。
Ebitengine v2.10.5 の Vorbis デコーダとリサンプラでmono/stereoの44.1kHz原本を
48kHz・stereo・float32 little-endian PCMへ統一する。
デコード上限は2秒。ピークの10%に最初に達するサンプルを主要な立ち上がりとし、
その1ms前から保持する再現可能な処理でswitch系の長い先行部分を補正する。
トリミングした先端の0.5msだけフェードし、主要なアタックとピークを保持する。
保持する1ms分は意図的な余裕で、知覚上の立ち上がりやスピーカー遅延が厳密に0になるとは主張しない。
原本の音量差は原則維持し、出力ピークが0.8以下になる固定ゲインと重なり分の余裕を設ける。
各拍の音色・ゲイン・ピッチは同じで、小節頭／PLAY頭にアクセントは付けない。

COUNTで4クリック、次の4拍は無音、PLAYの時刻0から4分音符間隔で鳴らし続ける。
クリックは `round(n × 60 × 48000 / BPM)` の絶対フレーム位置へ配置し、
半サンプルちょうどは上へ丸める。n=0〜3がCOUNT、4〜7は発音なし、8以降がPLAY。
BPMは既存のfloat64値を正確な有理数として扱い、長時間の位置計算も整数オーバーフローを検出する。
丸めた拍間隔の反復加算はしない。音声配置の丸め誤差は各拍で最大0.5サンプルであり、
それとは別に、デバイスと単調時計の速度差・開始遅延・バッファ遅延は実機で検証する。

PCMは単一の連続 `io.Reader` が必要分だけ生成し、Playerも一つだけ使う。
数分分の無音配列、拍ごとのPlayer生成、UpdateやTickerによる拍トリガーは使わない。
通常は波形の重なりをミックスする。長いCOUNT末尾がSILENTへはみ出すBPMでは境界前の
最大2msでフェードし、SILENTをゼロPCMに保つ（ストリームの再スタートはしない）。
メモリは選択音源のPCMと固定32音の重なり領域、Playerのバッファに限定される。
重なりが32音を超えるような極端なBPM、サンプル位置の範囲外はエラーにする。
サンプル生成の定常処理で配列を生成せず、絶対位置計算の小さな整数演算は拍ごとにだけ行う。

### 時計との対応と停止・復帰

音声デバイスのReadyとフォーカスを待ち、UpdateがPlay呼び出し前に取得した `wall` を
SessionのClockの初期基準にする。Play完了時刻にClockを合わせる方式ではない。ストリーム位置0を `-Timeline.Preparation` に対応付ける。
8拍目の絶対サンプル位置が譜面時刻0へ最大0.5サンプル（48kHzで約10.4µs）とDuration丸め（1ns未満）の量子化差で対応する。
COUNT内のTimelineとの対応も、ストリーム起点を共通にして1サンプル以内となる。
A/B、スクロール速度、SUDDEN+、TPSを音声BPMやサンプル位置へ適用しない。

フォーカス喪失では `PauseAndStopReading` で読み取りと再生を止め、SessionのClockも停止する。
復帰では保持したPCMバッファの続きをPlayし、COUNTのやり直し・拍の再挿入・Seekはしない。
アプリ終了時も読み取りを停止する。入力の保持キー抑制や判定ルールは従来どおり。

`-debug-audio` はレーン外に最大4回/秒、`Player.Position() - Preparation` と
`Clock-audio` の差、要求Playerバッファ50msを表示する。`-debug-input` と併用できる。
Positionはバックエンドが推定する進行位置で、物理的なスピーカー／ヘッドホンの出音時刻ではない。
50msはPlayerへ要求する値であり、OS・デバイス・Contextの全遅延の測定値ではない。
起動・停止コマンドから実出音までのバッファ遅延や長時間の位相差は残り得る。
毎フレームPositionへ時計を追従させたり、Seekや再生速度補正で位相を追いかけたりはしない。
実機で観察した映像・音声・入力のずれはA/Bの意味を維持して個別に評価する。

### 検証方法

通常の `go test ./...` は音声デバイスなしで、全5音源のデコードと立ち上がり保持、
48kHz化、安全な振幅、全拍の波形一致、長時間の絶対配置、COUNT/SILENT/PLAY、
読み取りサイズ非依存、停止・復帰後のPCM連続性と時計対応を検証する。
`go test -tags=graphics ./...` で従来の描画・入力回帰テストも実行できる。

実際のデバイス初期化・PCM消費・フォーカス停止経路を含む追加テストは、利用できる
画面と音声バックエンドがある環境で次を実行する（通常のgraphicsタグとは別実行）。

```sh
go test -tags=audiointegration -run TestAudioBackend -count=1 -timeout=30s .
```

クラウドではXvfbとALSA null出力でこの経路を検証できるが、null出力は実スピーカーの
音質・速度・レイテンシ検証にはならない。Windows実機では全5候補の実出音、4クリック→
4拍無音→均一クリック、BPM180/220の聴感と同期、Alt+Tab後の位相・重複の有無、
長時間ドリフト、実際のCPU／メモリ使用量、スピーカー・ヘッドホン遅延を別途確認する。
キー別打鍵音・音声設定UIはStep 5.5以降で、今回追加していない。

Step 5のクラウド検証（Go 1.26.9 / Linux）は、通常・graphicsテスト、音声パッケージの
レース検出付きテスト（文カバレッジ92.3%）、Xvfb＋ALSA nullの音声統合テスト
（レース検出付き）、`go vet`、Linux/Windows amd64ビルド、`go mod verify` が成功。
元の5音源バイナリと既存memo、判定・入力・時計のソースは変更していない。


## 音声同期の診断（Step 5.1）

`-debug-audio` を付けると、右側のレーン外に次を表示する。
`-debug-input` との併用も可能で、入力診断は上部、音声診断はその下に表示する。

| 表示 | 意味 |
| --- | --- |
| Clock | A/B補正前のゲームClock時刻（秒） |
| Pos | `Player.Position() - Preparation`（秒） |
| Clock-audio | `Clock - Pos`（符号付きms）。正ならClockの値が先行 |
| Delta | 現在の差から、連続再生区間の初回サンプルの差を引いた値（ms） |
| Segment | 現区間の初回サンプルからの経過秒。停止中は凍結し、再開後は取り直す |
| Initial Play | 初回 `Play()` の所要時間（ms） |
| Last Resume | 直近のフォーカス復帰時の `Play()` の所要時間（ms） |
| Last Pause | 直近のフォーカス喪失時の `PauseAndStopReading()` の所要時間（ms） |
| Initial begin-wall / end-wall | 初回Play呼び出し開始／終了と、そのUpdateの `wall` との差（ms） |
| Player buffer | Playerへ要求するバッファ長（50ms） |

未発生イベントや未取得の区間基準は `N/A`。実測0msと未実行を区別する。
初回Play・Pause・Resumeの既存の状態遷移でのみ、呼び出し直前／直後に `time.Now()` を取得し、
単調時計成分を保った `time.Time.Sub` で所要時間やwallとの差を計算する。
Pauseは読み取り完了待ちを含む呼び出し全体を測る。終了時のCloseはフォーカス停止の記録に含めない。
追加のPlay/Pause操作は行わない。

`Transport.DiagnosticSnapshot()` は、初回・直近Resume・直近Pauseの各呼び出し前後時刻とwall、
直近イベント種別・件数、および直近の参考位置サンプルを値コピーで返す。
記録順序は「呼び出し前時刻 → API呼び出し → 呼び出し後時刻 → イベント記録 → Session更新」。
Sessionは従来どおり渡されたwallでClockの開始・停止・再開を処理し、計測値を利用しない。
画面のbegin/end-wallは初回Playの値で、Resume/Pauseのwallとの差もSnapshotに保存する。

Positionサンプリングと文字列更新は最大4回/秒で、結果文字列を再利用する。
復帰時に区間基準を無効化し、次の定期サンプルで新しい基準を取る。
停止時間は区間経過時間へ加算しない。イベント記録は固定個数で、履歴を無制限に蓄積しない。
診断無効時は前後時刻取得・定期Positionサンプリング・診断文字列生成を行わない。

これらは**API呼び出し時間と推定再生位置の参考値**であり、実出音遅延を直接測定しない。
Play区間の中点をPCM出力開始と仮定しない。Positionの更新間隔・推定上の揺れや
バッファ遅延が差に影響するため、Deltaの変化だけで音声デバイスのクロックドリフトを証明できない。
Clockの基準変更、Seek、速度補正、Positionへの追従など、同期方式の変更は行っていない。

### Windows実機での確認

PowerShellでリポジトリのルートから実行する。

```powershell
go build -o bin/4k_game.exe .
.\bin\4k_game.exe -bpm 180 -debug-audio
.\bin\4k_game.exe -bpm 220 -debug-audio -debug-input
```

BPM180/220の起動直後にInitial Playとbegin/end-wallを記録する。
Alt+Tabで停止・復帰し、Last Pause/Resume、停止中のSegmentの凍結、復帰後のDelta基準リセットを確認する。
1〜5分連続再生してClock-audioとDeltaの変化を観察し、`-debug-input` 併用時の表示と負荷も比較する。
Windows実機の数値・実出音遅延・長時間の挙動・CPU使用率はクラウドのテストでは検証できない。

単体テストではFake Playerと注入した時刻を使い、実時間Sleepに依存せず、Ready/フォーカス待ち、
初回一度のみ、停止・再開の測定、区間の凍結／基準リセット、4Hz制限、無効時の非計測、N/Aを検証する。

Step 5.1のクラウド検証（Go 1.26.9 / Linux）は、`go test ./...`、Xvfbでの
`go test -tags=graphics ./...`、`go test -race ./internal/...`、`go vet ./...`、
Linux／Windows amd64ビルドが成功。Xvfb＋ALSA nullの音声統合テストもレース検出付きで成功し、
初回wallの保持、初回／停止／再開イベントと区間基準、入力診断との併用を確認した。
null出力での成功は、Windows実機の出音や同期精度を測定したことを意味しない。



## キー音（Step 5.5）

判定結果から独立して、既存Trackerが検出した実際のPressを1回だけ発音する。
空打ち・COUNT・SILENT・PLAY・Hit/BADに共通。自動MISS、Release、長押し、
OSリピート、非フォーカス、音声デバイス準備待ち、復帰直後の保持キーでは発音しない。
SILENTは自動メトロノームのみ無音。入力APIのサンプリング間に完結したPress/Releaseを
取りこぼす既存制約は残る。

| モード | 動作 |
| --- | --- |
| `off`（既定） | キー音なし。メトロノームは動作 |
| `unified` | `-keysound-sound` の1音を全レーンへ割り当て |
| `per-lane` | `-keysound-lanes` の4音をレーン0〜3へ割り当て。重複可 |

キー音IDは `bong_001`, `drop_002`, `drop_004`, `glass_005`, `toggle_001`。
メトロノーム用IDとは別の一覧。`-keys`を変更してもレーンへの音色割り当ては維持される。

```powershell
go run . -keysound-mode unified -keysound-sound drop_002 -keysound-volume 50 -metronome-volume 50
go run . -keysound-mode per-lane -keysound-lanes bong_001,drop_002,glass_005,toggle_001 -keys D,F,J,K
go run . -keysound-mode per-lane -keysound-lanes drop_004,drop_004,drop_004,drop_004 -metronome-sound glass_002
```

`-keysound-sound` の既定は `drop_002`、`-keysound-lanes` の既定は4レーンとも `drop_002`。
Offでは両方の音色設定を使用せず、Unifiedではlanesを使用せず、Per-laneではsoundを使用しない。
**使用しない引数も含めてすべて検証**し、不明なID、4個以外のlanes、未知のmode、
0〜100以外の音量は説明付きエラーとする。レーンID前後の空白は除去する。

キー音とメトロノームの音量は独立で、それぞれ0〜100%、初期値50%、0%は無音。
キー音音量は4レーン共通。100%は設計上の基準最大値で、元音源の5倍増幅ではない。
共有 `internal/audio.Settings` に百分率を保持し、内部ゲインと分離している。
メトロノームは既存PCMを変更せずPlayerゲインを `0.25 × 音量/100` とする。
キー音は共通ゲイン0.5で合成し、`0.70 × tanh(合成値/0.70) × 音量/100` の
滑らかな飽和処理で出力する。供給音源間の相対バランスを維持し、極端な重なりでは圧縮がかかる。
両方100%でもデジタル合成の絶対値は最大0.95以下となる設計。実機の音割れ・聴感は別途確認が必要。
従来と音量基準が変わるため、試聴はOS側も小音量から始めること。

キー音は32ボイスミキサーを廃止し、専用の連続Playerから**事前合成PCMを高々1つ**再生する。
起動時に現在の4レーンの音色配置について、4bitマスクの15種類（無音の`0000`を除く）の
PCMを生成する。bit0〜3はレーン0〜3に対応。5^4通りの音色配置を生成することはない。
Off／音量0ではデコード・合成を省略する。必要な音源だけ一度48kHz stereo F32へ変換し、
同じIDのデコード結果を共有する。合成後は元のデコードPCMへの参照を保持しない。
同じ音源を複数レーンに指定した場合も、選ばれたレーンの数だけ加算する。
音源長の違いは無音パディングし、上記の音量・飽和処理とF32バイト変換を事前に済ませる。
提供済みの加工音源には追加の先頭トリミングを行わない。

同じ `Session.UpdateAt()` の複数Pressを1つのマスクへまとめ、同時押しPCMを1回選択する。
次のUpdateのPressでは、**旧キー音を即時カットし、新しい同時押しPCMを先頭から再生**する。
Pressなし／ReleaseだけのUpdateは現在の再生を継続し、PCM終了後は無音になる。
フェード／クロスフェードや、時間差Press間の余韻の重なりはない。
任意長Readの途中でPressが来た場合に限り、すでに一部を返したstereoフレームの残り
（最大7バイト）を完了し、次のフレームから新しいPCMへ切り替えてバイト境界を守る。

Pressごとの合成・デコード・Player生成・Rewind・大きなコピー・追加メモリ確保はない。
Readerは事前生成したバイト列を最大256バイトずつ短いロック内でコピーする。
毎フレームの32スロット走査と`tanh`計算はなくなる一方、保存PCMは元の音源のみの方式より増える。
保持量は各マスクに選ばれた最長音源のバイト数の合計で、最大15本。連打回数では増加しない。

`KeyMixer` は `KeyStream` に改名した。Seekは累積Readバイト位置を管理し、
`Seek(0, io.SeekCurrent)` の問い合わせでは再生状態・部分フレームを変更しない。
`Seek(0, io.SeekStart)` は再生中PCM・部分フレーム・位置をリセットする。
無限ライブストリームには末尾や過去の打鍵履歴がないため、他のSeekは
`errors.ErrUnsupported` を含むエラーとし、状態を変更しない。
Ebitengine v2.10.5はPlayerの初回使用（Play／SetPosition）時に現在位置を問い合わせるため、
旧Seekがこの問い合わせを拒否して再生開始エラーになる問題を修正した。

既存Audio Contextを共有し、メトロノームの50msバッファ、連続PCMと拍配置、Clock、
A/B、TPS、Step 5.1診断は維持する。キー音Playerの要求バッファは10ms。
Readerの切り替えはすでに先読みされたPlayer／デバイスのPCMを取り消さないため、
実際の出音にはバッファ／OS／入力周期分の遅延が残る。10msは物理遅延の保証値ではない。
フォーカス喪失時はキー音を停止してRewindし、Playerのキューと再生中PCM・部分フレームを破棄する。
メトロノームは従来どおりバッファを保持して一時停止する。

音源は [Kenney UI Audio](https://kenney.nl/assets/ui-audio)（[CC0 1.0](https://creativecommons.org/publicdomain/zero/1.0/)）由来。
ユーザー提供ZIPの音量調整・末尾無音トリミング済みOGGを `assets/audio/` へ埋め込んでいる。
元音源・代替音の生成は行っていない。ファイルがない場合は提供ZIPから同名の5 OGGを配置して再ビルドする。
古い引き継ぎメモは履歴として保持し、確定仕様は
[Step 5.5実装メモ](memo/4k_game_step55_implemented_20261019.md)を参照。

### Windows実機での受入確認

**OS／アプリとも小音量から試聴**し、必要に応じ徐々に上げる。Windowsネイティブで
`go build -o 4k_game.exe .` を実行し、次を確認する。

1. Off／Unified／Per-laneで全5音色を選び、単音、重複割り当て、`-keys A,S,K,L` のレーン対応を確認。
2. COUNT・SILENT・PLAYで空打ち、ノーツのHit/BADを試す。Release、長押し、自動MISSでは鳴らないこと。
3. `-bpm 180` と `-bpm 220` で16分の同一レーン連打、0→1→2→3の階段、4レーン同時押しを各1分以上試す。
   新しいPressで旧音が切り替わり、同時押しは全レーン分聞こえること。欠落・不快なクリックノイズ・遅延悪化を記録する。
   BPM180の24分間隔（約55.56ms）の7連打も全5音源で確認する。
4. 各音量0／50／100を組み合わせ、一方を0にして他方の独立性を確認。
   両方100で同時押しとメトロノームが重なったとき、音割れ・圧縮の不快感を確認。
5. キー音の残響中にAlt+Tabし、停止を確認。キーを保持して復帰しても古い音や保持キーから発音せず、
   一度離してから押すと発音すること。メトロノームのCOUNT／SILENT／PLAYの継続も確認。
6. `-debug-input -debug-audio` を付けてOffと有効時を比較し、TPS、FPS、タスクマネージャーのCPU／メモリを
   開始時と10分後に記録。キー音による継続的なメモリ増加や周期的な停止がないこと。
7. 同じ出力デバイスで打鍵と出音を外部録音／高速度撮影し、相対遅延とばらつきを比較。
   `Player.Position()`や10ms設定だけで物理出音遅延を断定しない。

Step 5.5再生方式改修のクラウド検証では `go test ./...`、`go test -race ./internal/...`、
`go vet ./...`、Windows amd64クロスビルドが成功。全16マスク、重複音源・長さ差、
実5音源の旧同時押し計算との数値一致、同一Updateの合成、別Updateのカット、Seek、部分Read、
音量・有限値、連打時の追加確保なし、フォーカス、既存メトロノーム／同期診断を確認した。
`go test -tags=graphics -run TestKeyAudioWiring .` も成功。
全graphicsテストと音声統合テストはwindow system不足（`/dev/fb0`なし）で失敗し、描画・実デバイス統合は未検証。
Linux／WSLおよびWindows実機の出音・クリックノイズ・レイテンシ・CPU／メモリ・Alt+Tabは未検証。
ヘッドレステストやReaderベンチマークは物理出音の遅延を示す結果ではない。

## 実装計画

[実装方針と実装順](memo/4k_game_implementation_plan_20261012.txt)を参照。
Step 1 は [2026-10-13 の引き継ぎ方針](memo/4k_game_step1_handoff_20261013.txt)を優先。
Step 2 の判定名・コンボ仕様は今回の指定（P-CLEAN / CLEAN / BAD / MISS）を採用。
Step 4.5 は [入力計測と将来イベント設計メモ](memo/4k_game_step45_input_measurement_and_future_event_design_20261013.txt)を参照。
Step 5 は [メトロノーム引き継ぎ仕様](memo/4k_game_step5_metronome_handoff_20261018.md)を優先。
Step 5.5は[確定仕様と実装メモ](memo/4k_game_step55_implemented_20261019.md)を参照。
Step 6は結果画面、Step 7はパターン連結。既存の `memo/` はそのまま保持する。
