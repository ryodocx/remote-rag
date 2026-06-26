import os
import sys

# Ensure UTF-8
sys.stdout.reconfigure(encoding='utf-8')

from src.mcp_server.searcher import WikiSearcher

words = [
    "江戸幕府の崩壊", "ルネサンス期の芸術", "フランス革命の原因", "産業革命の影響", "冷戦時代の終結", 
    "メソポタミア文明の遺跡", "第一次世界大戦のきっかけ", "大航海時代の探検家", "明治維新の立役者", "古代エジプトのピラミッド",
    "ブラックホールの事象の地平線", "ミトコンドリアの役割", "プレートテクトニクス理論", "深海魚の生態", "光合成の仕組み", 
    "量子力学の不確定性原理", "地球温暖化のメカニズム", "相対性理論の基礎", "DNAの二重らせん構造", "ビッグバン理論",
    "スタジオジブリのアニメーション作品", "ハリウッド映画の歴史", "日本のアイドル文化", "ロールプレイングゲームの進化", "バーチャルYouTuberの活動", 
    "インディーズバンドのライブ", "少年漫画の王道ストーリー", "歌舞伎の演目と歴史", "クラシック音楽のオーケストラ", "ジャズの即興演奏",
    "少子高齢化問題への対策", "ベーシックインカムの導入議論", "働き方改革とワークライフバランス", "確定申告の手続き", "ふるさと納税の仕組み", 
    "スーパーマーケットの流通網", "リサイクル可能な素材", "ジェンダー平等の推進", "持続可能な開発目標の達成", "ユニバーサルデザインの導入",
    "スマートフォンの進化の歴史", "クラウドファンディングの成功事例", "キャッシュレス決済の普及", "ブレインマシンインターフェース", "ドローンの飛行ルール", 
    "暗号資産の取引所", "自動翻訳技術の精度", "ウェアラブル端末の健康管理", "拡張現実を活用したゲーム", "メタバース空間での経済活動",
    "アマゾン熱帯雨林の保護", "シルクロードの交易ルート", "ガラパゴス諸島の固有種", "オーロラが発生する条件", "サハラ砂漠の気候", 
    "ヒマラヤ山脈の形成", "グレートバリアリーフのサンゴ礁", "死海の塩分濃度", "グランドキャニオンの地層", "南極大陸の氷床",
    "免疫システムの働き", "ウイルスと細菌の違い", "睡眠不足がもたらす影響", "ストレスコーピングの手法", "腸内フローラの改善", 
    "有酸素運動のメリット", "ワクチン開発のプロセス", "生活習慣病の予防", "メンタルヘルスのケア", "抗生物質の耐性菌問題",
    "フランス料理のフルコース", "寿司の握り方", "コーヒー豆の焙煎方法", "発酵食品の健康効果", "日本酒の製造工程", 
    "地中海食の特徴", "ラーメンのスープ作り", "スパイスカレーのレシピ", "和菓子の伝統的な製法", "クラフトビールの醸造",
    "インフレーションとデフレーション", "株式市場の仕組み", "ベンチャーキャピタルの投資", "サプライチェーンの断絶", "企業買収と合併のプロセス", 
    "外国為替証拠金取引のリスク", "中央銀行の金融政策", "国内総生産の成長率", "サブプライムローン問題", "ブロックチェーンによる契約自動化",
    "印象派の絵画手法", "実存主義の哲学", "キュビスムの特徴", "ギリシャ神話の神々", "バロック音楽の魅力", 
    "現代アートの解釈", "シュルレアリスムの表現", "ポストモダニズムの建築", "ロマン派の文学", "浮世絵の多色摺り木版画"
]

def main():
    searcher = WikiSearcher()
    report = "# Qualitative Evaluation of Returned Documents (100 Longer Phrases Direct)\n\n"
    
    valid_count = 0
    total_count = 0
    
    for query in words:
        total_count += 1
        try:
            results = searcher.search(query, limit=5, search_type="hybrid")
            if not results:
                report += f"### Query: `{query}`\n*No results returned.*\n\n"
            else:
                top_res = results[0]
                relevance = top_res.get('relevance_score', 'N/A')
                text = top_res.get('text', '')
                snippet = text[:300] + "..." if len(text) > 300 else text
                report += f"### Query: `{query}`\n"
                report += f"- **Max Relevance**: {relevance}\n"
                report += f"- **Top Document Snippet**: {snippet}\n\n"
                valid_count += 1
        except Exception as e:
            report += f"### Query: `{query}`\n*Error: {e}*\n\n"

    report += f"---\n**Summary**: {valid_count} out of {total_count} queries returned documents.\n"
    
    out_path = r"C:\Users\ryotn\.gemini\antigravity\brain\258ce460-7f14-4094-8e10-4ccdcc9fdac5\qualitative_eval_long_direct.md"
    with open(out_path, "w", encoding='utf-8') as f:
        f.write(report)
        
    print(f"Direct qualitative report generated at {out_path}")

if __name__ == "__main__":
    main()
