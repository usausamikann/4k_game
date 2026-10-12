# 4K Game

Go + Ebitengine で作る固定4レーンのリズムゲーム。
現在は実装計画の Step 0 まで実装済みで、640 × 480 のウィンドウを表示する。
譜面・入力判定・音声・プレイ画面は今後実装する。

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

描画テストは実際にウィンドウを開き、数フレーム後に自動で閉じる。
現在のテストは起動・描画の検証用で、ゲームの判定処理を検証するものではない。

### 準備済み Codex Cloud 環境

クラウド環境では、各シェルで準備済みのツールとライブラリを有効にする。
画面のない環境では Xvfb による仮想画面を使う。

```sh
source /workspace/.cloud-4k/env.sh
cd /workspace/4k_game
go build -mod=readonly -o bin/4k_game .
xvfb-run -a -s '-screen 0 640x480x24 -nolisten tcp' go test -mod=readonly ./...
xvfb-run -a -s '-screen 0 640x480x24 -nolisten tcp' go run -mod=readonly .
```

最後のコマンドは実行を続けるので、終了時は Ctrl+C を使う。
仮想画面での検証後も、入力精度・音声同期・CPU負荷の最終確認は Windows 実機で行う。

## 実装計画

[実装方針と実装順](memo/4k_game_implementation_plan_20261012.txt)を参照。
次は Step 1（譜面データ・BPM変換）。既存の `memo/` はそのまま保持する。
