"""
ONNX量子化バックエンドを使用するカスタムCrossEncoderRerankerモジュール。
LanceDB の Reranker 基底クラスを正式に継承し、公開APIのみを使用します。
"""
import logging
from functools import cached_property
from typing import Union

import pyarrow as pa
from lancedb.rerankers import Reranker

logger = logging.getLogger(__name__)


class OnnxCrossEncoderReranker(Reranker):
    """
    ONNX ランタイム（オプションでINT8量子化）を使用する CrossEncoder ベースのReranker。
    
    LanceDB の CrossEncoderReranker の内部実装（_model）に依存せず、
    公式の Reranker 基底クラスの API を正式に実装しているため、
    LanceDB のバージョンアップに対して安定的に動作します。
    """
    
    def __init__(
        self,
        model_name: str = "cross-encoder/mmarco-mMiniLMv2-L12-H384-v1",
        column: str = "text",
        device: Union[str, None] = None,
        return_score: str = "relevance",
        trust_remote_code: bool = True,
        onnx_file_name: str = "onnx/model_qint8_avx2.onnx",
    ):
        """
        OnnxCrossEncoderRerankerの初期化。
        
        Args:
            model_name (str): HuggingFace上のCrossEncoderモデル名。
            column (str): 検索対象のテキストカラム名。
            device (str | None): 推論に使用するデバイス。Noneの場合は自動検出。
            return_score (str): 返却するスコアの種類（'relevance' or 'all'）。
            trust_remote_code (bool): リモートコードの実行を許可するか。
            onnx_file_name (str): 使用するONNXモデルファイル名。
                                  量子化モデルを使わない場合はNoneを指定。
        """
        super().__init__(return_score)
        self.model_name = model_name
        self.column = column
        self.device = device
        self.trust_remote_code = trust_remote_code
        self.onnx_file_name = onnx_file_name
        if self.device is None:
            try:
                import torch
                self.device = "cuda" if torch.cuda.is_available() else "cpu"
            except ImportError:
                self.device = "cpu"

    @cached_property
    def model(self):
        """CrossEncoder モデルの遅延読み込み。初回アクセス時のみロードされる。"""
        from sentence_transformers import CrossEncoder
        
        logger.info(f"Loading CrossEncoder '{self.model_name}' with ONNX backend...")
        
        model_kwargs = {}
        if self.onnx_file_name:
            model_kwargs["file_name"] = self.onnx_file_name
        
        model = CrossEncoder(
            self.model_name,
            device=self.device,
            trust_remote_code=self.trust_remote_code,
            backend="onnx",
            model_kwargs=model_kwargs,
        )
        logger.info("CrossEncoder model loaded with ONNX backend successfully.")
        return model

    def _rerank(self, result_set: pa.Table, query: str) -> pa.Table:
        """
        結果セットに対して CrossEncoder による関連性スコアを計算し、
        _relevance_score カラムを追加して返します。
        
        Args:
            result_set (pa.Table): LanceDB からの検索結果テーブル。
            query (str): 検索クエリ文字列。
            
        Returns:
            pa.Table: _relevance_score カラムが追加されたテーブル。
        """
        result_set = self._handle_empty_results(result_set)
        if len(result_set) == 0:
            return result_set

        passages = result_set[self.column].to_pylist()
        cross_inp = [[query, passage] for passage in passages]
        cross_scores = self.model.predict(cross_inp)
        
        result_set = result_set.append_column(
            "_relevance_score", pa.array(cross_scores, type=pa.float32())
        )
        return result_set

    def rerank_hybrid(
        self, query: str, vector_results: pa.Table, fts_results: pa.Table
    ) -> pa.Table:
        """ハイブリッド検索結果（ベクトル + FTS）の再評価。"""
        if self.score == "all":
            combined = self._merge_and_keep_scores(vector_results, fts_results)
        else:
            combined = self.merge_results(vector_results, fts_results)

        combined = self._rerank(combined, query)

        if self.score == "relevance":
            combined = self._keep_relevance_score(combined)

        return combined.sort_by([("_relevance_score", "descending")])

    def rerank_vector(self, query: str, vector_results: pa.Table) -> pa.Table:
        """ベクトル検索結果の再評価。"""
        vector_results = self._rerank(vector_results, query)
        if self.score == "relevance":
            vector_results = vector_results.drop_columns(["_distance"])
        return vector_results.sort_by([("_relevance_score", "descending")])

    def rerank_fts(self, query: str, fts_results: pa.Table) -> pa.Table:
        """FTS検索結果の再評価。"""
        fts_results = self._rerank(fts_results, query)
        if self.score == "relevance":
            fts_results = fts_results.drop_columns(["_score"])
        return fts_results.sort_by([("_relevance_score", "descending")])
