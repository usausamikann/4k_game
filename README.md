# 4K Game

Go + Ebitengine で作る固定4レーンのリズムゲーム。
現在は実装計画の Step 1 まで実装済み。単一パターンの JSON 読み込み・検証と
BPM による時刻変換を `internal/chart` に用意している。
アプリは引き続き 640 × 480 の最小ウィンドウを表示する。
入力判定・音声・ノーツ描画・パターン連結は今後実装する。

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

`go test ./...` は画面なしで譜面の単体テストを実行する。
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

## 実装計画

[実装方針と実装順](memo/4k_game_implementation_plan_20261012.txt)を参照。
Step 1 は [2026-10-13 の引き継ぎ方針](memo/4k_game_step1_handoff_20261013.txt)を優先。
次は Step 2（中点判定エンジン）。既存の `memo/` はそのまま保持する。
