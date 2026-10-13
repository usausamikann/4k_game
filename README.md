# 4K Game

Go + Ebitengine で作る固定4レーンのリズムゲーム。
現在は実装計画の Step 2 まで実装済み。単一パターンの JSON 読み込み・検証と
BPM による時刻変換を `internal/chart`、判定エンジンを `internal/judge` に用意している。
アプリは引き続き 640 × 480 の最小ウィンドウを表示する。
入力デバイスとの接続・音声・ノーツ描画・パターン連結は今後実装する。

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
ウィンドウを閉じると終了する。Windows 用の実行ファイルを作る場合は
`go build -o bin/4k_game.exe .` を使う。

`go test ./...` は画面なしで譜面と判定の単体テストを実行する。
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

## 実装計画

[実装方針と実装順](memo/4k_game_implementation_plan_20261012.txt)を参照。
Step 1 は [2026-10-13 の引き継ぎ方針](memo/4k_game_step1_handoff_20261013.txt)を優先。
Step 2 の判定名・コンボ仕様は今回の指定（P-CLEAN / CLEAN / BAD / MISS）を採用。
次は Step 3（4レーン描画とゲーム時計）。既存の `memo/` はそのまま保持する。
