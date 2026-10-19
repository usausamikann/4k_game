# Step 5.5 事前合成・単一再生・Seek修正（2026-10-19）

今回の確定仕様は過去の「余韻を重ねる／32ボイス」方針を上書きする。
3モード・5音源・CLI・独立音量・Pressの発音条件とTracker／Session接続は維持する。
メトロノームのStream／Transport／拍配置、Clock、A/B、TPS、診断、OGG、Ebitengine v2.10.5は変更しない。

## 事前合成と再生

`KeyMixer`／`NewKeyMixer`を、処理内容に合う`KeyStream`／`NewKeyStream`へ改名した。
現在の4レーン配置に必要なIDだけ起動時に一度デコードし、同じIDのPCMを共有する。
その配置についてmask 1〜15を合成する（mask 0は無音、Off／音量0は全合成を省略）。
同一音源の重複レーンも独立加算し、最長PCMまで無音パディングする。
旧方式と同じレーン順のfloat64加算、`0.70*tanh(sum*0.5/0.70)`、共通百分率音量、
float32変換を事前処理し、最終PCMをlittle-endianバイト列で保存する。

同じSession.UpdateAtのPressを4bitマスクへまとめ、1回だけ再生PCMとカーソルを切り替える。
別UpdateのPressでは旧PCMを捨て、新PCM先頭から再生する。Release／Pressなしでは切り替えない。
フェード、クロスフェード、複数ボイス、serial、枯渇管理はない。
任意バイト長Readでstereoフレームの途中まで返した場合のみ残り最大7バイトを完了してから切り替え、
不正なfloat32値や左右チャンネルのずれを防ぐ。これは追加の余韻・フェード処理ではない。
Readは最大256バイト単位のコピー／無音クリアだけを短いロック内で実行する。

## 負荷とメモリ

32スロット走査、実行時加算・tanh・F32エンコードを廃止した。Pressはポインタとカーソル更新のみ。
Press／Readの追加確保はテストで0、Playerは既存の1個を使い、PressごとのRewindも行わない。
ボイス配列と元のデコードPCMの常駐参照はなくなるが、**全体のPCM保存量は増える**。
保持PCMは15本の各最長ソース長の合計。48kHz stereo F32、既存の2秒上限で最大11,520,000バイト。
合成中だけは最大4種類のソースPCMも一時的に存在する。連打数やプレイ時間によって増加しない。
実音源のテスト2配置では合成PCMは660,360／636,264バイト。設定変更は新しいKeyStreamを
再構築すれば対応できるが、稼働中のGUI設定変更やPlayer交換機構は今回追加しない。

## Seekとライブラリ調査

Ebitengine v2.10.5 `audio/player.go` の`ensureStream`→`newTimeStream`は、Playerの初回使用時に
`Seek(0, io.SeekCurrent)`でソース位置を問い合わせる。NewPlayerF32のコンストラクター自体は
Readerへアクセスせず、Play／SetPosition経路で遅延初期化される。旧実装はこの問い合わせを
一般エラーにしており、Playerの再生開始を妨げていた。

新実装は実際にReadで返したバイト数をint64で累積し、Pressではこの絶対位置を変えない。
Currentの0問い合わせは状態・部分フレームに触らず位置を返す。Startの0は再生PCM、カーソル、
部分フレームと絶対位置をクリアする。無限ライブストリームのため末尾や過去の打鍵を再現する
任意Seekは提供せず、他の指定はerrors.ErrUnsupportedをラップして状態を維持する。
任意長Read／ゼロ長Read／int64位置上限もテストする。

Player.Rewindは`SetPosition(0)`→timeStream／Oto Seek経路を通る。
Oto v3.5.0 `internal/mux/mux.go` のSeekは読み込み完了を待ち、内部キューをクリアする。
フォーカス喪失時のPauseAndStopReading→Rewindは維持し、古いPCMとキューを破棄する。
音声Readerの切り替えだけでは既存の先読みを消せないため、10ms要求バッファは維持するが
物理出音にはそのほかのバックエンド・OS・入力周期の遅延も残る。

## 検証

READMEに実行結果とWindows手順を記載した。全マスク・重複・長さ差・実音源による旧同時押し
数値一致、Session経由の同時押し／時間差、部分Read／Seek、有限値、raceと追加確保なしを検証。
ReaderベンチマークはCPU側コピー処理のみであり、実音声の出力遅延やCPU使用率を測定した結果ではない。
Linux／WSL・Windowsの実音声、Alt+Tab、音割れ・クリックノイズと体感遅延は未検証。
試聴は小音量から開始し、BPM180／220連打・階段・4レーン同時押しに加えて、
BPM180の24分間隔7連打（約55.56ms）で音の切り替わりと不快なノイズがないか確認する。
