# 1拍パターン分類ツール（Draft）

tools/pattern-labeler/index.html をブラウザで開く。Go、npm、サーバー、ネット接続は不要。
既存の4Kゲームの実行コードは変更しない。

## 操作

1. 下が拍頭、上が拍末の4レーン表示を見る。
2. stream-light / stream-middle / stream-dense / jack-light / jack-middle /
   jack-dense / other から選ぶ（数字キー1〜7でも可）。
3. Next（またはEnter）で回答を確定して次の候補へ。
4. 直前の回答を修正するときは「戻って修正」を使う。
5. 必要に応じて入力欄から分類を追加できる。追加分も自動保存される。
6. 定期的に「JSONを出力」。CSVも同じ回答を表形式で出力できる。

ラベルを選んだだけでは未確定。Nextで確定した回答だけをエクスポートする。
未確定の選択もブラウザに保存するため、リロード後に続きから作業できる。

自動保存先はブラウザのlocalStorage（キー 4k_game.pattern_labeler.v1）。
ブラウザ／端末が変わると引き継がれず、ストレージ消去で失われる。
file://ページでもパス・プライバシー設定によって保存領域が変わったり
利用できないことがある。こまめなJSON出力を推奨。
「新しいセッション」は確認ダイアログのあと回答をリセットする。
分類名の追加は可能だが、既存ラベルの削除・改名・JSONインポートは未実装。

## パターンの候補

現段階は全列挙ではなくランダムサンプリング。
候補生成はgenerator.jsに独立させ、seed＋候補インデックスで決定的。
同じセッション内では既に確定した同一配置（time_scaleを含む）を避ける。
4/6/8/12/16等分のグリッドを使用し、stream系、jack系、同時押し系、
混合系の4種類の生成レシピから候補を作る（レシピ名は画面に表示しない）。
生成レシピは正解ラベルではない。候補の偏りが大きい可能性もある。

ノーツ位置は position: [a,b]、time_scaleは0または1。
全候補が960tickの整数に換算でき、time_scale=1でも割り切れる。
time_scale設定を変更しても現在表示中の候補は変えず、次の候補に反映する。
グリッドの配置は1拍を正規化した表示で、time_scale=1は実時間が半分となる。
今後、サンプリングの偏り、同一視する対称性、和音数、パターン長と密度、
カテゴリ体系そのものを検討する。

## エクスポート

JSONフォーマットは 4k_game_pattern_labels version 1。
labels[]には次を含む。

- id: sample-<seed>-<index>形式の安定した候補ID
- seed, generator_index: 生成位置の追跡
- category, labeled_at: 人手で付けた分類と確定時刻
- grid_divisions, sampling_recipe: 候補生成のメタデータ
- pattern: 現行internal/chart互換のname, time_scale, notes配列

カテゴリー一覧はcategory_optionsに同梱。other以外も追加可能。
sampling_recipeは生成上の参考情報であり、正解クラスとして扱わない。
CSVはnotes_json列にノーツ配列をJSON文字列として格納する。
後の分析・再読込を考えるとJSONを主データとして扱うのを推奨。

## テスト

標準のNode.jsのみでgeneratorの再現性・重複ノーツ禁止・tickの整合性・
候補の多様性・入力検証をテストする。

    node --test tools/pattern-labeler/generator.test.cjs

現段階でラベルの良し悪しは評価しない。UIの見た目・保存・出力は
ブラウザでの手動確認も必要。
