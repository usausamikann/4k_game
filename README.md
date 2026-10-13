# 4K Game

Go + Ebitengine で作る固定4レーンのリズムゲーム。
現在は実装計画の Step 4 まで実装済み。単一パターンの JSON 読み込み・検証と
BPM による時刻変換を `internal/chart`、判定エンジンを `internal/judge` に用意している。
アプリは 640 × 480 のウィンドウに4レーン・判定線と流下する TAP ノーツを表示する。
ゲーム時計と描画対象管理は GUI 非依存の `internal/play` に用意している。
D/F/J/K の入力判定、判定表示、タイミング調整 A/B、上部遮蔽 SUDDEN+ に対応。
音声・パターン連結・本格的な設定画面と結果画面は今後実装する。

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
  libxcursor-dev libxi-dev libxinerama-dev libxxf86vm-dev libgl1-mesa-dev
```

WSL でウィンドウを表示するには WSLg などが必要。
音声の実装時には ALSA の開発ライブラリも必要になる。

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
左から D/F/J/K を押すと判定し、押下中はレーン下部を強調表示する。
ウィンドウを閉じると終了する。Windows 用の実行ファイルを作る場合は
`go build -o bin/4k_game.exe .` を使う。

設定は個別に指定できる。BPMと速度は正の有限値を使う。
準備時間は固定8拍に変更したため、以前の `-prepare` は廃止（指定するとエラー）。

```sh
go run . -pattern patterns/sample.json -bpm 120 -speed 300 -offset-a 10 -offset-b -5 -sudden 20.0
go run . -keys A,S,L,P
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

OSの実イベント時刻は取得しておらず、補間・推定も行わない。既定の更新頻度60Hzでは
検出間隔は約16.7msで、負荷等によっても変わる。Update間で Press/Release が完結した
入力は取りこぼし得る。同じ Update の複数キーには同じ検出時刻を付ける。
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
| カウント・将来の音声 | 元の now | 拍時刻 T |

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

COUNTの4拍、SILENTの4拍、その後に譜面時刻0という構成。今回は音声を出さない。
BPM180の基準時刻は以下のとおり（表示上のミリ秒は丸めた値）。

| 区間 | 元のゲーム時刻 |
| --- | --- |
| カウント1〜4 | −2666.667 / −2333.333 / −2000 / −1666.667ms |
| 無音開始 | −1333.333ms |
| 譜面開始・将来のメトロノーム再開 | 0ms |

`Timeline.Preparation`、`CountTimes`、`SilenceStart` と `BeatTime(beat)` を Step 5 に利用できる。
8拍や各拍の絶対tickから個別に `chart.TickToTime` で計算し、丸めた1拍を加算しない。
停止・再開を含む元の時計を使うため、音声タイムラインにA/Bは適用しない。
カウントと無音中もノーツはスクロールする。最初のノーツの早側判定幅や負のオフセットも
既存の判定ルールどおり受付するため、時刻0は譜面・音声の基準であり、早側入力を
独立したゲートで無効にはしない。準備中の受付幅外の押下は空打ちとして記録される。

Step 4 の検証結果（Go 1.26.9 / Linux）：通常テスト、Xvfb上のgraphicsテスト、
input/judge/playのレース検出付きテスト、`go vet`、Linux/Windows amd64ビルドが成功。
入力・判定パッケージの文カバレッジは100%。GUIテストは制御されたキー状態を使い、
壁の遮蔽・レーン外の保持・判定線・Hit非表示・MISS色を実際のピクセルで確認する。
実機の打鍵とOS入力遅延を検証したものではない。音声は今回出力しない。

## 実装計画

[実装方針と実装順](memo/4k_game_implementation_plan_20261012.txt)を参照。
Step 1 は [2026-10-13 の引き継ぎ方針](memo/4k_game_step1_handoff_20261013.txt)を優先。
Step 2 の判定名・コンボ仕様は今回の指定（P-CLEAN / CLEAN / BAD / MISS）を採用。
次は Step 5（音声・メトロノーム）。既存の `memo/` はそのまま保持する。
