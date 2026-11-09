# stream譜面生成器（Draft / 2026-11-09）

`internal/streampool` の454実配置を使い、**固定長の1拍パターンの列**を生成する。
この段階ではゲームの再生・スケジューラ・キー音・入力判定を変更しない。
各拍の選択結果は `Sequence.PatternIndices` に `Pool.All` のインデックスとして保持する。

## 2つの索引 G と H

`BuildIndex(pool)` は、選択の前に次の配列を1度だけ作る。

- `G[firstMask][tier]`：先頭マスクが `firstMask` **と等しい**全パターンの`Pool.All`インデックス。
- `H[allowedMask][tier]`：先頭マスク`firstMask`が `firstMask & allowedMask == firstMask` を満たす全パターンのインデックス。

両索引とも長さ`[16][3][]int`。パターン実体のコピーは保持しない。
最初の拍は直前マスク0、次の拍以降は前拍の `LastMask` を `prev` として、
`allowed := Mask(15) &^ prev`、候補を `H[allowed][tier]` で一度に取得する。
`H[0][tier]` は空。最初の拍では `H[15][tier]` が全候補を含む。

索引の作成は `Pool.All` の固定順序を前提とするため、
一度生成した索引を使っている間は `Pool.All` を並べ替えないこと。

## 譜面の拍数

`GeneratorConfig` は `BPM` と `TargetMinutes` を指定する。
目標時間のプリセットは `DurationShort=1.5`, `DurationNormal=3`,
`DurationLong=5`, `DurationMarathon=10`（分）。

1パターンは四分音符1拍であり、

```text
BeatCount = 3 * round(BPM * TargetMinutes / 3)
```

で**3の倍数**に四捨五入する。**12の倍数には丸めない**。
同距離（ちょうど中点）なら上へ丸める。
例：BPM180で1.5/3/5/10分 → 270/540/900/1800拍。
実際の演奏時間は `BeatCount/BPM` 分（準備時間やキー音の余韻を含まない）。

## tierの残数比例抽選

設定は `TierWeights: [3]float64{1,1,1}` を初期値とし、
重みを変更すれば他の出現比率（例`{5,3,2}`）にもできる。
重みの正規化後に最大剰余法で整数目標 `TierTargets` を算出し、
`sum(TierTargets)==BeatCount` を満たす。

生成時は、各tierについて

1. `H[allowed][tier]`から境界縦連なしの候補を得る。
2. 直近8拍の左右負荷制限を満たさない候補を除く。
3. 合法候補があるtierのうち、`max(0, TierTargets[tier]-TierCounts[tier])`
   を重みとしてtierを抽選する（戻さない抽選）。
4. 選ばれたtierの合法候補を一様ランダムで1つ選ぶ。

あるtierに合法候補がなければそのtierを抽選から除外し、**他のtier**から選ぶ。
目標数に達していないtierがすべて使用不能なら、目標達成済みのtierも
フォールバック候補に含める。このため、**厳密な出現比率は理論上保証しない**。
実際の数は`Sequence.TierCounts`で確認できる。
全tierに合法候補がなければ、縦連や左右制約を破る代わりにエラーを返す。
巻き戻し探索・再生成は現段階では実装していない。

任意の一拍で同じtierが続くことは妨げない。シャッフルバッグ方式による
短期的な均等化は**行わない**。乱数は `Seed` により再現できる。

## 左右負荷制限

各パターンの `BalanceDelta = LeftNotes - RightNotes`（整数）を使う。

- 初期設定：`BalanceWindow=8`拍、`MaxBalance=8`ノーツ。
- 新候補を追加するとき、直近8拍のwindowから古い1拍を除く必要があれば先に除く。
- 新しい直近8拍について `abs(sum(BalanceDelta)) <= 8` を必須とする。
- 保存はring buffer（queue）と累積和で行う。密度の浮動小数計算は不要。

`3-1-3-1`や`2-2-2-2`は左右差がゼロの場合もある。
この制約はそれらの周期2パターンの連続を抑えるものではない。

## 今回は実装しないもの

- トリルや周期2配置の連続ペナルティ
- ゲーム本体での複数拍譜面の再生
- 譜面の音楽的フレーズ・テンプレート周期性
- 結果表示や難易度別の重み・BPMのUI

## API例と検証

```go
pool := streampool.Build()
index := streampool.BuildIndex(pool)
cfg := streampool.DefaultGeneratorConfig()
cfg.BPM = 180
cfg.TargetMinutes = streampool.DurationNormal
cfg.Seed = 42
sequence, err := streampool.Generate(pool, index, cfg)
if err != nil { /* handle error */ }
// sequence.PatternIndices は Pool.All のインデックス
// sequence.TierTargets / sequence.TierCounts で目標と実数を比較
```

テストは `go test ./internal/streampool`。
索引全要素の所属判定、3の倍数への丸め、重み変更・ゼロ重み、
乱数seedの再現、tierの実出現数、全拍境界の縦連禁止、
全拍での左右負荷制限、フォールバック、無効入力を確認する。
独立の検証としてBPM180・10分（1800拍）を500seed生成した際、
全試行で制約を満たし、実出現数は1:1:1だった（一般保証ではない）。
