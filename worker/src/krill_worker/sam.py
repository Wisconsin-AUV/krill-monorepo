import warnings
from collections.abc import Iterable, Iterator
from typing import Any

import numpy as np
import torch
from torch.nn.attention import SDPBackend, sdpa_kernel
from transformers import (
    Sam2VideoModel,
    Sam2VideoProcessor,
    Sam3Model,
    Sam3Processor,
    Sam3TrackerVideoModel,
    Sam3TrackerVideoProcessor,
)

from krill_worker.prelabelers import Frame
from krill_worker.track import Box, PointPrompt, Prompt, mask_box

# Score a found object needs to be proposed. Labelers delete false positives,
# so this leans toward recall.
EXEMPLAR_THRESHOLD = 0.4


def attention_implementation(device: str) -> str:
    """Pick eager attention when SDPA has no fused kernel for the GPU.

    SDPA's math fallback upcasts to fp32, which makes SAM's memory attention
    about half as fast as eager fp16 attention on cards like the RX 6600.
    """
    q = torch.zeros(1, 1, 8, 64, device=device, dtype=torch.float16)
    try:
        with (
            warnings.catch_warnings(action="ignore"),
            sdpa_kernel([SDPBackend.FLASH_ATTENTION, SDPBackend.EFFICIENT_ATTENTION]),
        ):
            torch.nn.functional.scaled_dot_product_attention(q, q, q)
    except RuntimeError:
        return "eager"
    return "sdpa"


class SamTracker:
    """Tracks one object through streamed frames with SAM 3 or SAM 2.1."""

    def __init__(self, model_id: str, device: str = "cuda") -> None:
        sam3 = "sam3" in model_id
        model_cls = Sam3TrackerVideoModel if sam3 else Sam2VideoModel
        processor_cls = Sam3TrackerVideoProcessor if sam3 else Sam2VideoProcessor
        self.model_id = model_id
        self.device = device
        # SAM 2.1 only segments what it is pointed at. SAM 3 can also find
        # every object that matches a visual exemplar.
        self.supports_exemplars = sam3
        self._attn = attention_implementation(device)
        self._detector: Any = None
        self._detector_processor: Any = None
        self.model = model_cls.from_pretrained(
            model_id, dtype=torch.float16, attn_implementation=self._attn
        )
        self.model = self.model.to(device).eval()
        self.processor = processor_cls.from_pretrained(model_id)
        config = self.model.config
        self.memory_frames = max(config.num_maskmem, config.max_object_pointers_in_encoder)

    def warm_up(self) -> None:
        """Track a blank clip so ROCm compiles its kernels before the first real click."""
        frame = np.zeros((576, 1024, 3), dtype=np.uint8)
        for _ in self.track([frame, frame], PointPrompt(0.5, 0.5)):
            pass

    def track(self, frames: Iterable[Frame], prompt: Prompt) -> Iterator[Box | None]:
        """Yield the object's box on each frame, or None where it is not visible.

        The prompt is on the first frame.
        """
        session = self.processor.init_video_session(
            inference_device=self.device, dtype=torch.float16
        )
        for i, frame in enumerate(frames):
            inputs = self.processor(images=frame, device=self.device, return_tensors="pt")
            size = frame.shape[:2]
            if i == 0:
                self._add_prompt(session, prompt, size)
            with torch.inference_mode():
                out = self.model(
                    inference_session=session, frame=inputs.pixel_values[0].to(torch.float16)
                )
            self._forget(session, i)
            masks = self.processor.post_process_masks([out.pred_masks], original_sizes=[size])[0]
            yield mask_box(masks[0, 0].cpu().numpy())

    def find_similar(self, frame: Frame, box: Box) -> list[Box]:
        """Return every object on the frame like the one in box, most confident first."""
        if not self.supports_exemplars:
            raise ValueError(f"{self.model_id} cannot search by exemplar")
        if self._detector is None:
            # Loaded on first use so a worker that never gets an exemplar
            # task keeps the card's memory for tracking.
            detector = Sam3Model.from_pretrained(
                self.model_id, dtype=torch.float16, attn_implementation=self._attn
            )
            self._detector = detector.to(self.device).eval()
            self._detector_processor = Sam3Processor.from_pretrained(self.model_id)
        h, w = frame.shape[:2]
        x, y, bw, bh = box
        inputs = self._detector_processor(
            images=frame,
            input_boxes=[[[x * w, y * h, (x + bw) * w, (y + bh) * h]]],
            input_boxes_labels=[[1]],
            return_tensors="pt",
        ).to(self.device)
        with torch.inference_mode():
            out = self._detector(
                pixel_values=inputs.pixel_values.to(torch.float16),
                input_ids=inputs.input_ids,
                attention_mask=inputs.attention_mask,
                input_boxes=inputs.input_boxes.to(torch.float16),
                input_boxes_labels=inputs.input_boxes_labels,
            )
        found = self._detector_processor.post_process_object_detection(
            out, threshold=EXEMPLAR_THRESHOLD, target_sizes=[(h, w)]
        )[0]
        scores = found["scores"].float().cpu().numpy()
        boxes = found["boxes"].float().cpu().numpy()
        out_boxes: list[Box] = []
        for i in np.argsort(-scores):
            x0, y0, x1, y1 = (float(v) for v in boxes[i])
            out_boxes.append((x0 / w, y0 / h, (x1 - x0) / w, (y1 - y0) / h))
        return out_boxes

    def _forget(self, session: Any, frame_idx: int) -> None:
        """Drop state the model no longer reads so memory stays flat on long clips.

        The session keeps every streamed frame and every frame's memory, but
        tracking only looks back memory_frames frames plus the prompt frame.
        Frames are set to None rather than removed because the session numbers
        new frames by how many it holds.
        """
        session.processed_frames[frame_idx] = None
        for outputs in session.output_dict_per_obj.values():
            outputs["non_cond_frame_outputs"].pop(frame_idx - self.memory_frames, None)

    def _add_prompt(self, session: Any, prompt: Prompt, size: tuple[int, int]) -> None:
        h, w = size
        if isinstance(prompt, PointPrompt):
            inputs = {
                "input_points": [[[[prompt.x * w, prompt.y * h]]]],
                "input_labels": [[[1]]],
            }
        else:
            x, y, bw, bh = prompt.box
            inputs = {"input_boxes": [[[x * w, y * h, (x + bw) * w, (y + bh) * h]]]}
        self.processor.add_inputs_to_inference_session(
            inference_session=session, frame_idx=0, obj_ids=1, original_size=size, **inputs
        )
