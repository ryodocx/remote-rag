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


## その他
- 実装見直し
- [ ] RABサーバの認証について、 https://github.com/ory/oathkeeper での置き換え、あるいは柔軟なフィルタ処理のためのOPAの導入を検討する。
- [ ] localでのトークン管理について、https://github.com/int128/oauth2cli などの導入を検討
- [ ] セッション中にトークン期限が切れた場合、能動的に切断する検討
- [ ] EXACT_MATCH_RELEVANCE_THRESHOLD DEFAULT_RELEVANCE_THRESHOLD をカスタム可能にする
