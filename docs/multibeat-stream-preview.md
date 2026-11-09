# 複数拍のstream譜面をJSONで読み込んで試遊する

このブランチは、PR #18のstream生成器をゲームの再生モードへ直接組み込むのではなく、
**独立したJSONファイルを既存の `-pattern` 引数から読み込む**ことを目的とする。

## 変更点

既存 `chart.Load()`, `chart.Schedule()` を複数拍対応にした。

```json
{
  "name": "example",
  "time_scale": 0,
  "length_beats": 2,
  "notes": [
    {"lane": 0, "position": [0, 4]},
    {"lane": 1, "position": [4, 4]},
    {"lane": 2, "position": [7, 4]}
  ]
}
```

- `length_beats` は任意の正の整数（最大100万拍）。省略した従来のJSONは1拍扱い。
- `position:[a,b]` は譜面の先頭からの**絶対拍位置 `a/b`**。
  従来の1拍JSONと表現は同じだが、複数拍では `a >= b` を許す。
- 複数拍の場合、`0 <= a/b < length_beats`。位置は960tickへ正確に変換できること。
- `time_scale` は従来どおり0または1。1なら譜面全体の時間も2倍速になる。
- 旧形式の`length_ticks`、ノーツの`tick` は引き続き読み込み不可。
- ゲーム側の処理は従来と同じ `chart.Load` → `chart.Schedule` → `play.NewSession`。
- 終了時刻は`length_beats`から計算できるが、**曲終了画面や自動終了はまだない**。
  プレイ後はウィンドウを閉じて終了する。

## Windowsでの手順

1. `feat/multibeat-stream-preview` ブランチをチェックアウトする。
2. チャット添付の`stream_120bpm_90sec_seed42.json`をダウンロードし、
   リポジトリの`patterns/`フォルダへ配置。
3. 次のコマンドで実行。

```powershell
go run . -pattern patterns/stream_120bpm_90sec_seed42.json -bpm 120 -speed 240 -keys 'A,D,;,]'
```

この譜面はBPM120・180拍・1.5分・1059ノーツ・seed42、
tierはlight/middle/heavyが60拍ずつ。連続16分配置で縦連なし。

## 別の譜面を生成する場合（ゲーム本体のCLIには組み込まない）

`cmd/export-stream` はゲームから独立したコマンド。

```powershell
go run ./cmd/export-stream -bpm 120 -minutes 1.5 -seed 123 -out patterns/stream-preview.json
go run . -pattern patterns/stream-preview.json -bpm 120
```

`-bpm`はJSONには格納されない。**出力時と再生時のBPMを一致させること**。
`-out`の親フォルダは予め作成する。生成器の設定は初期値（tier 1:1:1、
直近8拍の左右差8以下）。ゲーム本体への`-generate-stream`モード追加は行わない。

## テスト

```powershell
go test ./internal/chart ./internal/streampool ./cmd/export-stream
go test ./...
```

チャット添付の試遊JSONは独立に検証済み：720スロット全埋まり、
隣接マスクAND=0、通常の同時押し位置制約または2-2-2-2例外、
左右負荷ウィンドウ、tier比率に違反なし。

## キーバインド

デフォルトは従来どおりD/F/J/K。試遊ではフラグで変更可能：

```bash
go run . -pattern patterns/stream-preview.json -bpm 120 -keys 'A,D,;,]'
```

WSLのbashでは`;`がシェルのコマンド区切りにならないよう**全体を引用符で囲む**。対応キーは英字A〜Zと`;`・`]`。エイリアス`SEMICOLON`・`BRACKETRIGHT`も使える。
キー名はEbitenのキー定数（キー配列によって実際の刻印と異なる可能性あり）。
