# TODO

## 開発・機能追加
- [ ] **マルチモーダル検索の対応**: 画像やドキュメントファイルなどを含むマルチモーダルな検索が可能になるよう、ベクトルデータベース側やUI側の改修を検討する。
- [x] **他のツール提供の検討**: 現在の `search_wiki` ツールに加え、文書の要約ツールやメタデータ抽出ツールなどの提供を検討する。 (参照: `README.md` の MCPツール説明部分)
  - `list_wiki_pages` を追加済み
- [x] **モニタリングと可観測性の強化**: OpenTelemetry および Prometheus を用いたメトリクス出力（レイテンシ、キャッシュヒット率など）の導入。
  - OTel Tracing/Metrics および Prometheus メトリクスエンドポイントを実装済み
- [ ] **CLIツールの拡充と管理機能**: データベースの健全性チェックやFTS再構築、ドキュメント管理を容易にするCLI (`rrag-admin`) の整備。
- [ ] **多様なドキュメント取込 (PDF, 画像など)**: Markdown以外（PDF、Office文書、画像）からのテキスト抽出・チャンク化パイプラインの拡充。
- [ ] Pythonを廃止し、Goに一本化するなどが可能かの検討
 
## ドキュメント・配布
- [x] **Homebrew (Custom Tap) の詳細化**: 組織内配布用の具体的な Formula の定義例を `docs/DEVELOPMENT.md` に、詳細なインストール手順の案内を `README.md` にそれぞれ追記する。
  - GitHub Actionsで自動生成されるように対応済み
- [ ] **リポジトリURLの変更**: 公開/移行に向けてリポジトリのURL（およびドキュメント内の関連リンク）を変更・整理する。
- [ ] Web App Integrationに関するドキュメントの作成(Copilot Studio対応)

## その他
- 実装見直し
- [ ] RAGサーバの認証について、 https://github.com/ory/oathkeeper での置き換え、あるいは柔軟なフィルタ処理のためのOPAの導入を検討する。
- [x] localでのトークン管理について、https://github.com/int128/oauth2cli など既存実装の導入に価値があるかを検討
  - 検討完了。機能的な不足がないため現状の自前実装を維持し、将来的にUX向上やポート競合対策が必要になった際に導入を検討する。
- [x] **プロジェクト全体の構成見直しとバグ修正**: TOCTOUやgoroutineリークの修正、SSE/Transmitterのテスト実装と競合解消。
- [ ] セッション中にトークン期限が切れた場合、能動的に切断する検討
- [x] EXACT_MATCH_RELEVANCE_THRESHOLD DEFAULT_RELEVANCE_THRESHOLD をカスタム可能にする
  - 環境変数から読み込めるよう対応済み。
- [ ] 主要ユースケースの記載
  - ChatGPT / codex cli -> oktaで保護された任意のナレッジデータソース
- [ ] streamable HTTPの実装
- [ ] RAGサーバ部分を差し替え可能にする


## Chat AI Service 外部連携仕様メモ
- ChatGPT
  - https://developers.openai.com/api/docs/actions/getting-started
  - 機能名: GPT Actions
  - 必要なライセンス:
  - 対応プロトコル: REST API (OpenAPI schema必須？)
  - 認証方式: OAuth対応 https://developers.openai.com/api/docs/actions/authentication#oauth

- Claude
  - https://support.claude.com/en/articles/11176164-use-connectors-to-extend-claude-s-capabilities#h_4201f9e625
  - https://support.claude.com/en/articles/11175166-get-started-with-custom-connectors-using-remote-mcp
  - 機能名: Custom Connectors (Remote MCP)
- Gemini
  - https://docs.cloud.google.com/gemini/enterprise/docs/connectors/custom-mcp-server/set-up-custom-mcp-server?hl=ja
  - 機能名: MCP Data Store / Custom Actions
- MS Copilot