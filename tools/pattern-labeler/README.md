# 4K Game / リズム1111専用パターン分類器（Draft）

ブラウザで `tools/pattern-labeler/index.html` を開く。Go、npm、サーバー、
ネット接続は不要。ゲーム本体は変更していない。

## 対象

1拍を16分音符の4スロット（時刻0, 1/4, 1/2, 3/4）に分け、
**各スロットに必ず1個、4レーンのどれかに単音ノーツを置く。**
リズムマスクは `1111` 固定、time_scale=0、同時押しと休符は対象外。

4^4 = 256通り。左右反転 D↔K、F↔J で同一視すると
固定される非空譜面がないため、**256/2 = 128種類**。
これら128個を一度ずつ、seedでシャッフルした順に表示する。
`pattern-XXX` と `canonical_code` は以前の312種類版と同じ
5進4桁コードを使い、同じ配置には同じIDを割り当てる。
左右反転版はJSONに保存する。

## 操作

1. 4レーン図で下=拍頭、上=拍末の譜面を見る。
2. stream-light / stream-middle / stream-dense / jack-light /
   jack-middle / jack-dense / other / trill から選ぶ。
   `trill`は以前の70件の作業で追加された分類のため初期状態から用意。
3. `Next`（またはEnter）で確定して次の候補に進む。数字キー1〜8で選択可能。
4. 分類を追加したい場合は入力欄から追加する。最初の9分類は数字キーに対応。
5. 直前の回答を修正するときは「戻って修正」を使う。
6. 128件すべて回答すると完了表示。最後の回答も戻って修正可能。
7. 作業途中でもJSON / CSV出力可能。後の集計にはJSONがおすすめ。

分類ボタンを選択しただけでは未確定。Nextで確定した回答のみ出力される。
ブラウザのlocalStorageに未確定選択と回答を自動保存する。

## データ分離とバックアップ

今回の自動保存キーは `4k_game.pattern_labeler.v3_1111`。
**以前の312種類版(v2)の70件と、初期サンプル版(v1)を消去・移行しない。**
今回は0/128から始まる。旧版の回答はアップロード済みJSONにも残る。
異なる分類母集団を混ぜないことを優先している。
シークレットモード、別ブラウザ、ストレージ消去では復旧できないため
適宜JSONを書き出して保管してほしい。
「新しいセッション」で消えるのは今回のv3回答だけで、確認ダイアログが出る。

## エクスポート形式（version 3）

JSONのトップレベルに `format`, `version=3`, `seed`,
`sampling="exhaustive_full_rhythm_1111_mirror_reduced_4x16th"`,
`rhythm_mask="1111"`, `total=128`, `category_options` と `labels` を持つ。

各回答は `id`, `generator_index`, `category`, `labeled_at`,
`grid_divisions=4`, `rhythm_mask="1111"`, `canonical_code`,
`mirror_code`, `pattern`, `mirror_pattern` を保持する。
`pattern` と `mirror_pattern` はゲームの `internal/chart` と同じ
`name` / `time_scale` / `notes: [{lane, position:[slot,4]}]` 形式。
CSVには `rhythm_mask`, `notes_json`, `mirror_notes_json` を含む。

「ラベルは左右反転に対して不変」という前提で統合している。
BPM・パターン連結時の前後関係・リズムの欠損は今回は分類対象外。

## テスト

Node標準テストランナー:

    node --test tools/pattern-labeler/generator.test.cjs

128個の重複のない代表が鏡映を合わせて全256個の1111パターンを覆うこと、
全候補が16分音符4箇所に1ノーツずつ持つこと、IDの安定性、
鏡映の対合性、seedの再現性と入力検証を確認する。
UIの表示・エクスポート・自動保存はブラウザでの操作確認も必要。
