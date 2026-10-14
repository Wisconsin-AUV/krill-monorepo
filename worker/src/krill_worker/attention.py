"""Faster attention for GPUs where PyTorch has no fused attention kernel.

SAM's memory attention has each of the frame's 4096 tokens attend to about
29k memory tokens, four layers per frame. transformers' eager attention makes
two extra passes over that score matrix (scaling it, then a softmax upcast to
fp32), and applies the rotary embedding to the memory keys in fp32. On an
RX 6600 those passes cost as much as the matmuls themselves.
"""

from collections.abc import Callable
from typing import Any

import torch
from torch import nn
from transformers import AttentionInterface

IMPLEMENTATION = "krill_fp16"


def attention_forward(
    module: nn.Module,
    query: torch.Tensor,
    key: torch.Tensor,
    value: torch.Tensor,
    attention_mask: torch.Tensor | None,
    scaling: float,
    dropout: float = 0.0,
    **kwargs: Any,
) -> tuple[torch.Tensor, None]:
    """Eager attention that scales the queries, not the scores, and keeps the softmax in fp16."""
    scores = torch.matmul(query * scaling, key.transpose(2, 3))
    if attention_mask is not None:
        scores = scores + attention_mask
    # A mask in a wider dtype promotes the scores; the weights must match value again.
    weights = torch.softmax(scores, dim=-1).to(query.dtype)
    weights = nn.functional.dropout(weights, p=dropout, training=module.training)
    return torch.matmul(weights, value).transpose(1, 2).contiguous(), None


def rotary_pos_emb_2d(
    rotate_pairwise: Callable[[torch.Tensor], torch.Tensor],
) -> Callable[..., tuple[torch.Tensor, torch.Tensor]]:
    """Build a drop-in for transformers' apply_rotary_pos_emb_2d that stays in the model dtype."""

    def apply(
        q: torch.Tensor,
        k: torch.Tensor,
        cos: torch.Tensor,
        sin: torch.Tensor,
        num_k_exclude_rope: int = 0,
        repeat_freqs_k: bool = False,
    ) -> tuple[torch.Tensor, torch.Tensor]:
        cos = cos.to(q.dtype)
        sin = sin.to(q.dtype)
        q_embed = q * cos + rotate_pairwise(q) * sin
        n_rot = k.shape[-2] - num_k_exclude_rope
        k_rot, k_pass = k[..., :n_rot, :], k[..., n_rot:, :]
        if n_rot == 0:
            return q_embed, k
        if repeat_freqs_k and n_rot != q.shape[-2]:
            # The memory is a stack of frames; each gets the frame's positions.
            frames = k_rot.view(*k_rot.shape[:-2], n_rot // q.shape[-2], q.shape[-2], k.shape[-1])
            cos, sin = cos.unsqueeze(-3), sin.unsqueeze(-3)
            k_embed = (frames * cos + rotate_pairwise(frames) * sin).flatten(-3, -2)
        else:
            k_embed = k_rot * cos + rotate_pairwise(k_rot) * sin
        return q_embed, torch.cat([k_embed, k_pass], dim=-2)

    return apply


def register() -> str:
    """Register the attention with transformers and patch SAM's rotary embedding."""
    AttentionInterface.register(IMPLEMENTATION, attention_forward)
    from transformers.models.sam2_video import modeling_sam2_video
    from transformers.models.sam3_tracker_video import modeling_sam3_tracker_video

    modules: list[Any] = [modeling_sam2_video, modeling_sam3_tracker_video]
    for module in modules:
        module.apply_rotary_pos_emb_2d = rotary_pos_emb_2d(module.rotate_pairwise)
    return IMPLEMENTATION
