import asyncio
import sys
import os
import time

sys.path.append(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from src.mcp_server.searcher import WikiSearcher

def get_200_phrases():
    return [
        # History & Politics
        "大化の改新の目的", "源頼朝の幕府成立", "応仁の乱の影響", "関ヶ原の戦いの陣形", "徳川家康の外交政策",
        "黒船来航の衝撃", "大政奉還の意義", "明治維新の廃藩置県", "日清戦争の原因", "太平洋戦争の終戦",
        "冷戦構造の崩壊", "ベルリンの壁の建設", "キューバ危機の真相", "ベトナム戦争の泥沼化", "湾岸戦争の多国籍軍",
        "同時多発テロの衝撃", "アラブの春の波及", "ブレグジットの経済影響", "国連安保理の拒否権", "SDGsの達成目標",
        "古代ローマの水道橋", "ギリシャのポリス", "秦の始皇帝の兵馬俑", "マヤ文明のピラミッド", "インカ帝国のマチュピチュ",
        "産業革命の蒸気機関", "ルネサンスの三大巨匠", "大航海時代の香辛料貿易", "フランス革命のギロチン", "ロシア革命のレーニン",
        # Science & Technology
        "量子力学のシュレーディンガーの猫", "相対性理論の光速度不変", "ブラックホールの特異点", "宇宙のインフレーション", "ダークマターの正体",
        "DNAの二重らせん構造", "CRISPR-Cas9のゲノム編集", "iPS細胞の医療応用", "ミトコンドリアのATP合成", "光合成のカルビン回路",
        "人工知能のディープラーニング", "ブロックチェーンの分散型台帳", "量子コンピューターの重ね合わせ", "自動運転のLiDAR", "5G通信の超低遅延",
        "メタバースのVR技術", "クラウドコンピューティングのIaaS", "サイバーセキュリティのゼロトラスト", "IoTのセンサーネットワーク", "ビッグデータの統計解析",
        "プレートテクトニクスの海溝", "エルニーニョ現象の異常気象", "オゾン層のフロンガス", "地球温暖化の温室効果ガス", "マイクロプラスチックの海洋汚染",
        "再生可能エネルギーの太陽光発電", "原子力発電の核分裂", "燃料電池車の水素", "リチウムイオン電池の正極材", "全固体電池の実用化",
        # Culture & Arts
        "歌舞伎の隈取", "能楽の幽玄", "浮世絵の葛飾北斎", "茶道の千利休", "華道の池坊",
        "源氏物語の光源氏", "枕草子の清少納言", "万葉集の防人歌", "俳句の松尾芭蕉", "近代文学の夏目漱石",
        "印象派のクロード・モネ", "キュビスムのパブロ・ピカソ", "シュルレアリスムのダリ", "ポップアートのアンディ・ウォーホル", "バウハウスの建築",
        "クラシック音楽のベートーヴェン", "ジャズの即興演奏", "ロックンロールの誕生", "ヒップホップのサンプリング", "EDMのシンセサイザー",
        "ハリウッド映画の特撮", "フランス映画のヌーヴェルヴァーグ", "スタジオジブリの宮崎駿", "ディズニーのアニメーション", "ピクサーのCG技術",
        "少年ジャンプの友情努力勝利", "異世界転生ライトノベル", "深夜アニメの製作委員会方式", "ボーカロイドの初音ミク", "VTuberのスーパーチャット",
        # Society & Economy
        "少子高齢化の労働力不足", "待機児童問題の解消", "年金制度のマクロ経済スライド", "国民皆保険制度の維持", "ベーシックインカムの社会実験",
        "働き方改革のテレワーク", "ワークライフバランスの推進", "ジェンダー平等のガラスの天井", "LGBTQの同性婚", "ダイバーシティの企業経営",
        "アベノミクスの三本の矢", "インフレーションとデフレーション", "円安ドル高の輸出企業", "仮想通貨のビットコイン", "NFTのデジタルアート",
        "クラウドファンディングの資金調達", "シェアリングエコノミーのUber", "サブスクリプションの定額制", "キャッシュレス決済のPayPay", "ECサイトのAmazon",
        "ガバナンスとコンプライアンス", "サプライチェーンのリスク管理", "M&Aの企業買収", "IPOの新規株式公開", "ベンチャーキャピタルのシード投資",
        "ふるさと納税の返礼品", "確定申告の青色申告", "消費税のインボイス制度", "マイナンバーカードの保険証利用", "NISAの非課税投資枠",
        # Daily Life & Hobbies
        "ラーメンの豚骨スープ", "寿司の江戸前", "フランス料理のコース", "イタリア料理のパスタ", "中華料理の麻婆豆腐",
        "コーヒーのハンドドリップ", "紅茶のアールグレイ", "日本酒の純米大吟醸", "ワインのテロワール", "クラフトビールのIPA",
        "ダイエットの糖質制限", "筋トレのプロテイン", "ヨガの瞑想", "サウナのロウリュ", "キャンプの焚き火",
        "プロ野球のドラフト会議", "Jリーグの昇格争い", "大相撲の横綱", "フィギュアスケートの4回転ジャンプ", "オリンピックの金メダル",
        "RPGのレベル上げ", "FPSのエイム力", "格闘ゲームのコンボ", "ボードゲームのカタン", "トレーディングカードのレアカード",
        "一眼レフカメラのボケ味", "ロードバイクのヒルクライム", "熱帯魚のアクアリウム", "観葉植物のモンステラ", "DIYの電動インパクトドライバー",
        # Geography & Nature
        "富士山の宝永大噴火", "琵琶湖の固有種", "屋久島の縄文杉", "知床の流氷", "白川郷の合掌造り",
        "アマゾン川の熱帯雨林", "サハラ砂漠のオアシス", "ヒマラヤ山脈のエベレスト", "グレートバリアリーフのサンゴ", "グランドキャニオンの地層",
        "北極の白夜", "南極のオーロラ", "死海の塩分濃度", "ガラパゴス諸島の進化論", "マダガスカルのバオバブ",
        "地震の震度とマグニチュード", "津波の防波堤", "台風の進路予想", "火山のカルデラ", "竜巻のスーパーセル",
        "花粉症のアレルゲン", "インフルエンザのワクチン", "新型コロナウイルスのパンデミック", "抗生物質のペニシリン", "免疫の白血球",
        "ライオンのプライド", "ゾウの長い鼻", "キリンの首の骨", "ペンギンの泳ぎ方", "クジラの潮吹き",
        # Random / Edge Cases / Fictional (Should return nothing or low relevance)
        "超光速ワープドライブの開発", "火星の地下帝国", "四次元ポケットの仕組み", "魔法省の魔法使い", "ジェダイのライトセーバー",
        "ホグワーツ魔法魔術学校の組み分け", "サイヤ人の大猿化", "悪魔の実の能力者", "巨人のうなじの弱点", "エヴァンゲリオンのATフィールド",
        "どこでもドアの空間接続", "タイムマシンの親殺しのパラドックス", "タケコプターの揚力", "デスノートの死神の目", "錬金術の等価交換",
        "怪獣8号の防衛隊", "呪術高専の特級呪物", "鬼殺隊の呼吸法", "スーパーマリオのスター状態", "ポケモンのメガシンカ"
    ]

async def evaluate():
    searcher = WikiSearcher()
    phrases = get_200_phrases()
    print(f"Loaded {len(phrases)} phrases. Starting evaluation...")
    
    results_summary = []
    found_count = 0
    
    for i, phrase in enumerate(phrases):
        if i % 10 == 0:
            print(f"Processing query {i}/{len(phrases)}...")
            
        try:
            # Get up to 5 results
            results = searcher.search(phrase, limit=5)
        except Exception as e:
            results = []
            print(f"Error searching for {phrase}: {e}")
            
        if results:
            found_count += 1
            top_res = results[0]
            snippet = top_res["text"].replace("\n", " ")[:150]
            relevance = top_res.get("relevance_score", 0)
            
            results_summary.append(f"### Query: `{phrase}`\n")
            results_summary.append(f"- **Max Relevance**: {relevance}\n")
            results_summary.append(f"- **Top Document Title**: {top_res.get('title', 'N/A')}\n")
            results_summary.append(f"- **Top Document Snippet**: {snippet}...\n\n")
        else:
            results_summary.append(f"### Query: `{phrase}`\n")
            results_summary.append("*No results returned.*\n\n")

    report_path = os.path.join(
        os.path.dirname(os.path.abspath(__file__)),
        "qualitative_eval_200.md"
    )
    
    with open(report_path, "w", encoding="utf-8") as f:
        f.write(f"# Qualitative Evaluation of Returned Documents ({len(phrases)} Phrases)\n\n")
        f.write(f"**Total Queries Evaluated**: {len(phrases)}\n")
        f.write(f"**Queries with Results**: {found_count}\n\n")
        f.write("---\n\n")
        f.writelines(results_summary)
        f.write("---\n")
        f.write(f"**Summary**: {found_count} out of {len(phrases)} queries returned documents.\n")

    print(f"Evaluation complete. Report generated at {report_path}")

if __name__ == "__main__":
    asyncio.run(evaluate())
